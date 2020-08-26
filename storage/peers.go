package storage

import (
	"errors"
	"github.com/libp2p/go-libp2p-core/peer"
)

var peers = map[string]string{}

func GetPeerIdByPublicKey(publicKey string) (peer.ID, error) {
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
	peers[publicKey] = peerId
}
