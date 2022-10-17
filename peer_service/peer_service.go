package peer_service

import (
	"errors"
	"lachain-communication-hub/config"
	"lachain-communication-hub/host"
	"lachain-communication-hub/peer_service/connection"
	"lachain-communication-hub/peer_service/protocols"
	"lachain-communication-hub/utils"
	"strings"
	"sync"

	"github.com/juju/loggo"
	core "github.com/libp2p/go-libp2p-core"
	"github.com/libp2p/go-libp2p-core/crypto"
	"github.com/libp2p/go-libp2p-core/network"
	"github.com/libp2p/go-libp2p-core/peer"
	"github.com/libp2p/go-libp2p-core/protocol"
	ma "github.com/multiformats/go-multiaddr"
)

var log = loggo.GetLogger("peer_service")

type PeerService struct {
	host              core.Host
	myExternalAddress ma.Multiaddr
	connections       map[byte]map[string]*connection.Connection
	messages          map[byte]map[string][][]byte
	mutex             *sync.Mutex
	msgHandler        func([]byte)
	running           int32
	PublicKey         string
	Signature         []byte
	quit              chan struct{}
	networkName		  string
	version           int32
	minPeerVersion	  int32
	protocols		  *protocols.Protocols
}

func New(priv_key crypto.PrivKey, networkName string, version int32, minimalSupportedVersion int32,
	handler func([]byte)) *PeerService {
	localHost := host.BuildNamedHost(priv_key)
	log.Infof("my id: %v", localHost.ID())
	log.Infof("listening on: %v", localHost.Addrs())

	mut := &sync.Mutex{}
	peerService := new(PeerService)
	peerService.protocols = protocols.New(networkName, version, minimalSupportedVersion)
	peerService.host = localHost
	peerService.connections = make(map[byte]map[string]*connection.Connection)
	peerService.messages = make(map[byte]map[string][][]byte)
	allProtocolType := peerService.protocols.GetAllProtocolTypes()
	for _, protocolType := range allProtocolType {
		peerService.connections[protocolType] = make(map[string]*connection.Connection)
		peerService.messages[protocolType] = make(map[string][][]byte)
	}
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
	peerService.protocols.SetStreamHandlerMatch(&peerService.host, peerService.onConnect)

	mAddrs := config.GetBootstrapMultiaddrs()
	for i, bootstrapId := range config.GetBootstrapIDs() {
		peerService.connect(bootstrapId, mAddrs[i], protocols.CommonChannel)
	}
	return peerService
}

func (peerService *PeerService) ConnectPeersToChannel(peers []string, protocolType byte) {
	peerService.lock()
	defer peerService.unlock()

	for _, publicKey := range peers {
		// get connection from common channel
		// all peers should be connected to common channel
		// otherwise we will need bootstrap address of peer id

		con := peerService.connectionByPublicKey(publicKey, protocols.CommonChannel)
		if (con != nil) {
			peerService.connect(con.PeerId, con.PeerAddress, protocolType)
		}
	}
}

func (peerService *PeerService) DisconnectPeersFromChannel(peers []string, protocolType byte) {
	peerService.lock()
	defer peerService.unlock()
	
	for _, publicKey := range peers {
		conn := peerService.connectionByPublicKey(publicKey, protocolType)
		if (conn == nil) {
			continue
		}
		conn.Terminate()
		log.Debugf("Connection terminated %v for protocol %v", conn.PeerId.Pretty(), protocolType)
		delete(peerService.connections[protocolType], conn.PeerId.Pretty())
	}
}

func (peerService *PeerService) DisconnectChannel(protocolType byte) {
	peerService.lock()
	defer peerService.unlock()

	for peerId, conn := range peerService.connections[protocolType] {
		conn.Terminate()
		log.Debugf("Connection terminated %v for protocol %v", peerId, protocolType)
	}
	peerService.connections[protocolType] = make(map[string]*connection.Connection)
}

func (peerService *PeerService) connect(id peer.ID, address ma.Multiaddr, protocolType byte) {
	if id == peerService.host.ID() {
		return
	}
	if _, ok := peerService.connections[protocolType][id.Pretty()]; id == peerService.host.ID() || ok {
		return
	}
	protocolString, err := peerService.protocols.GetProtocol(protocolType)
	if (err != nil) {
		log.Errorf("Cannot connect to peer %v, invalid protocol. How did it happen?", id.Pretty())
		panic(err)
	}
	conn := connection.New(
		&peerService.host, id, protocolString, protocolType, peerService.myExternalAddress, address,  nil,
		peerService.updatePeerList, peerService.onPublicKeyRecovered, peerService.msgHandler,
		peerService.AvailableRelays, peerService.GetPeers,
	)
	log.Tracef("Connected to peer %v with protocol %v", id.Pretty(), protocolType)
	peerService.connections[protocolType][id.Pretty()] = conn
}

func (peerService *PeerService) onConnect(stream network.Stream) {
	peerService.lock()
	defer peerService.unlock()
	if peerService.running == 0 {
		return
	}
	gotProtocol := stream.Protocol()
	strProtocol := protocol.ConvertToStrings([]protocol.ID {gotProtocol})[0]
	
	id := stream.Conn().RemotePeer().Pretty()
	protocolType, err := peerService.protocols.GetProtocolType(strProtocol)
	if (err != nil) {
		log.Debugf("Cannot connect to peer %v, invalid protocol", id)
		return
	}
	log.Tracef("Got incoming stream from %v (%v) with protocol %v", id, stream.Conn().RemoteMultiaddr().String(), protocolType)
	if conn, ok := peerService.connections[protocolType][id]; ok {
		conn.SetInboundStream(stream)
		return
	}
	// TODO: manage peers to preserve important ones & exclude extra
	newConnect := connection.FromStream(
		&peerService.host, stream, peerService.myExternalAddress, peerService.Signature, strProtocol, protocolType,
		peerService.updatePeerList, peerService.onPublicKeyRecovered, peerService.msgHandler,
		peerService.AvailableRelays, peerService.GetPeers,
	)
	peerService.connections[protocolType][id] = newConnect
}

