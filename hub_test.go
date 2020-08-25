package main_test

import (
	"context"
	"encoding/hex"
	"fmt"
	crypto2 "github.com/ethereum/go-ethereum/crypto"
	"google.golang.org/grpc"
	"io"
	"lachain-communication-hub/config"
	server "lachain-communication-hub/grpc"
	"lachain-communication-hub/peer"
	"log"
	"testing"
	"time"

	pb "lachain-communication-hub/grpc/protobuf"
)

const (
	address1 = "localhost" + config.GRPCPort
	address2 = "localhost:50002"
)

func TestCommunication(t *testing.T) {
	// connect clients
	conn1, _ := registerPeer("_h1", config.GRPCPort, address1)
	defer conn1.Close()

	conn2, pub := registerPeer("_h2", ":50002", address2)
	defer conn2.Close()

	client := pb.NewCommunicationHubClient(conn1)
	stream, err := client.Communicate(context.Background())
	if err != nil {
		log.Fatalf("openn stream error %v", err)
	}

	ctx := stream.Context()
	done := make(chan bool)

	go func() {
		req := pb.InboundMessage{
			PublicKey: pub,
			Data:      []byte("test"),
		}
		if err := stream.Send(&req); err != nil {
			log.Fatalf("can not send %v", err)
		}
	}()

	// second goroutine receives data from stream
	// and saves result in max variable
	//
	// if stream is finished it closes done channel
	go func() {
		for {
			resp, err := stream.Recv()
			if err == io.EOF {
				close(done)
				return
			}
			if err != nil {
				log.Fatalf("can not receive %v", err)
			}
			log.Println("new msg received", string(resp.Data))
		}
	}()

	// third goroutine closes done channel
	// if context is done
	go func() {
		<-ctx.Done()
		if err := ctx.Err(); err != nil {
			log.Println(err)
		}
		close(done)
	}()

	<-done
	log.Println("finished")
}

func registerPeer(id string, port string, address string) (*grpc.ClientConn, []byte) {
	peer := peer.New(id)
	server.New(port, &peer)

	conn, err := grpc.Dial(address, grpc.WithInsecure(), grpc.WithBlock())
	if err != nil {
		log.Fatalf("did not connect: %v", err)
	}

	c := pb.NewCommunicationHubClient(conn)

	ctx, cancel := context.WithTimeout(context.Background(), time.Second)
	getKeyResult, err := c.GetKey(ctx, &pb.GetHubIdRequest{})
	if err != nil {
		log.Fatalf("could not: %v", err)
	}
	cancel()

	prv, err := crypto2.GenerateKey()
	pub := crypto2.CompressPubkey(&prv.PublicKey)

	fmt.Println("pubKey", hex.EncodeToString(pub))

	hash := crypto2.Keccak256Hash(getKeyResult.Id)
	signature, err := crypto2.Sign(hash.Bytes(), prv)
	if err != nil {
		panic(err)
	}

	ctx, cancel = context.WithTimeout(context.Background(), 2*time.Second)
	initR, err := c.Init(ctx, &pb.InitRequest{
		Signature: signature,
	})
	if err != nil {
		log.Fatalf("could not: %v", err)
	}
	cancel()

	log.Println("init result:", initR.Result)
	return conn, pub
}
