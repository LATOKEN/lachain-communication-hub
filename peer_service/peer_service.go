package peer_service

import (
	"errors"
	"fmt"
	"github.com/juju/loggo"
	core "github.com/libp2p/go-libp2p-core"
	"github.com/libp2p/go-libp2p-core/crypto"
	"github.com/libp2p/go-libp2p-core/network"
	"github.com/libp2p/go-libp2p-core/peer"
	"github.com/libp2p/go-libp2p-core/protocol"
	ma "github.com/multiformats/go-multiaddr"
	"lachain-communication-hub/config"
	"lachain-communication-hub/host"
	"lachain-communication-hub/peer_service/connection"
	"lachain-communication-hub/utils"
	"strings"
	"sync"
)

var log = loggo.GetLogger("peer_service")
var protocolFormat = "%s %d"

type PeerService struct {
	host              core.Host
	myExternalAddress ma.Multiaddr
	connections       map[string]*connection.Connection
	messages          map[string][][]byte
	mutex             *sync.Mutex
	msgHandler        func([]byte)
	running           int32
	PublicKey         string
	Signature         []byte
	quit              chan struct{}
	networkName		  string
	version           int32
	minPeerVersion	  int32
}

func New(priv_key crypto.PrivKey, networkName string, version int32, minimalSupportedVersion int32,
	handler func([]byte)) *PeerService {
	localHost := host.BuildNamedHost(priv_key)
	log.Infof("my id: %v", localHost.ID())
	log.Infof("listening on: %v", localHost.Addrs())

	mut := &sync.Mutex{}
	peerService := new(PeerService)
	peerService.host = localHost
	peerService.connections = make(map[string]*connection.Connection)
	peerService.messages = make(map[string][][]byte)
	peerService.mutex = mut
	peerService.running = 1
	peerService.quit = make(chan struct{})
	peerService.msgHandler = handler
	peerService.Signature = nil
	externalAddress, err := peerService.GetExternalMultiAddress()
	if err != nil {
		log.Errorf("Cannot determine my external address: %v", err)
		panic(err)
	}
	peerService.myExternalAddress = externalAddress
	peerService.networkName = networkName
	peerService.version = version
	peerService.minPeerVersion = minimalSupportedVersion
	protocolString := fmt.Sprintf(protocolFormat, peerService.networkName, peerService.version)
	peerService.host.SetStreamHandlerMatch(protocol.ID(protocolString), peerService.networkMatcher, peerService.onConnect)

	mAddrs := config.GetBootstrapMultiaddrs()
	for i, bootstrapId := range config.GetBootstrapIDs() {
		peerService.connect(bootstrapId, mAddrs[i])
	}
	return peerService
}

func (peerService *PeerService) networkMatcher(protocol string) bool {
	var network string
	var version int32
	_, err := fmt.Sscanf(protocol, protocolFormat, &network, &version)
	if err != nil {
		return false
	}
	if network != peerService.networkName {
		return false
	}
	if version < peerService.minPeerVersion {
		return false
	}
	return true
}

func (peerService *PeerService) connect(id peer.ID, address ma.Multiaddr) {
	if id == peerService.host.ID() {
		return
	}
	if _, ok := peerService.connections[id.Pretty()]; id == peerService.host.ID() || ok {
		return
	}
	protocolString := fmt.Sprintf(protocolFormat, peerService.networkName, peerService.version)
	conn := connection.New(
		&peerService.host, id, protocolString, peerService.myExternalAddress, address,  nil,
		peerService.updatePeerList, peerService.onPublicKeyRecovered, peerService.msgHandler,
		peerService.AvailableRelays, peerService.GetPeers,
	)
	peerService.connections[id.Pretty()] = conn
}

func (peerService *PeerService) onConnect(stream network.Stream) {
	peerService.lock()
	defer peerService.unlock()
	if peerService.running == 0 {
		return
	}
	id := stream.Conn().RemotePeer().Pretty()
	log.Tracef("Got incoming stream from %v (%v)", id, stream.Conn().RemoteMultiaddr().String())
	if conn, ok := peerService.connections[id]; ok {
		conn.SetInboundStream(stream)
		return
	}
	// TODO: manage peers to preserve important ones & exclude extra
	protocolString := fmt.Sprintf(protocolFormat, peerService.networkName, peerService.version)
	newConnect := connection.FromStream(
		&peerService.host, stream, peerService.myExternalAddress, peerService.Signature, protocolString,
		peerService.updatePeerList, peerService.onPublicKeyRecovered, peerService.msgHandler,
		peerService.AvailableRelays, peerService.GetPeers,
	)
	peerService.connections[id] = newConnect
}

func (peerService *PeerService) onPublicKeyRecovered(conn *connection.Connection, publicKey string) {
	if peerService.running == 0 {
		return
	}
	peerService.lock()
	defer peerService.unlock()
	log.Debugf("Sending %v postponed messages to peer %v with freshly recovered key %v", len(peerService.messages[publicKey]), conn.PeerId.Pretty(), publicKey)
	for _, msg := range peerService.messages[publicKey] {
		conn.Send(msg)
	}
	peerService.messages[publicKey] = nil
}

func (peerService *PeerService) updatePeerList(newPeers []*connection.Metadata) {
	if peerService.running == 0 {
		return
	}
	peerService.lock()
	defer peerService.unlock()
	log.Tracef("Got list of %v potential peers", len(newPeers))
	protocolString := fmt.Sprintf(protocolFormat, peerService.networkName, peerService.version)
	for _, newPeer := range newPeers {
		if newPeer.Id == peerService.host.ID() {
			continue
		}
		if conn, ok := peerService.connections[newPeer.Id.Pretty()]; ok {
			log.Tracef("Peer %v already has connection", newPeer.Id.Pretty())
			if newPeer.Addr != nil {
				conn.SetPeerAddress(newPeer.Addr)
			}
			continue
		}
		peerService.connections[newPeer.Id.Pretty()] = connection.New(
			&peerService.host, newPeer.Id, protocolString, peerService.myExternalAddress, newPeer.Addr,
			peerService.Signature,
			peerService.updatePeerList, peerService.onPublicKeyRecovered, peerService.msgHandler,
			peerService.AvailableRelays, peerService.GetPeers,
		)
	}
}

