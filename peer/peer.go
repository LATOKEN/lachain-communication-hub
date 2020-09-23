package peer

import (
	"context"
	"crypto/ecdsa"
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
const TagConnection = "connection"
const TagHub = "hub"

var log = loggo.GetLogger("peer")

type Peer struct {
	host           core.Host
	streams        map[string]network.Stream
	mutex          *sync.Mutex
	grpcMsgHandler func([]byte)
	running        int32
	msgChannels    map[string]chan []byte
	PublicKey      *ecdsa.PublicKey
	Signature      []byte
}

var globalQuit = make(chan struct{})

func GRPCHandlerMock([]byte) {
	log.Tracef("Skipped received message in the mock...")
}

func New(id string) *Peer {
	localHost := host.BuildNamedHost(types.Peer, id)

	fmt.Println("my id:", localHost.ID())
	fmt.Println("listening on:", localHost.Addrs())

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
				fmt.Errorf("%s", err)
			}
		}
	}

	mut := &sync.Mutex{}
	localPeer := new(Peer)
	localPeer.streams = make(map[string]network.Stream)
	localPeer.msgChannels = make(map[string]chan []byte)
	localPeer.host = localHost
	localPeer.mutex = mut
	localPeer.running = 1
	localPeer.SetStreamHandlerFn(GRPCHandlerMock)

	return localPeer
}

func KeyGen(count int) {
	host.GenerateKey(count)
}

func (localPeer *Peer) Register(signature []byte) bool {
	peerId, _ := localPeer.host.ID().Marshal()
	localPublicKey, err := utils.EcRecover(peerId, signature)
	if err != nil {
		log.Errorf("%s", err)
		return false
	}
	log.Debugf("localPubKey %s", utils.PublicKeyToHexString(localPublicKey))
	localPeer.SetPublicKey(localPublicKey)
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
			log.Debugf("We won't ask self, skipping registration")
			continue
		}
		if err := localPeer.registerOnPeer(bootstrap, signature); err != nil {
			log.Errorf("Can't register on bootstrap")
			continue
		}
		connected++
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
			log.Debugf("Tag info %v", localPeer.host.ConnManager().GetTagInfo(curr.Id).Tags[TagConnection])
			// skip bootstrap and self connection
			if curr.Id == localPeer.host.ID() || curr.Id == bootstrapId || localPeer.host.ConnManager().GetTagInfo(curr.Id).Tags[TagConnection] != 0 || curr.PublicKey == nil {
				continue
			}
			// just store connection
			storage.RegisterOrUpdatePeer(curr)
			// register on external peer even if this one is behind NAT
			if err := localPeer.registerOnPeer(curr, signature); err != nil {
				continue
			}

			localPeer.host.ConnManager().TagPeer(curr.Id, TagConnection, 1)
			// refresh connection's timestamp
			storage.UpdateRegisteredPeerById(curr.Id)
			connected++
		}
	}
	log.Debugf("Registered %v peers", connected)

	localPeer.host.SetStreamHandler("/register", registerHandlerForLocalPeer(localPeer))
	localPeer.host.SetStreamHandler("/hub", incomingConnectionEstablishmentHandler(localPeer))
	localPeer.host.SetStreamHandler("/getPeers", getPeerHandlerForLocalPeer(localPeer))

	return true
}

