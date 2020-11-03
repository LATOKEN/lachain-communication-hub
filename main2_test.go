package main_test

import (
	"fmt"

	"lachain-communication-hub/config"
	server "lachain-communication-hub/grpc"
	"lachain-communication-hub/host"
	"lachain-communication-hub/peer"
	"testing"

	"github.com/juju/loggo"

	p2p_peer "github.com/libp2p/go-libp2p-core/peer"
)

var (
	address1 = "127.0.0.1:50005"
	address2 = "127.0.0.1:50003"
)

var log = loggo.GetLogger("builder.go")

func TestCommunication(t *testing.T) {
	loggo.ConfigureLoggers("<root>=TRACE")
	// register addresses as bootstraps
	//pub1 := "037f447253723035341f32d2bf8c5dde284f677157c1262bd5849174d41b98e03d"
	//pub2 := "037f447253723035341f32d2bf8c5dde284f677157c1262bd5849174d41b98e03d"
	priv_key1 := host.GetPrivateKeyForHost("_h1")
	id1, _ := p2p_peer.IDFromPrivateKey(priv_key1)
	grpcAddress1 := p2p_peer.Encode(id1) + "@" + address1
	config.SetBootstrapAddress(grpcAddress1)
	peer1 := peer.New(priv_key1)
	server1 := server.New(":50005", peer1)
	go server1.Serve()

	priv_key2 := host.GetPrivateKeyForHost("_h2")
	id2, _ := p2p_peer.IDFromPrivateKey(priv_key2)
	grpcAddress2 := p2p_peer.Encode(id2) + "@" + address2
	config.SetBootstrapAddress(grpcAddress2)
	peer2 := peer.New(priv_key2)
	server2 := server.New(":50003", peer2)
	go server2.Serve()

	//config.SetBootstrapAddress(address2)
	// connect clients
	//conn1, _ := makeServerPeer("_h1", ":50005", address1)
	//defer conn1.Close()

	//conn2, pub := makeServerPeer("_h2", ":50003", address2)
	//defer conn2.Close()

	//client := pb.NewCommunicationHubClient(peer1)
	//stream, err := client.Communicate(context.Background())
	//if err != nil {
	//log.Errorf("open stream error %v", err)
	//}

	done := make(chan bool)

	go func() {
		peer1.BroadcastMessage([]byte("ping"))
	}()

	// go func() {
	// 	for {
	// 		resp, err := stream.Recv()
	// 		if err == io.EOF {
	// 			fmt.Println("EOF")
	// 			return
	// 		}
	// 		if err != nil {
	// 			log.Errorf("can not receive %v", err)
	// 		}
	// 		expectedResponse := []byte("73515441561657fdh437h7fh4387f7834")
	// 		log.Infof("received grpc message: %s", string(resp.Data))
	// 		log.Infof("len, %v", len(expectedResponse))
	// 		log.Infof("len, %v", len(resp.Data))
	// 		diff := false
	// 		for i, b := range resp.Data {
	// 			if b != expectedResponse[i] {
	// 				fmt.Printf("i %v\n", i)
	// 				diff = true
	// 				break
	// 			}
	// 		}
	// 		assert.Equal(t, diff, false)
	// 		assert.Equal(t, len(expectedResponse), len(resp.Data))
	// 		stream.CloseSend()
	// 		done <- true
	// 	}
	// }()

	<-done
	fmt.Println("finish")
}
