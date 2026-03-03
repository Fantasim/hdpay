package scanner

import (
	"context"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"math/big"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/Fantasim/hdpay/internal/shared/config"
	"github.com/Fantasim/hdpay/internal/shared/models"
	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/ethclient"
)

// buildMockAggregate3Response builds an ABI-encoded aggregate3 Result[] response.
// Each result is (bool success, bytes returnData) where returnData is a 32-byte uint256.
func buildMockAggregate3Response(results []struct {
	success bool
	balance *big.Int
}) []byte {
	n := len(results)

	// Calculate total size:
	// 32 (array offset=0x20) + 32 (array length) + n*32 (offsets) + n*(32+32+32+32) (element data)
	// Each element: success(32) + returnData offset(32) + returnData length(32) + returnData(32)
	offsetsSize := 32 * n
	elementSize := 128 // 4 × 32 per element
	totalSize := 64 + offsetsSize + elementSize*n
	buf := make([]byte, totalSize)

	// Array offset = 0x20.
	writeUint256(buf[0:32], 32)
	// Array length.
	writeUint256(buf[32:64], uint64(n))

	// Offsets — each offset is relative to the start of the offsets section (byte 64),
	// pointing to where the element data begins.
	offsetsSectionStart := 64
	dataSectionStart := offsetsSectionStart + offsetsSize
	for i := range n {
		// Element i data is at dataSectionStart + elementSize*i.
		// Offset relative to offsetsSectionStart = offsetsSize + elementSize*i.
		writeUint256(buf[offsetsSectionStart+32*i:offsetsSectionStart+32*(i+1)], uint64(offsetsSize+elementSize*i))
	}

	// Element data.
	for i, r := range results {
		elemStart := dataSectionStart + elementSize*i

		// success (bool)
		if r.success {
			buf[elemStart+31] = 1
		}

		// returnData offset within tuple = 64 (past success + this offset slot)
		writeUint256(buf[elemStart+32:elemStart+64], 64)

		// returnData length = 32
		writeUint256(buf[elemStart+64:elemStart+96], 32)

		// returnData (uint256 balance, big-endian 32 bytes)
		balBytes := r.balance.Bytes()
		copy(buf[elemStart+96+(32-len(balBytes)):elemStart+128], balBytes)
	}

	return buf
}

// newMulticallTestProvider creates a BSCMulticallProvider connected to a mock server.
func newMulticallTestProvider(t *testing.T, handler http.HandlerFunc) (*BSCMulticallProvider, *httptest.Server) {
	t.Helper()

	server := httptest.NewServer(handler)

	client, err := ethclient.Dial(server.URL)
	if err != nil {
		server.Close()
		t.Fatalf("failed to dial mock server: %v", err)
	}

	rl := NewRateLimiter("test-multicall", 100, 0)
	provider := &BSCMulticallProvider{
		name:          "TestMulticall3",
		client:        client,
		rl:            rl,
		rpcURL:        server.URL,
		multicallAddr: common.HexToAddress(config.Multicall3Address),
	}

	return provider, server
}

func TestBSCMulticallProvider_Metadata(t *testing.T) {
	provider := &BSCMulticallProvider{name: "Multicall3-BSC"}
	if provider.Name() != "Multicall3-BSC" {
		t.Errorf("expected name Multicall3-BSC, got %s", provider.Name())
	}
	if provider.Chain() != models.ChainBSC {
		t.Errorf("expected chain BSC, got %s", provider.Chain())
	}
	if provider.MaxBatchSize() != config.Multicall3BatchSize {
		t.Errorf("expected batch size %d, got %d", config.Multicall3BatchSize, provider.MaxBatchSize())
	}
}

