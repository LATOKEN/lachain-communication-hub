package main

import (
	"github.com/juju/loggo"
	"os"
	"os/signal"
	"syscall"
)

func main() {
	loggo.ConfigureLoggers("<root>=TRACE")
	var log = loggo.GetLogger("main")
	//if len(os.Args) <= 1 {
	//	localPeer := peer.New("_h1")
	//} else {
	//	if os.Args[1] == "-nolookup" {
	//		config.DisableIpLookup()
	//		localPeer := peer.New("_h1")
	//	} else if os.Args[1] == "-keygen" {
	//		count := 1
	//		if len(os.Args) > 2 {
	//			count, _ = strconv.Atoi(os.Args[2])
	//		}
	//		peer.KeyGen(count)
	//	} else if os.Args[1] == "-port" && len(os.Args) >= 2 {
	//		localPeer := peer.New("_h1")
	//	}
	//}

	// wait for a SIGINT or SIGTERM signal
	ch := make(chan os.Signal, 1)
	signal.Notify(ch, syscall.SIGINT, syscall.SIGTERM)
	<-ch
	log.Infof("Received signal, shutting down...")
}
