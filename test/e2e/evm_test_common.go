//go:build evm
// +build evm

// Package e2e contains shared utilities for EVM end-to-end tests.
//
// This file provides common functionality used across multiple EVM test files:
// - Docker and JWT setup for Reth EVM engines
// - Sequencer and full node initialization
// - P2P connection management
// - Transaction submission and verification utilities
// - Common constants and configuration values
//
// By centralizing these utilities, we eliminate code duplication and ensure
// consistent behavior across all EVM integration tests.
package e2e

import (
	"context"
	"encoding/hex"
	"fmt"
	"math/big"
	"net/http"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"testing"
	"time"

	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/ethclient"
	"github.com/golang-jwt/jwt/v5"
	"github.com/stretchr/testify/require"

	"github.com/rollkit/rollkit/execution/evm"
)

// Common constants used across EVM tests
const (
	// Docker configuration
	dockerPath = "../../execution/evm/docker"

	// Port configurations
	SequencerEthPort    = "8545"
	SequencerEnginePort = "8551"
	FullNodeEthPort     = "8555"
	FullNodeEnginePort  = "8561"
	DAPort              = "7980"
	RollkitRPCPort      = "7331"
	RollkitP2PPort      = "7676"
	FullNodeP2PPort     = "7677"
	FullNodeRPCPort     = "46657"

	// URL templates
	SequencerEthURL    = "http://localhost:" + SequencerEthPort
	SequencerEngineURL = "http://localhost:" + SequencerEnginePort
	FullNodeEthURL     = "http://localhost:" + FullNodeEthPort
	FullNodeEngineURL  = "http://localhost:" + FullNodeEnginePort
	DAAddress          = "http://localhost:" + DAPort
	RollkitRPCAddress  = "http://127.0.0.1:" + RollkitRPCPort

	// Test configuration
	DefaultBlockTime   = "1s"
	DefaultDABlockTime = "1m"
	DefaultTestTimeout = 30 * time.Second
	DefaultChainID     = "1234"
	DefaultGasLimit    = 22000

	// Test account configuration
	TestPrivateKey = "cece4f25ac74deb1468965160c7185e07dff413f23fcadb611b05ca37ab0a52e"
	TestToAddress  = "0x944fDcD1c868E3cC566C78023CcB38A32cDA836E"
	TestPassphrase = "secret"
)

// setupTestRethEngineE2E sets up a Reth EVM engine for E2E testing using Docker Compose.
// This creates the sequencer's EVM instance on standard ports (8545/8551).
//
// Returns: JWT secret string for authenticating with the EVM engine
func setupTestRethEngineE2E(t *testing.T) string {
	return evm.SetupTestRethEngine(t, dockerPath, "jwt.hex")
}

// setupTestRethEngineFullNode sets up a Reth EVM engine for full node testing.
// This creates a separate EVM instance using docker-compose-full-node.yml with:
// - Different ports (8555/8561) to avoid conflicts with sequencer
// - Separate JWT token generation and management
// - Independent Docker network and volumes
//
// Returns: JWT secret string for authenticating with the full node's EVM engine
func setupTestRethEngineFullNode(t *testing.T) string {
	jwtSecretHex := evm.SetupTestRethEngineFullNode(t, dockerPath, "jwt.hex")

	err := waitForRethContainerAt(t, jwtSecretHex, FullNodeEthURL, FullNodeEngineURL)
	require.NoError(t, err, "Reth container should be ready at full node ports")

	return jwtSecretHex
}

// decodeSecret decodes a hex-encoded JWT secret string into a byte slice.
func decodeSecret(jwtSecret string) ([]byte, error) {
	secret, err := hex.DecodeString(strings.TrimPrefix(jwtSecret, "0x"))
	if err != nil {
		return nil, fmt.Errorf("failed to decode JWT secret: %w", err)
	}
	return secret, nil
}

// getAuthToken creates a JWT token signed with the provided secret, valid for 1 hour.
func getAuthToken(jwtSecret []byte) (string, error) {
	token := jwt.NewWithClaims(jwt.SigningMethodHS256, jwt.MapClaims{
		"exp": time.Now().Add(time.Hour * 1).Unix(), // Expires in 1 hour
		"iat": time.Now().Unix(),
	})

	// Sign the token with the decoded secret
	authToken, err := token.SignedString(jwtSecret)
	if err != nil {
		return "", fmt.Errorf("failed to sign JWT token: %w", err)
	}
	return authToken, nil
}

