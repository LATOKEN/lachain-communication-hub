package peer

import (
	"github.com/libp2p/go-libp2p-core/network"
	"io"
	"lachain-communication-hub/communication"
	"log"
	"time"
)

func incomingConnectionEstablishmentHandler(onMsg func(msg []byte)) func(s network.Stream) {
	return func(s network.Stream) {
		go runHubMsgHandler(onMsg, s)
	}
}

func runHubMsgHandler(onMsg func(msg []byte), s network.Stream) {
	for {
		msg, err := communication.ReadOnce(s)
		if err != nil {
			if err == io.EOF {
				log.Println("connection reset")
				time.Sleep(2 * time.Second)
				continue
			}
			log.Println("Can't read message. Closing connection")
			log.Println(err)
			s.Close()
			break
		}
		err = processMessage(onMsg, s, msg)
		if err != nil {
			log.Println("Connection problem")
			s.Close()
			return
		}
	}
}

func processMessage(onMsg func([]byte), s network.Stream, msg []byte) error {
	if len(msg) == 0 {
		return nil
	}
	onMsg(msg)

	log.Println("received msg from peer:", s.Conn().RemotePeer(), "msg:", string(msg))

	switch string(msg) {
	case "ping":
		err := communication.Write(s, []byte("pong"))
		if err != nil {
			return err
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
	return nil
}

func confirmHandle(s network.Stream) {
	// read some to invoke handler // libp2p, wtf??
	data := make([]byte, 1)
	_, err := s.Read(data)
	if err != io.EOF {
		panic(err)
	}
}
