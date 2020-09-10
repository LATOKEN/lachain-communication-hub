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
	Id        string
	LastSeen  uint32
	Addr      []byte
}

func (pc *PeerConnection) toBytes() []byte {
	var buf bytes.Buffer
	enc := gob.NewEncoder(&buf)
	var addr []byte
	if pc.Addr == nil {
		addr = []byte{}
	} else {
		addr = pc.Addr.Bytes()
	}
	ser := PeerConnectionSerializable{
		PublicKey: crypto.CompressPubkey(pc.PublicKey),
		Id:        pc.Id.Pretty(),
		LastSeen:  pc.LastSeen,
		Addr:      addr,
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

	buf := bytes.NewBuffer(raw)
	var peerConn PeerConnectionSerializable
	dec := gob.NewDecoder(buf)
	err := dec.Decode(&peerConn)
	if err != nil {
		log.Fatal("decode error:", err)
	}

	pub, err := crypto.DecompressPubkey(peerConn.PublicKey)
	if err != nil {
		log.Fatal("decompress pub error:", err)
	}

	id, err := peer.Decode(peerConn.Id)
	if err != nil {
		log.Fatal("decompress id error:", err)
	}

	var addr ma.Multiaddr
	if len(peerConn.Addr) > 0 {
		addr, err = ma.NewMultiaddrBytes(peerConn.Addr)
		if err != nil {
			log.Fatal("decompress addr error:", err)
		}
	}

	return &PeerConnection{
		PublicKey: pub,
		Id:        id,
		LastSeen:  peerConn.LastSeen,
		Addr:      addr,
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
