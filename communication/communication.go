package communication

import (
	"bufio"
	"fmt"
	"encoding/binary"
)

func EncodeDelimited(msg []byte) []byte {
	newMsg := make([]byte, 4 + len(msg))
	binary.LittleEndian.PutUint32(newMsg, uint32(len(msg)))
	copy(newMsg[4:], msg)
	return newMsg
}

func ReadOnce(rw *bufio.ReadWriter) ([]byte, error) {
	limit := rw.Available()
	msg := make([]byte, limit)

	for {
		n, err := rw.Read(msg)
		if err != nil {
			return nil, err
		}
		if n == 0 {
			continue
		}

		return msg[:n], nil
	}

}

func WriteOnce(rw *bufio.ReadWriter, msg []byte) {
	_, err := rw.Write(msg)
	if err != nil {
		fmt.Println("Error writing to buffer")
		panic(err)
	}
	err = rw.Flush()
	if err != nil {
		fmt.Println("Error flushing buffer")
		panic(err)
	}
}
