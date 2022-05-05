package connection

import (
	"context"
	"errors"
	"lachain-communication-hub/communication"
	"lachain-communication-hub/config"
	"lachain-communication-hub/throughput"
	"lachain-communication-hub/utils"
	"math/rand"
	"sync"
	"time"

	"github.com/juju/loggo"
	core "github.com/libp2p/go-libp2p-core"
	"github.com/libp2p/go-libp2p-core/network"
	"github.com/libp2p/go-libp2p-core/peer"
	"github.com/libp2p/go-libp2p-core/protocol"
	swarm "github.com/libp2p/go-libp2p-swarm"
	ma "github.com/multiformats/go-multiaddr"
	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promauto"
	"go.uber.org/atomic"
)

var log = loggo.GetLogger("connection")

type Status int

const (
	NotConnected      = iota
	JustConnected     = iota
	HandshakeComplete = iota
	Terminated        = iota
)

var (
	sendErrors = promauto.NewCounterVec(prometheus.CounterOpts{
		Name: "lachain_hub_send_errors",
		Help: "The total number of errors during send",
	}, []string{"error"})
	receiveErrors = promauto.NewCounterVec(prometheus.CounterOpts{
		Name: "lachain_hub_receive_errors",
		Help: "The total number of errors during receive",
	}, []string{"error"})
	droppedMessages = promauto.NewCounter(prometheus.CounterOpts{
		Name: "lachain_hub_dropped_messages",
		Help: "The total number of messages dropped",
	})
	resendAttempts = promauto.NewHistogram(prometheus.HistogramOpts{
		Name:    "lachain_hub_resend_attempts",
		Help:    "The total number of messages dropped",
		Buckets: []float64{1, 2, 3, 4, 5},
	})
	inboundMessages = promauto.NewCounterVec(prometheus.CounterOpts{
		Name: "lachain_hub_inbound_messages",
		Help: "The total number of inbound messages",
	}, []string{"kind"})
	outboundReset = promauto.NewCounter(prometheus.CounterOpts{
		Name: "lachain_hub_outbound_resets",
		Help: "The total number of times when outbound stream was reset",
	})
	inboundReset = promauto.NewCounter(prometheus.CounterOpts{
		Name: "lachain_hub_inbound_reconnects",
		Help: "The total number of times when inbound stream was reset",
	})
	messagesInQueue = promauto.NewGaugeVec(prometheus.GaugeOpts{
		Name: "lachain_hub_queued_messages",
		Help: "Number of queued messages by peer",
	}, []string{"peer"})
)

const MaxTryToConnect = 3

type Connection struct {
	PeerId        peer.ID
	PeerAddress   ma.Multiaddr
	PeerPublicKey string
	PeerProtocol  string

	host                 *core.Host
	myAddress            ma.Multiaddr
	signature            []byte
	signatureSent        *atomic.Int32
	messageQueue         *utils.MessageQueue
	inboundStream        network.Stream
	outboundStream       network.Stream
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
	inboundTPS           *throughput.Calculator
	outboundTPS          *throughput.Calculator
}

func (connection *Connection) init(
	host *core.Host, id peer.ID, protocol string, myAddress ma.Multiaddr,
	onPeerListUpdate func([]*Metadata), onPublicKeyRecovered func(*Connection, string), onMessage func([]byte),
	availableRelays func() []peer.ID, getPeers func() []*Metadata,
) {
	connection.host = host
	connection.PeerId = id
	connection.PeerProtocol = protocol
	connection.lifecycleFinished = make(chan struct{})
	connection.sendCycleFinished = make(chan struct{})
	connection.peerCycleFinished = make(chan struct{})
	connection.myAddress = myAddress
	connection.signatureSent = atomic.NewInt32(0)
	connection.status = atomic.NewInt32(NotConnected)
	connection.messageQueue = utils.NewMessageQueue()
	connection.onPeerListUpdate = onPeerListUpdate
	connection.onPublicKeyRecovered = onPublicKeyRecovered
	connection.onMessage = onMessage
	connection.availableRelays = availableRelays
	connection.getPeers = getPeers
	connection.inboundTPS = throughput.New(time.Second, func(sum float64, n int32, duration time.Duration) {})
	connection.outboundTPS = throughput.New(time.Second, func(sum float64, n int32, duration time.Duration) {})
}

