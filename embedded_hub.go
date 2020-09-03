package main
import "C"
import (
	"github.com/juju/loggo"
	"lachain-communication-hub/config"
	server "lachain-communication-hub/grpc"
	"lachain-communication-hub/peer"
)

var localPeer *peer.Peer
var grpcServer *server.Server

//export StartHub
func StartHub() {
	localPeer = peer.New("_h1")
	grpcServer = server.New(config.GRPCPort, localPeer)
	grpcServer.Serve()
}

//export LogLevel
func LogLevel(s *C.char, len C.int) {
	loggo.ConfigureLoggers(C.GoStringN(s, len))
}

//export StopHub
func StopHub() {
	localPeer.Stop()
	grpcServer.Stop()
}

func main() {}
