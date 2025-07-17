package main

import (
	"context"
	"flag"
	"log"
	"sync"
	"time"

	nclpb "github.com/AbhiK57/Lazy-NCL/proto/ncl"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"
)

var (
	nclLeaderAddr = flag.String("ncl_leader", "localhost:50051", "Address of the NCL leader peer")
	pollInterval  = flag.Duration("poll_interval", 100*time.Millisecond, "Interval to poll NCL leader")
)

// stores resolved metadata for global sequence
type GlobalIndexEntry struct {
	RecordID string
	ShardID  string
}

// finalizes the global log order
type Orderer struct {
	nclClient nclpb.NCLPeerClient

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
	ctx, cancel := context.WithTimeout(context.Background(), 5 * time.Second)
	defer cancel()

	
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

	log.Println("Backround orderer begun. Polling NCL Leader...")
	orderer.Run() //need to write function RUN
}
