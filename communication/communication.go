package communication

import (
	"bufio"
	"encoding/binary"
	"github.com/libp2p/go-libp2p-core/network"
)

func EncodeDelimited(msg []byte) []byte {
	encoded := make([]byte, 4)
	binary.LittleEndian.PutUint32(encoded, uint32(len(msg)))
	encoded = append(encoded, msg...)
	return encoded
}

func ExtractLength(msg []byte) uint32 {
	length := binary.LittleEndian.Uint32(msg[:4])
	return length
}

func ReadOnce(stream network.Stream) ([]byte, error) {
	reader := bufio.NewReader(stream)
	return ReadFromReader(reader)
}

func ReadFromReader(reader *bufio.Reader) ([]byte, error) {
	msg := make([]byte, 4)

	_, err := reader.Read(msg)
	if err != nil {
		return nil, err
	}

	var result []byte

	bytesLeft := int(ExtractLength(msg))

	for bytesLeft > 0 {
		var l int
		if l = bytesLeft; bytesLeft > 4096 {
			if l = 4096; len(result) == 0 {
				l = 4092
			}
		}

		msg = make([]byte, l)
		n, err := reader.Read(msg)
		if err != nil {
			return nil, err
		}

		result = append(result, msg[:n]...)
		bytesLeft -= n
	}

	return result, nil
}

func Write(s network.Stream, msg []byte) error {
	encoded := EncodeDelimited(msg)
	_, err := s.Write(encoded)
	return err
}
