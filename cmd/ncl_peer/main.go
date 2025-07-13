package main

import (
	"context"
	"flag"
	"fmt"
	"log"
	"net"
	"sync"

	pb "github.com/AbhiK57/Lazy-NCL/proto/ncl"
	"google.golang.org/grpc"
)

var (
	port = flag.Int("port", 50051, "The server port")
)

// NCLPeer GRPC Server
type server struct {
	pb.UnimplementedNCLPeerServer
	mu  sync.Mutex
	log []*pb.Metadata
}

func NewServer() *server {
	return &server{
		log: make([]*pb.Metadata, 0),
	}
}

// adds metadata entry to in-memory log
func (s *server) AppendMetadata(ctx context.Context, req *pb.AppendMetadataRequest) (*pb.AppendMetadataResponse, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	s.log = append(s.log, req.Meta)
	log.Printf("Appended metadata for record_id: %s. Log size: %d", req.Meta.RecordId, len(s.log))

	return &pb.AppendMetadataResponse{}, nil
}

// streams metadata entries to a client
func (s *server) ReadMetadata(req *pb.ReadMetadataRequest, stream pb.NCLPeer_ReadMetadataServer) error {
	s.mu.Lock()
	//copy relevant part of log
	var streamEntries []*pb.Metadata
	if req.Offset < int64(len(s.log)) {
		streamEntries = s.log[req.Offset:]
	}
	s.mu.Unlock()

	log.Printf("Streaming %d entries starting at offset %d", len(streamEntries), req.Offset)
	for _, entry := range streamEntries {
		if err := stream.Send(entry); err != nil {
			return err
		}
	}
	return nil
}

func main() {
	flag.Parse()
	log.Printf("NCL Peer service starting on port %d", *port)

	lis, err := net.Listen("tcp", fmt.Sprintf(":%d", *port))
	if err != nil {
		log.Fatalf("Failed to listen: %v", err)
	}
	s := grpc.NewServer()
	pb.RegisterNCLPeerServer(s, NewServer())
	if err := s.Serve(lis); err != nil {
		log.Fatalf("failed to serve NCL peer server on GRPC: %v", err)
	}
}
