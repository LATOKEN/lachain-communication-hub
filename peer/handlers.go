package peer

import (
	"io"
	"lachain-communication-hub/communication"
	"lachain-communication-hub/storage"
	"lachain-communication-hub/types"
	"lachain-communication-hub/utils"
	"time"

	"github.com/juju/loggo"
	"github.com/libp2p/go-libp2p-core/network"
	ma "github.com/multiformats/go-multiaddr"
)

var handler = loggo.GetLogger("handler")

func incomingConnectionEstablishmentHandler(peer *Peer) func(s network.Stream) {
	log.Tracef("Incoming connection handler set")
	return func(s network.Stream) {
		handleHubConnection(peer, s)
	}
}

func getPeerHandlerForLocalPeer(localPeer *Peer) func(s network.Stream) {
	return func(s network.Stream) {
		handleGetPeers(localPeer, s)
	}
}

func registerHandlerForLocalPeer(localPeer *Peer) func(s network.Stream) {
	return func(s network.Stream) {
		handleRegister(localPeer, s)
	}
}

func handleHubConnection(peer *Peer, s network.Stream) {
	remotePeerId := s.Conn().RemotePeer()
	remotePeer, err := storage.GetPeerById(remotePeerId)
	if err != nil {
		log.Warningf("Peer not found with id %s", remotePeerId)
		s.Close()
		return
	}
	streamRegistered := peer.IsConnected(remotePeer.PublicKey)
	if !streamRegistered {
		peer.RegisterHubStream(remotePeer.PublicKey, s)
	}

	msgChannelExist := peer.IsMsgChannelExist(remotePeer.PublicKey)
	if !msgChannelExist {
		msgChannel := peer.NewMsgChannel(remotePeer.PublicKey)
		if msgChannel == nil {
			log.Warningf("Cant open msg channel with %s", remotePeer.PublicKey)
			peer.removeFromConnected(remotePeer.PublicKey)
			return
		}
		log.Tracef("Adding channel for key %s", string(remotePeer.PublicKey))
		peer.mutex.Lock()
		peer.msgChannels[remotePeer.PublicKey] = msgChannel
		peer.mutex.Unlock()
	}

	storage.SetPublicKeyConnected(remotePeer.PublicKey, true)

	peer.SendPostponedMessages(remotePeer.PublicKey)

	for {
		msg, err := communication.ReadOnce(s)
		if err != nil {
			if err == io.EOF {
				handler.Errorf("connection reset")
				peer.removeFromConnected(remotePeer.PublicKey)
				return
			}
			handler.Errorf("Can't read message. Closing connection")
			handler.Errorf("%s", err)
			peer.removeFromConnected(remotePeer.PublicKey)
			return
		}
		err = processMessage(peer, s, msg)
		if err != nil {
			handler.Errorf("Connection problem")
			peer.removeFromConnected(remotePeer.PublicKey)
			return
		}
		storage.UpdateRegisteredPeerById(remotePeerId)
	}
}

func processMessage(localPeer *Peer, s network.Stream, msg []byte) error {
	if len(msg) == 0 {
		return nil
	}
	handler.Tracef("received msg from peer: %s, message len = %d", s.Conn().RemotePeer(), len(msg))
	localPeer.msgHandler(msg)
	switch string(msg) {
	case "ping":
		err := communication.Write(s, []byte("73515441561657fdh437h7fh4387f7834"))
		if err != nil {
			return err
		}
		break

		//case "pong":
		//	time.Sleep(2 * time.Second)
		//	_, err := s.Write([]byte("ping"))
		//	if err != nil {
		//		panic(err)
		//	}
		//	break
	}
	return nil
}

func handleRegister(localPeer *Peer, s network.Stream) {

	if localPeer.Signature == nil {
		log.Debugf("We don't have signature yet, skipping registration")
		s.Close()
		return
	}

	log.Debugf("Peer registration")

	peerId, _ := s.Conn().RemotePeer().Marshal()

	signature, err := communication.ReadOnce(s)
	if err != nil {
		if err.Error() == "stream reset" {
			log.Errorf("Connection closed by peer")
			s.Close()
			return
		}
		log.Errorf("%s", err)
		s.Close()
		return
	}

	publicKey, err := utils.EcRecover(peerId, signature)
	if err != nil {
		log.Errorf("%s", err)
		s.Close()
		return
	}

	mAddrBytes, err := communication.ReadOnce(s)
	if err != nil {
		if err.Error() == "stream reset" {
			log.Errorf("Connection closed by peer")
			s.Close()
			return
		}
		log.Errorf("%s", err)
		s.Close()
		return
	}

	mAddr, _ := ma.NewMultiaddrBytes(mAddrBytes)

	regPeer := &types.PeerConnection{
		PublicKey: utils.PublicKeyToHexString(publicKey),
		Id:        s.Conn().RemotePeer(),
		LastSeen:  uint32(time.Now().Unix()),
		Addr:      mAddr,
	}

	err = communication.Write(s, localPeer.Signature)
	if err != nil {
		log.Errorf("%s", err)
		s.Close()
		return
	}

	s.Close()

	storage.RegisterOrUpdatePeer(regPeer)
	storage.SetPeerIdConnected(regPeer.Id.Pretty(), true)
}

func handleGetPeers(localPeer *Peer, s network.Stream) {

	peerConnections := storage.GetRecentPeers()

	if len(peerConnections) == 0 {
		err := communication.Write(s, []byte("0"))
		if err != nil {
			if err.Error() == "stream reset" {
				s.Close()
				log.Errorf("Connection closed by peer")
				return
			}
			panic(err)
		}
		return
	}

	var peersBytes []byte

	for _, peerConn := range peerConnections {
		if !storage.IsPublicKeyConnected(peerConn.PublicKey) {
			continue
		}
		peersBytes = append(peersBytes, peerConn.Encode()...)
	}

	err := communication.Write(s, peersBytes)
	if err != nil {
		if err.Error() == "stream reset" {
			s.Close()
			log.Errorf("Connection closed by peer")
			return
		}
		panic(err)
	}

	s.Close()
}
