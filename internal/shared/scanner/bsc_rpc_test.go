package scanner

import (
	"context"
	"encoding/json"
	"fmt"
	"math/big"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/Fantasim/hdpay/internal/shared/config"
	"github.com/Fantasim/hdpay/internal/shared/models"
	"github.com/ethereum/go-ethereum/ethclient"
)

// jsonRPCRequest is the expected JSON-RPC request body.
type jsonRPCRequest struct {
	JSONRPC string            `json:"jsonrpc"`
	ID      json.RawMessage   `json:"id"`
	Method  string            `json:"method"`
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

// batchHandler wraps a per-request handler to handle both single and batch JSON-RPC
// requests. go-ethereum's BatchCallContext sends a JSON array of requests.
func batchHandler(handleSingle func(req jsonRPCRequest) jsonRPCResponse) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		// Peek at first byte to determine single vs batch.
		var raw json.RawMessage
		if err := json.NewDecoder(r.Body).Decode(&raw); err != nil {
			http.Error(w, "invalid json", http.StatusBadRequest)
			return
		}

		if len(raw) > 0 && raw[0] == '[' {
			// Batch request.
			var reqs []jsonRPCRequest
			json.Unmarshal(raw, &reqs)

			resps := make([]jsonRPCResponse, len(reqs))
			for i, req := range reqs {
				resps[i] = handleSingle(req)
			}
			json.NewEncoder(w).Encode(resps)
		} else {
			// Single request.
			var req jsonRPCRequest
			json.Unmarshal(raw, &req)
			resp := handleSingle(req)
			json.NewEncoder(w).Encode(resp)
		}
	}
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

	rl := NewRateLimiter("test", 100, 0)
	provider := &BSCRPCProvider{
		name:      "BSCRPC",
		client:    client,
		rpcClient: client.Client(),
		rl:        rl,
		rpcURL:    server.URL,
	}

	return provider, server
}

