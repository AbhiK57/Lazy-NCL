// cmd/data_shard/main.go
package main

import (
	"flag"
	"fmt"
	"net"
	"os"
	"path/filepath"

	"context"
	"log"

	pb "github.com/AbhiK57/Lazy-NCL/proto/datashard"
	"github.com/linxGnu/grocksdb"
	"google.golang.org/grpc"
)

type server struct {
	pb.UnimplementedDataShardServer
	db *grocksdb.DB
}

var (
	port   = flag.Int("port", 50061, "The server port")
	dbPath = flag.String("db_path", "tmp/Lazy-NCL-shard", "Path to the RocksDB database")
)

// create new data shard
func NewServer(rocksdb *grocksdb.DB) *server {
	return &server{db: rocksdb}
}

// store a KV pair in RocksDB
func (s *server) PutData(ctx context.Context, req *pb.PutDataRequest) (*pb.PutDataResponse, error) {
	log.Printf("Received PutData request for record_id %s", req.RecordId)
	wo := grocksdb.NewDefaultWriteOptions()
	defer wo.Destroy()

	err := s.db.Put(wo, []byte(req.RecordId), req.DataPayload)
	if err != nil {
		log.Printf("Failed to put data for record_id %s: %v", req.RecordId, err)
		return nil, err
	}
	return &pb.PutDataResponse{}, nil
}

// Retrieve a value by key from RocksDB
func (s *server) GetData(ctx context.Context, req *pb.GetDataRequest) (*pb.GetDataResponse, error) {
	log.Printf("Received GetData request for record_id: %s", req.RecordId)
	ro := grocksdb.NewDefaultReadOptions()
	defer ro.Destroy()

	value, err := s.db.Get(ro, []byte(req.RecordId))
	if err != nil {
		log.Printf("Failed to get data for record_id %s: %v", req.RecordId, err)
		return nil, err
	}
	if value.Data() == nil {
		return nil, fmt.Errorf("Unable to find record: %s", req.RecordId)
	}
	defer value.Free()

	return &pb.GetDataResponse{DataPayload: value.Data()}, nil
}

func main() {
	flag.Parse()

	//check directory for db exists
	shardDBPath := filepath.Join(*dbPath, fmt.Sprintf("shard-%d", *port))

	if err := os.MkdirAll(shardDBPath, 0755); err != nil {
		log.Fatalf("Failed to create db path %s: %v", shardDBPath, err)
	}

	//set up rocksdb
	opts := grocksdb.NewDefaultOptions()
	opts.SetCreateIfMissing(true)
	defer opts.Destroy()

	db, err := grocksdb.OpenDb(opts, shardDBPath)
	if err != nil {
		log.Fatalf("Failed to open RocksDB: %v", err)
	}
	defer db.Close()

	log.Printf("Sharding starting on port %d, using db path: %s", *port, shardDBPath)

	//GRPC server setup
	lis, err := net.Listen("tcp", fmt.Sprintf(":%d", *port))
	if err != nil {
		log.Fatalf("failed to setup listener on port %d: %v", *port, err)
	}

	s := grpc.NewServer()
	pb.RegisterDataShardServer(s, NewServer(db))

	if err := s.Serve(lis); err != nil {
		log.Fatalf("Failed to serve: %v", err)
	}
}
