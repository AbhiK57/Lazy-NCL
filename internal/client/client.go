package client

import (
	"context"
	"fmt"
	"log"
	"sort"

	ctrlpb "github.com/AbhiK57/Lazy-NCL/proto/controller"
	dspb "github.com/AbhiK57/Lazy-NCL/proto/datashard"
	nclpb "github.com/AbhiK57/Lazy-NCL/proto/ncl"
	ordererpb "github.com/AbhiK57/Lazy-NCL/proto/orderer"
	"github.com/google/uuid"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"
)

// Client for interacting with the system
type Client struct {
	nclQuorumSize int
	nclPeers      []nclpb.NCLPeerClient
	dataShards    map[string]dspb.DataShardClient
	ordererClient ordererpb.OrdererClient
	//TODO: Connect to orderer and add read path
}

//Create new Client

func newClient(ctx context.Context, controllerAddr string, dataShardAddrs []string, f int, ordererAddr string) (*Client, error) {
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
	shardsMap := make(map[string]dspb.DataShardClient)
	for _, addr := range dataShardAddrs {
		conn, err := grpc.NewClient(addr, grpc.WithTransportCredentials(insecure.NewCredentials()))
		if err != nil {
			return nil, fmt.Errorf("Failed to connect to data shard %s: %w", addr, err)
		}
		shardsMap[addr] = dspb.NewDataShardClient(conn)
	}

	//connect to orderer
	ordererConn, err := grpc.NewClient(ordererAddr, grpc.WithTransportCredentials(insecure.NewCredentials()))
	if err != nil {
		return nil, fmt.Errorf("Failed to connect to orderer server: %w", err)
	}
	ordererClient := ordererpb.NewOrdererClient(ordererConn)

	if len(nclPeers) < 2*f+1 {
		return nil, fmt.Errorf("Not enough Log peers for f=%d fault tolerance", f)
	}

	return &Client{
		nclQuorumSize: f + 1,
		nclPeers:      nclPeers,
		dataShards:    shardsMap,
		ordererClient: ordererClient,
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
	keys := make([]string, 0, len(c.dataShards))
	for k := range c.dataShards {
		keys = append(keys, k)
	}
	sort.Strings(keys)

	shardIndex := int(hash(recordID) % uint32(len(keys)))
	shardKey := keys[shardIndex]
	targetShard := c.dataShards[shardKey]

	shardID := shardKey

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

// fetch record via global log position
func (c *Client) Read(ctx context.Context, position int64) ([]byte, error) {
	resolveReq := ordererpb.ResolvePositionRequest{GlobalPosition: position}
	resolveResp, err := c.ordererClient.ResolvePosition(ctx, &resolveReq)

	recordID := resolveResp.GetRecordId()
	shardID := resolveResp.GetShardId() // e.g., "localhost:50061"

	//fetc the data from the correct data shard.
	shardClient := c.dataShards[shardID]

	getReq := &dspb.GetDataRequest{RecordId: recordID}
	getResp, err := shardClient.GetData(ctx, getReq)
	if err != nil {
		return [], fmt.Errorf("could not get data for record %s from shard %s: %w", recordID, shardID, err)
	}

	log.Printf("Successfully read data for position %d (RecordID: %s)", position, recordID)
	return getResp.GetDataPayload(), nil
}
