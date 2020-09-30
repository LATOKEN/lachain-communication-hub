package peer

import (
	"context"
	"errors"
	"fmt"
	"github.com/juju/loggo"
	core "github.com/libp2p/go-libp2p-core"
	"github.com/libp2p/go-libp2p-core/network"
	"github.com/libp2p/go-libp2p-core/peer"
	swarm "github.com/libp2p/go-libp2p-swarm"
	ma "github.com/multiformats/go-multiaddr"
	"lachain-communication-hub/communication"
	"lachain-communication-hub/config"
	"lachain-communication-hub/host"
	"lachain-communication-hub/storage"
	"lachain-communication-hub/types"
	"lachain-communication-hub/utils"
	"strings"
	"sync"
	"sync/atomic"
	"time"
)

const MaxTryToConnect = 3

var log = loggo.GetLogger("peer")

type Peer struct {
	host           core.Host
	hubStreams     map[string]network.Stream
	mutex          *sync.Mutex
	grpcMsgHandler func([]byte)
	running        int32
	msgChannels    map[string]chan []byte
	PublicKey      string
	Signature      []byte
}

var globalQuit = make(chan struct{})

func GRPCHandlerMock([]byte) {
	log.Tracef("Skipped received message in the mock...")
}

func New(id string) *Peer {
	localHost := host.BuildNamedHost(types.Peer, id)
	log.Infof("my id: %s", localHost.ID())
	log.Infof("listening on: %s", localHost.Addrs())

	mAddrs := config.GetBootstrapMultiaddrs()
	for i, bootstrapId := range config.GetBootstrapIDs() {
		// skip if we are bootstrap
		if bootstrapId != localHost.ID() {
			bootstrapInfo := peer.AddrInfo{
				ID:    bootstrapId,
				Addrs: []ma.Multiaddr{mAddrs[i]},
			}

			// Connect to bootstrap
			if err := localHost.Connect(context.Background(), bootstrapInfo); err != nil {
				log.Errorf("%s", err)
			}
		}
	}

	mut := &sync.Mutex{}
	localPeer := new(Peer)
	localPeer.hubStreams = make(map[string]network.Stream)
	localPeer.msgChannels = make(map[string]chan []byte)
	localPeer.host = localHost
	localPeer.mutex = mut
	localPeer.running = 1
	localPeer.SetStreamHandlerFn(GRPCHandlerMock)

	localPeer.host.SetStreamHandler("/register", registerHandlerForLocalPeer(localPeer))
	localPeer.host.SetStreamHandler("/hub", incomingConnectionEstablishmentHandler(localPeer))
	localPeer.host.SetStreamHandler("/getPeers", getPeerHandlerForLocalPeer(localPeer))

	localPeer.startOldMsgCleaner()

	return localPeer
}

func KeyGen(count int) {
	host.GenerateKey(count)
}

func (localPeer *Peer) startOldMsgCleaner() {
	ticker := time.NewTicker(30 * time.Minute)
	go func() {
		for {
			select {
			case <-ticker.C:
				storage.RemoveOldPostponedMessages()
			case <-globalQuit:
				ticker.Stop()
				return
			}
		}
	}()
}

