package peer

import (
	"bufio"
	"fmt"
	"github.com/libp2p/go-libp2p-core/network"
	"io"
	"lachain-communication-hub/communication"
	"log"
	"time"
)

func incomingConnectionEstablishmentHandler(onMsg func(msg []byte)) func(s network.Stream) {
	return func(s network.Stream) {
		rw := bufio.NewReadWriter(bufio.NewReader(s), bufio.NewWriter(s))
		go runHubMsgHandler(rw, onMsg, s)
	}
}

func runHubMsgHandler(rw *bufio.ReadWriter, onMsg func(msg []byte), s network.Stream) {
	for {
		msg, err := communication.ReadOnce(rw)
		if err != nil {
			if err == io.EOF {
				fmt.Println("connection resetted")
				time.Sleep(2 * time.Second)
				continue
			}
			fmt.Println("Can't read message")
			fmt.Println(err)
			break
		}
		processMessage(onMsg, s, msg)
	}
}

func processMessage(onMsg func([]byte), s network.Stream, msg []byte) {
	if len(msg) == 0 {
		return
	}
	onMsg(msg)

	log.Println("received msg from peer:", s.Conn().RemotePeer(), "msg len:", len(msg))

	switch string(msg) {
	case "ping":
		_, err := s.Write([]byte("pong"))
		if err != nil {
			panic(err)
		}
		break

		//case "pong":
		//	time.Sleep(2 * time.Second)
		//	_, err := s.Write([]byte("ping"))
		//	if err != nil {
		//		panic(err)
		//	}
		//	break
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
