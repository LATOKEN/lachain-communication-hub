package main

//#include <string.h>
import "C"
import (
	"bytes"
	"crypto/rand"
	"encoding/binary"
	"encoding/hex"
	"fmt"
	"lachain-communication-hub/config"
	"lachain-communication-hub/peer_service"
	"net"
	"net/http"
	"os"
	"path/filepath"
	"sync"
	"time"
	"unsafe"

	"github.com/enriquebris/goconcurrentqueue"
	"github.com/juju/loggo"
	"github.com/libp2p/go-libp2p-core/crypto"
	"github.com/libp2p/go-libp2p-core/peer"
	"github.com/prometheus/client_golang/prometheus/promhttp"
)

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
func StartHub(bootstrapAddress *C.char, bootstrapAddressLen C.int, privKey unsafe.Pointer, privKeyLen C.int,
	networkName *C.char, networkNameLen C.int, version C.int, minimalSupportedVersion C.int, chainId C.int) {
	mutex.Lock()
	defer mutex.Unlock()
	config.ChainId = byte(chainId)
	config.SetBootstrapAddress(C.GoStringN(bootstrapAddress, bootstrapAddressLen))
	prvBytes := C.GoBytes(privKey, privKeyLen)
	prv, err2 := crypto.UnmarshalPrivateKey(prvBytes)
	if err2 != nil {
		panic(err2)
	}
	localPeer = peer_service.New(prv, C.GoStringN(networkName, networkNameLen), int32(version),
		int32(minimalSupportedVersion),
		ProcessMessage)
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
	//log.Tracef("GetMessages sending overall %d messages, %d bytes", c, ptr)
	return C.int(c)
}

//export Init
func Init(signaturePtr unsafe.Pointer, signatureLength C.int, metricsPort C.int) C.int {
	mutex.Lock()
	defer mutex.Unlock()
	log.Tracef("Received: Init Request")
	http.Handle("/metrics", promhttp.Handler())
	go func() {
		for {
			err := http.ListenAndServe(fmt.Sprintf(":%d", int(metricsPort)), nil)
			if err != nil {
				log.Errorf("Error while serving metrics: %v", err)
			}
		}
	}()

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
	//log.Tracef("SendMessage command to send %d bytes to %s", dataLen, hex.EncodeToString(pubKey))

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
	loggo.ReplaceDefaultWriter(
		loggo.NewSimpleWriter(os.Stderr,
			func(entry loggo.Entry) string {
				ts := entry.Timestamp.In(time.UTC).Format("2006/01/02 15:04:05.000")
				filename := filepath.Base(entry.Filename)
				return fmt.Sprintf("%s|%-5v|%-30v| %s", ts, entry.Level, fmt.Sprintf("%s %s:%d", entry.Module, filename, entry.Line), entry.Message)
			}))
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

//export GenerateNewKey
func GenerateNewKey(buffer unsafe.Pointer, bufferLen C.int) C.int {
	prv, _, err := crypto.GenerateECDSAKeyPair(rand.Reader)
	if err != nil {
		panic(err)
	}

	id, _ := peer.IDFromPrivateKey(prv)

	prvBytes, err := crypto.MarshalPrivateKey(prv)
	if err != nil {
		panic(err)
	}

	prvHex := fmt.Sprintf("%s,%s", hex.EncodeToString(prvBytes), id)

	if len(prvHex) > int(bufferLen) {
		return C.int(len(prvHex))
	}

	data := []byte(prvHex)
	C.memcpy(buffer, unsafe.Pointer(&data[0]), C.size_t(len(data)))
	return C.int(len(data))
}

func main() {}