func (localPeer *Peer) Register(signature []byte) bool {
	peerId, _ := localPeer.host.ID().Marshal()
	localPublicKey, err := utils.EcRecover(peerId, signature)
	if err != nil {
		log.Errorf("%s", err)
		return false
	}
	log.Debugf("localPubKey %s", utils.PublicKeyToHexString(localPublicKey))
	localPeer.SetPublicKey(utils.PublicKeyToHexString(localPublicKey))
	localPeer.SetSignature(signature)
	bootstrapMAddrs := config.GetBootstrapMultiaddrs()
	connected := 0
	for i, bootstrapId := range config.GetBootstrapIDs() {
		log.Debugf("Bootstrap %v/%v", i+1, len(bootstrapMAddrs))
		bootstrap := &types.PeerConnection{
			Id:   bootstrapId,
			Addr: bootstrapMAddrs[i],
		}
		if bootstrapId == localPeer.host.ID() {
			log.Debugf("skipping registration on ourselves")
			continue
		}

		if !storage.IsPeerIdConnected(bootstrapId.Pretty()) {
			if err := localPeer.registerOnPeer(bootstrap, signature); err != nil {
				log.Errorf("Can't register on bootstrap: %s", err)
				continue
			}

			storage.SetPeerIdConnected(bootstrap.Id.Pretty(), true)
			connected++
		}

		log.Debugf("registered on bootstrap")
		bootstrapStream, err := localPeer.host.NewStream(context.Background(), bootstrapId, "/getPeers")
		if err != nil {
			log.Errorf("%s", err)
			continue
		}
		peersBytes, err := communication.ReadOnce(bootstrapStream)
		bootstrapStream.Close()
		if err != nil {
			log.Errorf("%s", err)
			continue
		}
		if string(peersBytes) == "0" {
			log.Debugf("No peers received..")
			continue
		}
		peerConnections := types.DecodeArray(peersBytes)
		log.Debugf("Received %v peers", len(peerConnections))
		for i, curr := range peerConnections {
			log.Debugf("Registering on %v/%v", i+1, len(peerConnections))
			// skip bootstrap and self connection
			if curr.Id == localPeer.host.ID() || storage.IsPeerIdConnected(curr.Id.Pretty()) || curr.PublicKey == "" {
				continue
			}
			// just store connection
			storage.RegisterOrUpdatePeer(curr)
			// register on external peer even if this one is behind NAT
			if err := localPeer.registerOnPeer(curr, signature); err != nil {
				continue
			}

			storage.SetPeerIdConnected(curr.Id.Pretty(), true)
			// refresh connection's timestamp
			storage.UpdateRegisteredPeerById(curr.Id)
			connected++
		}
	}
	log.Debugf("Registered %v peers", connected)

	return true
}

func (localPeer *Peer) connectToPeer(publicKey string) (network.Stream, error) {
	localPeer.lock()
	defer localPeer.unlock()

	if localPeer.running == 0 {
		return nil, errors.New("not running")
	}

	if s, ok := localPeer.hubStreams[publicKey]; ok {
		return s, nil
	}

	log.Debugf("connecting to %s", publicKey)

	targetPeer, err := storage.GetPeerByPublicKey(publicKey)
	if err != nil {
		log.Debugf("Peer not found %s", publicKey)
		return nil, err
	}

	// Since we just tried and failed to dial, the dialer system will, by default
	// prevent us from redialing again so quickly. Since we know what we're doing, we
	// can use this ugly hack (it's on our TODO list to make it a little cleaner)
	// to tell the dialer "no, its okay, let's try this again"
	localPeer.host.Network().(*swarm.Swarm).Backoff().Clear(targetPeer.Id)

	connected := false

	if targetPeer.Addr != nil {
		if err := localPeer.establishDirectConnection(targetPeer); err != nil {
			return nil, err
		}
		connected = true
	}

	if !connected {
		err = localPeer.establishRelayedConnection(targetPeer.Id)
		connected = err == nil
	}

	if !connected {
		errCount := 0
		for _, relayPeer := range storage.GetAllPeers() {
			if storage.IsDirectlyConnected(relayPeer.Id) || relayPeer.Addr == nil {
				continue
			}
			if err := localPeer.establishDirectConnection(relayPeer); err != nil {
				log.Debugf("Can't establish connection with %s", relayPeer.Id)
				continue
			}
			if err := localPeer.connectToPeerUsingRelay(relayPeer.Id, targetPeer.Id); err != nil {
				//log.Debugf("Can't connect to %s through %s: %s", targetPeer.Id, relayPeer.Id, err)
				errCount++
				if errCount == MaxTryToConnect {
					break
				}
				continue
			}
			connected = true
		}
	}

	if !connected {
		return nil, errors.New(fmt.Sprintf("unable to connect to peer: %s", targetPeer.Id))
	}

	// Woohoo! we're connected!
	hubStream, err := localPeer.host.NewStream(context.Background(), targetPeer.Id, "/hub")
	if err != nil {
		log.Errorf("huh, this should have worked: %s", err)
		return nil, err
	}

	storage.SetPublicKeyConnected(publicKey, true)

	go func() {
		for localPeer.running != 0 {
			resp, err := localPeer.ReceiveResponseFromPeer(publicKey)
			if err != nil {
				localPeer.removeFromConnected(publicKey)
				return
			}
			err = processMessage(localPeer, hubStream, resp)
			if err != nil {
				localPeer.removeFromConnected(publicKey)
				return
			}
			storage.UpdateRegisteredPeerByPublicKey(publicKey)
		}
	}()

	localPeer.hubStreams[publicKey] = hubStream
	return hubStream, nil
}

