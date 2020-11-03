package main

import (
	"github.com/juju/loggo"
	"os"
	"os/signal"
	"syscall"
)

func main() {
	var log = loggo.GetLogger("main")
	loggo.ConfigureLoggers("<root>=TRACE")
	
	//priv_key := host.GetPrivateKeyForHost("_h1")
	//if len(os.Args) <= 1 {
	//	localPeer := peer.New(priv_key)
	//	s := server.New(config.GRPCPort, localPeer)
	//	go s.Serve()
	//} else {
	//	if os.Args[1] == "-nolookup" {
	//		config.DisableIpLookup()
	//		localPeer := peer.New(priv_key)
	//		server.New(config.GRPCPort, localPeer)
	//	} else if os.Args[1] == "-keygen" {
	//		count := 1
	//		if len(os.Args) > 2 {
	//			count, _ = strconv.Atoi(os.Args[2])
	//		}
	//		peer.KeyGen(count)
	//	} else if os.Args[1] == "-port" && len(os.Args) >= 2 {
	//		localPeer := peer.New(priv_key)
	//		s := server.New(os.Args[2], localPeer)
	//		go s.Serve()
	//	}
	//}

	// wait for a SIGINT or SIGTERM signal
	ch := make(chan os.Signal, 1)
	signal.Notify(ch, syscall.SIGINT, syscall.SIGTERM)
	<-ch
	log.Infof("Received signal, shutting down...")
}
