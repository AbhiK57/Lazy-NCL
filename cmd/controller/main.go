package main

import (
	"context"
	"flag"
	"fmt"
	"log"
	"net"
	"os"

	pb "github.com/AbhiK57/Lazy-NCL/proto/controller"
	"google.golang.org/grpc"
	"gopkg.in/yaml.v3"
)

var (
	port       = flag.Int("port", 50071, "The server port")
	configFile = flag.String("config", "config.yaml", "Path to the configuration file")
)

type Config struct {
	NCLPeers []string `yaml:"ncl_peers"`
}

type server struct {
	pb.UnimplementedControllerServer
	peerAddresses []string
}

func NewServer(configPath string) (*server, error) {
	data, err := os.ReadFile(configPath)
	if err != nil {
		return nil, fmt.Errorf("Failed to read the config file: %w", err)
	}
	var config Config
	if err := yaml.Unmarshal(data, &config); err != nil {
		return nil, fmt.Errorf("Failed to parse config file: %w", err)
	}

	log.Printf("Loaded %d NCL peer addresses from config", len(config.NCLPeers))
	return &server{peerAddresses: config.NCLPeers}, nil
}

// returns list of NCL peer addresses from the config.
func (s *server) GetPeerList(ctx context.Context, req *pb.GetPeerListRequest) (*pb.GetPeerListResponse, error) {
	log.Println("Received GetPeerList request.")
	return &pb.GetPeerListResponse{PeerAddresses: s.peerAddresses}, nil
}

func main() {
	flag.Parse()
	log.Printf("Controller service starting on port %d", *port)

	s, err := NewServer(*configFile)
	if err != nil {
		log.Fatalf("Failed to create server: %v", err)
	}

	lis, err := net.Listen("tcp", fmt.Sprintf(":%d", *port))
	if err != nil {
		log.Fatalf("Failed to listen: %v", err)
	}

	grpcServer := grpc.NewServer()
	pb.RegisterControllerServer(grpcServer, s)

	if err := grpcServer.Serve(lis); err != nil {
		log.Fatalf("Failed to serve controller server on GRPC: %v", err)
	}
}
