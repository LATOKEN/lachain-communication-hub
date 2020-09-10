package storage

import (
	"bytes"
	"crypto/ecdsa"
	"encoding/hex"
	"errors"
	"github.com/ethereum/go-ethereum/crypto"
	"github.com/juju/loggo"
	"github.com/libp2p/go-libp2p-core/peer"
	"lachain-communication-hub/types"
	"lachain-communication-hub/utils"
	"sync"
	"time"
)

var log = loggo.GetLogger("storage")

type peersWithMutex struct {
	peers []*types.PeerConnection
	mutex *sync.Mutex
}

type validatorsWithMutex struct {
	validators [][]byte
	mutex      *sync.Mutex
}

type currentConnectionsWithMutex struct {
	ids   []peer.ID
	mutex *sync.Mutex
}

var peersMu = peersWithMutex{mutex: &sync.Mutex{}}
var validatorsMu = validatorsWithMutex{mutex: &sync.Mutex{}}
var derectConnectionsMu = currentConnectionsWithMutex{mutex: &sync.Mutex{}}

func GetPeerByPublicKey(publicKey *ecdsa.PublicKey) (*types.PeerConnection, error) {
	peersMu.mutex.Lock()
	defer peersMu.mutex.Unlock()

	for _, peer := range peersMu.peers {
		if peer.PublicKey != nil && bytes.Equal(crypto.CompressPubkey(peer.PublicKey), crypto.CompressPubkey(publicKey)) {
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

func GetRecentPeers() []*types.PeerConnection {
	peersMu.mutex.Lock()
	defer peersMu.mutex.Unlock()

	var result []*types.PeerConnection

	for _, peerConn := range peersMu.peers {
		if peerConn.LastSeen > uint32(time.Now().Add(time.Duration(-3)*time.Hour).Unix()) {
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
			log.Debugf("peer successfully updated:  %s, %s", addingPeer.Id, addingPeer.Addr)
			return
		}
	}

	peersMu.peers = append(peersMu.peers, addingPeer)
	log.Debugf("peer successfully registered:  %s, %s", addingPeer.Id, addingPeer.Addr)
}

func UpdateRegisteredPeerById(id peer.ID) {
	peersMu.mutex.Lock()
	defer peersMu.mutex.Unlock()

	for i, peer := range peersMu.peers {
		if peer.Id == id {
			peersMu.peers[i].LastSeen = uint32(time.Now().Unix())
			log.Debugf("peer successfully updated:  %s, %s", peer.Id, peer.Addr)
			return
		}
	}
}

func UpdateRegisteredPeerByPublicKey(publicKey *ecdsa.PublicKey) {
	peersMu.mutex.Lock()
	defer peersMu.mutex.Unlock()

	for i, peer := range peersMu.peers {
		if peer.PublicKey != nil && bytes.Equal(crypto.CompressPubkey(peer.PublicKey), crypto.CompressPubkey(publicKey)) {
			peersMu.peers[i].LastSeen = uint32(time.Now().Unix())
			log.Debugf("peer successfully updated:  %s, %s, %s", hex.EncodeToString(crypto.CompressPubkey(peer.PublicKey)), peer.Id, peer.Addr)
			return
		}
	}
}

func IsConsensusPeer(publicKey *ecdsa.PublicKey) bool {
	validatorsMu.mutex.Lock()
	defer validatorsMu.mutex.Unlock()

	pubBytes := utils.PublicKeyToBytes(publicKey)

	for _, val := range validatorsMu.validators {
		if bytes.Equal(val, pubBytes) {
			return true
		}
	}
	return false
}

func SetValidators(_validators [][]byte) {
	validatorsMu.mutex.Lock()
	defer validatorsMu.mutex.Unlock()

	validatorsMu.validators = _validators
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

func RemoveConnection(id peer.ID) {
	derectConnectionsMu.mutex.Lock()
	defer derectConnectionsMu.mutex.Unlock()

	for i, connectedId := range derectConnectionsMu.ids {
		if id == connectedId {
			derectConnectionsMu.ids = append(derectConnectionsMu.ids[:i], derectConnectionsMu.ids[i+1:]...)
		}
	}
}
