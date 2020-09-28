package main

import "C"
import (
	"fmt"
	"github.com/juju/loggo"
	"lachain-communication-hub/config"
	server "lachain-communication-hub/grpc"
	"lachain-communication-hub/peer"
)

var localPeer *peer.Peer
var grpcServer *server.Server

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