// waitForRethContainerAt waits for the Reth container to be ready by polling HTTP endpoints.
// This function polls both the ETH JSON-RPC endpoint and the Engine API endpoint with JWT authentication
// to ensure both are fully ready before proceeding with tests.
//
// Parameters:
// - jwtSecret: JWT secret for engine authentication
// - ethURL: HTTP endpoint for ETH JSON-RPC calls (e.g., http://localhost:8545)
// - engineURL: HTTP endpoint for Engine API calls (e.g., http://localhost:8551)
//
// Returns: Error if timeout occurs, nil if both endpoints become ready
func waitForRethContainerAt(t *testing.T, jwtSecret, ethURL, engineURL string) error {
	t.Helper()
	client := &http.Client{Timeout: 100 * time.Millisecond}
	timer := time.NewTimer(DefaultTestTimeout)
	defer timer.Stop()
	for {
		select {
		case <-timer.C:
			return fmt.Errorf("timeout waiting for reth container to be ready")
		default:
			// Check ETH RPC endpoint
			rpcReq := strings.NewReader(`{"jsonrpc":"2.0","method":"net_version","params":[],"id":1}`)
			resp, err := client.Post(ethURL, "application/json", rpcReq)
			if err == nil && resp.StatusCode == http.StatusOK {
				if err := resp.Body.Close(); err != nil {
					return fmt.Errorf("failed to close response body: %w", err)
				}

				// Also check the engine URL with JWT authentication
				req, err := http.NewRequest("POST", engineURL, strings.NewReader(`{"jsonrpc":"2.0","method":"engine_getClientVersionV1","params":[],"id":1}`))
				if err != nil {
					return err
				}
				req.Header.Set("Content-Type", "application/json")
				secret, err := decodeSecret(jwtSecret)
				if err != nil {
					return err
				}
				authToken, err := getAuthToken(secret)
				if err != nil {
					return err
				}
				req.Header.Set("Authorization", fmt.Sprintf("Bearer %s", authToken))
				resp, err := client.Do(req)
				if err == nil && resp.StatusCode == http.StatusOK {
					if err := resp.Body.Close(); err != nil {
						return fmt.Errorf("failed to close response body: %w", err)
					}
					return nil
				}
			}
			time.Sleep(100 * time.Millisecond)
		}
	}
}

// extractP2PID extracts the P2P ID from sequencer logs for establishing peer connections.
// This function handles complex scenarios including:
// - P2P IDs split across multiple log lines due to terminal output wrapping
// - Multiple regex patterns to catch different log formats
// - Fallback to deterministic test P2P ID when sequencer P2P isn't active yet
//
// Returns: A valid P2P ID string that can be used for peer connections
func extractP2PID(t *testing.T, sut *SystemUnderTest) string {
	t.Helper()

	var p2pID string
	p2pRegex := regexp.MustCompile(`listening on address=/ip4/127\.0\.0\.1/tcp/7676/p2p/([A-Za-z0-9]+)`)
	p2pIDRegex := regexp.MustCompile(`/p2p/([A-Za-z0-9]+)`)

	// Use require.Eventually to poll for P2P ID log message instead of hardcoded sleep
	require.Eventually(t, func() bool {
		var allLogLines []string

		// Collect all available logs from both buffers
		sut.outBuff.Do(func(v any) {
			if v != nil {
				line := v.(string)
				allLogLines = append(allLogLines, line)
				if matches := p2pRegex.FindStringSubmatch(line); len(matches) == 2 {
					p2pID = matches[1]
				}
			}
		})

		sut.errBuff.Do(func(v any) {
			if v != nil {
				line := v.(string)
				allLogLines = append(allLogLines, line)
				if matches := p2pRegex.FindStringSubmatch(line); len(matches) == 2 {
					p2pID = matches[1]
				}
			}
		})

		// Handle split lines by combining logs and trying different patterns
		if p2pID == "" {
			combinedLogs := strings.Join(allLogLines, "")
			if matches := p2pRegex.FindStringSubmatch(combinedLogs); len(matches) == 2 {
				p2pID = matches[1]
			} else if matches := p2pIDRegex.FindStringSubmatch(combinedLogs); len(matches) == 2 {
				p2pID = matches[1]
			}
		}

		// Return true if P2P ID found, false to continue polling
		return p2pID != ""
	}, 10*time.Second, 200*time.Millisecond, "P2P ID should be available in sequencer logs")

	// If P2P ID found in logs, use it (this would be the ideal case)
	if p2pID != "" {
		t.Logf("Successfully extracted P2P ID from logs: %s", p2pID)
		return p2pID
	}

	// Pragmatic approach: The sequencer doesn't start P2P services until there are peers
	// Generate a deterministic P2P ID for the test
	fallbackID := "12D3KooWSequencerTestNode123456789012345678901234567890"
	t.Logf("⚠️  Failed to extract P2P ID from sequencer logs, using fallback test P2P ID: %s", fallbackID)
	t.Logf("⚠️  This indicates that P2P ID logging may have changed or failed - please verify log parsing is working correctly")

	return fallbackID
}

