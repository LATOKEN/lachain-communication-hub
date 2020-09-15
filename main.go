package main

import (
	"github.com/ethereum/go-ethereum/log"
	"github.com/juju/loggo"
	"lachain-communication-hub/config"
	server "lachain-communication-hub/grpc"
	"lachain-communication-hub/peer"
	"os"
	"os/signal"
	"syscall"
)

func main() {
	loggo.ConfigureLoggers("<root>=TRACE")
	if len(os.Args) <= 1 {
		localPeer := peer.New("_h1")
		s := server.New(config.GRPCPort, localPeer)
		go s.Serve()
	} else {
		if os.Args[1] == "-nolookup" {
			config.DisableIpLookup()
			localPeer := peer.New("_h1")
			server.New(config.GRPCPort, localPeer)
		}
		if os.Args[1] == "-port" && len(os.Args) >= 2 {
			localPeer := peer.New("_h1")
			s := server.New(os.Args[2], localPeer)
			go s.Serve()
		}
	}

	// wait for a SIGINT or SIGTERM signal
	ch := make(chan os.Signal, 1)
	signal.Notify(ch, syscall.SIGINT, syscall.SIGTERM)
	<-ch
	log.Info("Received signal, shutting down...")
}
