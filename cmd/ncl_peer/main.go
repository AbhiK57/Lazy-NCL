package main

import (
	"context"
	"flag"
	"log"
	"sync"

	pb "github.com/AbhiK57/Lazy-NCL/proto/ncl"
)

var (
	port = flag.Int("port", 50051, "The server port")
)

// NCLPeer GRPC Server
type server struct {
	pb.UnimplementedNCLPeerServer
	mu  sync.Mutex
	log *pb.Metadata
}

func NewServer() *server {
	return &server{
		log: &pb.Metadata{},
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