// setupSequencerNode initializes and starts the sequencer node with proper configuration.
// This function handles:
// - Node initialization with aggregator mode enabled
// - Sequencer-specific configuration (block time, DA layer connection)
// - JWT authentication setup for EVM engine communication
// - Waiting for node to become responsive on the RPC endpoint
//
// Parameters:
// - sequencerHome: Directory path for sequencer node data
// - jwtSecret: JWT secret for authenticating with EVM engine
// - genesisHash: Hash of the genesis block for chain validation
func setupSequencerNode(t *testing.T, sut *SystemUnderTest, sequencerHome, jwtSecret, genesisHash string) {
	t.Helper()

	// Initialize sequencer node
	output, err := sut.RunCmd(evmSingleBinaryPath,
		"init",
		"--rollkit.node.aggregator=true",
		"--rollkit.signer.passphrase", TestPassphrase,
		"--home", sequencerHome,
	)
	require.NoError(t, err, "failed to init sequencer", output)

	// Start sequencer node
	sut.ExecCmd(evmSingleBinaryPath,
		"start",
		"--evm.jwt-secret", jwtSecret,
		"--evm.genesis-hash", genesisHash,
		"--rollkit.node.block_time", DefaultBlockTime,
		"--rollkit.node.aggregator=true",
		"--rollkit.signer.passphrase", TestPassphrase,
		"--home", sequencerHome,
		"--rollkit.da.address", DAAddress,
		"--rollkit.da.block_time", DefaultDABlockTime,
	)
	sut.AwaitNodeUp(t, RollkitRPCAddress, 10*time.Second)
}

// setupFullNode initializes and starts the full node with P2P connection to sequencer.
// This function handles:
// - Full node initialization (non-aggregator mode)
// - Genesis file copying from sequencer to ensure chain consistency
// - P2P configuration to connect with the sequencer node
// - Different EVM engine ports (8555/8561) to avoid conflicts
// - DA layer connection for long-term data availability
//
// Parameters:
// - fullNodeHome: Directory path for full node data
// - sequencerHome: Directory path of sequencer (for genesis file copying)
// - fullNodeJwtSecret: JWT secret for full node's EVM engine
// - genesisHash: Hash of the genesis block for chain validation
// - p2pID: P2P ID of the sequencer node to connect to
func setupFullNode(t *testing.T, sut *SystemUnderTest, fullNodeHome, sequencerHome, fullNodeJwtSecret, genesisHash, p2pID string) {
	t.Helper()

	// Initialize full node
	output, err := sut.RunCmd(evmSingleBinaryPath,
		"init",
		"--home", fullNodeHome,
	)
	require.NoError(t, err, "failed to init full node", output)

	// Copy genesis file from sequencer to full node
	sequencerGenesis := filepath.Join(sequencerHome, "config", "genesis.json")
	fullNodeGenesis := filepath.Join(fullNodeHome, "config", "genesis.json")
	genesisData, err := os.ReadFile(sequencerGenesis)
	require.NoError(t, err, "failed to read sequencer genesis file")
	err = os.WriteFile(fullNodeGenesis, genesisData, 0644)
	require.NoError(t, err, "failed to write full node genesis file")

	// Start full node
	sut.ExecCmd(evmSingleBinaryPath,
		"start",
		"--home", fullNodeHome,
		"--evm.jwt-secret", fullNodeJwtSecret,
		"--evm.genesis-hash", genesisHash,
		"--rollkit.rpc.address", "127.0.0.1:"+FullNodeRPCPort,
		"--rollkit.p2p.listen_address", "/ip4/127.0.0.1/tcp/"+FullNodeP2PPort,
		"--rollkit.p2p.peers", "/ip4/127.0.0.1/tcp/"+RollkitP2PPort+"/p2p/"+p2pID,
		"--evm.engine-url", FullNodeEngineURL,
		"--evm.eth-url", FullNodeEthURL,
		"--rollkit.da.address", DAAddress,
		"--rollkit.da.block_time", DefaultDABlockTime,
	)
	sut.AwaitNodeUp(t, "http://127.0.0.1:"+FullNodeRPCPort, 10*time.Second)
}

// Global nonce counter to ensure unique nonces across multiple transaction submissions
var globalNonce uint64 = 0

