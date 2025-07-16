package main

import (
	"flag"
	"time"
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