func (localPeer *Peer) connectToPeer(publicKey *ecdsa.PublicKey) (network.Stream, error) {
	localPeer.mutex.Lock()
	defer localPeer.mutex.Unlock()

	if localPeer.running == 0 {
		return nil, errors.New("not running")
	}

	if s, ok := localPeer.streams[utils.PublicKeyToHexString(publicKey)]; ok {
		return s, nil
	}

	targetPeer, err := storage.GetPeerByPublicKey(publicKey)
	if err != nil {
		log.Debugf("Peer not found %s", utils.PublicKeyToHexString(publicKey))
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

	localPeer.host.ConnManager().TagPeer(targetPeer.Id, TagHub, 1)

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

	localPeer.streams[utils.PublicKeyToHexString(publicKey)] = hubStream
	return hubStream, nil
}

func (localPeer *Peer) RegisterStream(publicKey *ecdsa.PublicKey, stream network.Stream) {
	localPeer.mutex.Lock()
	defer localPeer.mutex.Unlock()
	localPeer.streams[utils.PublicKeyToHexString(publicKey)] = stream
}

func (localPeer *Peer) SendMessageToPeer(publicKey *ecdsa.PublicKey, msg []byte) {
	log.Tracef("Sending message to peer %s message length %d", utils.PublicKeyToHexString(publicKey), len(msg))

	localPeer.mutex.Lock()
	msgChannel, ok := localPeer.msgChannels[utils.PublicKeyToHexString(publicKey)]

	if ok {
		msgChannel <- msg
		localPeer.mutex.Unlock()
	} else {
		localPeer.mutex.Unlock()
		log.Tracef("No connection with peer: %s", utils.PublicKeyToHexString(publicKey))
	}
}

func (localPeer *Peer) BroadcastMessage(msg []byte) {
	activePeers := storage.GetRecentPeers()
	for _, peer := range activePeers {
		if peer.PublicKey == nil || localPeer.host.ConnManager().GetTagInfo(peer.Id).Tags[TagConnection] != 0 {
			continue
		}
		localPeer.SendMessageToPeer(peer.PublicKey, msg)
	}
}

func (localPeer *Peer) NewMsgChannel(publicKey *ecdsa.PublicKey) chan []byte {
	msgChannel := make(chan []byte)

	fmt.Println("new msg channel for", utils.PublicKeyToHexString(publicKey))

	go func() {
		for {
			select {
			case msg := <-msgChannel:
				if len(msg) == 0 {
					log.Debugf("Closing msg channel for %s", utils.PublicKeyToHexString(publicKey))
					return
				}
				if localPeer.running == 0 {
					continue
				}
				s, err := localPeer.connectToPeer(publicKey)
				if err != nil {
					log.Errorf("Can't establish connection with: %s", utils.PublicKeyToHexString(publicKey))
					log.Errorf("%s", err)
					localPeer.removeFromConnected(publicKey)
					continue
				}

				err = communication.Write(s, msg)
				if err != nil {
					log.Errorf("Can't connect to peer %s. Removing from connected", utils.PublicKeyToHexString(publicKey))
					localPeer.removeFromConnected(publicKey)
					continue
				}
				storage.UpdateRegisteredPeerByPublicKey(publicKey)
			case <-globalQuit:
				fmt.Println("quiting", utils.PublicKeyToHexString(publicKey))
			default:
			}
		}
	}()
	return msgChannel
}

func (localPeer *Peer) ReceiveResponseFromPeer(publicKey *ecdsa.PublicKey) ([]byte, error) {
	if localPeer.running == 0 {
		return nil, nil
	}
	s, ok := localPeer.GetStream(publicKey)
	if !ok {
		log.Errorf("Connection not found with %s", publicKey)
		return nil, errors.New("not found")
	}

	log.Tracef("Received msg from peer (we are conn initiator) %s", s.Conn().RemotePeer().Pretty())

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

func (localPeer *Peer) SetPublicKey(publicKey *ecdsa.PublicKey) {
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

func (localPeer *Peer) removeFromConnected(publicKey *ecdsa.PublicKey) {
	localPeer.mutex.Lock()
	defer localPeer.mutex.Unlock()

	peer, err := storage.GetPeerByPublicKey(publicKey)
	if err != nil {
		log.Errorf("Not found peer with pub %s", publicKey)
		return
	}

	storage.RemoveConnection(peer.Id)
	localPeer.host.ConnManager().UntagPeer(peer.Id, TagConnection)
	localPeer.host.ConnManager().UntagPeer(peer.Id, TagHub)
	if s, ok := localPeer.streams[utils.PublicKeyToHexString(publicKey)]; ok {
		localPeer.host.Network().ClosePeer(s.Conn().RemotePeer())
		s.Close()
		delete(localPeer.streams, utils.PublicKeyToHexString(publicKey))
	}
	if ch, ok := localPeer.msgChannels[utils.PublicKeyToHexString(publicKey)]; ok {
		close(ch)
		delete(localPeer.msgChannels, utils.PublicKeyToHexString(publicKey))
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

func (localPeer *Peer) GetStream(pubKey *ecdsa.PublicKey) (network.Stream, bool) {
	localPeer.mutex.Lock()
	defer localPeer.mutex.Unlock()
	s, ok := localPeer.streams[utils.PublicKeyToHexString(pubKey)]
	return s, ok
}

func (localPeer *Peer) IsConnected(publicKey *ecdsa.PublicKey) bool {
	localPeer.mutex.Lock()
	defer localPeer.mutex.Unlock()
	_, exist := localPeer.streams[utils.PublicKeyToHexString(publicKey)]
	return exist
}

func (localPeer *Peer) IsMsgChannelExist(publicKey *ecdsa.PublicKey) bool {
	localPeer.mutex.Lock()
	defer localPeer.mutex.Unlock()
	_, exist := localPeer.msgChannels[utils.PublicKeyToHexString(publicKey)]
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

	conn.PublicKey = publicKey

	s.Close()

	msgChannel := localPeer.NewMsgChannel(publicKey)
	localPeer.msgChannels[utils.PublicKeyToHexString(publicKey)] = msgChannel

	fmt.Println("Registered on peer")
	return nil
}

func (localPeer *Peer) Stop() {
	localPeer.mutex.Lock()
	defer localPeer.mutex.Unlock()
	atomic.StoreInt32(&localPeer.running, 0)
	for _, stream := range localPeer.streams {
		stream.Close()
	}
	localPeer.streams = make(map[string]network.Stream)
	if err := localPeer.host.ConnManager().Close(); err != nil {
		panic(err)
	}
	if err := localPeer.host.Network().Close(); err != nil {
		panic(err)
	}
	if err := localPeer.host.Peerstore().Close(); err != nil {
		panic(err)
	}
	localPeer.host.RemoveStreamHandler("/getPeers")
	localPeer.host.RemoveStreamHandler("/hub")
	localPeer.host.RemoveStreamHandler("/register")
	if err := localPeer.host.Close(); err != nil {
		panic(err)
	}
}
