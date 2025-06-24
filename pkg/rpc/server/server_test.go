package server

import (
	"context"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"connectrpc.com/connect"
	"github.com/libp2p/go-libp2p/core/peer"
	"github.com/multiformats/go-multiaddr"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
	"google.golang.org/protobuf/types/known/emptypb"

	"github.com/rollkit/rollkit/pkg/p2p"
	"github.com/rollkit/rollkit/test/mocks"
	"github.com/rollkit/rollkit/types"
	pb "github.com/rollkit/rollkit/types/pb/rollkit/v1"
)

func TestGetBlock(t *testing.T) {
	// Create a mock store
	mockStore := mocks.NewStore(t)

	// Create test data
	height := uint64(10)
	header := &types.SignedHeader{}
	data := &types.Data{}

	// Setup mock expectations
	mockStore.On("GetBlockData", mock.Anything, height).Return(header, data, nil)

	// Create server with mock store
	server := NewStoreServer(mockStore)

	// Test GetBlock with height
	t.Run("by height", func(t *testing.T) {
		req := connect.NewRequest(&pb.GetBlockRequest{
			Identifier: &pb.GetBlockRequest_Height{
				Height: height,
			},
		})
		resp, err := server.GetBlock(context.Background(), req)

		// Assert expectations
		require.NoError(t, err)
		require.NotNil(t, resp.Msg.Block)
		mockStore.AssertExpectations(t)
	})

	// Test GetBlock with hash
	t.Run("by hash", func(t *testing.T) {
		hash := []byte("test_hash")
		mockStore.On("GetBlockByHash", mock.Anything, hash).Return(header, data, nil)

		req := connect.NewRequest(&pb.GetBlockRequest{
			Identifier: &pb.GetBlockRequest_Hash{
				Hash: hash,
			},
		})
		resp, err := server.GetBlock(context.Background(), req)

		// Assert expectations
		require.NoError(t, err)
		require.NotNil(t, resp.Msg.Block)
		mockStore.AssertExpectations(t)
	})
}

func TestGetBlock_Latest(t *testing.T) {
	mockStore := mocks.NewStore(t)
	server := NewStoreServer(mockStore)

	header := &types.SignedHeader{}
	data := &types.Data{}

	// Expectation for GetHeight (which should be called by GetLatestBlockHeight)
	mockStore.On("Height", context.Background()).Return(uint64(20), nil).Once()
	// Expectation for GetBlockData with the latest height
	mockStore.On("GetBlockData", context.Background(), uint64(20)).Return(header, data, nil).Once()

	req := connect.NewRequest(&pb.GetBlockRequest{
		Identifier: &pb.GetBlockRequest_Height{
			Height: 0, // Indicates latest block
		},
	})
	resp, err := server.GetBlock(context.Background(), req)
	require.NoError(t, err)
	require.NotNil(t, resp.Msg.Block)
	mockStore.AssertExpectations(t)
}

func TestGetState(t *testing.T) {
	// Create a mock store
	mockStore := mocks.NewStore(t)

	// Create test data
	state := types.State{
		AppHash:         []byte("app_hash"),
		InitialHeight:   10,
		LastBlockHeight: 10,
		LastBlockTime:   time.Now(),
		ChainID:         "test-chain",
		Version: types.Version{
			Block: 1,
			App:   1,
		},
	}

	// Setup mock expectations
	mockStore.On("GetState", mock.Anything).Return(state, nil)

	// Create server with mock store
	server := NewStoreServer(mockStore)

	// Call GetState
	req := connect.NewRequest(&emptypb.Empty{})
	resp, err := server.GetState(context.Background(), req)

	// Assert expectations
	require.NoError(t, err)
	require.NotNil(t, resp.Msg.State)
	require.Equal(t, state.AppHash, resp.Msg.State.AppHash)
	require.Equal(t, state.InitialHeight, resp.Msg.State.InitialHeight)
	require.Equal(t, state.LastBlockHeight, resp.Msg.State.LastBlockHeight)
	require.Equal(t, state.LastBlockTime.UTC(), resp.Msg.State.LastBlockTime.AsTime())
	require.Equal(t, state.ChainID, resp.Msg.State.ChainId)
	require.Equal(t, state.Version.Block, resp.Msg.State.Version.Block)
	require.Equal(t, state.Version.App, resp.Msg.State.Version.App)
	mockStore.AssertExpectations(t)
}

func TestGetState_Error(t *testing.T) {
	mockStore := mocks.NewStore(t)
	mockStore.On("GetState", mock.Anything).Return(types.State{}, fmt.Errorf("state error"))
	server := NewStoreServer(mockStore)
	resp, err := server.GetState(context.Background(), connect.NewRequest(&emptypb.Empty{}))
	require.Error(t, err)
	require.Nil(t, resp)
}

