package communication

import (
	"bufio"
	"fmt"
)

func ReadOnce(rw *bufio.ReadWriter) ([]byte, error) {
	ln := rw.Available()
	msg := make([]byte, ln)

	n, err := rw.Read(msg)
	if err != nil {
		return nil, err
	}

	return msg[0:n], nil
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
