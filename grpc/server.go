package grpc

import (
	"context"
	"encoding/hex"
	"fmt"
	"google.golang.org/grpc"
	"io"
	"lachain-communication-hub/peer"
	"log"
	"net"

	pb "lachain-communication-hub/grpc/protobuf"
)

type server struct {
	pb.UnimplementedCommunicationHubServer
}

var Peer *peer.Peer

func (s *server) GetKey(ctx context.Context, in *pb.GetHubIdRequest) (*pb.GetHubIdReply, error) {
	log.Printf("Received: Get Key Request")
	return &pb.GetHubIdReply{
		Id: Peer.GetId(),
	}, nil
}

func (s *server) Init(ctx context.Context, in *pb.InitRequest) (*pb.InitReply, error) {
	log.Printf("Received: Init Request")
	Peer.Register(in.GetSignature())
	return &pb.InitReply{
		// TODO: check real result
		Result: true,
	}, nil
}

func (s *server) Communicate(srv pb.CommunicationHub_CommunicateServer) error {

	log.Println("start new server")

	ctx := srv.Context()

	onMsg := func(msg []byte) {
		resp := pb.OutboundMessage{Data: msg}
		if err := srv.Send(&resp); err != nil {
			log.Printf("send error %v", err)
		}
	}

	Peer.SetMsgHandler(onMsg)

	for {

		// exit if context is done
		// or continue
		select {
		case <-ctx.Done():
			return ctx.Err()
		default:
		}

		// receive data from stream
		req, err := srv.Recv()
		if err == io.EOF {
			// return will close stream from server side
			log.Println("exit")
			return nil
		}
		if err != nil {
			log.Printf("receive error %v", err)
			continue
		}

		fmt.Println("Sending message to peer", hex.EncodeToString(req.PublicKey))

		Peer.SendMessageToPeer(hex.EncodeToString(req.PublicKey), req.Data)
	}
}

func runServer(s *grpc.Server, lis net.Listener) {
	if err := s.Serve(lis); err != nil {
		log.Fatalf("failed to serve: %v", err)
	}
}

func New(port string, localPeer *peer.Peer) *server {
	Peer = localPeer
	lis, err := net.Listen("tcp", port)
	if err != nil {
		log.Fatalf("failed to listen: %v", err)
	}
	s := grpc.NewServer()
	server := &server{}
	pb.RegisterCommunicationHubServer(s, server)
	go runServer(s, lis)
	return server
}
