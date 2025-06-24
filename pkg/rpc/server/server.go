package server

import (
	"context"
	"fmt"
	"net/http"
	"time"

	"connectrpc.com/connect"
	"connectrpc.com/grpcreflect"
	"golang.org/x/net/http2"
	"golang.org/x/net/http2/h2c"
	"google.golang.org/protobuf/types/known/emptypb"
	"google.golang.org/protobuf/types/known/timestamppb"

	"github.com/rollkit/rollkit/pkg/p2p"
	"github.com/rollkit/rollkit/pkg/store"
	"github.com/rollkit/rollkit/types"
	pb "github.com/rollkit/rollkit/types/pb/rollkit/v1"
	rpc "github.com/rollkit/rollkit/types/pb/rollkit/v1/v1connect"
)

// StoreServer implements the StoreService defined in the proto file
type StoreServer struct {
	store store.Store
}

// NewStoreServer creates a new StoreServer instance
func NewStoreServer(store store.Store) *StoreServer {
	return &StoreServer{
		store: store,
	}
}

// GetBlock implements the GetBlock RPC method
func (s *StoreServer) GetBlock(
	ctx context.Context,
	req *connect.Request[pb.GetBlockRequest],
) (*connect.Response[pb.GetBlockResponse], error) {
	var header *types.SignedHeader
	var data *types.Data
	var err error

	switch identifier := req.Msg.Identifier.(type) {
	case *pb.GetBlockRequest_Height:
		fetchHeight := identifier.Height
		if fetchHeight == 0 {
			// Subcase 2a: Height is 0 -> Fetch latest block
			fetchHeight, err = s.store.Height(ctx)
			if err != nil {
				return nil, connect.NewError(connect.CodeInternal, fmt.Errorf("failed to get latest height: %w", err))
			}
			if fetchHeight == 0 {
				return nil, connect.NewError(connect.CodeNotFound, fmt.Errorf("store is empty, no latest block available"))
			}
		}
		// Fetch by the determined height (either specific or latest)
		header, data, err = s.store.GetBlockData(ctx, fetchHeight)

	case *pb.GetBlockRequest_Hash:
		hash := types.Hash(identifier.Hash)
		header, data, err = s.store.GetBlockByHash(ctx, hash)

	default:
		// This case handles potential future identifier types or invalid states
		return nil, connect.NewError(connect.CodeInvalidArgument, fmt.Errorf("invalid or unsupported identifier type provided"))
	}

	if err != nil {
		return nil, connect.NewError(connect.CodeInternal, fmt.Errorf("failed to retrieve block data: %w", err))
	}

	// Convert retrieved types to protobuf types
	pbHeader, err := header.ToProto()
	if err != nil {
		// Error during conversion indicates an issue with the retrieved data or proto definition
		return nil, connect.NewError(connect.CodeInternal, fmt.Errorf("failed to convert block header to proto format: %w", err))
	}
	pbData := data.ToProto() // Assuming data.ToProto() exists and doesn't return an error

	// Return the successful response
	return connect.NewResponse(&pb.GetBlockResponse{
		Block: &pb.Block{
			Header: pbHeader,
			Data:   pbData,
		},
	}), nil
}

// GetState implements the GetState RPC method
func (s *StoreServer) GetState(
	ctx context.Context,
	req *connect.Request[emptypb.Empty],
) (*connect.Response[pb.GetStateResponse], error) {
	state, err := s.store.GetState(ctx)
	if err != nil {
		return nil, connect.NewError(connect.CodeNotFound, err)
	}

	// Convert state to protobuf type
	pbState := &pb.State{
		AppHash:         state.AppHash,
		LastBlockHeight: state.LastBlockHeight,
		LastBlockTime:   timestamppb.New(state.LastBlockTime),
		DaHeight:        state.DAHeight,
		LastResultsHash: state.LastResultsHash,
		ChainId:         state.ChainID,
		Version: &pb.Version{
			Block: state.Version.Block,
			App:   state.Version.App,
		},
		InitialHeight: state.InitialHeight,
	}

	return connect.NewResponse(&pb.GetStateResponse{
		State: pbState,
	}), nil
}

// GetMetadata implements the GetMetadata RPC method
func (s *StoreServer) GetMetadata(
	ctx context.Context,
	req *connect.Request[pb.GetMetadataRequest],
) (*connect.Response[pb.GetMetadataResponse], error) {
	value, err := s.store.GetMetadata(ctx, req.Msg.Key)
	if err != nil {
		return nil, connect.NewError(connect.CodeNotFound, err)
	}

	return connect.NewResponse(&pb.GetMetadataResponse{
		Value: value,
	}), nil
}

// ListMetadataKeys implements the ListMetadataKeys RPC method
func (s *StoreServer) ListMetadataKeys(
	ctx context.Context,
	req *connect.Request[emptypb.Empty],
) (*connect.Response[pb.ListMetadataKeysResponse], error) {
	// Get the known metadata keys with descriptions
	knownKeys := types.GetKnownMetadataKeys()
	
	var metadataKeys []*pb.MetadataKey
	for key, description := range knownKeys {
		metadataKeys = append(metadataKeys, &pb.MetadataKey{
			Key:         key,
			Description: description,
		})
	}

	return connect.NewResponse(&pb.ListMetadataKeysResponse{
		Keys: metadataKeys,
	}), nil
}

