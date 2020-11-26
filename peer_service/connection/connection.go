package connection

import (
	"context"
	"errors"
	"github.com/enriquebris/goconcurrentqueue"
	"github.com/juju/loggo"
	core "github.com/libp2p/go-libp2p-core"
	"github.com/libp2p/go-libp2p-core/network"
	"github.com/libp2p/go-libp2p-core/peer"
	ma "github.com/multiformats/go-multiaddr"
	"go.uber.org/atomic"
	"lachain-communication-hub/communication"
	"lachain-communication-hub/utils"
	"math/rand"
	"sync"
	"time"
)

var log = loggo.GetLogger("connection")

type Status int

const (
	NotConnected      = iota
	JustConnected     = iota
	HandshakeComplete = iota
	Terminated        = iota
)

const MaxTryToConnect = 3

type Connection struct {
	PeerId        peer.ID
	PeerAddress   ma.Multiaddr
	PeerPublicKey string

	host                 *core.Host
	myAddress            ma.Multiaddr
	signature            []byte
	signatureSent        *atomic.Int32
	messageQueue         *goconcurrentqueue.FIFO
	stream               network.Stream
	streamLock           sync.Mutex
	status               *atomic.Int32
	onPeerListUpdate     func([]*Metadata)
	onPublicKeyRecovered func(*Connection, string)
	onMessage            func([]byte)
	availableRelays      func() []peer.ID
	getPeers             func() []*Metadata
	lifecycleFinished    chan struct{}
	sendCycleFinished    chan struct{}
	peerCycleFinished    chan struct{}
}

func New(
	host *core.Host, id peer.ID, myAddress ma.Multiaddr, peerAddress ma.Multiaddr, signature []byte,
	onPeerListUpdate func([]*Metadata), onPublicKeyRecovered func(*Connection, string), onMessage func([]byte),
	availableRelays func() []peer.ID, getPeers func() []*Metadata,
) *Connection {
	log.Debugf("Creating connection with peer %v (address %v)", id.Pretty(), peerAddress.String())
	connection := new(Connection)
	connection.host = host
	connection.PeerId = id
	connection.myAddress = myAddress
	connection.lifecycleFinished = make(chan struct{})
	connection.sendCycleFinished = make(chan struct{})
	connection.peerCycleFinished = make(chan struct{})
	connection.PeerAddress = peerAddress
	connection.signatureSent = atomic.NewInt32(0)
	connection.status = atomic.NewInt32(NotConnected)
	connection.messageQueue = goconcurrentqueue.NewFIFO()
	connection.onPeerListUpdate = onPeerListUpdate
	connection.onPublicKeyRecovered = onPublicKeyRecovered
	connection.onMessage = onMessage
	connection.availableRelays = availableRelays
	connection.getPeers = getPeers
	go connection.connectionLifeCycle()
	go connection.sendMessageCycle()
	go connection.sendPeersCycle()
	if signature != nil {
		connection.SetSignature(signature)
	}
	return connection
}

func FromStream(
	host *core.Host, stream *network.Stream, myAddress ma.Multiaddr,
	onPeerListUpdate func([]*Metadata), onPublicKeyRecovered func(*Connection, string), onMessage func([]byte),
	availableRelays func() []peer.ID, getPeers func() []*Metadata,
) *Connection {
	log.Debugf("Creating connection with peer %v from existing stream", (*stream).Conn().RemotePeer().Pretty())
	connection := new(Connection)
	connection.host = host
	connection.PeerId = (*stream).Conn().RemotePeer()
	connection.myAddress = myAddress
	connection.lifecycleFinished = make(chan struct{})
	connection.sendCycleFinished = make(chan struct{})
	connection.peerCycleFinished = make(chan struct{})
	connection.signatureSent = atomic.NewInt32(0)
	connection.status = atomic.NewInt32(NotConnected)
	connection.messageQueue = goconcurrentqueue.NewFIFO()
	connection.onPeerListUpdate = onPeerListUpdate
	connection.onPublicKeyRecovered = onPublicKeyRecovered
	connection.onMessage = onMessage
	connection.availableRelays = availableRelays
	connection.getPeers = getPeers
	go connection.connectionLifeCycle()
	go connection.sendMessageCycle()
	go connection.sendPeersCycle()
	return connection
}

func (connection *Connection) SetPeerAddress(address ma.Multiaddr) {
	connection.PeerAddress = address
}

func (connection *Connection) IsActive() bool {
	return connection.status.Load() != Terminated && connection.status.Load() != NotConnected && connection.stream != nil
}

