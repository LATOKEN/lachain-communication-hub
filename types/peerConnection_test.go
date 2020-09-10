package types

import (
	"fmt"
	"github.com/ethereum/go-ethereum/crypto"
	"github.com/libp2p/go-libp2p-core/peer"
	"github.com/magiconair/properties/assert"
	"testing"
)

func TestEncodeDecode(t *testing.T) {
	prv, err := crypto.GenerateKey()
	if err != nil {
		fmt.Errorf("could not GenerateKey: %v", err)
	}

	peerId, err := peer.Decode("QmaAV3KD9vWhDfrWutZGXy8hMoVU2FtCMirPEPpUPHszAZ")
	if err != nil {
		fmt.Errorf("could not parsae id: %v", err)
	}

	pc := PeerConnection{
		PublicKey: &prv.PublicKey,
		Id:        peerId,
		LastSeen:  uint32(23),
		Addr:      nil,
	}

	pcBytes := pc.toBytes()

	pc2 := PeerConnectionFromBytes(pcBytes)

	assert.Equal(t, pc.toBytes(), pc2.toBytes())
}

func TestEncodeDecodeArray(t *testing.T) {
	prv, err := crypto.GenerateKey()
	if err != nil {
		fmt.Errorf("could not GenerateKey: %v", err)
	}

	peerId, err := peer.Decode("QmaAV3KD9vWhDfrWutZGXy8hMoVU2FtCMirPEPpUPHszAZ")
	if err != nil {
		fmt.Errorf("could not parsae id: %v", err)
	}

	pc := PeerConnection{
		PublicKey: &prv.PublicKey,
		Id:        peerId,
		LastSeen:  uint32(23),
		Addr:      nil,
	}

	pcBytes := pc.Encode()

	pc2Bytes := pc.Encode()

	results := DecodeArray(append(pcBytes, pc2Bytes...))

	assert.Equal(t, len(results), 2)
	assert.Equal(t, results[0].toBytes(), pc.toBytes())
	assert.Equal(t, results[1].toBytes(), pc.toBytes())
}
