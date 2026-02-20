package scanner

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/Fantasim/hdpay/internal/config"
	"github.com/Fantasim/hdpay/internal/models"
)

// newSolanaRPCTestProvider creates a SolanaRPCProvider connected to a mock HTTP server.
func newSolanaRPCTestProvider(t *testing.T, handler http.HandlerFunc) (*SolanaRPCProvider, *httptest.Server) {
	t.Helper()

	server := httptest.NewServer(handler)

	rl := NewRateLimiter("test", 100)
	provider := &SolanaRPCProvider{
		client: server.Client(),
		rl:     rl,
		rpcURL: server.URL,
		name:   "TestSolana",
	}

	return provider, server
}

func TestSolanaRPCProvider_NativeBalanceSuccess(t *testing.T) {
	provider, server := newSolanaRPCTestProvider(t, func(w http.ResponseWriter, r *http.Request) {
		resp := solanaRPCResponse{
			JSONRPC: "2.0",
			ID:      1,
			Result: &struct {
				Context struct {
					Slot uint64 `json:"slot"`
				} `json:"context"`
				Value []json.RawMessage `json:"value"`
			}{
				Value: []json.RawMessage{
					json.RawMessage(`{"lamports": 5000000000, "owner": "11111111111111111111111111111111", "executable": false}`),
				},
			},
		}
		json.NewEncoder(w).Encode(resp)
	})
	defer server.Close()

	addresses := []models.Address{
		{Chain: models.ChainSOL, AddressIndex: 0, Address: "SoLAddr1111111111111111111111111111111111111"},
	}

	results, err := provider.FetchNativeBalances(context.Background(), addresses)
	if err != nil {
		t.Fatalf("FetchNativeBalances() error = %v", err)
	}

	if len(results) != 1 {
		t.Fatalf("expected 1 result, got %d", len(results))
	}

	// 5000000000 lamports = 5 SOL
	if results[0].Balance != "5000000000" {
		t.Errorf("expected balance 5000000000, got %s", results[0].Balance)
	}

	if results[0].Error != "" {
		t.Errorf("expected no error, got %s", results[0].Error)
	}

	if results[0].Source != "TestSolana" {
		t.Errorf("expected source TestSolana, got %s", results[0].Source)
	}
}

func TestSolanaRPCProvider_NativeNullAccount(t *testing.T) {
	provider, server := newSolanaRPCTestProvider(t, func(w http.ResponseWriter, r *http.Request) {
		resp := solanaRPCResponse{
			JSONRPC: "2.0",
			ID:      1,
			Result: &struct {
				Context struct {
					Slot uint64 `json:"slot"`
				} `json:"context"`
				Value []json.RawMessage `json:"value"`
			}{
				Value: []json.RawMessage{
					json.RawMessage(`null`),
				},
			},
		}
		json.NewEncoder(w).Encode(resp)
	})
	defer server.Close()

	addresses := []models.Address{
		{Chain: models.ChainSOL, AddressIndex: 0, Address: "SoLAddr1111111111111111111111111111111111111"},
	}

	results, err := provider.FetchNativeBalances(context.Background(), addresses)
	if err != nil {
		t.Fatalf("FetchNativeBalances() error = %v", err)
	}

	if len(results) != 1 {
		t.Fatalf("expected 1 result, got %d", len(results))
	}

	if results[0].Balance != "0" {
		t.Errorf("expected balance 0 for null account, got %s", results[0].Balance)
	}

	if results[0].Error != "" {
		t.Errorf("expected no error for null account, got %s", results[0].Error)
	}
}

