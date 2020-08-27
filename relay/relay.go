package relay

import (
	"bufio"
	"encoding/hex"
	"fmt"
	crypto3 "github.com/ethereum/go-ethereum/crypto"
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

	log.Println("handleGetPeerAddr")

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
		log.Println("Peer not found with public key:", string(publicKey))
		_, err = s.Write([]byte("0"))
		if err != nil {
			return
		}
	} else {
		log.Println("Found peer with public key:", string(publicKey))
		_, err = s.Write([]byte(peerId.Pretty()))
		if err != nil {
			return
		}
	}

	s.Close()
	log.Println("closing getPeerAddr stream")
}

func handleRegister(s network.Stream) {

	rw := bufio.NewReadWriter(bufio.NewReader(s), bufio.NewWriter(s))

	peerId, err := s.Conn().RemotePeer().Marshal()

	signature, err := communication.ReadOnce(rw)
	if err != nil {
		if err.Error() == "stream reset" {
			fmt.Println("Connection closed by peer")
			return
		}
		panic(err)
	}

	publicKey, err := utils.EcRecover(peerId, signature)
	if err != nil {
		fmt.Println(err)
		return
	}

	pubHex := hex.EncodeToString(crypto3.CompressPubkey(publicKey))

	fmt.Printf("Peer registration, id: %s, pubKey: %s\n", s.Conn().RemotePeer().Pretty(), pubHex)

	storage.RegisterPeer(pubHex, s.Conn().RemotePeer().Pretty())

	s.Close()
}
