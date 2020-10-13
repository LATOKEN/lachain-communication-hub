package grpc

import (
	"bytes"
	"context"
	"encoding/hex"
	"fmt"
	"github.com/juju/loggo"
	"google.golang.org/grpc"
	"io"
	"lachain-communication-hub/peer"
	"lachain-communication-hub/storage"
	"net"
	"strings"
	"time"

	pb "lachain-communication-hub/grpc/protobuf"
)

var log = loggo.GetLogger("server")

var ZeroPub = make([]byte, 33)

var serverStop = make(chan struct{})

type Server struct {
	pb.UnimplementedCommunicationHubServer
	peer       *peer.Peer
	grpcServer *grpc.Server
	Serve      func()
}

func (s *Server) GetKey(ctx context.Context, in *pb.GetHubIdRequest) (*pb.GetHubIdReply, error) {
	log.Tracef("Received: Get Key Request")
	return &pb.GetHubIdReply{
		Id: s.peer.GetId(),
	}, nil
}

func (s *Server) Init(ctx context.Context, in *pb.InitRequest) (*pb.InitReply, error) {
	log.Tracef("Received: Init Request")
	result := s.peer.Register(in.GetSignature())
	return &pb.InitReply{
		Result: result,
	}, nil
}

func (s *Server) Communicate(stream pb.CommunicationHub_CommunicateServer) error {

	log.Debugf("Started new communication server")

	ctx := stream.Context()

	onMsg := func(msg []byte) {
		log.Tracef("On message callback is called")
		select {
		case <-ctx.Done():
			log.Errorf("Unable to send msg via rpc")
			s.peer.SetStreamHandlerFn(peer.GRPCHandlerMock)
			return
		default:
		}

		log.Tracef("Received msg, sending via rpc to client")
		resp := pb.OutboundMessage{Data: msg}
		if err := stream.Send(&resp); err != nil {
			log.Errorf("Unable to send msg via rpc")
			s.peer.SetStreamHandlerFn(peer.GRPCHandlerMock)
		}
	}

	s.peer.SetStreamHandlerFn(onMsg)

	unprocessedMsgs := storage.TakeSavedGRPCMessages()

	for _, msg := range unprocessedMsgs {
		onMsg(msg)
	}

	for {

		// exit if context is done
		// or continue
		select {
		case <-serverStop:
			return nil
		case <-ctx.Done():
			log.Errorf("Communication error: %s", ctx.Err())
			return ctx.Err()
		default:
		}

		commErr := make(chan struct{})
		result := make(chan struct{})

		go func() {
			// receive data from stream
			req, err := stream.Recv()
			if err == io.EOF {
				// return will close stream from server side
				log.Errorf("Communication error: %s", err)
				close(commErr)
				return
			}
			close(result)
			if err != nil {
				if strings.Contains(err.Error(), "context canceled") {
					log.Errorf("Communication closed")
					close(commErr)
					return
				}

				log.Errorf("Communication error: %s", err)
				close(commErr)
				return
			}

			log.Tracef("GRPC command to send message to %s", hex.EncodeToString(req.PublicKey))

			if bytes.Equal(req.PublicKey, ZeroPub) {
				s.peer.BroadcastMessage(req.Data)
			} else {
				pub := hex.EncodeToString(req.PublicKey)
				s.peer.SendMessageToPeer(pub, req.Data, true)
			}
		}()

		t := time.NewTimer(5 * time.Second)

		select {
		case <-t.C:
			continue
		case <-result:
			t.Stop()
			continue
		case <-commErr:
			t.Stop()
			return nil
		}
	}
}

func runServer(s *grpc.Server, lis net.Listener) {
	log.Infof("GRPC server is listening on %s", lis.Addr())
	if err := s.Serve(lis); err != nil {
		log.Errorf("failed to serve: %v", err)
	}
}

func New(port string, p *peer.Peer) *Server {
	lis, err := net.Listen("tcp", port)
	if err != nil {
		panic(fmt.Sprintf("failed to listen: %v", err))
	}
	s := grpc.NewServer()
	server := &Server{
		peer:       p,
		grpcServer: s,
		Serve: func() {
			runServer(s, lis)
		},
	}
	pb.RegisterCommunicationHubServer(s, server)
	return server
}

func (s *Server) Stop() {

	stopped := make(chan struct{})

	go func() {
		close(serverStop)
		s.grpcServer.GracefulStop()
		close(stopped)
	}()

	t := time.NewTimer(10 * time.Second)
	select {
	case <-t.C:
		s.grpcServer.Stop()
	case <-stopped:
		log.Debugf("GRPC Server gracefully stopped")
		t.Stop()
	}
}
