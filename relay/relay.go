package relay

import (
	"bufio"
	"fmt"
	"github.com/libp2p/go-libp2p-core/network"
	"lachain-communication-hub/communication"
	"lachain-communication-hub/host"
	"lachain-communication-hub/storage"
	"lachain-communication-hub/types"

	"github.com/libp2p/go-libp2p-core/peer"
)

func Run() {

	// Tell the host to relay connections for other peers
	relayHost := host.BuildNamedHost(types.Relay, "_relay")

	h2info := peer.AddrInfo{
		ID:    relayHost.ID(),
		Addrs: relayHost.Addrs(),
	}

	relayHost.SetStreamHandler("/getPeerAddr", handleGetPeerAddr)
	relayHost.SetStreamHandler("/register", handleRegister)

	fmt.Println("Listening on")
	fmt.Println(h2info)
}

func handleGetPeerAddr(s network.Stream) {

	rw := bufio.NewReadWriter(bufio.NewReader(s), bufio.NewWriter(s))

	publicKey, err := communication.ReadOnce(rw)
	if err != nil {
		if err.Error() == "stream reset" {
			fmt.Println("Connection closed by peer")
			return
		}
		panic(err)
	}

	if peerId, err := storage.GetPeerIdByPublicKey(string(publicKey)); err != nil {
		communication.WriteOnce(rw, []byte("0"))
	} else {
		communication.WriteOnce(rw, []byte(peerId.Pretty()))
	}

	s.Close()
}

func handleRegister(s network.Stream) {

	rw := bufio.NewReadWriter(bufio.NewReader(s), bufio.NewWriter(s))

	peerId := s.Conn().RemotePeer().Pretty()

	publicKey, err := communication.ReadOnce(rw)
	if err != nil {
		if err.Error() == "stream reset" {
			fmt.Println("Connection closed by peer")
			return
		}
		panic(err)
	}

	fmt.Println("Peer registration", peerId)

	storage.RegisterPeer(string(publicKey), peerId)

	s.Close()
}
