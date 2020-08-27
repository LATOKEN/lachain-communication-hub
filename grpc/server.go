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
	peer *peer.Peer
}

func (s *server) GetKey(ctx context.Context, in *pb.GetHubIdRequest) (*pb.GetHubIdReply, error) {
	log.Printf("Received: Get Key Request")
	return &pb.GetHubIdReply{
		Id: s.peer.GetId(),
	}, nil
}

func (s *server) Init(ctx context.Context, in *pb.InitRequest) (*pb.InitReply, error) {
	log.Printf("Received: Init Request")
	s.peer.Register(in.GetSignature())
	return &pb.InitReply{
		// TODO: check real result
		Result: true,
	}, nil
}

func (s *server) Communicate(stream pb.CommunicationHub_CommunicateServer) error {

	log.Println("Started new communication server")

	ctx := stream.Context()

	onMsg := func(msg []byte) {
		select {
		case <-ctx.Done():
			log.Println("Unable to send msg via rpc")
			s.peer.SetStreamHandler(peer.GRPCHandlerMock)
			return
		default:
		}

		log.Println("Received msg, sending via rpc to client")
		resp := pb.OutboundMessage{Data: msg}
		if err := stream.Send(&resp); err != nil {
			log.Println("Unable to send msg via rpc")
			s.peer.SetStreamHandler(peer.GRPCHandlerMock)
		}
	}

	s.peer.SetStreamHandler(onMsg)

	for {

		// exit if context is done
		// or continue
		select {
		case <-ctx.Done():
			return ctx.Err()
		default:
		}

		// receive data from stream
		req, err := stream.Recv()
		if err == io.EOF {
			// return will close stream from server side
			log.Println("exit")
			return err
		}
		if err != nil {
			return err
		}

		fmt.Println("Sending message to peer", hex.EncodeToString(req.PublicKey), "message length", len(req.Data))

		s.peer.SendMessageToPeer(hex.EncodeToString(req.PublicKey), req.Data)
	}
}

func runServer(s *grpc.Server, lis net.Listener) {
	log.Println("GRPC server is listening on", lis.Addr())
	if err := s.Serve(lis); err != nil {
		log.Fatalf("failed to serve: %v", err)
	}
}

func New(port string, localPeer *peer.Peer) *server {
	p := localPeer
	lis, err := net.Listen("tcp", port)
	if err != nil {
		log.Fatalf("failed to listen: %v", err)
	}
	s := grpc.NewServer()
	server := &server{peer: p}
	pb.RegisterCommunicationHubServer(s, server)
	go runServer(s, lis)
	return server
}
