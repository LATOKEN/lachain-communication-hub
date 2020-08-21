package main

import (
	"fmt"
	"lachain-communication-hub/peer"
	"lachain-communication-hub/relay"
	"os"
	"os/signal"
	"syscall"
)

func main() {

	if len(os.Args) <= 1 {
		fmt.Println("Try passing args [-relay, -h1, -h2]")
		os.Exit(1)
	}

	switch os.Args[1] {
	case "-relay":
		relay.Run()
		break
	case "-h1":
		peer.New("_h1")
		break
	case "-h2":
		localPeer := peer.New("_h2")

		response := localPeer.RequestDataFromPeer(
			"0803125b3059301306072a8648ce3d020106082a8648ce3d03010703420004b3e577160c1203e21d39da3f6a612c1d2d9eb505f228f53cc762c2527f637fcd68811d206baa71e51f2fc0499eb9cd4e28db70dbfb527573f3fb5e951702a6a6",
			[]byte("ping"),
		)
		fmt.Println("Peer answered:", string(response))
		break
	}

	// wait for a SIGINT or SIGTERM signal
	ch := make(chan os.Signal, 1)
	signal.Notify(ch, syscall.SIGINT, syscall.SIGTERM)
	<-ch
	fmt.Println("Received signal, shutting down...")
}