func (connection *Connection) connectionLifeCycle() {
	openStreamBackoff := time.Second
	connection.signatureSent.Store(0)
	for connection.status.Load() != Terminated {
		if err := connection.checkStream(); err != nil {
			log.Tracef("Can't connect to peer %v, will retry in %v", connection.PeerId, openStreamBackoff)
			time.Sleep(openStreamBackoff)
			if openStreamBackoff < time.Minute {
				openStreamBackoff *= 2
			}
			continue
		} else {
			openStreamBackoff = time.Second
		}

		if !connection.signatureSent.CAS(0, 1) && len(connection.signature) > 0 {
			go connection.sendSignature()
		}

		frame, err := communication.ReadOnce(connection.stream)
		if err != nil {
			log.Errorf("Skipped message from peer %v, resetting connection: %v", connection.PeerId.Pretty(), err)
			connection.resetStream()
			continue
		}

		switch frame.Kind() {
		case communication.Message:
			connection.onMessage(frame.Data())
			break
		case communication.Signature:
			connection.handleSignature(frame.Data())
			break
		case communication.GetPeersReply:
			connection.handlePeers(frame.Data())
			break
		default:
			log.Errorf("Unknown frame kind received from peer %v: %v", connection.PeerId.Pretty(), frame.Kind())
		}
	}
	close(connection.lifecycleFinished)
}

func (connection *Connection) sendMessageCycle() {
	backoff := time.Millisecond
	var msgToSend []byte = nil
	for connection.status.Load() != Terminated {
		if msgToSend == nil {
			value, err := connection.messageQueue.DequeueOrWaitForNextElement()
			if err != nil {
				log.Errorf("Failed to wait for message to send to peer %v: %v", connection.PeerId.Pretty(), err)
			}
			if value == nil {
				log.Tracef("Got terminating message, finishing send cycle for peer %v", connection.PeerId.Pretty())
				break
			}
			msgToSend = value.([]byte)
		}

		if connection.stream != nil {
			err := communication.Write(connection.stream, communication.NewFrame(communication.Message, msgToSend))
			if err == nil {
				backoff = time.Millisecond
				msgToSend = nil
				continue
			}
			log.Errorf("Error while sending message: %v", err)
		}
		log.Tracef("Resending message to peer %v in %v (nil stream: %v)", connection.PeerId.Pretty(), backoff, connection.stream == nil)
		time.Sleep(backoff)
		if backoff < time.Second {
			backoff *= 2
		}
		continue
	}
	close(connection.sendCycleFinished)
}

func (connection *Connection) sendPeersCycle() {
	for connection.status.Load() != Terminated {
		connection.sendPeers()
		select {
		case <-connection.peerCycleFinished:
			return
		case <-time.After(time.Second * 10):
			continue
		}
	}
}

func (connection *Connection) Send(msg []byte) {
	if msg == nil {
		log.Errorf("Got empty message to send to peer %v, ignoring", connection.PeerId.Pretty())
		return
	}

	if err := connection.messageQueue.Enqueue(msg); err != nil {
		log.Errorf("Failed to queue message (this might be critical) to peer %v: %v", connection.PeerId.Pretty(), err)
	}
}

func (connection *Connection) sendSignature() {
	backoff := time.Second
	for connection.status.Load() != Terminated {
		if connection.stream != nil {
			if len(connection.signature) != 65 {
				panic("bad signature length!")
			}
			var payload []byte
			payload = append(payload, connection.signature...)
			payload = append(payload, connection.myAddress.Bytes()...)

			err := communication.Write(connection.stream, communication.NewFrame(communication.Signature, payload))
			if err == nil {
				backoff = time.Second
				break
			}
			log.Errorf("Error while sending signature to peer %v: %v", connection.PeerId.Pretty(), err)
		}
		log.Tracef("Resending signature to peer %v in %v (nil stream: %v)", connection.PeerId.Pretty(), backoff, connection.stream == nil)
		time.Sleep(backoff)
		if backoff < time.Minute {
			backoff *= 2
		}
	}
}

func (connection *Connection) SetSignature(signature []byte) {
	connection.signature = signature
	if connection.signatureSent.CAS(0, 1) {
		go connection.sendSignature()
	}
}

func (connection *Connection) handleSignature(data []byte) {
	if connection.status.Load() == Terminated {
		return
	}
	signature, addressBytes := data[:65], data[65:]
	peerIdBytes, err := connection.PeerId.Marshal()
	if err != nil {
		log.Errorf("Cannot create payload for signature check from peer %v: %v", connection.PeerId.Pretty(), err)
		return
	}
	publicKey, err := utils.EcRecover(peerIdBytes, signature)
	if err != nil {
		log.Errorf("Signature check failed for peer %v, resetting connection: %v", connection.PeerId.Pretty(), err)
		connection.resetStream()
		return
	}
	address, err := ma.NewMultiaddrBytes(addressBytes)
	if err != nil {
		log.Errorf("Peer %v sent incorrect address, resetting connection: %v", connection.PeerId.Pretty(), err)
		connection.resetStream()
		return
	}
	connection.PeerPublicKey = utils.PublicKeyToHexString(publicKey)
	connection.PeerAddress = address
	connection.status.CAS(NotConnected, HandshakeComplete)
	connection.status.CAS(JustConnected, HandshakeComplete)
	connection.onPublicKeyRecovered(connection, connection.PeerPublicKey)
}

