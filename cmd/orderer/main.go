package main

import (
	"context"
	"flag"
	"fmt"
	"log"
	"net"
	"sync"
	"time"

	nclpb "github.com/AbhiK57/Lazy-NCL/proto/ncl"
	ordererpb "github.com/AbhiK57/Lazy-NCL/proto/orderer"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"
	"google.golang.org/grpc/reflection"
)

var (
	nclLeaderAddr = flag.String("ncl_leader", "localhost:50051", "Address of the NCL leader peer")
	pollInterval  = flag.Duration("poll_interval", 100*time.Millisecond, "Interval to poll NCL leader")
	rpcPort       = flag.Int("port", 50081, "The RPC port for orderer service")
)

// stores resolved metadata for global sequence
type GlobalIndexEntry struct {
	RecordID string
	ShardID  string
}

// finalizes the global log order
type Orderer struct {
	nclClient nclpb.NCLPeerClient
	ordererpb.UnimplementedOrdererServer

	mu                sync.RWMutex
	globalSequenceNum int64
	lastNCLOffset     int64
	globalIndex       map[int64]GlobalIndexEntry
}

// creates new orderer instance
func NewOrderer(nclClient nclpb.NCLPeerClient) *Orderer {
	return &Orderer{
		nclClient:   nclClient,
		globalIndex: make(map[int64]GlobalIndexEntry),
	}
}

func (o *Orderer) Run() {
	ticker := time.NewTicker(*pollInterval)
	defer ticker.Stop()

	for range ticker.C {
		o.pollAndOrder()
	}
}

func (o *Orderer) pollAndOrder() {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	stream, err := o.nclClient.ReadMetadata(ctx, &nclpb.ReadMetadataRequest{Offset: o.lastNCLOffset})

	if err != nil {
		log.Printf("Unable to read metadata from NCL leader: %v", err)
		return
	}

	for {
		metadata, err := stream.Recv()
		if err != nil {
			if err.Error() == "EOF" {
				break
			}
			log.Printf("Error receiving from NCL stream: %v", err)
		}

		o.mu.Lock()

		//TODO: Handle deduplication

		seq := o.globalSequenceNum
		o.globalIndex[seq] = GlobalIndexEntry{
			RecordID: metadata.RecordId,
			ShardID:  metadata.ShardId,
		}

		o.globalSequenceNum++
		o.lastNCLOffset++
		o.mu.Unlock()
	}

}

func main() {
	flag.Parse()

	//connect to NCL leader
	conn, err := grpc.NewClient(*nclLeaderAddr, grpc.WithTransportCredentials(insecure.NewCredentials()))
	if err != nil {
		log.Fatalf("Failed to connect to NCL leader: %v", err)
	}
	defer conn.Close()

	nclClient := nclpb.NewNCLPeerClient(conn)
	orderer := NewOrderer(nclClient)

	//start GRPC server async
	go func() {
		lis, err := net.Listen("tcp", fmt.Sprintf(":%d", *rpcPort))
		if err != nil {
			log.Fatalf("Failed to listen on port %d: %v", *rpcPort, err)
		}

		s := grpc.NewServer()
		ordererpb.RegisterOrdererServer(s, orderer)
		reflection.Register(s)
		log.Printf("Orderer RPC server listening at %v", lis.Addr())
		if err := s.Serve(lis); err != nil {
			log.Fatalf("Failed to serve RPC: %v", err)
		}
	}()

	log.Println("Backround orderer begun. Polling NCL Leader...")
	orderer.Run() //need to write function RUN
}
