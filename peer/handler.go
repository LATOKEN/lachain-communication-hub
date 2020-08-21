package peer

import (
	"bufio"
	"fmt"
	"github.com/libp2p/go-libp2p-core/network"
	"io"
	"lachain-communication-hub/communication"
)

func handleHubMsg(s network.Stream) {
	defer s.Close()
	rw := bufio.NewReadWriter(bufio.NewReader(s), bufio.NewWriter(s))

	msg, err := communication.ReadOnce(rw)
	if err != nil {
		fmt.Println("Can't read message")
		fmt.Println(err)
	}

	fmt.Println("received msg from peer:", s.Conn().RemotePeer())
	fmt.Println("msg:", string(msg))

	switch string(msg) {
	case "ping":
		_, err := s.Write([]byte("pong"))
		if err != nil {
			panic(err)
		}
	}
}

func confirmHandle(s network.Stream) {
	// read some to invoke handler // libp2p, wtf??
	data := make([]byte, 1)
	_, err := s.Read(data)
	if err != io.EOF {
		panic(err)
	}
}