func (connection *Connection) sendPeers() {
	if connection.stream != nil {
		peerConnections := connection.getPeers()
		if len(peerConnections) == 0 {
			return
		}
		err := communication.Write(connection.stream, communication.NewFrame(communication.GetPeersReply, EncodeArray(peerConnections)))
		if err != nil {
			log.Errorf("Cannot send peer list to peer %v: %v", connection.PeerId.Pretty(), err)
		}
		return
	}
	log.Errorf("Cannot send peer list to peer %v: no connection yet", connection.PeerId.Pretty())
}

func (connection *Connection) handlePeers(data []byte) {
	peerConnections := DecodeArray(data)
	log.Debugf("Received %v peers from peer %v", len(peerConnections), connection.PeerId.Pretty())
	connection.onPeerListUpdate(peerConnections)
}

func (connection *Connection) connect(peerId peer.ID, peerAddress ma.Multiaddr) error {
	if peerAddress != nil {
		targetPeerInfo := peer.AddrInfo{
			ID:    peerId,
			Addrs: []ma.Multiaddr{peerAddress},
		}
		err := (*connection.host).Connect(context.Background(), targetPeerInfo)
		if err != nil {
			return err
		}
		return nil
	}

	errCount := 0
	peers := append([]peer.ID(nil), connection.availableRelays()...)
	rand.Seed(time.Now().UnixNano())
	rand.Shuffle(len(peers), func(i, j int) { peers[i], peers[j] = peers[j], peers[i] })
	for _, connectedPeerId := range peers {
		if err := connection.connectToPeerUsingRelay(connectedPeerId, peerId); err != nil {
			log.Debugf("Can't connect to %v through %v: %v", peerAddress, connectedPeerId, err)
			errCount++
			if errCount == MaxTryToConnect {
				return errors.New("cant_connect")
			}
			continue
		}
		return nil
	}
	return errors.New("can't connect")
}

func (connection *Connection) connectToPeerUsingRelay(relayId peer.ID, targetPeerId peer.ID) error {
	relayedAddr, err := ma.NewMultiaddr("/p2p/" + relayId.Pretty() + "/p2p-circuit/p2p/" + targetPeerId.Pretty())
	if err != nil {
		return err
	}
	targetPeerInfo := peer.AddrInfo{
		ID:    targetPeerId,
		Addrs: []ma.Multiaddr{relayedAddr},
	}
	err = (*connection.host).Connect(context.Background(), targetPeerInfo)
	if err != nil {
		return err
	}
	return nil
}

func (connection *Connection) SetStream(stream network.Stream) {
	if connection.status.Load() == Terminated {
		return
	}
	log.Tracef("Updating stream for connection with peer %v", stream.Conn().RemotePeer().Pretty())
	connection.streamLock.Lock()
	defer connection.streamLock.Unlock()
	if connection.stream != nil {
		if err := connection.stream.Reset(); err != nil {
			log.Errorf("Failed to reset stream: %v", err)
		}
	}
	connection.stream = stream
	connection.signatureSent.Store(0)
	connection.status.CAS(NotConnected, JustConnected)
	connection.status.CAS(HandshakeComplete, JustConnected)
}

func (connection *Connection) checkStream() error {
	connection.streamLock.Lock()
	defer connection.streamLock.Unlock()
	if (*connection.host).Network().Connectedness(connection.PeerId) != network.Connected {
		log.Debugf("Peer %v lacks connectedness, calling connect", connection.PeerId.Pretty())
		err := connection.connect(connection.PeerId, connection.PeerAddress)
		if err != nil {
			return err
		}
		connection.signatureSent.Store(0)
	}
	if connection.stream == nil {
		log.Debugf("Peer %v has no stream, creating one", connection.PeerId.Pretty())
		stream, err := (*connection.host).NewStream(context.Background(), connection.PeerId, "/")
		if err != nil {
			return err
		}
		connection.stream = stream
		connection.signatureSent.Store(0)
		connection.status.CAS(NotConnected, JustConnected)
		connection.status.CAS(HandshakeComplete, JustConnected)
	}
	return nil
}

func (connection *Connection) resetStream() {
	log.Debugf("Resetting stream to peer %s", connection.stream.Conn().RemotePeer().Pretty())
	if err := connection.stream.Reset(); err != nil {
		log.Errorf("Failed to reset stream: %v", err)
	}
	connection.signatureSent.Store(0)
	connection.stream = nil
}

func (connection *Connection) Terminate() {
	connection.status.Store(Terminated)
	if err := connection.messageQueue.Enqueue(nil); err != nil {
		log.Errorf("Can't enqueue terminal message in message queue: %v", err)
	}
	if connection.stream != nil {
		err := connection.stream.Reset()
		if err != nil {
			log.Errorf("Can't reset stream while terminating connection: %v", err)
		}
	}
	<-connection.lifecycleFinished
	log.Debugf("Lifecycle finished")
	<-connection.sendCycleFinished
	log.Debugf("SendCycle finished")
	close(connection.peerCycleFinished)
	log.Debugf("PeerCycle finished")
}
