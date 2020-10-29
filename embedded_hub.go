package main

import "C"
import (
	"bytes"
	"encoding/hex"
	"fmt"
	"lachain-communication-hub/config"
	server "lachain-communication-hub/grpc"
	"lachain-communication-hub/host"
	"lachain-communication-hub/peer"
	"unsafe"

	"github.com/juju/loggo"
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
	priv_key := host.GetPrivateKeyForHost("_h1")
	localPeer = peer.New(priv_key)
	grpcServer = server.New(C.GoStringN(grpcAddress, grpcAddressLen), localPeer)
	grpcServer.Serve()
}

//export SendMessage
func SendMessage(pubKeyPtr unsafe.Pointer, pubKeyLen C.int, dataPtr unsafe.Pointer, dataLen C.int) {
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
