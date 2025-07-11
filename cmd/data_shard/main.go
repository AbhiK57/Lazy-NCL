// cmd/data_shard/main.go
package main

import (
	"flag"

	"github.com/linxGnu/grocksdb"
	pb "github.com/AbhiK57/Lazy-NCL/proto/datashard"
)


var (
	port = flag.Int("port", 50061, "The server port")
	dbPath = flag.String("db_path", "tmp/Lazy-NCL-shard", "Path to the RocksDB database")
)

type server struct {
	pb.UnimplementedDataShardServer
	db *grocksdb.DB
}