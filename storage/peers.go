package storage

import (
	"errors"
	"github.com/libp2p/go-libp2p-core/peer"
)

var peers = map[string]string{
	"0xB8CD3195faf7da8a87A2816B9b4bBA2A19D25dAb": "Qmbr1fBbXotXMwneykKpijK4ddvir1Zj2Zck3RiimSSfsD",
}

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
