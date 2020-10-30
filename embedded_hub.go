package main

import "C"
import (
	"bytes"
	"encoding/hex"
	"fmt"
	"github.com/juju/loggo"
	"lachain-communication-hub/config"
	server "lachain-communication-hub/grpc"
	"lachain-communication-hub/peer"
)

var localPeer *peer.Peer
var grpcServer *server.Server

var log = loggo.GetLogger("embedded_hub")
var ZeroPub = make([]byte, 33)

//export StartHub
func StartHub(
	grpcAddress *C.char, grpcAddressLen C.int,
	bootstrapAddress *C.char, bootstrapAddressLen C.int,
) {
	config.SetBootstrapAddress(C.GoStringN(bootstrapAddress, bootstrapAddressLen))
	localPeer = peer.New("_h1")
	grpcServer = server.New(C.GoStringN(grpcAddress, grpcAddressLen), localPeer)
	grpcServer.Serve()
}

//export SendMessage
func SendMessage(pubKeyPtr *C.char, pubKeyLen C.int, dataPtr *C.char, dataLen C.int) {
	pubKey := C.GoBytes(pubKeyPtr, pubKeyLen)
	data := C.GoBytes(dataPtr, dataLen)
	log.Tracef("SendMessage command to send %d bytes to %s", dataLen, hex.EncodeToString(pubKey))

	if bytes.Equal(pubKey, ZeroPub) {
		localPeer.BroadcastMessage(data)
	} else {
		pub := hex.EncodeToString(pubKey)
		localPeer.SendMessageToPeer(pub, data, true)
	}
}

//export LogLevel
func LogLevel(s *C.char, len C.int) {
	loggo.ConfigureLoggers(C.GoStringN(s, len))
}

//export StopHub
func StopHub() {
	fmt.Println("Exit received")
	localPeer.Stop()
	grpcServer.Stop()
}

func main() {}