// submitTransactionAndGetBlockNumber submits a transaction to the sequencer and returns inclusion details.
// This function:
// - Creates a random transaction with proper nonce sequencing
// - Submits it to the sequencer's EVM endpoint
// - Waits for the transaction to be included in a block
// - Returns both the transaction hash and the block number where it was included
//
// Returns:
// - Transaction hash for later verification
// - Block number where the transaction was included
//
// This is used in full node sync tests to verify that both nodes
// include the same transaction in the same block number.
func submitTransactionAndGetBlockNumber(t *testing.T, sequencerClient *ethclient.Client) (common.Hash, uint64) {
	t.Helper()

	// Submit transaction to sequencer EVM with unique nonce
	tx := evm.GetRandomTransaction(t, TestPrivateKey, TestToAddress, DefaultChainID, DefaultGasLimit, &globalNonce)
	evm.SubmitTransaction(t, tx)

	// Wait for transaction to be included and get block number
	ctx := context.Background()
	var txBlockNumber uint64
	require.Eventually(t, func() bool {
		receipt, err := sequencerClient.TransactionReceipt(ctx, tx.Hash())
		if err == nil && receipt != nil && receipt.Status == 1 {
			txBlockNumber = receipt.BlockNumber.Uint64()
			return true
		}
		return false
	}, 20*time.Second, 1*time.Second)

	return tx.Hash(), txBlockNumber
}

// setupCommonEVMTest performs common setup for EVM tests including DA and EVM engine initialization.
// This helper reduces code duplication across multiple test functions.
//
// Parameters:
// - needsFullNode: whether to set up a full node EVM engine in addition to sequencer
//
// Returns: jwtSecret, fullNodeJwtSecret (empty if needsFullNode=false), genesisHash
func setupCommonEVMTest(t *testing.T, sut *SystemUnderTest, needsFullNode bool) (string, string, string) {
	t.Helper()

	// Reset global nonce for each test to ensure clean state
	globalNonce = 0

	// Start local DA
	localDABinary := "local-da"
	if evmSingleBinaryPath != "evm-single" {
		localDABinary = filepath.Join(filepath.Dir(evmSingleBinaryPath), "local-da")
	}
	sut.ExecCmd(localDABinary)
	t.Log("Started local DA")
	time.Sleep(100 * time.Millisecond)

	// Start EVM engines
	jwtSecret := setupTestRethEngineE2E(t)
	var fullNodeJwtSecret string
	if needsFullNode {
		fullNodeJwtSecret = setupTestRethEngineFullNode(t)
	}

	// Get genesis hash
	genesisHash := evm.GetGenesisHash(t)
	t.Logf("Genesis hash: %s", genesisHash)

	return jwtSecret, fullNodeJwtSecret, genesisHash
}

// checkTxIncludedAt checks if a transaction was included in a block at the specified EVM endpoint.
// This utility function connects to the provided EVM endpoint and queries for the
// transaction receipt to determine if the transaction was successfully included.
//
// Parameters:
// - txHash: Hash of the transaction to check
// - ethURL: EVM endpoint URL to query (e.g., http://localhost:8545)
//
// Returns: true if transaction is included with success status, false otherwise
func checkTxIncludedAt(t *testing.T, txHash common.Hash, ethURL string) bool {
	t.Helper()
	rpcClient, err := ethclient.Dial(ethURL)
	if err != nil {
		return false
	}
	defer rpcClient.Close()
	receipt, err := rpcClient.TransactionReceipt(context.Background(), txHash)
	return err == nil && receipt != nil && receipt.Status == 1
}

// checkBlockInfoAt retrieves block information at a specific height including state root.
// This function connects to the specified EVM endpoint and queries for the block header
// to get the block hash, state root, transaction count, and other block metadata.
//
// Parameters:
// - ethURL: EVM endpoint URL to query (e.g., http://localhost:8545)
// - blockHeight: Height of the block to retrieve (use nil for latest)
//
// Returns: block hash, state root, transaction count, block number, and error
func checkBlockInfoAt(t *testing.T, ethURL string, blockHeight *uint64) (common.Hash, common.Hash, int, uint64, error) {
	t.Helper()

	ctx := context.Background()
	ethClient, err := ethclient.Dial(ethURL)
	if err != nil {
		return common.Hash{}, common.Hash{}, 0, 0, fmt.Errorf("failed to create ethereum client: %w", err)
	}
	defer ethClient.Close()

	var blockNumber *big.Int
	if blockHeight != nil {
		blockNumber = new(big.Int).SetUint64(*blockHeight)
	}

	// Get the block header
	header, err := ethClient.HeaderByNumber(ctx, blockNumber)
	if err != nil {
		return common.Hash{}, common.Hash{}, 0, 0, fmt.Errorf("failed to get block header: %w", err)
	}

	blockHash := header.Hash()
	stateRoot := header.Root
	blockNum := header.Number.Uint64()

	// Get the full block to count transactions
	block, err := ethClient.BlockByNumber(ctx, header.Number)
	if err != nil {
		return blockHash, stateRoot, 0, blockNum, fmt.Errorf("failed to get full block: %w", err)
	}

	txCount := len(block.Transactions())
	return blockHash, stateRoot, txCount, blockNum, nil
}

// min returns the minimum of two uint64 values
func min(a, b uint64) uint64 {
	if a < b {
		return a
	}
	return b
}
