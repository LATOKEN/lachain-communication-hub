package peer

import (
	"context"
	"encoding/hex"
	"errors"
	"github.com/juju/loggo"
	core "github.com/libp2p/go-libp2p-core"
	"github.com/libp2p/go-libp2p-core/network"
	"github.com/libp2p/go-libp2p-core/peer"
	swarm "github.com/libp2p/go-libp2p-swarm"
	ma "github.com/multiformats/go-multiaddr"
	"lachain-communication-hub/communication"
	"lachain-communication-hub/config"
	"lachain-communication-hub/host"
	"lachain-communication-hub/types"
	"strings"
	"sync"
	"sync/atomic"
)

var log = loggo.GetLogger("peer")

type Peer struct {
	host           core.Host
	streams        map[string]network.Stream
	streamsLock    *sync.Mutex
	grpcMsgHandler func([]byte)
	running        int32
}

func GRPCHandlerMock([]byte) {}

func New(id string) Peer {
	localHost := host.BuildNamedHost(types.Peer, id)

	relayInfo := peer.AddrInfo{
		ID:    config.GetRelayID(),
		Addrs: []ma.Multiaddr{config.GetRelayMultiaddr()},
	}

	// Connect to relay
	if err := localHost.Connect(context.Background(), relayInfo); err != nil {
		panic(err)
	}
	mut := &sync.Mutex{}
	localPeer := Peer{localHost, make(map[string]network.Stream), mut, nil, 1}
	localPeer.SetStreamHandlerFn(GRPCHandlerMock)
	localPeer.host.SetStreamHandler("/hub", incomingConnectionEstablishmentHandler(&localPeer))

	return localPeer
}

func (localPeer *Peer) Stop() {
	atomic.StoreInt32(&localPeer.running, 0)
	for s := range localPeer.streams {
		localPeer.streams[s].Close()
	}
	localPeer.streams = make(map[string]network.Stream)
	if err := localPeer.host.Close(); err != nil {
		panic(err)
	}
}

func (localPeer *Peer) register(signature []byte) {

	s, err := localPeer.host.NewStream(context.Background(), config.GetRelayID(), "/register")
	if err != nil {
		panic(err)
	}

	log.Debugf("peer registration")
	log.Debugf("signature %s", hex.EncodeToString(signature))
	log.Debugf("peerId %s", localPeer.host.ID())

	err = communication.Write(s, signature)
	if err != nil {
		log.Errorf("cannot register: %s", err)
	}

	extAddr, err := localPeer.GetExternalMultiAddress()
	if err != nil {
		err = communication.Write(s, []byte("0"))
		if err != nil {
			log.Errorf("cannot register: %s", err)
		}
	} else {
		extAddrBytes := extAddr.Bytes()
		err = communication.Write(s, extAddrBytes)
		if err != nil {
			log.Errorf("cannot register: %s", err)
		}
	}

	resp, err := communication.ReadOnce(s)
	if err != nil {
		log.Errorf("cannot register: %s", err)
	}

	if string(resp) != "1" {
		log.Errorf("cannot register: %s", string(resp))
	}

	s.Close()
}

func (localPeer *Peer) Register(signature []byte) {
	localPeer.register(signature)
}