func TestBSCMulticallProvider_NativeBalanceSuccess(t *testing.T) {
	bal1 := big.NewInt(1_500_000_000_000_000_000) // 1.5 BNB
	bal2 := big.NewInt(0)
	bal3 := big.NewInt(250_000_000_000_000)       // 0.00025 BNB

	mockResults := []struct {
		success bool
		balance *big.Int
	}{
		{true, bal1},
		{true, bal2},
		{true, bal3},
	}

	provider, server := newMulticallTestProvider(t, func(w http.ResponseWriter, r *http.Request) {
		var req jsonRPCRequest
		json.NewDecoder(r.Body).Decode(&req)

		responseData := buildMockAggregate3Response(mockResults)
		result := fmt.Sprintf(`"0x%s"`, hex.EncodeToString(responseData))

		resp := jsonRPCResponse{
			JSONRPC: "2.0",
			ID:      req.ID,
			Result:  json.RawMessage(result),
		}
		json.NewEncoder(w).Encode(resp)
	})
	defer server.Close()

	addresses := []models.Address{
		{Chain: models.ChainBSC, AddressIndex: 0, Address: "0x1111111111111111111111111111111111111111"},
		{Chain: models.ChainBSC, AddressIndex: 1, Address: "0x2222222222222222222222222222222222222222"},
		{Chain: models.ChainBSC, AddressIndex: 2, Address: "0x3333333333333333333333333333333333333333"},
	}

	results, err := provider.FetchNativeBalances(context.Background(), addresses)
	if err != nil {
		t.Fatalf("FetchNativeBalances() error = %v", err)
	}

	if len(results) != 3 {
		t.Fatalf("expected 3 results, got %d", len(results))
	}

	// Check balances.
	if results[0].Balance != bal1.String() {
		t.Errorf("result[0] balance: expected %s, got %s", bal1.String(), results[0].Balance)
	}
	if results[1].Balance != "0" {
		t.Errorf("result[1] balance: expected 0, got %s", results[1].Balance)
	}
	if results[2].Balance != bal3.String() {
		t.Errorf("result[2] balance: expected %s, got %s", bal3.String(), results[2].Balance)
	}

	// Check no errors.
	for i, r := range results {
		if r.Error != "" {
			t.Errorf("result[%d] unexpected error: %s", i, r.Error)
		}
		if r.Source != "TestMulticall3" {
			t.Errorf("result[%d] expected source TestMulticall3, got %s", i, r.Source)
		}
	}
}

func TestBSCMulticallProvider_NativeBalancePartialFailure(t *testing.T) {
	bal1 := big.NewInt(1_000_000_000_000_000_000)

	mockResults := []struct {
		success bool
		balance *big.Int
	}{
		{true, bal1},
		{false, big.NewInt(0)},  // failed subcall
		{true, big.NewInt(0)},   // success but zero balance
	}

	provider, server := newMulticallTestProvider(t, func(w http.ResponseWriter, r *http.Request) {
		var req jsonRPCRequest
		json.NewDecoder(r.Body).Decode(&req)

		responseData := buildMockAggregate3Response(mockResults)
		result := fmt.Sprintf(`"0x%s"`, hex.EncodeToString(responseData))

		resp := jsonRPCResponse{
			JSONRPC: "2.0",
			ID:      req.ID,
			Result:  json.RawMessage(result),
		}
		json.NewEncoder(w).Encode(resp)
	})
	defer server.Close()

	addresses := []models.Address{
		{Chain: models.ChainBSC, AddressIndex: 0, Address: "0x1111111111111111111111111111111111111111"},
		{Chain: models.ChainBSC, AddressIndex: 1, Address: "0x2222222222222222222222222222222222222222"},
		{Chain: models.ChainBSC, AddressIndex: 2, Address: "0x3333333333333333333333333333333333333333"},
	}

	results, err := provider.FetchNativeBalances(context.Background(), addresses)
	if err != nil {
		t.Fatalf("FetchNativeBalances() error = %v", err)
	}

	if len(results) != 3 {
		t.Fatalf("expected 3 results, got %d", len(results))
	}

	// result[0]: success, funded
	if results[0].Balance != bal1.String() {
		t.Errorf("result[0] balance: expected %s, got %s", bal1.String(), results[0].Balance)
	}
	if results[0].Error != "" {
		t.Errorf("result[0] unexpected error: %s", results[0].Error)
	}

	// result[1]: failed subcall
	if results[1].Balance != "0" {
		t.Errorf("result[1] balance: expected 0, got %s", results[1].Balance)
	}
	if results[1].Error == "" {
		t.Error("result[1] expected error annotation for failed subcall")
	}

	// result[2]: success, zero balance
	if results[2].Balance != "0" {
		t.Errorf("result[2] balance: expected 0, got %s", results[2].Balance)
	}
	if results[2].Error != "" {
		t.Errorf("result[2] unexpected error: %s", results[2].Error)
	}
}

