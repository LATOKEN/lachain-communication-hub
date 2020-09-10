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

var log = loggo.GetLogger("peer")

type Peer struct {
	host           core.Host
	streams        map[*ecdsa.PublicKey]network.Stream
	mutex          *sync.Mutex
	grpcMsgHandler func([]byte)
	running        int32
	msgChannels    map[*ecdsa.PublicKey]chan []byte
	PublicKey      *ecdsa.PublicKey
}

func GRPCHandlerMock([]byte) {
	log.Tracef("Skipped received message in the mock...")
}

func New(id string) *Peer {
	localHost := host.BuildNamedHost(types.Peer, id)

	fmt.Println("my id:", localHost.ID())
	fmt.Println("listening on:", localHost.Addrs())

	// if we are not bootstrap
	if config.GetBootstrapID() != localHost.ID() {
		bootstrapInfo := peer.AddrInfo{
			ID:    config.GetBootstrapID(),
			Addrs: []ma.Multiaddr{config.GetBootstrapMultiaddr()},
		}

		// Connect to bootstrap
		if err := localHost.Connect(context.Background(), bootstrapInfo); err != nil {
			panic(err)
		}
	}

	mut := &sync.Mutex{}
	localPeer := new(Peer)
	localPeer.streams = make(map[*ecdsa.PublicKey]network.Stream)
	localPeer.msgChannels = make(map[*ecdsa.PublicKey]chan []byte)
	localPeer.host = localHost
	localPeer.mutex = mut
	localPeer.running = 1
	localPeer.SetStreamHandlerFn(GRPCHandlerMock)
	localPeer.host.SetStreamHandler("/hub", incomingConnectionEstablishmentHandler(localPeer))
	localPeer.host.SetStreamHandler("/getPeers", handleGetPeers)
	localPeer.host.SetStreamHandler("/register", handleRegister)

	return localPeer
}

func (localPeer *Peer) Stop() {
	atomic.StoreInt32(&localPeer.running, 0)
	for i := range localPeer.streams {
		localPeer.streams[i].Close()
	}
	localPeer.streams = make(map[*ecdsa.PublicKey]network.Stream)
	if err := localPeer.host.Close(); err != nil {
		panic(err)
	}
}

func (localPeer *Peer) Register(signature []byte) {

	peerId, _ := localPeer.host.ID().Marshal()
	localPublicKey, err := utils.EcRecover(peerId, signature)
	if err != nil {
		log.Errorf("%s", err)
		return
	}
	fmt.Println("localPubKey", utils.PublicKeyToHexString(localPublicKey))
	localPeer.SetPublicKey(localPublicKey)

	bootstrap := &types.PeerConnection{
		Id:   config.GetBootstrapID(),
		Addr: config.GetBootstrapMultiaddr(),
	}

	if err := localPeer.registerOnPeer(bootstrap, signature); err != nil {
		log.Errorf("Can't register on bootstrap")
	}

	if config.GetBootstrapID() == localPeer.host.ID() {
		log.Debugf("We won't ask self, skipping registration")
		return
	}

	bootstrapStream, err := localPeer.host.NewStream(context.Background(), config.GetBootstrapID(), "/getPeers")
	if err != nil {
		panic(err)
	}

	peersBytes, err := communication.ReadOnce(bootstrapStream)
	if err != nil {
		bootstrapStream.Close()
		panic(err)
	}
	bootstrapStream.Close()

	if string(peersBytes) == "0" {
		log.Debugf("No peers received..")
		return
	}

	for _, curr := range types.DecodeArray(peersBytes) {
		if localPeer.host.ID() == curr.Id {
			continue
		}
		storage.RegisterOrUpdatePeer(curr)
		if err := localPeer.registerOnPeer(curr, signature); err != nil {
			continue
		}
		storage.UpdateRegisteredPeerById(curr.Id)
	}
}