func (localPeer *Peer) RegisterStream(publicKey string, stream network.Stream) {
	localPeer.lock()
	defer localPeer.unlock()
	localPeer.hubStreams[publicKey] = stream
}

func (localPeer *Peer) SendMessageToPeer(publicKey string, msg []byte, ensureSent bool) (result bool) {
	log.Tracef("Sending message to peer %s message length %d", publicKey, len(msg))

	localPeer.lock()
	msgChannel, ok := localPeer.msgChannels[publicKey]
	localPeer.unlock()

	if ok {
		defer func() {
			// recover from panic caused by writing to a closed channel
			if r := recover(); r != nil {
				err := fmt.Errorf("%v", r)
				fmt.Printf("write: error writing on channel: %v\n", err)
				result = false
				if ensureSent {
					localPeer.storeMsgForFutureSend(publicKey, msg)
				}
			}
		}()
		msgChannel <- msg
		return true
	} else {
		if ensureSent {
			localPeer.storeMsgForFutureSend(publicKey, msg)
		}
		log.Tracef("No connection with peer: %s", publicKey)
		return false
	}
}

func (localPeer *Peer) storeMsgForFutureSend(publicKey string, msg []byte) {
	storage.StoreMessageToSendOnConnect(publicKey, msg)
}

func (localPeer *Peer) BroadcastMessage(msg []byte) {
	activePeers := storage.GetRecentPeers()
	if len(activePeers) == 0 {
		log.Tracef("There are no connected peers")
		return
	}
	for _, peer := range activePeers {
		log.Tracef("peer %s connected: %v", peer.PublicKey, storage.IsPublicKeyConnected(peer.PublicKey))
		if peer.PublicKey == "" || !storage.IsPublicKeyConnected(peer.PublicKey) {
			continue
		}
		localPeer.SendMessageToPeer(peer.PublicKey, msg, false)
	}
}

func (localPeer *Peer) NewMsgChannel(publicKey string) chan []byte {
	msgChannel := make(chan []byte)

	_, err := localPeer.connectToPeer(publicKey)
	if err != nil {
		log.Errorf("Can't establish connection with: %s", publicKey)
		log.Errorf("%s", err)
		localPeer.removeFromConnected(publicKey)
		return nil
	}

	go func() {
		for {
			select {
			case msg := <-msgChannel:
				if len(msg) == 0 {
					log.Debugf("Closing msg channel for %s", publicKey)
					localPeer.removeFromConnected(publicKey)
					return
				}
				localPeer.lock()
				if localPeer.running == 0 {
					localPeer.unlock()
					continue
				}
				localPeer.unlock()
				s, err := localPeer.connectToPeer(publicKey)
				if err != nil {
					log.Errorf("Can't establish connection with: %s", publicKey)
					log.Errorf("%s", err)
					localPeer.removeFromConnected(publicKey)
					continue
				}

				err = communication.Write(s, msg)
				if err != nil {
					log.Errorf("Can't connect to peer %s. Removing from connected", publicKey)
					localPeer.removeFromConnected(publicKey)
					continue
				}
				storage.UpdateRegisteredPeerByPublicKey(publicKey)
			case <-globalQuit:
				log.Debugf("will no longer receive msgs from %s", publicKey)
				return
			default:
				time.Sleep(100 * time.Millisecond)
			}
		}
	}()
	return msgChannel
}

func (localPeer *Peer) ReceiveResponseFromPeer(publicKey string) ([]byte, error) {
	localPeer.lock()
	if localPeer.running == 0 {
		localPeer.unlock()
		return nil, nil
	}
	localPeer.unlock()
	s, ok := localPeer.GetStream(publicKey)
	if !ok {
		log.Errorf("Connection not found with %s", publicKey)
		return nil, errors.New("not found")
	}

	log.Tracef("Received msg from peer (we are conn initiator) %s", publicKey)

	msg, err := communication.ReadOnce(s)
	if err != nil {
		if err.Error() == "stream reset" {
			log.Errorf("Connection closed by peer")
			localPeer.removeFromConnected(publicKey)
			return nil, errors.New("conn reset")
		}
		return nil, err
	}
	return msg, nil
}

func (localPeer *Peer) SetStreamHandlerFn(callback func(msg []byte)) {
	localPeer.grpcMsgHandler = callback
	log.Tracef("Messaged handling callback to (%p) is set for peer (%p)", callback, localPeer)
}