func TestBSCMulticallProvider_TokenBalanceSuccess(t *testing.T) {
	bal1 := big.NewInt(500_000_000) // 500 USDC

	mockResults := []struct {
		success bool
		balance *big.Int
	}{
		{true, bal1},
	}

	provider, server := newMulticallTestProvider(t, func(w http.ResponseWriter, r *http.Request) {
		var req jsonRPCRequest
		json.NewDecoder(r.Body).Decode(&req)

		responseData := buildMockAggregate3Response(mockResults)
		result := fmt.Sprintf(`"0x%s"`, hex.EncodeToString(responseData))

		resp := jsonRPCResponse{
			JSONRPC: "2.0",
			ID:      req.ID,
			Result:  json.RawMessage(result),
		}
		json.NewEncoder(w).Encode(resp)
	})
	defer server.Close()

	addresses := []models.Address{
		{Chain: models.ChainBSC, AddressIndex: 0, Address: "0x1111111111111111111111111111111111111111"},
	}

	results, err := provider.FetchTokenBalances(context.Background(), addresses, models.TokenUSDC, "0xabc123")
	if err != nil {
		t.Fatalf("FetchTokenBalances() error = %v", err)
	}

	if len(results) != 1 {
		t.Fatalf("expected 1 result, got %d", len(results))
	}

	if results[0].Balance != bal1.String() {
		t.Errorf("expected balance %s, got %s", bal1.String(), results[0].Balance)
	}
}

func TestBSCMulticallProvider_EmptyAddresses(t *testing.T) {
	provider := &BSCMulticallProvider{name: "test"}
	results, err := provider.FetchNativeBalances(context.Background(), nil)
	if err != nil {
		t.Fatalf("unexpected error for empty addresses: %v", err)
	}
	if results != nil {
		t.Errorf("expected nil results for empty addresses, got %v", results)
	}
}

func TestBSCMulticallProvider_TokenEmptyContract(t *testing.T) {
	provider := &BSCMulticallProvider{name: "test"}
	addresses := []models.Address{
		{Chain: models.ChainBSC, AddressIndex: 0, Address: "0x1111111111111111111111111111111111111111"},
	}
	_, err := provider.FetchTokenBalances(context.Background(), addresses, models.TokenUSDC, "")
	if err == nil {
		t.Fatal("expected error for empty contract address")
	}
}

func TestBSCMulticallProvider_RPCError(t *testing.T) {
	provider, server := newMulticallTestProvider(t, func(w http.ResponseWriter, r *http.Request) {
		var req jsonRPCRequest
		json.NewDecoder(r.Body).Decode(&req)

		resp := jsonRPCResponse{
			JSONRPC: "2.0",
			ID:      req.ID,
			Error: &struct {
				Code    int    `json:"code"`
				Message string `json:"message"`
			}{Code: -32005, Message: "rate limit exceeded"},
		}
		json.NewEncoder(w).Encode(resp)
	})
	defer server.Close()

	addresses := []models.Address{
		{Chain: models.ChainBSC, AddressIndex: 0, Address: "0x1111111111111111111111111111111111111111"},
	}

	_, err := provider.FetchNativeBalances(context.Background(), addresses)
	if err == nil {
		t.Fatal("expected error for RPC failure")
	}
}

// ── ABI Encoding Unit Tests ──────────────────────────────────────────────────

func TestEncodeGetEthBalance(t *testing.T) {
	addr := common.HexToAddress("0x1234567890abcdef1234567890abcdef12345678")
	encoded := encodeGetEthBalance(addr)

	if len(encoded) != 36 {
		t.Fatalf("expected 36 bytes, got %d", len(encoded))
	}

	// Check selector (getEthBalance).
	expectedSelector := "4d2301cc"
	gotSelector := hex.EncodeToString(encoded[:4])
	if gotSelector != expectedSelector {
		t.Errorf("selector: expected %s, got %s", expectedSelector, gotSelector)
	}

	// Check address is correctly padded (12 zero bytes + 20 address bytes).
	for i := 4; i < 16; i++ {
		if encoded[i] != 0 {
			t.Errorf("byte %d should be 0 (padding), got %d", i, encoded[i])
		}
	}
	gotAddr := common.BytesToAddress(encoded[16:36])
	if gotAddr != addr {
		t.Errorf("address: expected %s, got %s", addr.Hex(), gotAddr.Hex())
	}
}

