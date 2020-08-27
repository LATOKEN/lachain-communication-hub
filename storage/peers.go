package storage

import (
	"errors"
	"github.com/libp2p/go-libp2p-core/peer"
	"log"
	"sync"
)

var peers = map[string]string{}

var mutex = &sync.Mutex{}

func GetPeerIdByPublicKey(publicKey string) (peer.ID, error) {
	mutex.Lock()
	defer mutex.Unlock()
	if peerIdStr, ok := peers[publicKey]; ok {
		id, err := peer.Decode(peerIdStr)
		if err != nil {
			return id, err
		}
		return id, nil
	}
	return "", errors.New("not found")

}

func RegisterPeer(publicKey string, peerId string) {
	mutex.Lock()
	defer mutex.Unlock()
	peers[publicKey] = peerId
	log.Printf("peer successfully registered:  %s, %s", publicKey, peerId)
}