func (localPeer *Peer) connectToPeer(publicKey *ecdsa.PublicKey) (network.Stream, error) {
	if localPeer.running == 0 {
		return nil, errors.New("not running")
	}

	if s, ok := localPeer.GetStream(publicKey); ok {
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
		for _, relayPeer := range storage.GetAllPeers() {
			if storage.IsDirectlyConnected(relayPeer.Id) || relayPeer.Addr == nil {
				continue
			}
			if err := localPeer.establishDirectConnection(relayPeer); err != nil {
				log.Debugf("Can't establish connection with %s", relayPeer.Id)
				continue
			}
			if err := localPeer.connectToPeerUsingRelay(relayPeer.Id, targetPeer.Id); err != nil {
				log.Debugf("Can't connect to %s through %s: %s", targetPeer.Id, relayPeer.Id, err)
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

	localPeer.RegisterStream(publicKey, hubStream)
	return hubStream, nil
}

func (localPeer *Peer) GetPeerPublicKeyById(peerId peer.ID) (string, error) {
	relayStream, err := localPeer.host.NewStream(context.Background(), config.GetBootstrapID(), "/getPeerPublicKeyById")
	if err != nil {
		return "", err
	}

	peerIdBinary, err := peerId.MarshalBinary()
	if err != nil {
		return "", err
	}

	err = communication.Write(relayStream, peerIdBinary)
	if err != nil {
		relayStream.Close()
		return "", err
	}

	peerPubKeyBytes, err := communication.ReadOnce(relayStream)
	if err != nil {
		relayStream.Close()
		return "", err
	}

	relayStream.Close()

	return string(peerPubKeyBytes), nil
}

func (localPeer *Peer) RegisterStream(publicKey *ecdsa.PublicKey, stream network.Stream) {
	localPeer.mutex.Lock()
	defer localPeer.mutex.Unlock()
	localPeer.streams[publicKey] = stream
}

func (localPeer *Peer) SendMessageToPeer(publicKey *ecdsa.PublicKey, msg []byte) {
	localPeer.mutex.Lock()
	defer localPeer.mutex.Unlock()
	if msgChannel, ok := localPeer.msgChannels[publicKey]; ok {
		msgChannel <- msg
	} else {
		msgChannel = localPeer.SendingChannel(publicKey)
		localPeer.msgChannels[publicKey] = msgChannel
		msgChannel <- msg
	}
}

func (localPeer *Peer) SendingChannel(publicKey *ecdsa.PublicKey) chan []byte {
	messages := make(chan []byte)

	go func() {
		for {
			msg := <-messages

			if localPeer.running == 0 {
				continue
			}
			s, err := localPeer.connectToPeer(publicKey)
			if err != nil {
				log.Errorf("Can't establish connection with: %s", utils.PublicKeyToHexString(publicKey))
				log.Errorf("%s", err)
				continue
			}

			err = communication.Write(s, msg)
			if err != nil {
				log.Errorf("Cant connect to peer %s. Removing from connected", publicKey)
				localPeer.removeFromConnected(publicKey)
				continue
			}
			storage.UpdateRegisteredPeerByPublicKey(publicKey)
		}
	}()
	return messages
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
	if s, ok := localPeer.streams[publicKey]; ok {
		localPeer.host.Network().ClosePeer(s.Conn().RemotePeer())
		s.Close()
		delete(localPeer.streams, publicKey)
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
	s, ok := localPeer.streams[pubKey]
	return s, ok
}

func (localPeer *Peer) IsStreamWithPeerRegistered(peerId peer.ID) bool {
	localPeer.mutex.Lock()
	defer localPeer.mutex.Unlock()
	for _, s := range localPeer.streams {
		if s.Conn().RemotePeer() == peerId {
			return true
		}
	}
	return false
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
	for _, connectedPeerId := range storage.GetDirectlyConnectedPeerIds() {
		if err := localPeer.connectToPeerUsingRelay(connectedPeerId, targetPeerId); err != nil {
			log.Debugf("Can't connect to %s through %s: %s", targetPeerId, connectedPeerId, err)
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
		log.Debugf("can't connect to peer: %s", peerInfo)
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

	resp, err := communication.ReadOnce(s)
	if err != nil {
		log.Errorf("cannot register: %s", err)
		return err
	}

	if string(resp) != "1" {
		log.Warningf("Cannot register on %s. Response: %s", conn.Id.Pretty(), string(resp))
		return err
	}

	s.Close()
	return nil
}