func TestGetMetadata(t *testing.T) {
	// Create a mock store
	mockStore := mocks.NewStore(t)

	// Create test data
	key := "test_key"
	value := []byte("test_value")

	// Setup mock expectations
	mockStore.On("GetMetadata", mock.Anything, key).Return(value, nil)

	// Create server with mock store
	server := NewStoreServer(mockStore)

	// Call GetMetadata
	req := connect.NewRequest(&pb.GetMetadataRequest{
		Key: key,
	})
	resp, err := server.GetMetadata(context.Background(), req)

	// Assert expectations
	require.NoError(t, err)
	require.Equal(t, value, resp.Msg.Value)
	mockStore.AssertExpectations(t)
}

func TestGetMetadata_Error(t *testing.T) {
	mockStore := mocks.NewStore(t)
	mockStore.On("GetMetadata", mock.Anything, "bad").Return(nil, fmt.Errorf("meta error"))
	server := NewStoreServer(mockStore)
	resp, err := server.GetMetadata(context.Background(), connect.NewRequest(&pb.GetMetadataRequest{Key: "bad"}))
	require.Error(t, err)
	require.Nil(t, resp)
}

func TestP2PServer_GetPeerInfo(t *testing.T) {
	mockP2P := &mocks.P2PRPC{}
	addr, err := multiaddr.NewMultiaddr("/ip4/127.0.0.1/tcp/4001")
	require.NoError(t, err)
	mockP2P.On("GetPeers").Return([]peer.AddrInfo{{ID: "id1", Addrs: []multiaddr.Multiaddr{addr}}}, nil)
	server := NewP2PServer(mockP2P)
	resp, err := server.GetPeerInfo(context.Background(), connect.NewRequest(&emptypb.Empty{}))
	require.NoError(t, err)
	require.Len(t, resp.Msg.Peers, 1)
	mockP2P.AssertExpectations(t)

	// Error case
	mockP2P2 := &mocks.P2PRPC{}
	mockP2P2.On("GetPeers").Return(nil, fmt.Errorf("p2p error"))
	server2 := NewP2PServer(mockP2P2)
	resp2, err2 := server2.GetPeerInfo(context.Background(), connect.NewRequest(&emptypb.Empty{}))
	require.Error(t, err2)
	require.Nil(t, resp2)
}

func TestP2PServer_GetNetInfo(t *testing.T) {
	mockP2P := &mocks.P2PRPC{}
	netInfo := p2p.NetworkInfo{ID: "nid", ListenAddress: []string{"addr1"}}
	mockP2P.On("GetNetworkInfo").Return(netInfo, nil)
	server := NewP2PServer(mockP2P)
	resp, err := server.GetNetInfo(context.Background(), connect.NewRequest(&emptypb.Empty{}))
	require.NoError(t, err)
	require.Equal(t, netInfo.ID, resp.Msg.NetInfo.Id)
	mockP2P.AssertExpectations(t)

	// Error case
	mockP2P2 := &mocks.P2PRPC{}
	mockP2P2.On("GetNetworkInfo").Return(p2p.NetworkInfo{}, fmt.Errorf("netinfo error"))
	server2 := NewP2PServer(mockP2P2)
	resp2, err2 := server2.GetNetInfo(context.Background(), connect.NewRequest(&emptypb.Empty{}))
	require.Error(t, err2)
	require.Nil(t, resp2)
}

func TestHealthServer_Livez(t *testing.T) {
	h := NewHealthServer()
	resp, err := h.Livez(context.Background(), connect.NewRequest(&emptypb.Empty{}))
	require.NoError(t, err)
	require.Equal(t, pb.HealthStatus_PASS, resp.Msg.Status)
}

func TestStoreServer_ListMetadataKeys(t *testing.T) {
	// Create a mock store
	mockStore := mocks.NewStore(t)

	// Create server with mock store
	server := NewStoreServer(mockStore)

	// Test ListMetadataKeys
	req := connect.NewRequest(&emptypb.Empty{})
	resp, err := server.ListMetadataKeys(context.Background(), req)

	// Assert expectations
	require.NoError(t, err)
	require.NotNil(t, resp)
	require.NotNil(t, resp.Msg)
	require.NotEmpty(t, resp.Msg.Keys)

	// Check that we get the expected metadata keys
	expectedKeys := types.GetKnownMetadataKeys()

	require.Len(t, resp.Msg.Keys, len(expectedKeys))

	for _, metadataKey := range resp.Msg.Keys {
		expectedDesc, exists := expectedKeys[metadataKey.Key]
		require.True(t, exists, "Unexpected key: %s", metadataKey.Key)
		require.Equal(t, expectedDesc, metadataKey.Description)
	}
}

