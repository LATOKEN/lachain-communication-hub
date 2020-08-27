package communication

import (
	"bufio"
	"fmt"
	"io/ioutil"
)

func ReadOnce(rw *bufio.ReadWriter) ([]byte, error) {
	return ioutil.ReadAll(rw)
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
