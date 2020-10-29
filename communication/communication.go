package communication

import (
	"bufio"
	"encoding/binary"
	"hash/crc32"

	"github.com/libp2p/go-libp2p-core/network"
)

type MsgIntegrityError struct{}

func (MsgIntegrityError) Error() string {
	return "Message integrity check failed."
}

func EncodeDelimited(msg []byte) []byte {
	buf := make([]byte, 8+len(msg))                            // to avoid reallocation
	binary.LittleEndian.PutUint32(buf[:4], uint32(len(msg)+4)) // len of message + crc32
	copy(buf[4:], msg)
	binary.LittleEndian.PutUint32(buf[4+len(msg):], crc32.ChecksumIEEE(msg))
	return buf
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

	bytesLeft := int(ExtractLength(msg))

	if bytesLeft < 4 { // message size is too small to contain checksum
		err = MsgIntegrityError{}
		return nil, err
	}

	// read the message itself and checksum
	var result []byte

	msg = make([]byte, 4096)
	for bytesLeft > 0 {
		n, err := reader.Read(msg)
		if err != nil {
			return nil, err
		}

		result = append(result, msg[:n]...)
		bytesLeft -= n
	}

	// check the checksum
	checkSum := binary.LittleEndian.Uint32(result[len(result)-4:])
	if checkSum != crc32.ChecksumIEEE(result[:len(result)-4]) { // mismatched checksum
		err = MsgIntegrityError{}
		return nil, err
	}

	return result[:len(result)-4], nil
}

func Write(s network.Stream, msg []byte) error {
	encoded := EncodeDelimited(msg)
	_, err := s.Write(encoded)
	return err
}
