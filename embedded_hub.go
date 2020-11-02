package main

//#include <string.h>
import "C"
import (
	"bytes"
	"encoding/hex"
	"fmt"
	"github.com/juju/loggo"
	"lachain-communication-hub/config"
	server "lachain-communication-hub/grpc"
	"lachain-communication-hub/peer"
	"unsafe"
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

//export GetKey
func GetKey(buffer unsafe.Pointer, maxLength C.int) C.int {
	log.Tracef("Received: Get Key Request")
	id := localPeer.GetId()
	if id == nil || len(id) > int(maxLength) {
		return 0
	}
	C.memcpy(buffer, unsafe.Pointer(&id[0]), C.size_t(len(id)))
	return C.int(len(id))
}

//export Init
func Init(signaturePtr unsafe.Pointer, signatureLength C.int) C.int {
	log.Tracef("Received: Init Request")
	signature := C.GoBytes(signaturePtr, signatureLength)
	if localPeer.Register(signature) {
		return 1
	}
	return 0
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
