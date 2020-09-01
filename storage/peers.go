package storage

import (
	"errors"
	"github.com/libp2p/go-libp2p-core/peer"
	"github.com/multiformats/go-multiaddr"
	"log"
	"sync"
)

var peerIds = map[string]string{}
var peerPublicAddresses = map[string]multiaddr.Multiaddr{}

var mutex = &sync.Mutex{}

func GetPeerIdByPublicKey(publicKey string) (peer.ID, error) {
	mutex.Lock()
	defer mutex.Unlock()
	if peerIdStr, ok := peerIds[publicKey]; ok {
		id, err := peer.Decode(peerIdStr)
		if err != nil {
			return id, err
		}
		return id, nil
	}
	return "", errors.New("not found")
}

func GetPeerPublicKeyById(peerId peer.ID) string {
	mutex.Lock()
	defer mutex.Unlock()
	return getPubKeyById(peerId)
}

func GetPeerAddrByPublicKey(publicKey string) multiaddr.Multiaddr {
	mutex.Lock()
	defer mutex.Unlock()
	if addr, ok := peerPublicAddresses[publicKey]; ok {
		return addr
	}
	return nil
}

func RegisterPeer(publicKey string, peerId peer.ID, publicAddr multiaddr.Multiaddr) {
	mutex.Lock()
	defer mutex.Unlock()
	peerIds[publicKey] = peerId.Pretty()
	peerPublicAddresses[publicKey] = publicAddr
	log.Printf("peer successfully registered:  %s, %s, %s", publicKey, peerId, publicAddr)
}

func getPubKeyById(peerId peer.ID) string {
	for pub, idStr := range peerIds {
		id, _ := peer.Decode(idStr)
		if id == peerId {
			return pub
		}
	}
	return ""
}
