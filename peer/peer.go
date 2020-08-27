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
	host           core.Host
	streams        map[string]network.Stream
	grpcMsgHandler func([]byte)
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
	localPeer := Peer{localHost, make(map[string]network.Stream), nil}

	defaultMsgHandler := func(msg []byte) {
		fmt.Println(localPeer.host.ID(), "Here is should be grpc callback with msg:", string(msg))
	}

	localPeer.SetStreamHandler(defaultMsgHandler)

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

	//confirmHandle(s)

	s.Close()
}

func (localPeer *Peer) Register(signature []byte) {
	localPeer.register(signature)
}

func (localPeer *Peer) connectToPeer(publicKey string) error {
	if _, ok := localPeer.streams[publicKey]; ok {
		return nil
	}

	relayStream, err := localPeer.host.NewStream(context.Background(), config.GetRelayID(), "/getPeerAddr")
	if err != nil {
		return err
	}

	_, err = relayStream.Write([]byte(publicKey))
	if err != nil {
		return err
	}

	rw := bufio.NewReadWriter(bufio.NewReader(relayStream), bufio.NewWriter(relayStream))

	peerIdBytes, err := communication.ReadOnce(rw)
	if err != nil {
		return err
	}

	relayStream.Close()

	peerId, err := peer.Decode(string(peerIdBytes))
	if err != nil {
		fmt.Println("Peer not found")
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
		fmt.Println("huh, this should have worked: ", err)
		return err
	}

	go func() {
		for {
			resp := localPeer.ReceiveResponseFromPeer(publicKey)
			processMessage(localPeer.grpcMsgHandler, hubStream, resp)
			//fmt.Println("response: ", string(resp))
		}
	}()

	localPeer.streams[publicKey] = hubStream
	return nil
}

func (localPeer *Peer) SendMessageToPeer(publicKey string, msg []byte) {

	err := localPeer.connectToPeer(publicKey)
	if err != nil {
		fmt.Println("Can't establish connection with", publicKey)
		log.Error(err)
		return
	}

	_, err = localPeer.streams[publicKey].Write(msg)
	if err != nil {
		panic(err)
	}
	fmt.Println("msg sent")
}

func (localPeer *Peer) ReceiveResponseFromPeer(publicKey string) []byte {
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

func (localPeer *Peer) SetStreamHandler(callback func(msg []byte)) {
	localPeer.grpcMsgHandler = callback
	localPeer.host.SetStreamHandler("/hub", incomingConnectionEstablishmentHandler(callback))
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
