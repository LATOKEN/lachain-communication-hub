package communication

import (
	"bufio"
	"encoding/binary"
	"hash/crc32"
	"io"
	"lachain-communication-hub/utils"

	"github.com/libp2p/go-libp2p-core/network"
)

type MsgIntegrityError struct{}

func (MsgIntegrityError) Error() string {
	return "Message integrity check failed."
}

type FrameKind byte

const (
	Message         		= 0
	Signature       		= 1
	GetPeersReply   		= 2
	MessageConfirmRequest	= 3
	ConfirmReply			= 4
)

type MessageFrame struct {
	kind	FrameKind
	msgId	uint64
	data	[]byte
}

func NewFrame(kind FrameKind, data []byte) MessageFrame {
	return MessageFrame{
		kind: kind,
		msgId: utils.GetRandomUInt64(),
		data: data,
	}
}

func NewFrameWithId(kind FrameKind, data []byte, msgId uint64) MessageFrame {
	return MessageFrame{
		kind: kind,
		msgId: msgId,
		data: data,
	}
}

func (frame *MessageFrame) Encode() []byte {
	kindLen := 1
	msgIdLen := 8
	msgLength := kindLen + msgIdLen + len(frame.data) + 4	// kind + msgId + data + crc32
	buf := make([]byte, 4 + msgLength)	// to avoid reallocation, 4 extra bytes to put the length of the whole msg
	offset := 0
	binary.LittleEndian.PutUint32(buf[offset:offset+4], uint32(msgLength))
	offset += 4
	buf[offset] = byte(frame.kind)
	offset += kindLen
	binary.LittleEndian.PutUint64(buf[offset:offset+msgLength], frame.msgId)
	offset += msgIdLen
	copy(buf[offset:], frame.data)
	offset += len(frame.data)
	// creating checksum with msgId will prevent others to spam with different msgId, because it is costly to compute the checksum
	checksum := crc32.ChecksumIEEE(buf[offset-len(frame.data)-msgIdLen:offset])
	binary.LittleEndian.PutUint32(buf[offset:], checksum)
	return buf
}

func (frame *MessageFrame) Data() []byte {
	return frame.data
}

func (frame *MessageFrame) Kind() FrameKind {
	return frame.kind
}

func (frame *MessageFrame) MsgId() uint64 {
	return frame.msgId
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
	kindLen := 1
	msgIdLen := 8
	checkSum := binary.LittleEndian.Uint32(result[len(result)-4:])
	if checkSum != crc32.ChecksumIEEE(result[kindLen:len(result)-4]) { // mismatched checksum
		return MessageFrame{}, MsgIntegrityError{}
	}

	return MessageFrame{
		kind: FrameKind(result[0]),
		msgId: binary.LittleEndian.Uint64(result[kindLen:kindLen+msgIdLen]),
		data: result[kindLen+msgIdLen:len(result)-4],
	}, nil
}

func Write(s network.Stream, frame MessageFrame) error {
	_, err := s.Write(frame.Encode())
	return err
}
