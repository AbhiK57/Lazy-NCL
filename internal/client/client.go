package client

import (
	"context"
	"fmt"
	"log"

	ctrlpb "github.com/AbhiK57/Lazy-NCL/proto/controller"
	dspb "github.com/AbhiK57/Lazy-NCL/proto/datashard"
	nclpb "github.com/AbhiK57/Lazy-NCL/proto/ncl"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"
)

// Client for interacting with the system
type Client struct {
	nclQuorumSize int
	nclPeers      []nclpb.NCLPeerClient
	dataShards    []dspb.DataShardClient
	//TODO: Connect to orderer and add read path
}

//Create new Client

func newClient(ctx context.Context, controllerAddr string, dataShardAddrs []string, f int) (*Client, error) {
	//connect to contoller
	ctrlConn, err := grpc.NewClient(controllerAddr, grpc.WithTransportCredentials(insecure.NewCredentials()))
	if err != nil {
		return nil, fmt.Errorf("Failed to connect to controller: %w", err)
	}
	defer ctrlConn.Close()
	ctrlClient := ctrlpb.NewControllerClient(ctrlConn)

	//get log peer list from controller
	resp, err := ctrlClient.GetPeerList(ctx, &ctrlpb.GetPeerListRequest{})
	if err != nil {
		return nil, fmt.Errorf("Failed to get Peer List from controller: %w", err)
	}
	nclPeerAddrs := resp.GetPeerAddresses()
	log.Printf("Found NCL Peers: %v", nclPeerAddrs)

	//connect to all peers
	var nclPeers []nclpb.NCLPeerClient
	for _, addr := range nclPeerAddrs {
		conn, err := grpc.NewClient(addr, grpc.WithTransportCredentials(insecure.NewCredentials()))
		if err != nil {
			//TODO: Impllement more robust error handling
			return nil, fmt.Errorf("Failed to connect to log peer %s: %w", addr, err)
		}

		//TODO: Manage connections' lifecycle
		nclPeers = append(nclPeers, nclpb.NewNCLPeerClient(conn))
	}

	//Connect to data shards
	var dataShards []dspb.DataShardClient
	for _, addr := range dataShardAddrs {
		conn, err := grpc.NewClient(addr, grpc.WithTransportCredentials(insecure.NewCredentials()))
		if err != nil {
			return nil, fmt.Errorf("Failed to connect to data shard %s: %w", addr, err)
		}
		dataShards = append(dataShards, dspb.NewDataShardClient(conn))
	}

	if len(nclPeers) < 2*f+1 {
		return nil, fmt.Errorf("Not enough Log peers for f=%d fault tolerance", f)
	}

	return &Client{
		nclQuorumSize: f + 1,
		nclPeers:      nclPeers,
		dataShards:    dataShards,
	}, nil
}
