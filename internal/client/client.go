package client

import (
	"context"
	"fmt"
	"log"

	ctrlpb "github.com/AbhiK57/Lazy-NCL/proto/controller"
	dspb "github.com/AbhiK57/Lazy-NCL/proto/datashard"
	nclpb "github.com/AbhiK57/Lazy-NCL/proto/ncl"
	"github.com/google/uuid"
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

// simple non-cryptographic hash for sharding.
func hash(s string) uint32 {
	h := uint32(2166136261)
	for i := 0; i < len(s); i++ {
		h = h ^ uint32(s[i])
		h = h * 16777619
	}
	return h
}

// Performs the 1-RTT split write to data and log layers
func (c *Client) Append(ctx context.Context, data []byte) (string, error) {
	recordID := uuid.New().String()
	//hash the record id
	shardIndex := int(hash(recordID) % uint32(len(c.dataShards)))
	targetShard := c.dataShards[shardIndex]
	shardID := fmt.Sprintf("shard-%d", shardIndex) //TODO: get this from config

	metadata := &nclpb.Metadata{
		RecordId: recordID,
		ShardId:  shardID,
	}

	//dispatch in parallel
	//use channel to collect goroutine errors
	errChannel := make(chan error, len(c.nclPeers)+1)
	//channel to count successful log acknowledgements
	nclAckChannel := make(chan struct{}, len(c.nclPeers))
	//channel for the single data acknowledgement
	dataAckChannel := make(chan struct{}, 1)

	//data dispatch
	go func() {
		req := &dspb.PutDataRequest{RecordId: recordID, DataPayload: data}
		_, err := targetShard.PutData(ctx, req)
		if err != nil {
			errChannel <- fmt.Errorf("Data path write failed: %w", err)
			return
		}
		dataAckChannel <- struct{}{}
	}()

	//metadata dispatch
	for _, peer := range c.nclPeers {
		go func(p nclpb.NCLPeerClient) {
			req := &nclpb.AppendMetadataRequest{Meta: metadata}
			_, err := p.AppendMetadata(ctx, req)
			if err != nil {
				//some failures are expected, need to reach a quorum of successes
				log.Printf("NCL Peer write failed: %v", err)
				return
			}
			nclAckChannel <- struct{}{}
		}(peer)
	}

	return recordID, c.waitForQuorum(ctx, dataAckChannel, nclAckChannel, errChannel)
}

func (c *Client) waitForQuorum(ctx context.Context, dataAckChannel <-chan struct{}, nclAckChannel <-chan struct{}, errChannel <-chan error) error {
	nclAcks := 0
	dataAck := false

	//loop until quorum
	for {
		if dataAck && nclAcks >= c.nclQuorumSize {
			log.Printf("Quorum reached (Data: %t, Logs: %d/%d)", dataAck, nclAcks, c.nclQuorumSize)
		}

		select {
		case <-nclAckChannel:
			nclAcks++
		case <-dataAckChannel:
			dataAck = true
		case err := <-errChannel:
			//return error on first fatal error
			//TODO: Implement full complexity for tolerating certain errors
			return err
		case <-ctx.Done():
			return fmt.Errorf("Context cancelled while waiting for quorum. Quorum Reached (Data: %t, Logs: %d/%d)", dataAck, nclAcks, c.nclQuorumSize)
		}
	}
}