func (peerService *PeerService) SetSignature(signature []byte) bool {
	peerService.lock()
	defer peerService.unlock()
	peerId, err := peerService.host.ID().Marshal()
	if err != nil {
		log.Errorf("SetSignature: can't form data for signature check: %v", err)
		return false
	}
	localPublicKey, err := utils.EcRecover(peerId, signature, config.ChainId)
	if err != nil {
		log.Errorf("%v", err)
		return false
	}
	log.Debugf("Recovered public key from outside: %v", utils.PublicKeyToHexString(localPublicKey))
	peerService.Signature = signature
	for _, conn := range peerService.connections {
		conn.SetSignature(signature)
	}
	return true
}

func (peerService *PeerService) AvailableRelays() []peer.ID {
	peerService.lock()
	defer peerService.unlock()
	var result []peer.ID
	for _, conn := range peerService.connections {
		if conn.IsActive() && conn.PeerAddress != nil {
			result = append(result, conn.PeerId)
		}
	}
	return result
}

func (peerService *PeerService) GetPeers() []*connection.Metadata {
	peerService.lock()
	defer peerService.unlock()
	var result []*connection.Metadata
	for _, conn := range peerService.connections {
		if conn.IsActive() {
			result = append(result, &connection.Metadata{
				PublicKey: conn.PeerPublicKey,
				Id:        conn.PeerId,
				LastSeen:  0, // TODO: restore last seen mechanism
				Addr:      conn.PeerAddress,
			})
		}
	}
	return result
}

func (peerService *PeerService) connectionByPublicKey(publicKey string) *connection.Connection {
	for _, conn := range peerService.connections {
		if conn.PeerPublicKey == publicKey {
			return conn
		}
	}
	return nil
}

func (peerService *PeerService) SendMessageToPeer(publicKey string, msg []byte) bool {
	peerService.lock()
	defer peerService.unlock()

	if conn := peerService.connectionByPublicKey(publicKey); conn != nil {
		//log.Tracef("Sending message to peer %v message length %d", conn.PeerId.Pretty(), len(msg))
		conn.Send(msg)
		return true
	}
	log.Tracef("Postponed message to peer %v message length %d", publicKey, len(msg))
	peerService.storeMessage(publicKey, msg)
	return false
}

func (peerService *PeerService) BroadcastMessage(msg []byte) {
	peerService.lock()
	defer peerService.unlock()
	for _, conn := range peerService.connections {
		if !conn.IsActive() && len(conn.PeerPublicKey) > 0 {
			continue
		}
		log.Tracef("Broadcasting to active peer %v (%v)", conn.PeerPublicKey, conn.PeerId.Pretty())
		conn.Send(msg)
	}
}

func (peerService *PeerService) GetId() []byte {
	if peerService.host == nil {
		return nil
	}
	id, err := peerService.host.ID().Marshal()
	if err != nil {
		return nil
	}
	return id
}

func (peerService *PeerService) GetExternalMultiAddress() (ma.Multiaddr, error) {
	extIp := config.GetP2PExternalIP()
	if extIp == "" {
		return nil, errors.New("GetExternalMultiAddress: external IP cannot be determined")
	}

	for _, addr := range peerService.host.Addrs() {
		if strings.Contains(addr.String(), extIp) {
			return addr, nil
		}
	}
	if len(peerService.host.Addrs()) > 0 {
		return peerService.host.Addrs()[0], nil
	}
	return nil, errors.New("no_external_multiaddr")
}

func (peerService *PeerService) IsConnected(publicKey string) bool {
	peerService.lock()
	defer peerService.unlock()
	conn, exist := peerService.connections[publicKey]
	return exist && conn.IsActive()
}

func (peerService *PeerService) lock() {
	peerService.mutex.Lock()
}

func (peerService *PeerService) unlock() {
	peerService.mutex.Unlock()
}

func (peerService *PeerService) Stop() {
	log.Debugf("Stop signal received")
	peerService.lock()
	defer peerService.unlock()
	if peerService.running == 0 {
		return
	}
	close(peerService.quit)
	peerService.running = 0
	for pubKey, conn := range peerService.connections {
		conn.Terminate()
		log.Debugf("Connection terminated %v", pubKey)
	}
	peerService.connections = nil
	if err := peerService.host.ConnManager().Close(); err != nil {
		panic(err)
	}
	log.Debugf("Closed ConnManager")
	if err := peerService.host.Network().Close(); err != nil {
		panic(err)
	}
	log.Debugf("Closed Network")
	if err := peerService.host.Peerstore().Close(); err != nil {
		panic(err)
	}
	log.Debugf("Closed Peerstore")
	protocolString := fmt.Sprintf(protocolFormat, peerService.networkName, peerService.version)
	peerService.host.RemoveStreamHandler(protocol.ID(protocolString))
	log.Debugf("Removed Handlers")
	if err := peerService.host.Close(); err != nil {
		panic(err)
	}
	log.Debugf("Closed host")
}

func (peerService *PeerService) storeMessage(key string, msg []byte) {
	if conn := peerService.connectionByPublicKey(key); conn != nil {
		conn.Send(msg)
	} else {
		peerService.messages[key] = append(peerService.messages[key], msg)
	}
}
