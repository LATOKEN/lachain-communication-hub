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
	"sync/atomic"
)

var log = loggo.GetLogger("peer")

type Peer struct {
	host           core.Host
	streams        map[string]network.Stream
	grpcMsgHandler func([]byte)
	running        int32
}

func GRPCHandlerMock(msg []byte) {}

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
	localPeer := Peer{localHost, make(map[string]network.Stream), nil, 1}
	localPeer.SetStreamHandler(GRPCHandlerMock)

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

func (localPeer *Peer) connectToPeer(publicKey string) error {
	if localPeer.running == 0 {
		return nil
	}
	if _, ok := localPeer.streams[publicKey]; ok {
		return nil
	}

	relayStream, err := localPeer.host.NewStream(context.Background(), config.GetRelayID(), "/getPeerAddr")
	if err != nil {
		return err
	}

	err = communication.Write(relayStream, []byte(publicKey))
	if err != nil {
		return err
	}

	peerIdBytes, err := communication.ReadOnce(relayStream)
	if err != nil {
		return err
	}

	relayStream.Close()

	peerId, err := peer.Decode(string(peerIdBytes))
	if err != nil {
		log.Errorf("Peer not found")
		return err
	}

	// Creates a relay address
	relayedAddr, err := ma.NewMultiaddr("/p2p/" + config.GetRelayID().Pretty() + "/p2p-circuit/p2p/" + peerId.Pretty())
	if err != nil {
		return err
	}

	// Since we just tried and failed to dial, the dialer system will, by default
	// prevent us from redialing again so quickly. Since we know what we're doing, we
	// can use this ugly hack (it's on our TODO list to make it a little cleaner)
	// to tell the dialer "no, its okay, let's try this again"
	localPeer.host.Network().(*swarm.Swarm).Backoff().Clear(peerId)

	relayedPeerInfo := peer.AddrInfo{
		ID:    peerId,
		Addrs: []ma.Multiaddr{relayedAddr},
	}
	if err := localPeer.host.Connect(context.Background(), relayedPeerInfo); err != nil {
		return err
	}

	// Woohoo! we're connected!
	hubStream, err := localPeer.host.NewStream(context.Background(), peerId, "/hub")
	if err != nil {
		log.Errorf("huh, this should have worked: %s", err)
		return err
	}

	go func() {
		for localPeer.running != 0 {
			resp, err := localPeer.ReceiveResponseFromPeer(publicKey)
			if err != nil {
				localPeer.removeFromConected(publicKey)
				return
			}
			processMessage(localPeer, hubStream, resp)
			//log.Println("response: ", string(resp))
		}
	}()

	localPeer.streams[publicKey] = hubStream
	return nil
}

func (localPeer *Peer) SendMessageToPeer(publicKey string, msg []byte) {
	if localPeer.running == 0 {
		return
	}
	err := localPeer.connectToPeer(publicKey)
	if err != nil {
		log.Errorf("Can't establish connection with: %s", publicKey)
		log.Errorf("%s", err)
		return
	}

	err = communication.Write(localPeer.streams[publicKey], msg)
	if err != nil {
		log.Errorf("Cant connect to peer %s. Removing from connected", publicKey)
		localPeer.removeFromConected(publicKey)
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
			localPeer.removeFromConected(publicKey)
			return nil, errors.New("conn reset")
		}
		return nil, err
	}
	return msg, nil
}

func (localPeer *Peer) SetStreamHandler(callback func(msg []byte)) {
	localPeer.grpcMsgHandler = callback
	localPeer.host.SetStreamHandler("/hub", incomingConnectionEstablishmentHandler(localPeer))
}

func (localPeer *Peer) GetId() []byte {
	id, err := localPeer.host.ID().Marshal()
	if err != nil {
		panic(err)
	}
	return id
}

func (localPeer *Peer) removeFromConected(publicKey string) {
	if _, ok := localPeer.streams[publicKey]; ok {
		localPeer.streams[publicKey].Close()
		delete(localPeer.streams, publicKey)
	}
}
