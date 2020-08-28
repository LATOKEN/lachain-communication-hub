package main

import "C"
import (
	"lachain-communication-hub/config"
	server "lachain-communication-hub/grpc"
	"lachain-communication-hub/peer"
)

var localPeer peer.Peer
var grpcServer *server.Server

//export StartHub
func StartHub() {
	localPeer = peer.New("_h1")
	grpcServer = server.New(config.GRPCPort, &localPeer)
}

//export StopHub
func StopHub() {
	// TODO: ??
}

func main() {}