func (peerService *PeerService) onPublicKeyRecovered(conn *connection.Connection, publicKey string) {
	if peerService.running == 0 {
		return
	}
	peerService.lock()
	defer peerService.unlock()
	protocolType, err := peerService.protocols.GetProtocolType(conn.PeerProtocol)
	if (err != nil) {
		log.Errorf("Connection to peer %v with public key %v has invalid protocol. How did it happen?", conn.PeerId.Pretty(), publicKey)
		panic(err)
	}
	log.Debugf(
		"Sending %v postponed messages to peer %v with protocol %v with freshly recovered key %v", 
		len(peerService.messages[protocolType][publicKey]), conn.PeerId.Pretty(), protocolType, publicKey,
	)
	for _, msg := range peerService.messages[protocolType][publicKey] {
		conn.Send(msg)
	}
	peerService.messages[protocolType][publicKey] = nil
}

func (peerService *PeerService) updatePeerList(newPeers []*connection.Metadata, peerId peer.ID) {
	if peerService.running == 0 {
		return
	}
	peerService.lock()
	defer peerService.unlock()
	log.Tracef("Got list of %v potential peers", len(newPeers))
	for _, newPeer := range newPeers {
		if newPeer.Id == peerService.host.ID() {
			continue
		}
		protocolType, err := peerService.protocols.GetProtocolType(newPeer.Protocol)
		if (err != nil) {
			log.Debugf("peer %v sent peer list with unsupported protocol", peerId.Pretty())
			continue
		}
		if conn, ok := peerService.connections[protocolType][newPeer.Id.Pretty()]; ok {
			log.Tracef("Peer %v already has connection", newPeer.Id.Pretty())
			if newPeer.Addr != nil {
				conn.SetPeerAddress(newPeer.Addr)
			}
			continue
		}
		peerService.connections[protocolType][newPeer.Id.Pretty()] = connection.New(
			&peerService.host, newPeer.Id, newPeer.Protocol, protocolType, peerService.myExternalAddress, newPeer.Addr,
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
	for _, connectionByProtocol := range peerService.connections {
		for _, conn := range connectionByProtocol {
			conn.SetSignature(signature)
		}
	}
	return true
}

func (peerService *PeerService) AvailableRelays(protocolType byte, peerId peer.ID) []peer.ID {
	peerService.lock()
	defer peerService.unlock()
	var result []peer.ID
	for _, conn := range peerService.connections[protocolType] {
		if conn.IsActive() && conn.PeerAddress != nil {
			result = append(result, conn.PeerId)
		}
	}
	return result
}

func (peerService *PeerService) GetPeers(protocolType byte) []*connection.Metadata {
	peerService.lock()
	defer peerService.unlock()
	var result []*connection.Metadata
	for _, conn := range peerService.connections[protocolType] {
		if conn.IsActive() {
			result = append(result, &connection.Metadata{
				PublicKey: conn.PeerPublicKey,
				Id:        conn.PeerId,
				LastSeen:  0, // TODO: restore last seen mechanism
				Addr:      conn.PeerAddress,
				Protocol:  conn.PeerProtocol,
			})
		}
	}
	return result
}

func (peerService *PeerService) connectionByPublicKey(publicKey string, protocolType byte) *connection.Connection {
	for _, conn := range peerService.connections[protocolType] {
		if conn.PeerPublicKey == publicKey {
			return conn
		}
	}
	return nil
}

func (peerService *PeerService) SendMessageToPeer(publicKey string, protocolType byte, msg []byte) bool {
	peerService.lock()
	defer peerService.unlock()

	if conn := peerService.connectionByPublicKey(publicKey, protocolType); conn != nil {
		//log.Tracef("Sending message to peer %v message length %d", conn.PeerId.Pretty(), len(msg))
		conn.Send(msg)
		return true
	}
	log.Tracef("Postponed message to peer %v with protocol %v. Message length %d", publicKey, protocolType, len(msg))
	peerService.storeMessage(publicKey, protocolType, msg)
	return false
}

func (peerService *PeerService) BroadcastMessage(protocolType byte, msg []byte) {
	peerService.lock()
	defer peerService.unlock()
	for _, conn := range peerService.connections[protocolType] {
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

func (peerService *PeerService) IsConnected(publicKey string, protocolType byte) bool {
	peerService.lock()
	defer peerService.unlock()
	conn, exist := peerService.connections[protocolType][publicKey]
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
	for protocolType, connectionByProtocol := range peerService.connections {
		for pubKey, conn := range connectionByProtocol {
			conn.Terminate()
			log.Debugf("Connection terminated %v for protocol %v", pubKey, protocolType)
		}
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
	peerService.protocols.RemoveStreamHandler(&peerService.host)
	log.Debugf("Removed Handlers")
	if err := peerService.host.Close(); err != nil {
		panic(err)
	}
	log.Debugf("Closed host")
}

func (peerService *PeerService) storeMessage(key string, protocolType byte, msg []byte) {
	if conn := peerService.connectionByPublicKey(key, protocolType); conn != nil {
		conn.Send(msg)
	} else {
		peerService.messages[protocolType][key] = append(peerService.messages[protocolType][key], msg)
	}
}
