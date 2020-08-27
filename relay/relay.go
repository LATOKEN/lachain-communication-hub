package relay

import (
	"fmt"
	"github.com/libp2p/go-libp2p-core/network"
	"lachain-communication-hub/communication"
	"lachain-communication-hub/host"
	"lachain-communication-hub/storage"
	"lachain-communication-hub/types"
	"lachain-communication-hub/utils"
	"log"

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

	publicKey, err := communication.ReadOnce(s)
	if err != nil {
		if err.Error() == "stream reset" {
			fmt.Println("Connection closed by peer")
			return
		}
		panic(err)
	}

	if peerId, err := storage.GetPeerIdByPublicKey(string(publicKey)); err != nil {
		log.Println("Peer not found with public key:", string(publicKey), err)
		err = communication.Write(s, []byte("0"))
		if err != nil {
			return
		}
	} else {
		log.Println("Found peer with public key:", string(publicKey))
		err = communication.Write(s, []byte(peerId.Pretty()))
		if err != nil {
			return
		}
	}

	s.Close()
}

func handleRegister(s network.Stream) {

	peerId, err := s.Conn().RemotePeer().Marshal()

	signature, err := communication.ReadOnce(s)
	if err != nil {
		if err.Error() == "stream reset" {
			fmt.Println("Connection closed by peer")
			return
		}
		panic(err)
	}

	publicKey, err := utils.EcRecover(peerId, signature)
	if err != nil {
		log.Println(err)
		return
	}

	storage.RegisterPeer(utils.PublicKeyToHexString(publicKey), s.Conn().RemotePeer().Pretty())

	err = communication.Write(s, []byte("1"))
	if err != nil {
		log.Println(err)
		return
	}

	s.Close()
}