func TestSolanaRPCProvider_NativePartialResults(t *testing.T) {
	// Request 3 addresses, server returns only 2.
	provider, server := newSolanaRPCTestProvider(t, func(w http.ResponseWriter, r *http.Request) {
		resp := solanaRPCResponse{
			JSONRPC: "2.0",
			ID:      1,
			Result: &struct {
				Context struct {
					Slot uint64 `json:"slot"`
				} `json:"context"`
				Value []json.RawMessage `json:"value"`
			}{
				Value: []json.RawMessage{
					json.RawMessage(`{"lamports": 1000, "owner": "11111111111111111111111111111111", "executable": false}`),
					json.RawMessage(`{"lamports": 2000, "owner": "11111111111111111111111111111111", "executable": false}`),
					// Third result missing.
				},
			},
		}
		json.NewEncoder(w).Encode(resp)
	})
	defer server.Close()

	addresses := []models.Address{
		{Chain: models.ChainSOL, AddressIndex: 0, Address: "addr0"},
		{Chain: models.ChainSOL, AddressIndex: 1, Address: "addr1"},
		{Chain: models.ChainSOL, AddressIndex: 2, Address: "addr2"},
	}

	results, err := provider.FetchNativeBalances(context.Background(), addresses)
	if err != nil {
		t.Fatalf("FetchNativeBalances() error = %v", err)
	}

	if len(results) != 3 {
		t.Fatalf("expected 3 results, got %d", len(results))
	}

	if results[0].Balance != "1000" {
		t.Errorf("addr0: expected 1000, got %s", results[0].Balance)
	}

	if results[1].Balance != "2000" {
		t.Errorf("addr1: expected 2000, got %s", results[1].Balance)
	}

	// Third should have error annotation.
	if results[2].Error == "" {
		t.Error("addr2: expected error annotation for missing result")
	}
	if results[2].Balance != "0" {
		t.Errorf("addr2: expected balance 0, got %s", results[2].Balance)
	}
}

func TestSolanaRPCProvider_NativeRPCError(t *testing.T) {
	provider, server := newSolanaRPCTestProvider(t, func(w http.ResponseWriter, r *http.Request) {
		resp := solanaRPCResponse{
			JSONRPC: "2.0",
			ID:      1,
			Error: &struct {
				Code    int    `json:"code"`
				Message string `json:"message"`
			}{Code: -32005, Message: "Node is behind"},
		}
		json.NewEncoder(w).Encode(resp)
	})
	defer server.Close()

	addresses := []models.Address{
		{Chain: models.ChainSOL, AddressIndex: 0, Address: "addr0"},
	}

	_, err := provider.FetchNativeBalances(context.Background(), addresses)
	if err == nil {
		t.Fatal("expected error for RPC error response")
	}
}

func TestSolanaRPCProvider_NativeNilResult(t *testing.T) {
	provider, server := newSolanaRPCTestProvider(t, func(w http.ResponseWriter, r *http.Request) {
		// Return response with no error and no result.
		w.Header().Set("Content-Type", "application/json")
		w.Write([]byte(`{"jsonrpc":"2.0","id":1}`))
	})
	defer server.Close()

	addresses := []models.Address{
		{Chain: models.ChainSOL, AddressIndex: 0, Address: "addr0"},
	}

	_, err := provider.FetchNativeBalances(context.Background(), addresses)
	if err == nil {
		t.Fatal("expected error for nil result")
	}
}

func TestSolanaRPCProvider_RateLimited(t *testing.T) {
	provider, server := newSolanaRPCTestProvider(t, func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Retry-After", "10")
		w.WriteHeader(http.StatusTooManyRequests)
	})
	defer server.Close()

	addresses := []models.Address{
		{Chain: models.ChainSOL, AddressIndex: 0, Address: "addr0"},
	}

	_, err := provider.FetchNativeBalances(context.Background(), addresses)
	if err == nil {
		t.Fatal("expected error for rate limited response")
	}

	if !config.IsTransient(err) {
		t.Errorf("expected transient error, got: %v", err)
	}
}