func New(
	host *core.Host, id peer.ID, protocol string, myAddress ma.Multiaddr, peerAddress ma.Multiaddr,
	signature []byte,
	onPeerListUpdate func([]*Metadata), onPublicKeyRecovered func(*Connection, string), onMessage func([]byte),
	availableRelays func() []peer.ID, getPeers func() []*Metadata,
) *Connection {
	log.Debugf("Creating connection with peer %v (address %v)", id.Pretty(), peerAddress.String())
	connection := new(Connection)
	connection.init(host, id, protocol, myAddress, onPeerListUpdate, onPublicKeyRecovered, onMessage, availableRelays, getPeers)
	connection.PeerAddress = peerAddress
	go connection.receiveMessageCycle()
	go connection.sendMessageCycle()
	go connection.sendPeersCycle()
	if signature != nil {
		connection.SetSignature(signature)
	}
	return connection
}

func FromStream(
	host *core.Host, stream network.Stream, myAddress ma.Multiaddr, signature []byte, protocol string,
	onPeerListUpdate func([]*Metadata), onPublicKeyRecovered func(*Connection, string), onMessage func([]byte),
	availableRelays func() []peer.ID, getPeers func() []*Metadata,
) *Connection {
	log.Debugf("Creating connection with peer %v from inbound stream", stream.Conn().RemotePeer().Pretty())
	connection := new(Connection)
	connection.init(host, stream.Conn().RemotePeer(), protocol, myAddress, onPeerListUpdate, onPublicKeyRecovered, onMessage, availableRelays, getPeers)
	connection.PeerAddress = stream.Conn().RemoteMultiaddr()
	connection.inboundStream = stream
	if signature != nil {
		connection.SetSignature(signature)
	}
	go connection.receiveMessageCycle()
	go connection.sendMessageCycle()
	go connection.sendPeersCycle()
	return connection
}

func (connection *Connection) SetPeerAddress(address ma.Multiaddr) {
	connection.PeerAddress = address
}

func (connection *Connection) IsActive() bool {
	return connection.status.Load() != Terminated && connection.status.Load() != NotConnected &&
		(connection.inboundStream != nil || connection.outboundStream != nil)
}

func (connection *Connection) receiveMessageCycle() {
	for connection.status.Load() != Terminated {
		if connection.inboundStream != nil {
			frame, err := communication.ReadOnce(connection.inboundStream)
			if err != nil {
				receiveErrors.WithLabelValues(err.Error()).Inc()
				log.Errorf("Skipped message from peer %v, resetting connection: %v", connection.PeerId.Pretty(), err)
				connection.resetInboundStream()
				continue
			}
			connection.inboundTPS.AddMeasurement(float64(len(frame.Data())))

			switch frame.Kind() {
			case communication.Message:
				inboundMessages.WithLabelValues("message").Inc()
				connection.onMessage(frame.Data())
				break
			case communication.Signature:
				inboundMessages.WithLabelValues("signature").Inc()
				connection.handleSignature(frame.Data())
				break
			case communication.GetPeersReply:
				inboundMessages.WithLabelValues("peers").Inc()
				connection.handlePeers(frame.Data())
				break
			default:
				log.Errorf("Unknown frame kind received from peer %v: %v", connection.PeerId.Pretty(), frame.Kind())
			}
		} else {
			time.Sleep(time.Second)
		}
	}
	close(connection.lifecycleFinished)
}