// GetAllMetadata implements the GetAllMetadata RPC method
func (s *StoreServer) GetAllMetadata(
	ctx context.Context,
	req *connect.Request[emptypb.Empty],
) (*connect.Response[pb.GetAllMetadataResponse], error) {
	// Get the known metadata keys
	knownKeys := types.GetKnownMetadataKeysList()

	var metadataEntries []*pb.MetadataEntry

	// Retrieve all known metadata
	for _, key := range knownKeys {
		value, err := s.store.GetMetadata(ctx, key)
		if err != nil {
			// Skip keys that don't exist or can't be retrieved
			continue
		}

		metadataEntries = append(metadataEntries, &pb.MetadataEntry{
			Key:   key,
			Value: value,
		})
	}

	return connect.NewResponse(&pb.GetAllMetadataResponse{
		Metadata: metadataEntries,
	}), nil
}

// P2PServer implements the P2PService defined in the proto file
type P2PServer struct {
	// Add dependencies needed for P2P functionality
	peerManager p2p.P2PRPC
}

// NewP2PServer creates a new P2PServer instance
func NewP2PServer(peerManager p2p.P2PRPC) *P2PServer {
	return &P2PServer{
		peerManager: peerManager,
	}
}

// GetPeerInfo implements the GetPeerInfo RPC method
func (p *P2PServer) GetPeerInfo(
	ctx context.Context,
	req *connect.Request[emptypb.Empty],
) (*connect.Response[pb.GetPeerInfoResponse], error) {
	peers, err := p.peerManager.GetPeers()
	if err != nil {
		return nil, connect.NewError(connect.CodeInternal, fmt.Errorf("failed to get peer info: %w", err))
	}

	// Convert to protobuf format
	pbPeers := make([]*pb.PeerInfo, len(peers))
	for i, peer := range peers {
		pbPeers[i] = &pb.PeerInfo{
			Id:      peer.ID.String(),
			Address: peer.String(),
		}
	}

	return connect.NewResponse(&pb.GetPeerInfoResponse{
		Peers: pbPeers,
	}), nil
}

// GetNetInfo implements the GetNetInfo RPC method
func (p *P2PServer) GetNetInfo(
	ctx context.Context,
	req *connect.Request[emptypb.Empty],
) (*connect.Response[pb.GetNetInfoResponse], error) {
	netInfo, err := p.peerManager.GetNetworkInfo()
	if err != nil {
		return nil, connect.NewError(connect.CodeInternal, fmt.Errorf("failed to get network info: %w", err))
	}

	pbNetInfo := &pb.NetInfo{
		Id:              netInfo.ID,
		ListenAddresses: netInfo.ListenAddress,
	}

	return connect.NewResponse(&pb.GetNetInfoResponse{
		NetInfo: pbNetInfo,
	}), nil
}

// HealthServer implements the HealthService defined in the proto file
type HealthServer struct{}

// NewHealthServer creates a new HealthServer instance
func NewHealthServer() *HealthServer {
	return &HealthServer{}
}

// Livez implements the HealthService.Livez RPC
func (h *HealthServer) Livez(
	ctx context.Context,
	req *connect.Request[emptypb.Empty],
) (*connect.Response[pb.GetHealthResponse], error) {
	// always return healthy
	return connect.NewResponse(&pb.GetHealthResponse{
		Status: pb.HealthStatus_PASS,
	}), nil
}

// NewServiceHandler creates a new HTTP handler for Store, P2P and Health services
func NewServiceHandler(store store.Store, peerManager p2p.P2PRPC) (http.Handler, error) {
	storeServer := NewStoreServer(store)
	p2pServer := NewP2PServer(peerManager)
	healthServer := NewHealthServer()

	mux := http.NewServeMux()

	compress1KB := connect.WithCompressMinBytes(1024)
	reflector := grpcreflect.NewStaticReflector(
		rpc.StoreServiceName,
		rpc.P2PServiceName,
		rpc.HealthServiceName,
	)
	mux.Handle(grpcreflect.NewHandlerV1(reflector, compress1KB))
	mux.Handle(grpcreflect.NewHandlerV1Alpha(reflector, compress1KB))

	// Register StoreService
	storePath, storeHandler := rpc.NewStoreServiceHandler(storeServer)
	mux.Handle(storePath, storeHandler)

	// Register P2PService
	p2pPath, p2pHandler := rpc.NewP2PServiceHandler(p2pServer)
	mux.Handle(p2pPath, p2pHandler)

	// Register HealthService
	healthPath, healthHandler := rpc.NewHealthServiceHandler(healthServer)
	mux.Handle(healthPath, healthHandler)

	// Register custom HTTP endpoints
	RegisterCustomHTTPEndpoints(mux)

	// Use h2c to support HTTP/2 without TLS
	return h2c.NewHandler(mux, &http2.Server{
		IdleTimeout:          120 * time.Second,
		MaxReadFrameSize:     1 << 24,
		MaxConcurrentStreams: 100,
		ReadIdleTimeout:      30 * time.Second,
		PingTimeout:          15 * time.Second,
	}), nil
}