func TestBSCRPCProvider_NativeBalanceSuccess(t *testing.T) {
	expectedBalance := big.NewInt(1_500_000_000_000_000_000) // 1.5 BNB in wei

	handler := batchHandler(func(req jsonRPCRequest) jsonRPCResponse {
		return jsonRPCResponse{
			JSONRPC: "2.0",
			ID:      req.ID,
			Result:  json.RawMessage(fmt.Sprintf(`"0x%x"`, expectedBalance)),
		}
	})

	provider, server := newBSCRPCTestProvider(t, handler)
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

func TestBSCRPCProvider_NativeBatchSuccess(t *testing.T) {
	// 5 different balances for a batch of 5 addresses.
	balances := []*big.Int{
		big.NewInt(1_000_000_000_000_000_000), // 1 BNB
		big.NewInt(0),
		big.NewInt(500_000_000_000_000), // 0.0005 BNB
		big.NewInt(0),
		big.NewInt(2_300_000_000_000_000_000), // 2.3 BNB
	}

	callIdx := 0
	handler := batchHandler(func(req jsonRPCRequest) jsonRPCResponse {
		idx := callIdx
		callIdx++
		if idx >= len(balances) {
			idx = len(balances) - 1
		}
		return jsonRPCResponse{
			JSONRPC: "2.0",
			ID:      req.ID,
			Result:  json.RawMessage(fmt.Sprintf(`"0x%x"`, balances[idx])),
		}
	})

	provider, server := newBSCRPCTestProvider(t, handler)
	defer server.Close()

	addresses := make([]models.Address, 5)
	for i := range addresses {
		addresses[i] = models.Address{
			Chain:        models.ChainBSC,
			AddressIndex: i,
			Address:      fmt.Sprintf("0x%040d", i+1),
		}
	}

	results, err := provider.FetchNativeBalances(context.Background(), addresses)
	if err != nil {
		t.Fatalf("FetchNativeBalances() error = %v", err)
	}

	if len(results) != 5 {
		t.Fatalf("expected 5 results, got %d", len(results))
	}

	for i, r := range results {
		if r.Balance != balances[i].String() {
			t.Errorf("result[%d]: expected balance %s, got %s", i, balances[i].String(), r.Balance)
		}
		if r.Error != "" {
			t.Errorf("result[%d]: unexpected error: %s", i, r.Error)
		}
	}
}

func TestBSCRPCProvider_NativeBatchPartialFailure(t *testing.T) {
	callIdx := 0
	handler := batchHandler(func(req jsonRPCRequest) jsonRPCResponse {
		idx := callIdx
		callIdx++
		// Elements 1 and 3 fail.
		if idx == 1 || idx == 3 {
			return jsonRPCResponse{
				JSONRPC: "2.0",
				ID:      req.ID,
				Error: &struct {
					Code    int    `json:"code"`
					Message string `json:"message"`
				}{Code: -32000, Message: "internal error"},
			}
		}
		return jsonRPCResponse{
			JSONRPC: "2.0",
			ID:      req.ID,
			Result:  json.RawMessage(`"0xde0b6b3a7640000"`), // 1 BNB
		}
	})

	provider, server := newBSCRPCTestProvider(t, handler)
	defer server.Close()

	addresses := make([]models.Address, 5)
	for i := range addresses {
		addresses[i] = models.Address{
			Chain:        models.ChainBSC,
			AddressIndex: i,
			Address:      fmt.Sprintf("0x%040d", i+1),
		}
	}

	results, err := provider.FetchNativeBalances(context.Background(), addresses)
	if err != nil {
		t.Fatalf("unexpected total error: %v", err)
	}

	if len(results) != 5 {
		t.Fatalf("expected 5 results, got %d", len(results))
	}

	// Elements 0, 2, 4 should have balances.
	for _, i := range []int{0, 2, 4} {
		if results[i].Balance == "0" {
			t.Errorf("result[%d]: expected funded balance, got 0", i)
		}
		if results[i].Error != "" {
			t.Errorf("result[%d]: unexpected error: %s", i, results[i].Error)
		}
	}

	// Elements 1, 3 should have errors.
	for _, i := range []int{1, 3} {
		if results[i].Balance != "0" {
			t.Errorf("result[%d]: expected 0 balance, got %s", i, results[i].Balance)
		}
		if results[i].Error == "" {
			t.Errorf("result[%d]: expected error annotation", i)
		}
	}
}

func TestBSCRPCProvider_NativeBalanceZero(t *testing.T) {
	handler := batchHandler(func(req jsonRPCRequest) jsonRPCResponse {
		return jsonRPCResponse{
			JSONRPC: "2.0",
			ID:      req.ID,
			Result:  json.RawMessage(`"0x0"`),
		}
	})

	provider, server := newBSCRPCTestProvider(t, handler)
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
	handler := batchHandler(func(req jsonRPCRequest) jsonRPCResponse {
		return jsonRPCResponse{
			JSONRPC: "2.0",
			ID:      req.ID,
			Error: &struct {
				Code    int    `json:"code"`
				Message string `json:"message"`
			}{Code: -32000, Message: "internal error"},
		}
	})

	provider, server := newBSCRPCTestProvider(t, handler)
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
	expectedBalance := big.NewInt(500_000_000) // 500 USDC (6 decimals)

	handler := batchHandler(func(req jsonRPCRequest) jsonRPCResponse {
		if req.Method == "eth_call" {
			result := fmt.Sprintf(`"0x%064x"`, expectedBalance)
			return jsonRPCResponse{
				JSONRPC: "2.0",
				ID:      req.ID,
				Result:  json.RawMessage(result),
			}
		}
		return jsonRPCResponse{
			JSONRPC: "2.0",
			ID:      req.ID,
			Result:  json.RawMessage(`"0x0"`),
		}
	})

	provider, server := newBSCRPCTestProvider(t, handler)
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
	handler := batchHandler(func(req jsonRPCRequest) jsonRPCResponse {
		return jsonRPCResponse{
			JSONRPC: "2.0",
			ID:      req.ID,
			Error: &struct {
				Code    int    `json:"code"`
				Message string `json:"message"`
			}{Code: -32000, Message: "execution reverted"},
		}
	})

	provider, server := newBSCRPCTestProvider(t, handler)
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
	handler := batchHandler(func(req jsonRPCRequest) jsonRPCResponse {
		if req.Method == "eth_call" {
			return jsonRPCResponse{
				JSONRPC: "2.0",
				ID:      req.ID,
				Result:  json.RawMessage(`"0x1234"`),
			}
		}
		return jsonRPCResponse{
			JSONRPC: "2.0",
			ID:      req.ID,
			Result:  json.RawMessage(`"0x0"`),
		}
	})

	provider, server := newBSCRPCTestProvider(t, handler)
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

func TestBSCRPCProvider_EmptyAddresses(t *testing.T) {
	provider := &BSCRPCProvider{name: "test"}
	results, err := provider.FetchNativeBalances(context.Background(), nil)
	if err != nil {
		t.Fatalf("unexpected error for empty addresses: %v", err)
	}
	if results != nil {
		t.Errorf("expected nil results for empty addresses, got %v", results)
	}
}

func TestBSCRPCProvider_Metadata(t *testing.T) {
	provider := &BSCRPCProvider{name: "BSCRPC"}
	if provider.Name() != "BSCRPC" {
		t.Errorf("expected name BSCRPC, got %s", provider.Name())
	}
	if provider.Chain() != models.ChainBSC {
		t.Errorf("expected chain BSC, got %s", provider.Chain())
	}
	if provider.MaxBatchSize() != config.BSCRPCBatchSize {
		t.Errorf("expected batch size %d, got %d", config.BSCRPCBatchSize, provider.MaxBatchSize())
	}
}