func (localPeer *Peer) SetPublicKey(publicKey string) {
	localPeer.PublicKey = publicKey
}

func (localPeer *Peer) SetSignature(signature []byte) {
	localPeer.Signature = signature
}

func (localPeer *Peer) GetId() []byte {
	id, err := localPeer.host.ID().Marshal()
	if err != nil {
		panic(err)
	}
	return id
}

func (localPeer *Peer) removeFromConnected(publicKey string) {
	localPeer.lock()
	defer localPeer.unlock()

	peer, err := storage.GetPeerByPublicKey(publicKey)
	if err != nil {
		log.Errorf("Not found peer with pub %s", publicKey)
		return
	}

	storage.RemoveConnection(peer.Id)
	storage.SetPublicKeyConnected(peer.PublicKey, false)
	storage.SetPeerIdConnected(peer.Id.Pretty(), false)
	if s, ok := localPeer.hubStreams[publicKey]; ok {
		localPeer.host.Network().ClosePeer(s.Conn().RemotePeer())
		s.Close()
		delete(localPeer.hubStreams, publicKey)
	}
	if ch, ok := localPeer.msgChannels[publicKey]; ok {
		close(ch)
		delete(localPeer.msgChannels, publicKey)
	}
}

func (localPeer *Peer) GetExternalMultiAddress() (ma.Multiaddr, error) {
	extIp := config.GetP2PExternalIP()
	if extIp == "" {
		return nil, errors.New("GetExternalMultiAddress: extIp not found")
	}
	addresses := localPeer.host.Network().Peerstore().Addrs(localPeer.host.ID())

	for _, addr := range addresses {
		if strings.Contains(addr.String(), extIp) {
			return addr, nil
		}
	}
	return nil, errors.New("GetExternalMultiAddress: addr not found")
}

func (localPeer *Peer) GetStream(pubKey string) (network.Stream, bool) {
	localPeer.lock()
	defer localPeer.unlock()
	s, ok := localPeer.hubStreams[pubKey]
	return s, ok
}

func (localPeer *Peer) IsConnected(publicKey string) bool {
	localPeer.lock()
	defer localPeer.unlock()
	_, exist := localPeer.hubStreams[publicKey]
	return exist
}

func (localPeer *Peer) IsMsgChannelExist(publicKey string) bool {
	localPeer.lock()
	defer localPeer.unlock()
	_, exist := localPeer.msgChannels[publicKey]
	return exist
}

func (localPeer *Peer) establishDirectConnection(targetPeer *types.PeerConnection) error {
	targetPeerInfo := peer.AddrInfo{
		ID:    targetPeer.Id,
		Addrs: []ma.Multiaddr{targetPeer.Addr},
	}

	err := localPeer.host.Connect(context.Background(), targetPeerInfo)
	if err != nil {
		return err
	}

	targetPeer.LastSeen = uint32(time.Now().Unix())
	storage.RegisterOrUpdatePeer(targetPeer)

	storage.AddConnection(targetPeer.Id)
	return nil
}

func (localPeer *Peer) connectToPeerUsingRelay(relayId peer.ID, targetPeerId peer.ID) error {
	relayedAddr, err := ma.NewMultiaddr("/p2p/" + relayId.Pretty() + "/p2p-circuit/p2p/" + targetPeerId.Pretty())
	if err != nil {
		return err
	}

	targetPeerInfo := peer.AddrInfo{
		ID:    targetPeerId,
		Addrs: []ma.Multiaddr{relayedAddr},
	}

	err = localPeer.host.Connect(context.Background(), targetPeerInfo)
	if err != nil {
		return err
	}

	return nil
}

func (localPeer *Peer) establishRelayedConnection(targetPeerId peer.ID) error {
	errCount := 0
	for _, connectedPeerId := range storage.GetDirectlyConnectedPeerIds() {
		if err := localPeer.connectToPeerUsingRelay(connectedPeerId, targetPeerId); err != nil {
			log.Debugf("Can't connect to %s through %s: %s", targetPeerId, connectedPeerId, err)
			errCount++
			if errCount == MaxTryToConnect {
				return errors.New("can't connect")
			}
			continue
		}
		return nil
	}
	return errors.New("can't connect")
}

