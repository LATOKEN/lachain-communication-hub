package relay

import (
	"github.com/juju/loggo"
	"github.com/libp2p/go-libp2p-core/network"
	"github.com/libp2p/go-libp2p-core/peer"
	"lachain-communication-hub/communication"
	"lachain-communication-hub/host"
	"lachain-communication-hub/storage"
	"lachain-communication-hub/types"
	"lachain-communication-hub/utils"
)

var log = loggo.GetLogger("builder")

func Run() {

	// Tell the host to relay connections for other peers
	relayHost := host.BuildNamedHost(types.Relay, "_relay")

	h2info := peer.AddrInfo{
		ID:    relayHost.ID(),
		Addrs: relayHost.Addrs(),
	}

	relayHost.SetStreamHandler("/getPeerAddr", handleGetPeerAddr)
	relayHost.SetStreamHandler("/register", handleRegister)

	log.Infof("Listening on: ", h2info)
}

func handleGetPeerAddr(s network.Stream) {

	publicKey, err := communication.ReadOnce(s)
	if err != nil {
		if err.Error() == "stream reset" {
			log.Errorf("Connection closed by peer")
			return
		}
		panic(err)
	}

	if peerId, err := storage.GetPeerIdByPublicKey(string(publicKey)); err != nil {
		log.Warningf("Peer not found with public key %s: %s", string(publicKey), err)
		err = communication.Write(s, []byte("0"))
		if err != nil {
			return
		}
	} else {
		log.Debugf("Found peer with public key: %s", string(publicKey))
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
			log.Errorf("Connection closed by peer")
			return
		}
		panic(err)
	}

	publicKey, err := utils.EcRecover(peerId, signature)
	if err != nil {
		log.Errorf("%s", err)
		return
	}

	storage.RegisterPeer(utils.PublicKeyToHexString(publicKey), s.Conn().RemotePeer().Pretty())

	err = communication.Write(s, []byte("1"))
	if err != nil {
		log.Errorf("%s", err)
		return
	}

	s.Close()
}
