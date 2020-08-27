package peer

import (
	"context"
	"encoding/hex"
	"errors"
	"fmt"
	core "github.com/libp2p/go-libp2p-core"
	"github.com/libp2p/go-libp2p-core/network"
	"github.com/libp2p/go-libp2p-core/peer"
	swarm "github.com/libp2p/go-libp2p-swarm"
	ma "github.com/multiformats/go-multiaddr"
	"lachain-communication-hub/communication"
	"lachain-communication-hub/config"
	"lachain-communication-hub/host"
	"lachain-communication-hub/types"
	"log"
)

type Peer struct {
	host           core.Host
	streams        map[string]network.Stream
	grpcMsgHandler func([]byte)
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
	localPeer := Peer{localHost, make(map[string]network.Stream), nil}

	localPeer.SetStreamHandler(GRPCHandlerMock)

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

	err = communication.Write(s, signature)
	if err != nil {
		log.Fatal("cannot register", err)
	}

	resp, err := communication.ReadOnce(s)
	if err != nil {
		log.Fatal("cannot register", err)
	}

	if string(resp) != "1" {
		log.Fatal("cannot register", string(resp))
	}

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
			resp, err := localPeer.ReceiveResponseFromPeer(publicKey)
			if err != nil {
				localPeer.removeFromConected(publicKey)
				return
			}
			processMessage(localPeer, hubStream, resp)
			//fmt.Println("response: ", string(resp))
		}
	}()

	localPeer.streams[publicKey] = hubStream
	return nil
}

func (localPeer *Peer) SendMessageToPeer(publicKey string, msg []byte) {

	err := localPeer.connectToPeer(publicKey)
	if err != nil {
		log.Println("Can't establish connection with", publicKey)
		log.Println(err)
		return
	}

	err = communication.Write(localPeer.streams[publicKey], msg)
	if err != nil {
		log.Printf("Cant connect to peer %s. Removing from connected", publicKey)
		localPeer.removeFromConected(publicKey)
		return
	}
}

func (localPeer *Peer) ReceiveResponseFromPeer(publicKey string) ([]byte, error) {
	s, ok := localPeer.streams[publicKey]
	if !ok {
		fmt.Println("Connection not found with", publicKey)
		return nil, errors.New("not found")
	}

	msg, err := communication.ReadOnce(s)
	if err != nil {
		if err.Error() == "stream reset" {
			log.Println("Connection closed by peer")
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
		delete(localPeer.streams, publicKey)
	}
}