func TestStoreServer_GetAllMetadata(t *testing.T) {
	// Create a mock store
	mockStore := mocks.NewStore(t)

	// Setup mock expectations for metadata retrieval
	testData := map[string][]byte{
		types.DAIncludedHeightKey:             {0x01, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00}, // height 1 as bytes
		types.LastBatchDataKey:               []byte("batch_data"),
		types.LastSubmittedHeaderHeightKey:   {0x02, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00}, // height 2 as bytes
		types.LastSubmittedDataHeightKey:     {0x03, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00}, // height 3 as bytes
	}

	for key, value := range testData {
		mockStore.On("GetMetadata", mock.Anything, key).Return(value, nil)
	}

	// Create server with mock store
	server := NewStoreServer(mockStore)

	// Test GetAllMetadata
	req := connect.NewRequest(&emptypb.Empty{})
	resp, err := server.GetAllMetadata(context.Background(), req)

	// Assert expectations
	require.NoError(t, err)
	require.NotNil(t, resp)
	require.NotNil(t, resp.Msg)
	require.Len(t, resp.Msg.Metadata, len(testData))

	// Verify the returned metadata matches expected data
	returnedData := make(map[string][]byte)
	for _, entry := range resp.Msg.Metadata {
		returnedData[entry.Key] = entry.Value
	}

	for key, expectedValue := range testData {
		actualValue, exists := returnedData[key]
		require.True(t, exists, "Missing key: %s", key)
		require.Equal(t, expectedValue, actualValue, "Value mismatch for key: %s", key)
	}

	mockStore.AssertExpectations(t)
}

func TestStoreServer_GetAllMetadata_WithMissingKeys(t *testing.T) {
	// Create a mock store
	mockStore := mocks.NewStore(t)

	// Setup mock expectations - some keys exist, some don't
	testData := map[string][]byte{
		types.DAIncludedHeightKey: {0x01, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00}, // height 1 as bytes
		types.LastBatchDataKey:   []byte("batch_data"),
	}

	// Mock successful calls for existing keys
	for key, value := range testData {
		mockStore.On("GetMetadata", mock.Anything, key).Return(value, nil)
	}

	// Mock failed calls for non-existing keys
	mockStore.On("GetMetadata", mock.Anything, types.LastSubmittedHeaderHeightKey).Return(nil, fmt.Errorf("key not found"))
	mockStore.On("GetMetadata", mock.Anything, types.LastSubmittedDataHeightKey).Return(nil, fmt.Errorf("key not found"))

	// Create server with mock store
	server := NewStoreServer(mockStore)

	// Test GetAllMetadata
	req := connect.NewRequest(&emptypb.Empty{})
	resp, err := server.GetAllMetadata(context.Background(), req)

	// Assert expectations - should succeed even with missing keys
	require.NoError(t, err)
	require.NotNil(t, resp)
	require.NotNil(t, resp.Msg)
	require.Len(t, resp.Msg.Metadata, len(testData)) // Only existing keys should be returned

	// Verify the returned metadata matches expected data
	returnedData := make(map[string][]byte)
	for _, entry := range resp.Msg.Metadata {
		returnedData[entry.Key] = entry.Value
	}

	for key, expectedValue := range testData {
		actualValue, exists := returnedData[key]
		require.True(t, exists, "Missing key: %s", key)
		require.Equal(t, expectedValue, actualValue, "Value mismatch for key: %s", key)
	}

	mockStore.AssertExpectations(t)
}

func TestHealthLiveEndpoint(t *testing.T) {
	assert := require.New(t)

	// Create mock dependencies
	mockStore := mocks.NewStore(t)
	mockP2PManager := &mocks.P2PRPC{} // Assuming this mock is sufficient or can be adapted

	// Create the service handler
	handler, err := NewServiceHandler(mockStore, mockP2PManager)
	assert.NoError(err)
	assert.NotNil(handler)

	// Create a new HTTP test server
	server := httptest.NewServer(handler)
	defer server.Close()

	// Make a GET request to the /health/live endpoint
	resp, err := http.Get(server.URL + "/health/live")
	assert.NoError(err)
	defer resp.Body.Close()

	// Check the status code
	assert.Equal(http.StatusOK, resp.StatusCode)

	// Check the response body
	body, err := io.ReadAll(resp.Body)
	assert.NoError(err)
	assert.Equal("OK\n", string(body)) // fmt.Fprintln adds a newline
}
