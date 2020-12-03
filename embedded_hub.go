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
	"lachain-communication-hub/peer_service"
	"net"
	"net/http"
	"sync"
	"unsafe"
)

import _ "net/http/pprof"

var localPeer *peer_service.PeerService

var log = loggo.GetLogger("embedded_hub")
var ZeroPub = make([]byte, 33)

var messages = goconcurrentqueue.NewFIFO()
var mutex = &sync.Mutex{}
var profilerPort C.int = -1

func ProcessMessage(msg []byte) {
	messages.Enqueue(msg)
}

//export StartHub
func StartHub(bootstrapAddress *C.char, bootstrapAddressLen C.int) {
	mutex.Lock()
	defer mutex.Unlock()
	config.SetBootstrapAddress(C.GoStringN(bootstrapAddress, bootstrapAddressLen))
	priv_key := host.GetPrivateKeyForHost("_h1")
	localPeer = peer_service.New(priv_key, ProcessMessage)
}

//export GetKey
func GetKey(buffer unsafe.Pointer, maxLength C.int) C.int {
	mutex.Lock()
	defer mutex.Unlock()
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
	mutex.Lock()
	defer mutex.Unlock()
	n := messages.GetLen()
	if n == 0 {
		return C.int(-1)
	}
	c := 0
	ptr := uint32(0)
	//log.Tracef("GetMessages sending %d messages", n)
	for i := 0; i < n; i++ {
		msgP, err := messages.Get(0)
		if err != nil {
			//log.Errorf("Failed to fetch message %d", i)
			return -1
		}
		msg := msgP.([]byte)
		l := uint32(len(msg))
		if ptr+l+uint32(4) > uint32(maxLength) {
			//log.Errorf("Not sending message %d, overflow", i)
			break
		}
		err = messages.Remove(0)
		if err != nil {
			//log.Errorf("Can't remove message %d", i)
			return -1
		}
		msg = append([]byte{0, 0, 0, 0}, msg...)
		binary.LittleEndian.PutUint32(msg[:4], l)

		//log.Tracef("Writing 4 + %d bytes to buffer[%d..%d)", l, ptr, ptr + l + 4)
		C.memcpy(unsafe.Pointer(uintptr(buffer)+uintptr(ptr)), unsafe.Pointer(&msg[0]), C.size_t(l+4))
		ptr += l + 4
		c += 1
	}
	log.Tracef("GetMessages sending overall %d messages, %d bytes", c, ptr)
	return C.int(c)
}

//export Init
func Init(signaturePtr unsafe.Pointer, signatureLength C.int) C.int {
	mutex.Lock()
	defer mutex.Unlock()
	log.Tracef("Received: Init Request")
	signature := C.GoBytes(signaturePtr, signatureLength)
	if localPeer.SetSignature(signature) {
		return 1
	}
	return 0
}

//export SendMessage
func SendMessage(pubKeyPtr unsafe.Pointer, pubKeyLen C.int, dataPtr unsafe.Pointer, dataLen C.int) {
	mutex.Lock()
	defer mutex.Unlock()
	pubKey := C.GoBytes(pubKeyPtr, pubKeyLen)
	data := C.GoBytes(dataPtr, dataLen)
	log.Tracef("SendMessage command to send %d bytes to %s", dataLen, hex.EncodeToString(pubKey))

	if bytes.Equal(pubKey, ZeroPub) {
		localPeer.BroadcastMessage(data)
	} else {
		pub := hex.EncodeToString(pubKey)
		localPeer.SendMessageToPeer(pub, data)
	}
}

//export LogLevel
func LogLevel(s *C.char, len C.int) {
	mutex.Lock()
	defer mutex.Unlock()
	loggo.ConfigureLoggers(C.GoStringN(s, len))
}

//export StopHub
func StopHub() {
	mutex.Lock()
	defer mutex.Unlock()
	fmt.Println("Exit received")
	localPeer.Stop()
}

//export StartProfiler
func StartProfiler() C.int {
	if profilerPort != -1 {
		return profilerPort
	}
	portChannel := make(chan int)
	go func() {
		listener, err := net.Listen("tcp", ":0")
		if err != nil {
			panic(err)
		}

		p := listener.Addr().(*net.TCPAddr).Port
		log.Debugf("Using pprof port: %v", p)
		portChannel <- p

		if err := http.Serve(listener, nil); err != nil {
			log.Errorf("Failed to listen on pprof port %v: %v", p, err)
			profilerPort = -1
			return
		}
	}()
	return C.int(<-portChannel)
}

func main() {}
