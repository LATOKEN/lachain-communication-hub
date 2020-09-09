package types

import (
	"bytes"
	"crypto/ecdsa"
	"encoding/binary"
	"encoding/gob"
	"github.com/ethereum/go-ethereum/crypto"
	"github.com/libp2p/go-libp2p-core/peer"
	ma "github.com/multiformats/go-multiaddr"
	"lachain-communication-hub/communication"
	"log"
)

type PeerConnection struct {
	PublicKey *ecdsa.PublicKey
	Id        peer.ID
	LastSeen  uint32
	Addr      ma.Multiaddr
}

type PeerConnectionSerializable struct {
	PublicKey []byte
	Id        peer.ID
	LastSeen  uint32
	Addr      ma.Multiaddr
}

func (pc *PeerConnection) toBytes() []byte {
	var buf bytes.Buffer
	enc := gob.NewEncoder(&buf)

	ser := PeerConnectionSerializable{
		PublicKey: crypto.CompressPubkey(pc.PublicKey),
		Id:        pc.Id,
		LastSeen:  pc.LastSeen,
		Addr:      pc.Addr,
	}

	err := enc.Encode(ser)
	if err != nil {
		log.Fatal("encode error:", err)
	}
	return buf.Bytes()
}

func (pc *PeerConnection) Encode() []byte {
	var data = pc.toBytes()
	length := make([]byte, 4)
	binary.LittleEndian.PutUint32(length, uint32(len(data)))
	return append(length, data...)
}

func PeerConnectionFromBytes(raw []byte) *PeerConnection {
	var buf bytes.Buffer
	buf.Write(raw)
	var peerConn PeerConnectionSerializable
	dec := gob.NewDecoder(&buf)
	err := dec.Decode(&peerConn)
	if err != nil {
		log.Fatal("decode error:", err)
	}

	pub, err := crypto.DecompressPubkey(peerConn.PublicKey)
	if err != nil {
		log.Fatal("decompress error:", err)
	}

	return &PeerConnection{
		PublicKey: pub,
		Id:        peerConn.Id,
		LastSeen:  peerConn.LastSeen,
		Addr:      peerConn.Addr,
	}
}

func DecodeArray(raw []byte) []*PeerConnection {
	var result []*PeerConnection
	for cursor := 0; cursor < len(raw); {
		length := communication.ExtractLength(raw[cursor:])
		cursor += 4
		current := raw[cursor : cursor+int(length)]
		result = append(result, PeerConnectionFromBytes(current))
		cursor += int(length)
	}
	return result
}