func TestEncodeBalanceOfCall(t *testing.T) {
	addr := common.HexToAddress("0xabcdefabcdefabcdefabcdefabcdefabcdefabcd")
	encoded := encodeBalanceOfCall(addr)

	if len(encoded) != 36 {
		t.Fatalf("expected 36 bytes, got %d", len(encoded))
	}

	// Check selector (balanceOf).
	expectedSelector := "70a08231"
	gotSelector := hex.EncodeToString(encoded[:4])
	if gotSelector != expectedSelector {
		t.Errorf("selector: expected %s, got %s", expectedSelector, gotSelector)
	}
}

func TestEncodeDecodeAggregate3Roundtrip(t *testing.T) {
	calls := []multicallCall3{
		{
			Target:       common.HexToAddress("0xcA11bde05977b3631167028862bE2a173976CA11"),
			AllowFailure: true,
			CallData:     encodeGetEthBalance(common.HexToAddress("0x1111111111111111111111111111111111111111")),
		},
		{
			Target:       common.HexToAddress("0x55d398326f99059fF775485246999027B3197955"),
			AllowFailure: true,
			CallData:     encodeBalanceOfCall(common.HexToAddress("0x2222222222222222222222222222222222222222")),
		},
	}

	encoded, err := encodeAggregate3(calls)
	if err != nil {
		t.Fatalf("encodeAggregate3 error: %v", err)
	}

	// Check selector.
	expectedSelector := "82ad56cb"
	gotSelector := hex.EncodeToString(encoded[:4])
	if gotSelector != expectedSelector {
		t.Errorf("selector: expected %s, got %s", expectedSelector, gotSelector)
	}

	// Verify the calldata is non-trivially long (2 calls with 36-byte calldata each).
	if len(encoded) < 300 {
		t.Errorf("encoded too short: %d bytes, expected >300", len(encoded))
	}
}

func TestDecodeAggregate3Results(t *testing.T) {
	bal1 := big.NewInt(1_000_000_000_000_000_000)
	bal2 := big.NewInt(0)

	mockResults := []struct {
		success bool
		balance *big.Int
	}{
		{true, bal1},
		{false, bal2},
	}

	encoded := buildMockAggregate3Response(mockResults)

	decoded, err := decodeAggregate3Results(encoded, 2)
	if err != nil {
		t.Fatalf("decodeAggregate3Results error: %v", err)
	}

	if len(decoded) != 2 {
		t.Fatalf("expected 2 results, got %d", len(decoded))
	}

	if !decoded[0].Success {
		t.Error("result[0] expected success=true")
	}
	got1 := parseUint256(decoded[0].ReturnData)
	if got1.Cmp(bal1) != 0 {
		t.Errorf("result[0] balance: expected %s, got %s", bal1.String(), got1.String())
	}

	if decoded[1].Success {
		t.Error("result[1] expected success=false")
	}
}

func TestDecodeAggregate3Results_TooShort(t *testing.T) {
	_, err := decodeAggregate3Results([]byte{0x00}, 1)
	if err == nil {
		t.Fatal("expected error for too-short output")
	}
}

func TestDecodeAggregate3Results_CountMismatch(t *testing.T) {
	mockResults := []struct {
		success bool
		balance *big.Int
	}{
		{true, big.NewInt(0)},
	}
	encoded := buildMockAggregate3Response(mockResults)

	_, err := decodeAggregate3Results(encoded, 5) // expect 5 but encoded 1
	if err == nil {
		t.Fatal("expected error for count mismatch")
	}
}

func TestParseUint256(t *testing.T) {
	tests := []struct {
		name   string
		data   []byte
		expect string
	}{
		{"empty", nil, "0"},
		{"zero 32-byte", make([]byte, 32), "0"},
		{"1 BNB", func() []byte {
			b := make([]byte, 32)
			v := big.NewInt(1_000_000_000_000_000_000)
			copy(b[32-len(v.Bytes()):], v.Bytes())
			return b
		}(), "1000000000000000000"},
		{"short data", []byte{0x01, 0x00}, "256"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := parseUint256(tt.data)
			if got.String() != tt.expect {
				t.Errorf("expected %s, got %s", tt.expect, got.String())
			}
		})
	}
}
