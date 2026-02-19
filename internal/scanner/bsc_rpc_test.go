package scanner

import (
	"context"
	"encoding/json"
	"fmt"
	"math/big"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/Fantasim/hdpay/internal/models"
	"github.com/ethereum/go-ethereum/ethclient"
)

// jsonRPCRequest is the expected JSON-RPC request body.
type jsonRPCRequest struct {
	JSONRPC string        `json:"jsonrpc"`
	ID      json.RawMessage `json:"id"`
	Method  string        `json:"method"`
	Params  []json.RawMessage `json:"params"`
}

// jsonRPCResponse is a generic JSON-RPC response.
type jsonRPCResponse struct {
	JSONRPC string          `json:"jsonrpc"`
	ID      json.RawMessage `json:"id"`
	Result  json.RawMessage `json:"result,omitempty"`
	Error   *struct {
		Code    int    `json:"code"`
		Message string `json:"message"`
	} `json:"error,omitempty"`
}

// newBSCRPCTestProvider creates a BSCRPCProvider connected to a mock JSON-RPC server.
func newBSCRPCTestProvider(t *testing.T, handler http.HandlerFunc) (*BSCRPCProvider, *httptest.Server) {
	t.Helper()

	server := httptest.NewServer(handler)

	client, err := ethclient.Dial(server.URL)
	if err != nil {
		server.Close()
		t.Fatalf("failed to dial mock server: %v", err)
	}

	rl := NewRateLimiter("test", 100)
	provider := &BSCRPCProvider{
		client: client,
		rl:     rl,
		rpcURL: server.URL,
	}

	return provider, server
}

func TestBSCRPCProvider_NativeBalanceSuccess(t *testing.T) {
	// eth_getBalance returns hex-encoded balance.
	expectedBalance := big.NewInt(1_500_000_000_000_000_000) // 1.5 BNB in wei

	provider, server := newBSCRPCTestProvider(t, func(w http.ResponseWriter, r *http.Request) {
		var req jsonRPCRequest
		json.NewDecoder(r.Body).Decode(&req)

		resp := jsonRPCResponse{
			JSONRPC: "2.0",
			ID:      req.ID,
			Result:  json.RawMessage(fmt.Sprintf(`"0x%x"`, expectedBalance)),
		}
		json.NewEncoder(w).Encode(resp)
	})
	defer server.Close()

	addresses := []models.Address{
		{Chain: models.ChainBSC, AddressIndex: 0, Address: "0x1234567890abcdef1234567890abcdef12345678"},
	}

	results, err := provider.FetchNativeBalances(context.Background(), addresses)
	if err != nil {
		t.Fatalf("FetchNativeBalances() error = %v", err)
	}

	if len(results) != 1 {
		t.Fatalf("expected 1 result, got %d", len(results))
	}

	if results[0].Balance != expectedBalance.String() {
		t.Errorf("expected balance %s, got %s", expectedBalance.String(), results[0].Balance)
	}

	if results[0].Error != "" {
		t.Errorf("expected no error, got %s", results[0].Error)
	}

	if results[0].Source != "BSCRPC" {
		t.Errorf("expected source BSCRPC, got %s", results[0].Source)
	}
}

func TestBSCRPCProvider_NativeBalanceZero(t *testing.T) {
	provider, server := newBSCRPCTestProvider(t, func(w http.ResponseWriter, r *http.Request) {
		var req jsonRPCRequest
		json.NewDecoder(r.Body).Decode(&req)

		resp := jsonRPCResponse{
			JSONRPC: "2.0",
			ID:      req.ID,
			Result:  json.RawMessage(`"0x0"`),
		}
		json.NewEncoder(w).Encode(resp)
	})
	defer server.Close()

	addresses := []models.Address{
		{Chain: models.ChainBSC, AddressIndex: 0, Address: "0x0000000000000000000000000000000000000000"},
	}

	results, err := provider.FetchNativeBalances(context.Background(), addresses)
	if err != nil {
		t.Fatalf("FetchNativeBalances() error = %v", err)
	}

	if len(results) != 1 {
		t.Fatalf("expected 1 result, got %d", len(results))
	}

	if results[0].Balance != "0" {
		t.Errorf("expected balance 0, got %s", results[0].Balance)
	}
}

func TestBSCRPCProvider_NativeAllFail(t *testing.T) {
	provider, server := newBSCRPCTestProvider(t, func(w http.ResponseWriter, r *http.Request) {
		var req jsonRPCRequest
		json.NewDecoder(r.Body).Decode(&req)

		resp := jsonRPCResponse{
			JSONRPC: "2.0",
			ID:      req.ID,
			Error: &struct {
				Code    int    `json:"code"`
				Message string `json:"message"`
			}{Code: -32000, Message: "internal error"},
		}
		json.NewEncoder(w).Encode(resp)
	})
	defer server.Close()

	addresses := []models.Address{
		{Chain: models.ChainBSC, AddressIndex: 0, Address: "0x1111111111111111111111111111111111111111"},
		{Chain: models.ChainBSC, AddressIndex: 1, Address: "0x2222222222222222222222222222222222222222"},
	}

	results, err := provider.FetchNativeBalances(context.Background(), addresses)
	if err == nil {
		t.Fatal("expected error when all addresses fail")
	}

	if len(results) != 2 {
		t.Fatalf("expected 2 results, got %d", len(results))
	}
	for i, r := range results {
		if r.Error == "" {
			t.Errorf("result %d: expected error annotation", i)
		}
	}
}

