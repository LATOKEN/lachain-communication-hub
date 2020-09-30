package storage

import (
	"errors"
	"github.com/juju/loggo"
	"github.com/libp2p/go-libp2p-core/peer"
	"lachain-communication-hub/types"
	"sync"
	"time"
)

var log = loggo.GetLogger("storage")

const messageExpirationTime = 30 * 60

type peersWithMutex struct {
	peers []*types.PeerConnection
	mutex *sync.Mutex
}

type connectedPublicKeysWithMutex struct {
	connected map[string]bool
	mutex     *sync.Mutex
}

type connectedPeersWithMutex struct {
	connected map[string]bool
	mutex     *sync.Mutex
}

type postponedMessagesWithMutex struct {
	messages   map[string][][]byte
	lastUpdate map[string]int64
	mutex      *sync.Mutex
}

type currentConnectionsWithMutex struct {
	ids   []peer.ID
	mutex *sync.Mutex
}

var peersMu = peersWithMutex{mutex: &sync.Mutex{}}

var connectedPublicKeysMu = connectedPublicKeysWithMutex{
	connected: make(map[string]bool),
	mutex:     &sync.Mutex{},
}

var connectedPeersIdsMu = connectedPeersWithMutex{
	connected: make(map[string]bool),
	mutex:     &sync.Mutex{},
}

var derectConnectionsMu = currentConnectionsWithMutex{
	mutex: &sync.Mutex{},
}

var postponedMessagesMu = postponedMessagesWithMutex{
	messages:   make(map[string][][]byte),
	lastUpdate: make(map[string]int64),
	mutex:      &sync.Mutex{},
}

func GetPeerByPublicKey(publicKey string) (*types.PeerConnection, error) {
	peersMu.mutex.Lock()
	defer peersMu.mutex.Unlock()

	for _, peer := range peersMu.peers {
		if publicKey == peer.PublicKey {
			return peer, nil
		}
	}
	return nil, errors.New("GetPeerByPublicKey: not found")
}

func GetAllPeers() []*types.PeerConnection {
	peersMu.mutex.Lock()
	defer peersMu.mutex.Unlock()

	return peersMu.peers
}

func StoreMessageToSendOnConnect(publicKey string, msg []byte) {
	postponedMessagesMu.mutex.Lock()
	defer postponedMessagesMu.mutex.Unlock()
	postponedMessagesMu.messages[publicKey] = append(postponedMessagesMu.messages[publicKey], msg)
	postponedMessagesMu.lastUpdate[publicKey] = time.Now().Unix()
	log.Tracef("Saved postponed msg for %s. Total msg count: %v", publicKey, len(postponedMessagesMu.messages[publicKey]))
}

func GetPostponedMessages(publicKey string) [][]byte {
	postponedMessagesMu.mutex.Lock()
	defer postponedMessagesMu.mutex.Unlock()
	return postponedMessagesMu.messages[publicKey]
}

func RemoveOldPostponedMessages() {
	postponedMessagesMu.mutex.Lock()
	defer postponedMessagesMu.mutex.Unlock()
	now := time.Now().Unix()
	msgCount := 0
	for pkHex, timeUpd := range postponedMessagesMu.lastUpdate {
		if now-timeUpd > messageExpirationTime {
			msgCount += len(postponedMessagesMu.messages)
			delete(postponedMessagesMu.messages, pkHex)
			delete(postponedMessagesMu.lastUpdate, pkHex)
		}
	}
	log.Debugf("Removed %v old postponed messages", msgCount)
}

func ClearPostponedMessages(publicKey string) {
	postponedMessagesMu.mutex.Lock()
	defer postponedMessagesMu.mutex.Unlock()
	pkHex := publicKey
	msgCount := len(postponedMessagesMu.messages[pkHex])
	delete(postponedMessagesMu.messages, pkHex)
	delete(postponedMessagesMu.lastUpdate, pkHex)
	log.Debugf("Cleared %v postponed messages for %s", msgCount, publicKey)
}

func GetRecentPeers() []*types.PeerConnection {
	peersMu.mutex.Lock()
	defer peersMu.mutex.Unlock()

	var result []*types.PeerConnection

	for _, peerConn := range peersMu.peers {
		if peerConn.LastSeen > uint32(time.Now().Add(time.Duration(-2)*time.Minute).Unix()) {
			result = append(result, peerConn)
		}
	}

	return result
}

