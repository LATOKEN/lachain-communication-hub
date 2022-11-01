package communication

import (
	"bufio"
	"encoding/binary"
	"hash/crc32"
	"io"

	"github.com/libp2p/go-libp2p-core/network"
)

type MsgIntegrityError struct{}

func (MsgIntegrityError) Error() string {
	return "Message integrity check failed."
}

type FrameKind byte

const (
	Message         = 0
	Signature       = 1
	GetPeersReply   = 2
)

type MessageFrame struct {
	kind FrameKind
	data []byte
}

func NewFrame(kind FrameKind, data []byte) MessageFrame {
	return MessageFrame{
		kind: kind,
		data: data,
	}
}

func (frame *MessageFrame) Encode() []byte {
	buf := make([]byte, 9+len(frame.data))                            // to avoid reallocation
	binary.LittleEndian.PutUint32(buf[:4], uint32(len(frame.data)+5)) // len of message + crc32
	buf[4] = byte(frame.kind)
	copy(buf[5:], frame.data)
	binary.LittleEndian.PutUint32(buf[5+len(frame.data):], crc32.ChecksumIEEE(frame.data))
	return buf
}

func (frame *MessageFrame) Data() []byte {
	return frame.data
}

func (frame *MessageFrame) Kind() FrameKind {
	return frame.kind
}

func ExtractLength(msg []byte) uint32 {
	length := binary.LittleEndian.Uint32(msg[:4])
	return length
}

func ReadOnce(stream network.Stream) (MessageFrame, error) {
	reader := bufio.NewReader(stream)
	return ReadFromReader(reader)
}

func ReadFromReader(reader *bufio.Reader) (MessageFrame, error) {
	msg := make([]byte, 4)

	_, err := io.ReadFull(reader, msg)
	if err != nil {
		return MessageFrame{}, err
	}

	bytesLeft := int(ExtractLength(msg))

	if bytesLeft < 4 { // message size is too small to contain checksum
		err = MsgIntegrityError{}
		return MessageFrame{}, err
	}
	result := make([]byte, bytesLeft)
	_, err = io.ReadFull(reader, result)
	if err != nil {
		return MessageFrame{}, err
	}

	// check the checksum
	checkSum := binary.LittleEndian.Uint32(result[len(result)-4:])
	if checkSum != crc32.ChecksumIEEE(result[1:len(result)-4]) { // mismatched checksum
		return MessageFrame{}, MsgIntegrityError{}
	}

	return MessageFrame{
		kind: FrameKind(result[0]),
		data: result[1:len(result)-4],
	}, nil
}

func Write(s network.Stream, frame MessageFrame) error {
	_, err := s.Write(frame.Encode())
	return err
}