func TestBSCRPCProvider_TokenBalanceSuccess(t *testing.T) {
	// balanceOf returns ABI-encoded uint256.
	expectedBalance := big.NewInt(500_000_000) // 500 USDC (6 decimals)

	provider, server := newBSCRPCTestProvider(t, func(w http.ResponseWriter, r *http.Request) {
		var req jsonRPCRequest
		json.NewDecoder(r.Body).Decode(&req)

		if req.Method == "eth_call" {
			// Return ABI-encoded uint256 (32 bytes, left-padded).
			result := fmt.Sprintf(`"0x%064x"`, expectedBalance)
			resp := jsonRPCResponse{
				JSONRPC: "2.0",
				ID:      req.ID,
				Result:  json.RawMessage(result),
			}
			json.NewEncoder(w).Encode(resp)
			return
		}

		// Default: return empty/zero for other methods.
		resp := jsonRPCResponse{
			JSONRPC: "2.0",
			ID:      req.ID,
			Result:  json.RawMessage(`"0x0"`),
		}
		json.NewEncoder(w).Encode(resp)
	})
	defer server.Close()

	addresses := []models.Address{
		{Chain: models.ChainBSC, AddressIndex: 0, Address: "0x1234567890abcdef1234567890abcdef12345678"},
	}

	results, err := provider.FetchTokenBalances(context.Background(), addresses, models.TokenUSDC, "0xabc123")
	if err != nil {
		t.Fatalf("FetchTokenBalances() error = %v", err)
	}

	if len(results) != 1 {
		t.Fatalf("expected 1 result, got %d", len(results))
	}

	if results[0].Balance != expectedBalance.String() {
		t.Errorf("expected balance %s, got %s", expectedBalance.String(), results[0].Balance)
	}

	if results[0].Error != "" {
		t.Errorf("expected no error, got %s", results[0].Error)
	}
}

func TestBSCRPCProvider_TokenEmptyContractAddress(t *testing.T) {
	provider := &BSCRPCProvider{}

	addresses := []models.Address{
		{Chain: models.ChainBSC, AddressIndex: 0, Address: "0x1234567890abcdef1234567890abcdef12345678"},
	}

	_, err := provider.FetchTokenBalances(context.Background(), addresses, models.TokenUSDC, "")
	if err == nil {
		t.Fatal("expected error for empty contract address")
	}
}

func TestBSCRPCProvider_TokenAllFail(t *testing.T) {
	provider, server := newBSCRPCTestProvider(t, func(w http.ResponseWriter, r *http.Request) {
		var req jsonRPCRequest
		json.NewDecoder(r.Body).Decode(&req)

		resp := jsonRPCResponse{
			JSONRPC: "2.0",
			ID:      req.ID,
			Error: &struct {
				Code    int    `json:"code"`
				Message string `json:"message"`
			}{Code: -32000, Message: "execution reverted"},
		}
		json.NewEncoder(w).Encode(resp)
	})
	defer server.Close()

	addresses := []models.Address{
		{Chain: models.ChainBSC, AddressIndex: 0, Address: "0x1111111111111111111111111111111111111111"},
	}

	results, err := provider.FetchTokenBalances(context.Background(), addresses, models.TokenUSDC, "0xabc123")
	if err == nil {
		t.Fatal("expected error when all token fetches fail")
	}

	if len(results) != 1 {
		t.Fatalf("expected 1 result, got %d", len(results))
	}
	if results[0].Error == "" {
		t.Error("expected error annotation")
	}
}

func TestBSCRPCProvider_TokenMalformedResponse(t *testing.T) {
	provider, server := newBSCRPCTestProvider(t, func(w http.ResponseWriter, r *http.Request) {
		var req jsonRPCRequest
		json.NewDecoder(r.Body).Decode(&req)

		if req.Method == "eth_call" {
			// Return fewer than 32 bytes (malformed).
			resp := jsonRPCResponse{
				JSONRPC: "2.0",
				ID:      req.ID,
				Result:  json.RawMessage(`"0x1234"`),
			}
			json.NewEncoder(w).Encode(resp)
			return
		}

		resp := jsonRPCResponse{
			JSONRPC: "2.0",
			ID:      req.ID,
			Result:  json.RawMessage(`"0x0"`),
		}
		json.NewEncoder(w).Encode(resp)
	})
	defer server.Close()

	addresses := []models.Address{
		{Chain: models.ChainBSC, AddressIndex: 0, Address: "0x1234567890abcdef1234567890abcdef12345678"},
	}

	results, err := provider.FetchTokenBalances(context.Background(), addresses, models.TokenUSDC, "0xabc123")
	if err == nil {
		t.Fatal("expected error for malformed response")
	}

	if len(results) != 1 {
		t.Fatalf("expected 1 result, got %d", len(results))
	}
	if results[0].Error == "" {
		t.Error("expected error annotation for malformed response")
	}
}

func TestBSCRPCProvider_Metadata(t *testing.T) {
	provider := &BSCRPCProvider{}
	if provider.Name() != "BSCRPC" {
		t.Errorf("expected name BSCRPC, got %s", provider.Name())
	}
	if provider.Chain() != models.ChainBSC {
		t.Errorf("expected chain BSC, got %s", provider.Chain())
	}
	if provider.MaxBatchSize() != 1 {
		t.Errorf("expected batch size 1, got %d", provider.MaxBatchSize())
	}
}
