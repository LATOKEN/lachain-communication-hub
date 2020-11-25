package connection

import (
	"bytes"
	"encoding/binary"
	"encoding/gob"
	"github.com/libp2p/go-libp2p-core/peer"
	ma "github.com/multiformats/go-multiaddr"
	"lachain-communication-hub/communication"
	"lachain-communication-hub/utils"
)

type Metadata struct {
	PublicKey string
	Id        peer.ID
	LastSeen  uint32
	Addr      ma.Multiaddr
}

type MetadataSerializable struct {
	PublicKey []byte
	Id        string
	LastSeen  uint32
	Addr      []byte
}

func (pc *Metadata) toBytes() []byte {
	var buf bytes.Buffer
	enc := gob.NewEncoder(&buf)
	var addr []byte
	if pc.Addr == nil {
		addr = []byte{}
	} else {
		addr = pc.Addr.Bytes()
	}

	ser := MetadataSerializable{
		PublicKey: utils.HexToBytes(pc.PublicKey),
		Id:        pc.Id.Pretty(),
		LastSeen:  pc.LastSeen,
		Addr:      addr,
	}

	err := enc.Encode(ser)
	if err != nil {
		panic(err)
	}
	return buf.Bytes()
}

func (pc *Metadata) Encode() []byte {
	var data = pc.toBytes()
	length := make([]byte, 4)
	binary.LittleEndian.PutUint32(length, uint32(len(data)))
	return append(length, data...)
}

func PeerConnectionFromBytes(raw []byte) *Metadata {

	buf := bytes.NewBuffer(raw)
	var peerConn MetadataSerializable
	dec := gob.NewDecoder(buf)
	err := dec.Decode(&peerConn)
	if err != nil {
		panic(err)
	}

	pub := utils.BytesToHex(peerConn.PublicKey)

	id, err := peer.Decode(peerConn.Id)
	if err != nil {
		panic(err)
	}

	var addr ma.Multiaddr
	if len(peerConn.Addr) > 0 {
		addr, err = ma.NewMultiaddrBytes(peerConn.Addr)
		if err != nil {
			panic(err)
		}
	}

	return &Metadata{
		PublicKey: pub,
		Id:        id,
		LastSeen:  peerConn.LastSeen,
		Addr:      addr,
	}
}

func EncodeArray(connections []*Metadata) []byte {
	var result []byte
	for _, conn := range connections {
		result = append(result, conn.Encode()...)
	}
	return result
}

func DecodeArray(raw []byte) []*Metadata {
	var result []*Metadata
	for cursor := 0; cursor < len(raw); {
		length := communication.ExtractLength(raw[cursor:])
		cursor += 4
		current := raw[cursor : cursor+int(length)]
		result = append(result, PeerConnectionFromBytes(current))
		cursor += int(length)
	}
	return result
}