func (localPeer *Peer) connectToPeer(publicKey string) (network.Stream, error) {
	if localPeer.running == 0 {
		return nil, errors.New("not running")
	}

	if s, ok := localPeer.GetStream(publicKey); ok {
		return s, nil
	}

	relayStream, err := localPeer.host.NewStream(context.Background(), config.GetRelayID(), "/getPeerAddr")
	if err != nil {
		return nil, err
	}

	err = communication.Write(relayStream, []byte(publicKey))
	if err != nil {
		relayStream.Close()
		return nil, err
	}

	peerIdBytes, err := communication.ReadOnce(relayStream)
	if err != nil {
		relayStream.Close()
		return nil, err
	}

	peerAddrBytes, err := communication.ReadOnce(relayStream)
	if err != nil {
		relayStream.Close()
		return nil, err
	}

	relayStream.Close()

	peerId, err := peer.Decode(string(peerIdBytes))
	if err != nil {
		log.Errorf("Peer not found")
		return nil, err
	}

	peerAddr, _ := ma.NewMultiaddrBytes(peerAddrBytes)

	if peerAddr == nil {
		// Creates a relay address
		peerAddr, err = ma.NewMultiaddr("/p2p/" + config.GetRelayID().Pretty() + "/p2p-circuit/p2p/" + peerId.Pretty())
		if err != nil {
			return nil, err
		}
	}

	// Since we just tried and failed to dial, the dialer system will, by default
	// prevent us from redialing again so quickly. Since we know what we're doing, we
	// can use this ugly hack (it's on our TODO list to make it a little cleaner)
	// to tell the dialer "no, its okay, let's try this again"
	localPeer.host.Network().(*swarm.Swarm).Backoff().Clear(peerId)

	relayedPeerInfo := peer.AddrInfo{
		ID:    peerId,
		Addrs: []ma.Multiaddr{peerAddr},
	}
	if err := localPeer.host.Connect(context.Background(), relayedPeerInfo); err != nil {
		return nil, err
	}

	// Woohoo! we're connected!
	hubStream, err := localPeer.host.NewStream(context.Background(), peerId, "/hub")
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
		}
	}()

	localPeer.RegisterConnection(publicKey, hubStream)
	return hubStream, nil
}

func (localPeer *Peer) GetPeerPublicKeyById(peerId peer.ID) (string, error) {
	relayStream, err := localPeer.host.NewStream(context.Background(), config.GetRelayID(), "/getPeerPublicKeyById")
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

func (localPeer *Peer) RegisterConnection(publicKey string, stream network.Stream) {
	localPeer.streamsLock.Lock()
	defer localPeer.streamsLock.Unlock()
	localPeer.streams[publicKey] = stream
}

func (localPeer *Peer) SendMessageToPeer(publicKey string, msg []byte) {
	if localPeer.running == 0 {
		return
	}
	s, err := localPeer.connectToPeer(publicKey)
	if err != nil {
		log.Errorf("Can't establish connection with: %s", publicKey)
		log.Errorf("%s", err)
		return
	}

	err = communication.Write(s, msg)
	if err != nil {
		log.Errorf("Cant connect to peer %s. Removing from connected", publicKey)
		localPeer.removeFromConnected(publicKey)
		return
	}
}

func (localPeer *Peer) ReceiveResponseFromPeer(publicKey string) ([]byte, error) {
	if localPeer.running == 0 {
		return nil, nil
	}
	s, ok := localPeer.streams[publicKey]
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
}

func (localPeer *Peer) GetId() []byte {
	id, err := localPeer.host.ID().Marshal()
	if err != nil {
		panic(err)
	}
	return id
}

func (localPeer *Peer) removeFromConnected(publicKey string) {
	localPeer.streamsLock.Lock()
	defer localPeer.streamsLock.Unlock()
	if s, ok := localPeer.streams[publicKey]; ok {
		localPeer.host.Network().ClosePeer(s.Conn().RemotePeer())
		s.Close()
		delete(localPeer.streams, publicKey)
	}
}

func (localPeer *Peer) GetExternalMultiAddress() (ma.Multiaddr, error) {
	extIp := config.GetP2PExternalIP()
	if extIp == "" {
		return nil, errors.New("not found")
	}
	addresses := localPeer.host.Network().Peerstore().Addrs(localPeer.host.ID())

	for _, addr := range addresses {
		if strings.Contains(addr.String(), extIp) {
			return addr, nil
		}
	}
	return nil, errors.New("not found")
}

func (localPeer *Peer) GetStream(pubKey string) (network.Stream, bool) {
	localPeer.streamsLock.Lock()
	defer localPeer.streamsLock.Unlock()
	s, ok := localPeer.streams[pubKey]
	return s, ok
}

func (localPeer *Peer) IsConnectionWithPeerIdExists(peerId peer.ID) bool {
	for _, s := range localPeer.streams {
		if s.Conn().RemotePeer() == peerId {
			return true
		}
	}
	return false
}