func GetPeerById(peerId peer.ID) (*types.PeerConnection, error) {
	peersMu.mutex.Lock()
	defer peersMu.mutex.Unlock()

	for _, peer := range peersMu.peers {
		if peer.Id == peerId {
			return peer, nil
		}
	}
	return nil, errors.New("GetPeerById: not found")
}

func RegisterOrUpdatePeer(addingPeer *types.PeerConnection) {
	peersMu.mutex.Lock()
	defer peersMu.mutex.Unlock()

	for i, peer := range peersMu.peers {
		if peer.Id == addingPeer.Id {
			peersMu.peers[i] = addingPeer
			//log.Debugf("peer successfully updated:  %s, %s", addingPeer.Id, addingPeer.Addr)
			return
		}
	}

	peersMu.peers = append(peersMu.peers, addingPeer)
	//log.Debugf("peer successfully registered:  %s, %s", addingPeer.Id, addingPeer.Addr)
}

func UpdateRegisteredPeerById(id peer.ID) {
	peersMu.mutex.Lock()
	defer peersMu.mutex.Unlock()

	for i, peer := range peersMu.peers {
		if peer.Id == id {
			peersMu.peers[i].LastSeen = uint32(time.Now().Unix())
			//log.Debugf("peer successfully updated:  %s, %s", peer.Id, peer.Addr)
			return
		}
	}
}

func UpdateRegisteredPeerByPublicKey(publicKey string) {
	peersMu.mutex.Lock()
	defer peersMu.mutex.Unlock()

	for i, peer := range peersMu.peers {
		if peer.PublicKey == publicKey {
			peersMu.peers[i].LastSeen = uint32(time.Now().Unix())
			//log.Debugf("peer successfully updated:  %s, %s, %s", hex.EncodeToString(crypto.CompressPubkey(peer.PublicKey)), peer.Id, peer.Addr)
			return
		}
	}
}

func AddConnection(id peer.ID) {
	derectConnectionsMu.mutex.Lock()
	defer derectConnectionsMu.mutex.Unlock()

	for _, connectedId := range derectConnectionsMu.ids {
		if id == connectedId {
			return
		}
	}

	derectConnectionsMu.ids = append(derectConnectionsMu.ids, id)
}

func IsDirectlyConnected(id peer.ID) bool {
	derectConnectionsMu.mutex.Lock()
	defer derectConnectionsMu.mutex.Unlock()

	for _, connectedId := range derectConnectionsMu.ids {
		if id == connectedId {
			return true
		}
	}

	return false
}

func GetDirectlyConnectedPeerIds() []peer.ID {
	derectConnectionsMu.mutex.Lock()
	defer derectConnectionsMu.mutex.Unlock()

	return derectConnectionsMu.ids
}

func SetPublicKeyConnected(publicKey string, flag bool) {
	connectedPublicKeysMu.mutex.Lock()
	defer connectedPublicKeysMu.mutex.Unlock()

	connectedPublicKeysMu.connected[publicKey] = flag
}

func IsPublicKeyConnected(publicKey string) bool {
	connectedPublicKeysMu.mutex.Lock()
	defer connectedPublicKeysMu.mutex.Unlock()

	return connectedPublicKeysMu.connected[publicKey]
}

func SetPeerIdConnected(id string, flag bool) {
	connectedPeersIdsMu.mutex.Lock()
	defer connectedPeersIdsMu.mutex.Unlock()

	connectedPeersIdsMu.connected[id] = flag
}

func IsPeerIdConnected(id string) bool {
	connectedPeersIdsMu.mutex.Lock()
	defer connectedPeersIdsMu.mutex.Unlock()

	return connectedPeersIdsMu.connected[id]
}

func RemoveConnection(id peer.ID) {
	derectConnectionsMu.mutex.Lock()
	defer derectConnectionsMu.mutex.Unlock()

	for i, connectedId := range derectConnectionsMu.ids {
		if id == connectedId {
			derectConnectionsMu.ids = append(derectConnectionsMu.ids[:i], derectConnectionsMu.ids[i+1:]...)
		}
	}
}