func (connection *Connection) sendMessageCycle() {
	lastSuccess := time.Now()
	disconnectThreshold := time.Minute * 2
	openStreamBackoff := time.Second
	sendBackoff := time.Millisecond
	var msgToSend []byte = nil
	attempts := 0

	connection.signatureSent.Store(0)
	for connection.status.Load() != Terminated {
		if err := connection.checkOutboundStream(); err != nil {
			log.Tracef("Can't connect to peer %v, will retry in %v", connection.PeerId, openStreamBackoff)
			time.Sleep(openStreamBackoff)
			if openStreamBackoff < time.Minute {
				openStreamBackoff *= 2
			}
			continue
		} else {
			openStreamBackoff = time.Second
		}

		if len(connection.signature) > 0 && connection.signatureSent.CAS(0, 1) {
			go connection.sendSignature()
		}

		if msgToSend == nil {
			value, err := connection.messageQueue.DequeueOrWait()
			messagesInQueue.WithLabelValues(connection.PeerId.Pretty()).Set(float64(connection.messageQueue.GetLen()))
			if err != nil {
				log.Errorf("Failed to wait for message to send to peer %v: %v", connection.PeerId.Pretty(), err)
			}
			if value == nil {
				log.Tracef("Got terminating message, finishing send cycle for peer %v", connection.PeerId.Pretty())
				break
			}
			msgToSend = value
			lastSuccess = time.Now() // reset last success since we got new message
		}

		attempts += 1
		if connection.outboundStream != nil {
			frame := communication.NewFrame(communication.Message, msgToSend)
			connection.streamLock.Lock()
			err := communication.Write(connection.outboundStream, frame)
			connection.streamLock.Unlock()
			if err == nil {
				resendAttempts.Observe(float64(attempts))
				attempts = 0
				//log.Tracef("Sent message (len = %d bytes) to peer %v", len(frame.Data()), connection.PeerId.Pretty())
				connection.outboundTPS.AddMeasurement(float64(len(frame.Data())))
				sendBackoff = time.Millisecond
				msgToSend = nil
				lastSuccess = time.Now()
				continue
			}
			sendErrors.WithLabelValues(err.Error()).Inc()
			log.Errorf("Error while sending message (len = %d bytes) to peer %v: %v", len(frame.Data()), connection.PeerId.Pretty(), err)
			connection.resetOutboundStream()
		}
		log.Tracef("Outbound stream for peer %v is not ready", connection.PeerId.Pretty())
		if time.Now().Sub(lastSuccess) < disconnectThreshold {
			log.Tracef("Resending message to peer %v in %v (nil stream: %v)", connection.PeerId.Pretty(), sendBackoff, connection.outboundStream == nil)
			time.Sleep(sendBackoff)
			if sendBackoff < time.Second {
				sendBackoff *= 2
			}
		} else {
			droppedMessages.Add(float64(connection.messageQueue.GetLen() + 1))
			log.Warningf("Can't send message to peer %v for more than %v, cleaning messages", connection.PeerId.Pretty(), disconnectThreshold)
			connection.messageQueue.Clear()
			msgToSend = nil
		}
	}
	close(connection.sendCycleFinished)
}

func (connection *Connection) sendPeersCycle() {
	for connection.status.Load() != Terminated {
		connection.sendPeers()
		select {
		case <-connection.peerCycleFinished:
			return
		case <-time.After(time.Minute):
			continue
		}
	}
}

func (connection *Connection) Send(msg []byte) {
	if msg == nil {
		log.Errorf("Got empty message to send to peer %v, ignoring", connection.PeerId.Pretty())
		return
	}

	connection.messageQueue.Enqueue(msg)
	messagesInQueue.WithLabelValues(connection.PeerId.Pretty()).Set(float64(connection.messageQueue.GetLen()))
}

