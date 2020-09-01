package relay

import (
	"fmt"
	"github.com/libp2p/go-libp2p-core/network"
	ma "github.com/multiformats/go-multiaddr"
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
	relayHost.SetStreamHandler("/getPeerPublicKeyById", handleGetPeerPublicKeyById)
	relayHost.SetStreamHandler("/register", handleRegister)

	fmt.Println("Listening on")
	fmt.Println(h2info)
}

func handleGetPeerAddr(s network.Stream) {

	publicKey, err := communication.ReadOnce(s)
	if err != nil {
		if err.Error() == "stream reset" {
			fmt.Println("Connection closed by peer")
			s.Close()
			return
		}
		panic(err)
	}

	if peerId, err := storage.GetPeerIdByPublicKey(string(publicKey)); err != nil {
		log.Println("Peer id not found with public key:", string(publicKey), err)
		err = communication.Write(s, []byte("0"))
		if err != nil {
			if err.Error() == "stream reset" {
				s.Close()
				fmt.Println("Connection closed by peer")
				return
			}
			panic(err)
		}
	} else {
		log.Println("Found peer id with public key:", string(publicKey))
		err = communication.Write(s, []byte(peerId.Pretty()))
		if err != nil {
			if err.Error() == "stream reset" {
				s.Close()
				fmt.Println("Connection closed by peer")
				return
			}
			panic(err)
		}
	}

	if peerAddr := storage.GetPeerAddrByPublicKey(string(publicKey)); peerAddr == nil {
		log.Println("Peer addr not found with public key:", string(publicKey))
		err = communication.Write(s, []byte("0"))
		if err != nil {
			if err.Error() == "stream reset" {
				s.Close()
				fmt.Println("Connection closed by peer")
				return
			}
			panic(err)
		}
	} else {
		log.Println("Found peer addr with public key:", string(publicKey))
		err = communication.Write(s, peerAddr.Bytes())
		if err != nil {
			if err.Error() == "stream reset" {
				s.Close()
				fmt.Println("Connection closed by peer")
				return
			}
			panic(err)
		}
	}

	s.Close()
}

func handleGetPeerPublicKeyById(s network.Stream) {

	peerIdBinary, err := communication.ReadOnce(s)
	if err != nil {
		if err.Error() == "stream reset" {
			fmt.Println("Connection closed by peer")
			s.Close()
			return
		}
		panic(err)
	}

	peerId, err := peer.IDFromBytes(peerIdBinary)
	if err != nil {
		s.Close()
		return
	}

	if publicKey := storage.GetPeerPublicKeyById(peerId); publicKey == "" {
		log.Println("Peer pub key not found with peerId:", peerId.Pretty())
		err = communication.Write(s, []byte("0"))
		if err != nil {
			if err.Error() == "stream reset" {
				s.Close()
				fmt.Println("Connection closed by peer")
				return
			}
			panic(err)
		}
	} else {
		log.Println("Found peer pub key with peer id:", publicKey, peerId.Pretty())
		err = communication.Write(s, []byte(publicKey))
		if err != nil {
			if err.Error() == "stream reset" {
				s.Close()
				fmt.Println("Connection closed by peer")
				return
			}
			panic(err)
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
			s.Close()
			return
		}
		panic(err)
	}

	publicKey, err := utils.EcRecover(peerId, signature)
	if err != nil {
		log.Println(err)
		s.Close()
		return
	}

	mAddrBytes, err := communication.ReadOnce(s)
	if err != nil {
		if err.Error() == "stream reset" {
			fmt.Println("Connection closed by peer")
			s.Close()
			return
		}
		panic(err)
	}

	mAddr, _ := ma.NewMultiaddrBytes(mAddrBytes)

	storage.RegisterPeer(utils.PublicKeyToHexString(publicKey), s.Conn().RemotePeer(), mAddr)

	err = communication.Write(s, []byte("1"))
	if err != nil {
		log.Println(err)
		return
	}
	s.Close()
}