func TestSolanaRPCProvider_TokenBalanceSuccess(t *testing.T) {
	provider, server := newSolanaRPCTestProvider(t, func(w http.ResponseWriter, r *http.Request) {
		resp := solanaRPCResponse{
			JSONRPC: "2.0",
			ID:      1,
			Result: &struct {
				Context struct {
					Slot uint64 `json:"slot"`
				} `json:"context"`
				Value []json.RawMessage `json:"value"`
			}{
				Value: []json.RawMessage{
					json.RawMessage(`{
						"lamports": 2039280,
						"owner": "TokenkegQfeZyiNwAJbNbGKPFXCWuBvf9Ss623VQ5DA",
						"data": {
							"program": "spl-token",
							"parsed": {
								"type": "account",
								"info": {
									"mint": "EPjFWdd5AufqSSqeM2qN1xzybapC8G4wEGGkZwyTDt1v",
									"owner": "SoLAddr1111111111111111111111111111111111111",
									"tokenAmount": {
										"amount": "25000000",
										"decimals": 6
									}
								}
							}
						}
					}`),
				},
			},
		}
		json.NewEncoder(w).Encode(resp)
	})
	defer server.Close()

	// Use a real-looking Solana address to avoid ATA derivation issues.
	// The ATA derivation needs valid base58 addresses. Use the test wallet.
	addresses := []models.Address{
		{Chain: models.ChainSOL, AddressIndex: 0, Address: "3Cy3YNTFywCmxoxt8n7UH6hg6dLo5uACowX3CFceaSnx"},
	}

	results, err := provider.FetchTokenBalances(context.Background(), addresses, models.TokenUSDC, config.SOLUSDCMint)
	if err != nil {
		t.Fatalf("FetchTokenBalances() error = %v", err)
	}

	if len(results) != 1 {
		t.Fatalf("expected 1 result, got %d", len(results))
	}

	if results[0].Balance != "25000000" {
		t.Errorf("expected balance 25000000, got %s", results[0].Balance)
	}

	if results[0].Error != "" {
		t.Errorf("expected no error, got %s", results[0].Error)
	}
}

func TestSolanaRPCProvider_TokenNullATA(t *testing.T) {
	provider, server := newSolanaRPCTestProvider(t, func(w http.ResponseWriter, r *http.Request) {
		resp := solanaRPCResponse{
			JSONRPC: "2.0",
			ID:      1,
			Result: &struct {
				Context struct {
					Slot uint64 `json:"slot"`
				} `json:"context"`
				Value []json.RawMessage `json:"value"`
			}{
				Value: []json.RawMessage{
					json.RawMessage(`null`),
				},
			},
		}
		json.NewEncoder(w).Encode(resp)
	})
	defer server.Close()

	addresses := []models.Address{
		{Chain: models.ChainSOL, AddressIndex: 0, Address: "3Cy3YNTFywCmxoxt8n7UH6hg6dLo5uACowX3CFceaSnx"},
	}

	results, err := provider.FetchTokenBalances(context.Background(), addresses, models.TokenUSDC, config.SOLUSDCMint)
	if err != nil {
		t.Fatalf("FetchTokenBalances() error = %v", err)
	}

	if len(results) != 1 {
		t.Fatalf("expected 1 result, got %d", len(results))
	}

	if results[0].Balance != "0" {
		t.Errorf("expected balance 0 for null ATA, got %s", results[0].Balance)
	}
}

// TestSolanaRPCProvider_NativeMalformedJSON tests that malformed JSON RPC response is handled gracefully.
func TestSolanaRPCProvider_NativeMalformedJSON(t *testing.T) {
	provider, server := newSolanaRPCTestProvider(t, func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.Write([]byte(`{totally broken json!!!`))
	})
	defer server.Close()

	addresses := []models.Address{
		{Chain: models.ChainSOL, AddressIndex: 0, Address: "addr0"},
	}

	_, err := provider.FetchNativeBalances(context.Background(), addresses)
	if err == nil {
		t.Fatal("expected error for malformed JSON response")
	}
}

// TestSolanaRPCProvider_NativeContextCancellation tests that context cancellation returns an error.
func TestSolanaRPCProvider_NativeContextCancellation(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	cancel() // Cancel immediately.

	provider := &SolanaRPCProvider{
		client: http.DefaultClient,
		rl:     NewRateLimiter("test", 100),
		rpcURL: "http://localhost:1", // won't be called
		name:   "TestSolana",
	}

	addresses := []models.Address{
		{Chain: models.ChainSOL, AddressIndex: 0, Address: "addr0"},
	}

	_, err := provider.FetchNativeBalances(ctx, addresses)
	if err == nil {
		t.Fatal("expected error on context cancellation")
	}
}

func TestSolanaRPCProvider_Metadata(t *testing.T) {
	provider := &SolanaRPCProvider{name: "SolanaRPC"}
	if provider.Name() != "SolanaRPC" {
		t.Errorf("expected name SolanaRPC, got %s", provider.Name())
	}
	if provider.Chain() != models.ChainSOL {
		t.Errorf("expected chain SOL, got %s", provider.Chain())
	}
	if provider.MaxBatchSize() != config.ScanBatchSizeSolanaRPC {
		t.Errorf("expected batch size %d, got %d", config.ScanBatchSizeSolanaRPC, provider.MaxBatchSize())
	}
}