func (localPeer *Peer) establishConnection(targetPeer *types.PeerConnection) error {
	if targetPeer.Addr == nil {
		return localPeer.establishRelayedConnection(targetPeer.Id)
	}
	return localPeer.establishDirectConnection(targetPeer)
}

func (localPeer *Peer) registerOnPeer(conn *types.PeerConnection, signature []byte) error {

	err := localPeer.establishConnection(conn)
	if err != nil {
		return err
	}

	peerInfo := peer.AddrInfo{
		ID:    conn.Id,
		Addrs: []ma.Multiaddr{conn.Addr},
	}

	if err := localPeer.host.Connect(context.Background(), peerInfo); err != nil {
		//log.Debugf("can't connect to peer: %s", peerInfo)
		return err
	}

	s, err := localPeer.host.NewStream(context.Background(), conn.Id, "/register")
	if err != nil {
		log.Debugf("can't register on peer: %s", peerInfo)
		return err
	}

	err = communication.Write(s, signature)
	if err != nil {
		log.Errorf("cannot register: %s", err)
		return err
	}

	extAddr, err := localPeer.GetExternalMultiAddress()
	if err != nil {
		err = communication.Write(s, []byte("0"))
		if err != nil {
			log.Errorf("cannot register: %s", err)
			return err
		}
	} else {
		extAddrBytes := extAddr.Bytes()
		err = communication.Write(s, extAddrBytes)
		if err != nil {
			log.Errorf("cannot register: %s", err)
			return err
		}
	}

	remoteSignature, err := communication.ReadOnce(s)
	if err != nil {
		if err.Error() == "stream reset" {
			log.Errorf("Connection closed by peer")
			s.Close()
			return err
		}
		log.Errorf("%s", err)
		s.Close()
		return err
	}

	remotePeerId, _ := s.Conn().RemotePeer().Marshal()
	publicKey, err := utils.EcRecover(remotePeerId, remoteSignature)
	if err != nil {
		log.Errorf("%s", err)
		s.Close()
		return err
	}

	conn.PublicKey = utils.PublicKeyToHexString(publicKey)

	s.Close()

	log.Debugf("creating new msg channel for %s", conn.PublicKey)
	msgChannel := localPeer.NewMsgChannel(conn.PublicKey)
	if msgChannel == nil {
		log.Warningf("Cant open msg channel with %s", conn.PublicKey)
		localPeer.removeFromConnected(conn.PublicKey)
		return errors.New("no conn")
	}

	localPeer.msgChannels[conn.PublicKey] = msgChannel

	localPeer.SendPostponedMessages(conn.PublicKey)

	log.Debugf("Registered on peer %s", conn.PublicKey)
	return nil
}

func (localPeer *Peer) SendPostponedMessages(publicKey string) {
	messages := storage.GetPostponedMessages(publicKey)
	if len(messages) == 0 {
		return
	}
	for _, msg := range messages {
		if !localPeer.SendMessageToPeer(publicKey, msg, false) {
			return
		}
	}
	storage.ClearPostponedMessages(publicKey)
	log.Debugf("Sent %v postponed messages to %s", len(messages), publicKey)
}

func (localPeer *Peer) lock() {
	localPeer.mutex.Lock()
}

func (localPeer *Peer) unlock() {
	localPeer.mutex.Unlock()
}

func (localPeer *Peer) Stop() {
	localPeer.lock()
	close(globalQuit)
	defer localPeer.unlock()
	atomic.StoreInt32(&localPeer.running, 0)
	streamsLen := len(localPeer.hubStreams)
	i := 0
	for _, stream := range localPeer.hubStreams {
		stream.Close()
		i++
		log.Debugf("Stream closed %v/%v", i, streamsLen)
	}
	localPeer.hubStreams = nil
	if err := localPeer.host.ConnManager().Close(); err != nil {
		panic(err)
	}
	log.Debugf("Closed ConnManager")
	if err := localPeer.host.Network().Close(); err != nil {
		panic(err)
	}
	log.Debugf("Closed Network")
	if err := localPeer.host.Peerstore().Close(); err != nil {
		panic(err)
	}
	log.Debugf("Closed Peerstore")
	localPeer.host.RemoveStreamHandler("/getPeers")
	localPeer.host.RemoveStreamHandler("/hub")
	localPeer.host.RemoveStreamHandler("/register")
	log.Debugf("Removed Handlers")
	if err := localPeer.host.Close(); err != nil {
		panic(err)
	}
	log.Debugf("closed host")
}