func (connection *Connection) sendSignature() {
	log.Debugf("Sending signature to peer %v", connection.PeerId.Pretty())
	backoff := time.Second
	for connection.status.Load() != Terminated {
		if connection.outboundStream != nil {
			sigLen := 65
			if config.ChainId >= 110 {
				sigLen = 66
			}
			if len(connection.signature) != sigLen {
				panic("bad signature length!")
			}
			var payload []byte
			payload = append(payload, connection.signature...)
			payload = append(payload, connection.myAddress.Bytes()...)

			frame := communication.NewFrame(communication.Signature, payload)
			connection.streamLock.Lock()
			err := communication.Write(connection.outboundStream, frame)
			connection.streamLock.Unlock()
			if err == nil {
				log.Tracef("Sent signature (len = %d bytes) to peer %v", len(frame.Data()), connection.PeerId.Pretty())
				connection.outboundTPS.AddMeasurement(float64(len(frame.Data())))
				backoff = time.Second
				break
			}
			log.Errorf("Error while sending signature (len = %d bytes) to peer %v: %v", len(frame.Data()), connection.PeerId.Pretty(), err)
		}
		log.Tracef("Resending signature to peer %v in %v (nil stream: %v)", connection.PeerId.Pretty(), backoff, connection.outboundStream == nil)
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
	sigLen := 65
	if config.ChainId >= 110 {
		sigLen = 66
	}
	signature, addressBytes := data[:sigLen], data[sigLen:]
	peerIdBytes, err := connection.PeerId.Marshal()
	if err != nil {
		log.Errorf("Cannot create payload for signature check fr7om peer %v: %v", connection.PeerId.Pretty(), err)
		return
	}
	publicKey, err := utils.EcRecover(peerIdBytes, signature, config.ChainId)
	if err != nil {
		log.Errorf("Signature check failed for peer %v, resetting connection: %v", connection.PeerId.Pretty(), err)
		connection.resetInboundStream()
		return
	}
	address, err := ma.NewMultiaddrBytes(addressBytes)
	if err != nil {
		log.Errorf("Peer %v sent incorrect address, resetting connection: %v", connection.PeerId.Pretty(), err)
		connection.resetInboundStream()
		return
	}
	connection.PeerPublicKey = utils.PublicKeyToHexString(publicKey)
	log.Tracef("Recovered public key %v for peer %v from signature", connection.PeerPublicKey, connection.PeerId.Pretty())
	connection.PeerAddress = address
	connection.status.CAS(NotConnected, HandshakeComplete)
	connection.status.CAS(JustConnected, HandshakeComplete)
	connection.onPublicKeyRecovered(connection, connection.PeerPublicKey)
}

func (connection *Connection) sendPeers() {
	if connection.outboundStream != nil {
		peerConnections := connection.getPeers()
		if len(peerConnections) == 0 {
			return
		}
		msg := EncodeArray(peerConnections)
		frame := communication.NewFrame(communication.GetPeersReply, msg)
		connection.streamLock.Lock()
		err := communication.Write(connection.outboundStream, frame)
		connection.streamLock.Unlock()
		if err != nil {
			log.Warningf("Cannot send peer list (len = %d bytes) to peer %v: %v", len(msg), connection.PeerId.Pretty(), err)
		} else {
			log.Tracef("Sent peer list (len = %d bytes) to peer %v", len(msg), connection.PeerId.Pretty())
			connection.outboundTPS.AddMeasurement(float64(len(frame.Data())))
		}
		return
	}
	log.Warningf("Cannot send peer list to peer %v: no connection yet", connection.PeerId.Pretty())
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

func (connection *Connection) SetInboundStream(stream network.Stream) {
	if connection.status.Load() == Terminated {
		return
	}
	log.Tracef("Updating stream for connection with peer %v", stream.Conn().RemotePeer().Pretty())
	connection.streamLock.Lock()
	defer connection.streamLock.Unlock()
	if connection.inboundStream != nil {
		inboundReset.Inc()
		if err := connection.inboundStream.Reset(); err != nil {
			log.Errorf("Failed to reset stream: %v", err)
		}
	}
	connection.inboundStream = stream
	connection.signatureSent.Store(0)
	connection.status.CAS(NotConnected, JustConnected)
	connection.status.CAS(HandshakeComplete, JustConnected)
}

func (connection *Connection) checkOutboundStream() error {
	connection.streamLock.Lock()
	defer connection.streamLock.Unlock()
	state := (*connection.host).Network().Connectedness(connection.PeerId)
	if state != network.Connected {
		log.Debugf("Peer %v lacks connectedness (%v), calling connect", connection.PeerId.Pretty(), state)
		// Since we just tried and failed to dial, the dialer system will, by default
		// prevent us from redialing again so quickly. Since we know what we're doing, we
		// can use this ugly hack (it's on our TODO list to make it a little cleaner)
		// to tell the dialer "no, its okay, let's try this again"
		connection.resetInboundStream()
		connection.resetOutboundStream()
		(*connection.host).Network().(*swarm.Swarm).Backoff().Clear(connection.PeerId)
		err := connection.connect(connection.PeerId, connection.PeerAddress)
		if err != nil {
			return err
		}
		connection.signatureSent.Store(0)
	}
	if connection.outboundStream == nil {
		log.Debugf("Peer %v has no stream, creating one", connection.PeerId.Pretty())
		stream, err := (*connection.host).NewStream(context.Background(), connection.PeerId,
			protocol.ID(connection.PeerProtocol))
		if err != nil {
			return err
		}
		connection.outboundStream = stream
		connection.signatureSent.Store(0)
		connection.status.CAS(NotConnected, JustConnected)
		connection.status.CAS(HandshakeComplete, JustConnected)
	}
	return nil
}

func (connection *Connection) resetInboundStream() {
	if connection.inboundStream == nil {
		return
	}
	inboundReset.Inc()
	log.Debugf("Resetting inbound stream to peer %s", connection.inboundStream.Conn().RemotePeer().Pretty())
	if err := connection.inboundStream.Reset(); err != nil {
		log.Errorf("Failed to reset stream: %v", err)
	}
	connection.inboundStream = nil
}

func (connection *Connection) resetOutboundStream() {
	if connection.outboundStream == nil {
		return
	}
	outboundReset.Inc()
	log.Debugf("Resetting outbound stream to peer %s", connection.outboundStream.Conn().RemotePeer().Pretty())
	if err := connection.outboundStream.Reset(); err != nil {
		log.Errorf("Failed to reset stream: %v", err)
	}
	connection.outboundStream = nil
}

func (connection *Connection) Terminate() {
	connection.status.Store(Terminated)
	connection.messageQueue.Enqueue(nil)
	connection.resetInboundStream()
	connection.resetOutboundStream()
	<-connection.lifecycleFinished
	log.Debugf("Lifecycle finished")
	<-connection.sendCycleFinished
	log.Debugf("SendCycle finished")
	close(connection.peerCycleFinished)
	log.Debugf("PeerCycle finished")
}
