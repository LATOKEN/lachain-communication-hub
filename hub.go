package main

import (
	"fmt"
	"lachain-communication-hub/config"
	server "lachain-communication-hub/grpc"
	"lachain-communication-hub/peer"
	"lachain-communication-hub/relay"
	"os"
	"os/signal"
	"syscall"
)

func main() {

	if len(os.Args) <= 1 {
		localPeer := peer.New("_h1")
		server.New(config.GRPCPort, &localPeer)
	} else {
		if os.Args[1] == "-relay" {
			relay.Run()
		}
	}

	// wait for a SIGINT or SIGTERM signal
	ch := make(chan os.Signal, 1)
	signal.Notify(ch, syscall.SIGINT, syscall.SIGTERM)
	<-ch
	fmt.Println("Received signal, shutting down...")
}
