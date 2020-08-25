package peer

import (
	"bufio"
	"context"
	"encoding/hex"
	"fmt"
	core "github.com/libp2p/go-libp2p-core"
	"github.com/libp2p/go-libp2p-core/network"
	"github.com/libp2p/go-libp2p-core/peer"
	swarm "github.com/libp2p/go-libp2p-swarm"
	ma "github.com/multiformats/go-multiaddr"
	"github.com/opentracing/opentracing-go/log"
	"lachain-communication-hub/communication"
	"lachain-communication-hub/config"
	"lachain-communication-hub/host"
	"lachain-communication-hub/types"
)

type Peer struct {
	host    core.Host
	streams map[string]network.Stream
}

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
	localPeer := Peer{localHost, make(map[string]network.Stream)}

	return localPeer
}

func (localPeer *Peer) register(signature []byte) {

	s, err := localPeer.host.NewStream(context.Background(), config.GetRelayID(), "/register")
	if err != nil {
		panic(err)
	}

	fmt.Println("peer registration")
	fmt.Println("signature", hex.EncodeToString(signature))
	fmt.Println("peerId", localPeer.host.ID())

	_, err = s.Write(signature)
	if err != nil {
		panic(err)
	}

	confirmHandle(s)

	s.Close()
}

func (localPeer *Peer) Register(signature []byte) {
	localPeer.register(signature)
}

func (localPeer *Peer) connectToPeer(publicKey string) (network.Stream, error) {

	if conn, ok := localPeer.streams[publicKey]; ok {
		return conn, nil
	}

	s, err := localPeer.host.NewStream(context.Background(), config.GetRelayID(), "/getPeerAddr")
	if err != nil {
		return nil, err
	}

	_, err = s.Write([]byte(publicKey))
	if err != nil {
		return nil, err
	}

	rw := bufio.NewReadWriter(bufio.NewReader(s), bufio.NewWriter(s))

	peerIdBytes, err := communication.ReadOnce(rw)
	if err != nil {
		return nil, err
	}

	s.Close()

	peerId, err := peer.Decode(string(peerIdBytes))
	if err != nil {
		fmt.Println("Peer not found")
		return nil, err
	}

	fmt.Println("reqesuted peer id", peerId)

	// Creates a relay address
	relayedAddr, err := ma.NewMultiaddr("/p2p/" + config.GetRelayID().Pretty() + "/p2p-circuit/p2p/" + peerId.Pretty())
	if err != nil {
		return nil, err
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
		panic(err)
	}

	// Woohoo! we're connected!
	s, err = localPeer.host.NewStream(context.Background(), peerId, "/hub")
	if err != nil {
		fmt.Println("huh, this should have worked: ", err)
		return nil, err
	}

	localPeer.streams[publicKey] = s
	return s, nil
}

func (localPeer *Peer) SendMessageToPeer(publicKey string, msg []byte) {

	fmt.Println("This peer Id:", localPeer.host.ID())
	s, err := localPeer.connectToPeer(publicKey)
	if err != nil {
		fmt.Println("Can't establish connection with", publicKey)
		log.Error(err)
		return
	}

	_, err = s.Write(msg)
	if err != nil {
		panic(err)
	}
}

func (localPeer *Peer) ReceiveResponseFromPeer(publicKey string) []byte {
	fmt.Println("reading response from peer")
	s, ok := localPeer.streams[publicKey]
	if !ok {
		fmt.Println("Connection not found with", publicKey)
		return nil
	}

	rw := bufio.NewReadWriter(bufio.NewReader(s), bufio.NewWriter(s))

	msg, err := communication.ReadOnce(rw)
	if err != nil {
		if err.Error() == "stream reset" {
			fmt.Println("Connection closed by peer")
			localPeer.removeFromConected(publicKey)
			return nil
		}
		panic(err)
	}
	return msg
}

func (localPeer *Peer) SetMsgHandler(callback func(msg []byte)) {
	localPeer.host.SetStreamHandler("/hub", hubMsgHandler(callback))
}

func (localPeer *Peer) GetId() []byte {
	id, err := localPeer.host.ID().Marshal()
	if err != nil {
		panic(err)
	}
	return id
}

func (localPeer *Peer) RequestDataFromPeer(publicKey string, data []byte) []byte {
	localPeer.SendMessageToPeer(publicKey, data)
	return localPeer.ReceiveResponseFromPeer(publicKey)
}

func (localPeer *Peer) removeFromConected(publicKey string) {
	if _, ok := localPeer.streams[publicKey]; ok {
		delete(localPeer.streams, publicKey)
	}
}
