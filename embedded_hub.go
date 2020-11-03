package main

//#include <string.h>
import "C"
import (
	"bytes"
	"encoding/binary"
	"encoding/hex"
	"fmt"
	"github.com/enriquebris/goconcurrentqueue"
	"github.com/juju/loggo"
	"lachain-communication-hub/config"
	"lachain-communication-hub/host"
	"lachain-communication-hub/peer"
	"unsafe"
)

var localPeer *peer.Peer

var log = loggo.GetLogger("embedded_hub")
var ZeroPub = make([]byte, 33)

var messages = goconcurrentqueue.NewFIFO()

func ProcessMessage(msg []byte) {
	messages.Enqueue(msg)
}

//export StartHub
func StartHub(bootstrapAddress *C.char, bootstrapAddressLen C.int) {
	config.SetBootstrapAddress(C.GoStringN(bootstrapAddress, bootstrapAddressLen))
	priv_key := host.GetPrivateKeyForHost("_h1")
	localPeer = peer.New(priv_key)
	localPeer.SetStreamHandlerFn(ProcessMessage)
}

//export GetKey
func GetKey(buffer unsafe.Pointer, maxLength C.int) C.int {
	log.Tracef("Received: Get Key Request")
	if localPeer == nil {
		return 0
	}
	id := localPeer.GetId()
	if id == nil || len(id) > int(maxLength) {
		return 0
	}
	C.memcpy(buffer, unsafe.Pointer(&id[0]), C.size_t(len(id)))
	return C.int(len(id))
}

//export GetMessages
func GetMessages(buffer unsafe.Pointer, maxLength C.int) C.int {
	n := messages.GetLen()
	if n == 0 {
		return C.int(-1)
	}
	c := 0
	ptr := uint32(0)
	for i := 0; i < n; i++ {
		msgP, err := messages.Get(0)
		if err != nil {
			return -1
		}
		msg := msgP.([]byte)
		l := uint32(len(msg))
		if ptr+uint32(4) > uint32(maxLength) {
			break
		}
		err = messages.Remove(0)
		if err != nil {
			return -1
		}
		msg = append([]byte{0, 0, 0, 0}, msg...)
		binary.LittleEndian.PutUint32(msg[:4], l)

		C.memcpy(unsafe.Pointer(uintptr(buffer)+uintptr(ptr)), unsafe.Pointer(&msg[0]), C.size_t(l+4))
		ptr += l + 4
		c += 1
	}
	return C.int(c)
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
}

func main() {}
