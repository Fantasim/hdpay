package tx

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strconv"
	"sync/atomic"
	"testing"
	"time"

	"github.com/mr-tron/base58"

	"github.com/Fantasim/hdpay/internal/config"
	"github.com/Fantasim/hdpay/internal/models"
)

// --- Mock SOL RPC Client ---

type mockSOLRPCClient struct {
	getLatestBlockhashFn    func(ctx context.Context) ([32]byte, uint64, error)
	sendTransactionFn       func(ctx context.Context, txBase64 string) (string, error)
	getSignatureStatusesFn  func(ctx context.Context, sigs []string) ([]SOLSignatureStatus, error)
	getAccountInfoFn        func(ctx context.Context, addr string) (bool, uint64, error)
	getBalanceFn            func(ctx context.Context, addr string) (uint64, error)
}

func (m *mockSOLRPCClient) GetLatestBlockhash(ctx context.Context) ([32]byte, uint64, error) {
	if m.getLatestBlockhashFn != nil {
		return m.getLatestBlockhashFn(ctx)
	}
	return [32]byte{0xab, 0xcd}, 1000, nil
}

func (m *mockSOLRPCClient) SendTransaction(ctx context.Context, txBase64 string) (string, error) {
	if m.sendTransactionFn != nil {
		return m.sendTransactionFn(ctx, txBase64)
	}
	return "5MockSignatureAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAA", nil
}

func (m *mockSOLRPCClient) GetSignatureStatuses(ctx context.Context, sigs []string) ([]SOLSignatureStatus, error) {
	if m.getSignatureStatusesFn != nil {
		return m.getSignatureStatusesFn(ctx, sigs)
	}
	confirmed := "confirmed"
	return []SOLSignatureStatus{{
		Slot:               100,
		ConfirmationStatus: &confirmed,
	}}, nil
}

func (m *mockSOLRPCClient) GetAccountInfo(ctx context.Context, addr string) (bool, uint64, error) {
	if m.getAccountInfoFn != nil {
		return m.getAccountInfoFn(ctx, addr)
	}
	return true, 1_000_000, nil
}

func (m *mockSOLRPCClient) GetBalance(ctx context.Context, addr string) (uint64, error) {
	if m.getBalanceFn != nil {
		return m.getBalanceFn(ctx, addr)
	}
	return 1_000_000_000, nil // 1 SOL
}

// --- Mock DB that satisfies *db.DB interface for recording ---
// (We need a nil-safe way to handle the database calls. The consolidation service
// calls s.database.InsertTransaction but in tests we may pass nil.
// In the actual test, we just need to verify the code doesn't panic.)

// --- SOL Native Sweep Tests ---

func TestSOLNativeSweep_SingleAddress(t *testing.T) {
	mnemonicPath := writeTempMnemonic(t, testMnemonic24)
	ks := NewKeyService(mnemonicPath, "testnet")

	broadcastCalled := false
	mock := &mockSOLRPCClient{
		getBalanceFn: func(ctx context.Context, addr string) (uint64, error) {
			return 1_000_000_000, nil // 1 SOL
		},
		sendTransactionFn: func(ctx context.Context, txBase64 string) (string, error) {
			broadcastCalled = true
			return "5MockSigAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAA", nil
		},
	}

	svc := NewSOLConsolidationService(ks, mock, nil, "testnet", nil)

	addresses := []models.AddressWithBalance{{
		Chain:         models.ChainSOL,
		AddressIndex:  0,
		Address:       "3Cy3YNTFywCmxoxt8n7UH6hg6dLo5uACowX3CFceaSnx",
		NativeBalance: "1000000000",
	}}

	result, err := svc.ExecuteNativeSweep(context.Background(), addresses, "5frqxtii9LeGq2bz3dSNokvZcEooF483MzeU24JrhcTA", "test-sweep")
	if err != nil {
		t.Fatalf("ExecuteNativeSweep error = %v", err)
	}

	if !broadcastCalled {
		t.Error("expected SendTransaction to be called")
	}

	if result.SuccessCount != 1 {
		t.Errorf("successCount = %d, want 1", result.SuccessCount)
	}

	if len(result.TxResults) != 1 {
		t.Fatalf("txResults count = %d, want 1", len(result.TxResults))
	}

	// Amount should be balance - fee = 1_000_000_000 - 5_000 = 999_995_000
	expectedAmount := strconv.FormatUint(1_000_000_000-config.SOLBaseTransactionFee, 10)
	if result.TxResults[0].Amount != expectedAmount {
		t.Errorf("amount = %s, want %s", result.TxResults[0].Amount, expectedAmount)
	}
}

func TestSOLNativeSweep_MultipleAddresses(t *testing.T) {
	mnemonicPath := writeTempMnemonic(t, testMnemonic24)
	ks := NewKeyService(mnemonicPath, "testnet")

	sendCount := 0
	mock := &mockSOLRPCClient{
		getBalanceFn: func(ctx context.Context, addr string) (uint64, error) {
			return 500_000_000, nil // 0.5 SOL
		},
		sendTransactionFn: func(ctx context.Context, txBase64 string) (string, error) {
			sendCount++
			return "5MockSig" + strconv.Itoa(sendCount), nil
		},
	}

	svc := NewSOLConsolidationService(ks, mock, nil, "testnet", nil)

	addresses := []models.AddressWithBalance{
		{AddressIndex: 0, Address: "3Cy3YNTFywCmxoxt8n7UH6hg6dLo5uACowX3CFceaSnx", NativeBalance: "500000000"},
		{AddressIndex: 1, Address: "5frqxtii9LeGq2bz3dSNokvZcEooF483MzeU24JrhcTA", NativeBalance: "500000000"},
		{AddressIndex: 2, Address: "3SuKj3MZU9dMZ9oR1R7afttihZFkWpfUmduuv9rmfMa1", NativeBalance: "500000000"},
	}

	result, err := svc.ExecuteNativeSweep(context.Background(), addresses, "11111111111111111111111111111111", "test-sweep")
	if err != nil {
		t.Fatalf("ExecuteNativeSweep error = %v", err)
	}

	if result.SuccessCount != 3 {
		t.Errorf("successCount = %d, want 3", result.SuccessCount)
	}

	if sendCount != 3 {
		t.Errorf("sendCount = %d, want 3", sendCount)
	}
}

func TestSOLNativeSweep_InsufficientBalance(t *testing.T) {
	mnemonicPath := writeTempMnemonic(t, testMnemonic24)
	ks := NewKeyService(mnemonicPath, "testnet")

	mock := &mockSOLRPCClient{
		getBalanceFn: func(ctx context.Context, addr string) (uint64, error) {
			return 3000, nil // Less than 5000 fee
		},
	}

	svc := NewSOLConsolidationService(ks, mock, nil, "testnet", nil)

	addresses := []models.AddressWithBalance{{
		AddressIndex:  0,
		Address:       "3Cy3YNTFywCmxoxt8n7UH6hg6dLo5uACowX3CFceaSnx",
		NativeBalance: "3000",
	}}

	result, err := svc.ExecuteNativeSweep(context.Background(), addresses, "11111111111111111111111111111111", "test-sweep")
	if err != nil {
		t.Fatalf("ExecuteNativeSweep error = %v", err)
	}

	if result.SuccessCount != 0 {
		t.Errorf("successCount = %d, want 0", result.SuccessCount)
	}
	if result.FailCount != 1 {
		t.Errorf("failCount = %d, want 1", result.FailCount)
	}
	if result.TxResults[0].Error == "" {
		t.Error("expected error message for insufficient balance")
	}
}

func TestSOLNativeSweep_ConfirmationTimeout(t *testing.T) {
	mnemonicPath := writeTempMnemonic(t, testMnemonic24)
	ks := NewKeyService(mnemonicPath, "testnet")

	// Return no confirmation status to trigger timeout.
	mock := &mockSOLRPCClient{
		getBalanceFn: func(ctx context.Context, addr string) (uint64, error) {
			return 1_000_000_000, nil
		},
		getSignatureStatusesFn: func(ctx context.Context, sigs []string) ([]SOLSignatureStatus, error) {
			return []SOLSignatureStatus{{}}, nil // Empty status = not confirmed.
		},
	}

	svc := NewSOLConsolidationService(ks, mock, nil, "testnet", nil)

	addresses := []models.AddressWithBalance{{
		AddressIndex:  0,
		Address:       "3Cy3YNTFywCmxoxt8n7UH6hg6dLo5uACowX3CFceaSnx",
		NativeBalance: "1000000000",
	}}

	// Use a short timeout context.
	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()

	result, err := svc.ExecuteNativeSweep(ctx, addresses, "11111111111111111111111111111111", "test-sweep")
	if err != nil {
		t.Fatalf("ExecuteNativeSweep error = %v", err)
	}

	// Confirmation is now async — broadcast success counts as success.
	// The background goroutine will update tx_state later.
	if result.SuccessCount != 1 {
		t.Errorf("successCount = %d, want 1", result.SuccessCount)
	}
}

func TestSOLNativeSweep_ConfirmationFailed(t *testing.T) {
	mnemonicPath := writeTempMnemonic(t, testMnemonic24)
	ks := NewKeyService(mnemonicPath, "testnet")

	mock := &mockSOLRPCClient{
		getBalanceFn: func(ctx context.Context, addr string) (uint64, error) {
			return 1_000_000_000, nil
		},
		getSignatureStatusesFn: func(ctx context.Context, sigs []string) ([]SOLSignatureStatus, error) {
			return []SOLSignatureStatus{{
				Slot: 50,
				Err:  map[string]interface{}{"InstructionError": []interface{}{0, "InsufficientFunds"}},
			}}, nil
		},
	}

	svc := NewSOLConsolidationService(ks, mock, nil, "testnet", nil)

	addresses := []models.AddressWithBalance{{
		AddressIndex:  0,
		Address:       "3Cy3YNTFywCmxoxt8n7UH6hg6dLo5uACowX3CFceaSnx",
		NativeBalance: "1000000000",
	}}

	result, err := svc.ExecuteNativeSweep(context.Background(), addresses, "11111111111111111111111111111111", "test-sweep")
	if err != nil {
		t.Fatalf("ExecuteNativeSweep error = %v", err)
	}

	// Confirmation is now async — broadcast success counts as success.
	// On-chain failure is detected by background goroutine and updates tx_state.
	if result.SuccessCount != 1 {
		t.Errorf("successCount = %d, want 1", result.SuccessCount)
	}
}

// --- SOL Token Sweep Tests ---

func TestSOLTokenSweep_WithExistingATA(t *testing.T) {
	mnemonicPath := writeTempMnemonic(t, testMnemonic24)
	ks := NewKeyService(mnemonicPath, "testnet")

	sendCalled := false
	mock := &mockSOLRPCClient{
		getBalanceFn: func(ctx context.Context, addr string) (uint64, error) {
			return 100_000_000, nil // 0.1 SOL — enough for fee
		},
		getAccountInfoFn: func(ctx context.Context, addr string) (bool, uint64, error) {
			return true, 2_039_280, nil // ATA exists
		},
		sendTransactionFn: func(ctx context.Context, txBase64 string) (string, error) {
			sendCalled = true
			return "5MockSigToken", nil
		},
	}

	svc := NewSOLConsolidationService(ks, mock, nil, "testnet", nil)

	addresses := []models.AddressWithBalance{{
		AddressIndex:  0,
		Address:       "3Cy3YNTFywCmxoxt8n7UH6hg6dLo5uACowX3CFceaSnx",
		NativeBalance: "100000000",
		TokenBalances: []models.TokenBalanceItem{{
			Symbol:  models.TokenUSDC,
			Balance: "20000000", // 20 USDC
		}},
	}}

	result, err := svc.ExecuteTokenSweep(
		context.Background(),
		addresses,
		"5frqxtii9LeGq2bz3dSNokvZcEooF483MzeU24JrhcTA",
		models.TokenUSDC,
		config.SOLDevnetUSDCMint,
		"test-sweep",
		nil,
	)
	if err != nil {
		t.Fatalf("ExecuteTokenSweep error = %v", err)
	}

	if !sendCalled {
		t.Error("expected SendTransaction to be called")
	}

	if result.SuccessCount != 1 {
		t.Errorf("successCount = %d, want 1", result.SuccessCount)
	}

	if result.TxResults[0].Amount != "20000000" {
		t.Errorf("amount = %s, want 20000000", result.TxResults[0].Amount)
	}
}

func TestSOLTokenSweep_WithATACreation(t *testing.T) {
	mnemonicPath := writeTempMnemonic(t, testMnemonic24)
	ks := NewKeyService(mnemonicPath, "testnet")

	mock := &mockSOLRPCClient{
		getBalanceFn: func(ctx context.Context, addr string) (uint64, error) {
			return 100_000_000, nil // Enough for fee + rent
		},
		getAccountInfoFn: func(ctx context.Context, addr string) (bool, uint64, error) {
			return false, 0, nil // ATA does NOT exist
		},
	}

	svc := NewSOLConsolidationService(ks, mock, nil, "testnet", nil)

	addresses := []models.AddressWithBalance{{
		AddressIndex:  0,
		Address:       "3Cy3YNTFywCmxoxt8n7UH6hg6dLo5uACowX3CFceaSnx",
		NativeBalance: "100000000",
		TokenBalances: []models.TokenBalanceItem{{
			Symbol:  models.TokenUSDC,
			Balance: "20000000",
		}},
	}}

	result, err := svc.ExecuteTokenSweep(
		context.Background(),
		addresses,
		"5frqxtii9LeGq2bz3dSNokvZcEooF483MzeU24JrhcTA",
		models.TokenUSDC,
		config.SOLDevnetUSDCMint,
		"test-sweep",
		nil,
	)
	if err != nil {
		t.Fatalf("ExecuteTokenSweep error = %v", err)
	}

	if result.SuccessCount != 1 {
		t.Errorf("successCount = %d, want 1", result.SuccessCount)
	}
}

func TestSOLTokenSweep_InsufficientSOLForFee(t *testing.T) {
	mnemonicPath := writeTempMnemonic(t, testMnemonic24)
	ks := NewKeyService(mnemonicPath, "testnet")

	mock := &mockSOLRPCClient{
		getBalanceFn: func(ctx context.Context, addr string) (uint64, error) {
			return 100, nil // Way too little for fee
		},
		getAccountInfoFn: func(ctx context.Context, addr string) (bool, uint64, error) {
			return true, 2_039_280, nil
		},
	}

	svc := NewSOLConsolidationService(ks, mock, nil, "testnet", nil)

	addresses := []models.AddressWithBalance{{
		AddressIndex:  0,
		Address:       "3Cy3YNTFywCmxoxt8n7UH6hg6dLo5uACowX3CFceaSnx",
		NativeBalance: "100",
		TokenBalances: []models.TokenBalanceItem{{
			Symbol:  models.TokenUSDC,
			Balance: "20000000",
		}},
	}}

	result, err := svc.ExecuteTokenSweep(
		context.Background(),
		addresses,
		"5frqxtii9LeGq2bz3dSNokvZcEooF483MzeU24JrhcTA",
		models.TokenUSDC,
		config.SOLDevnetUSDCMint,
		"test-sweep",
		nil,
	)
	if err != nil {
		t.Fatalf("ExecuteTokenSweep error = %v", err)
	}

	if result.FailCount != 1 {
		t.Errorf("failCount = %d, want 1", result.FailCount)
	}
}

// --- Confirmation Polling Tests ---

func TestWaitForSOLConfirmation_Success(t *testing.T) {
	confirmed := "confirmed"
	mock := &mockSOLRPCClient{
		getSignatureStatusesFn: func(ctx context.Context, sigs []string) ([]SOLSignatureStatus, error) {
			return []SOLSignatureStatus{{
				Slot:               42,
				ConfirmationStatus: &confirmed,
			}}, nil
		},
	}

	slot, err := WaitForSOLConfirmation(context.Background(), mock, "testSig")
	if err != nil {
		t.Fatalf("WaitForSOLConfirmation error = %v", err)
	}
	if slot != 42 {
		t.Errorf("slot = %d, want 42", slot)
	}
}

func TestWaitForSOLConfirmation_Failed(t *testing.T) {
	mock := &mockSOLRPCClient{
		getSignatureStatusesFn: func(ctx context.Context, sigs []string) ([]SOLSignatureStatus, error) {
			return []SOLSignatureStatus{{
				Slot: 50,
				Err:  "InsufficientFunds",
			}}, nil
		},
	}

	_, err := WaitForSOLConfirmation(context.Background(), mock, "testSig")
	if err == nil {
		t.Fatal("expected error for failed transaction")
	}
	if !errors.Is(err, config.ErrSOLTxFailed) {
		t.Errorf("error = %v, want %v", err, config.ErrSOLTxFailed)
	}
}

// --- Preview Tests ---

func TestSOLNativeSweep_Preview(t *testing.T) {
	mnemonicPath := writeTempMnemonic(t, testMnemonic24)
	ks := NewKeyService(mnemonicPath, "testnet")
	mock := &mockSOLRPCClient{}

	svc := NewSOLConsolidationService(ks, mock, nil, "testnet", nil)

	addresses := []models.AddressWithBalance{
		{AddressIndex: 0, Address: "addr1", NativeBalance: "1000000000"},  // 1 SOL
		{AddressIndex: 1, Address: "addr2", NativeBalance: "500000000"},   // 0.5 SOL
		{AddressIndex: 2, Address: "addr3", NativeBalance: "1000"},        // Too little (< 5000 fee)
	}

	preview, err := svc.PreviewNativeSweep(context.Background(), addresses, "dest")
	if err != nil {
		t.Fatalf("PreviewNativeSweep error = %v", err)
	}

	if preview.InputCount != 2 {
		t.Errorf("inputCount = %d, want 2 (third address too small)", preview.InputCount)
	}

	expectedFee := 2 * config.SOLBaseTransactionFee
	if preview.TotalFee != strconv.FormatUint(uint64(expectedFee), 10) {
		t.Errorf("totalFee = %s, want %d", preview.TotalFee, expectedFee)
	}

	expectedNet := uint64(1_000_000_000-config.SOLBaseTransactionFee) + uint64(500_000_000-config.SOLBaseTransactionFee)
	if preview.NetAmount != strconv.FormatUint(expectedNet, 10) {
		t.Errorf("netAmount = %s, want %d", preview.NetAmount, expectedNet)
	}
}

func TestSOLTokenSweep_Preview_WithATACreation(t *testing.T) {
	mnemonicPath := writeTempMnemonic(t, testMnemonic24)
	ks := NewKeyService(mnemonicPath, "testnet")

	mock := &mockSOLRPCClient{
		getAccountInfoFn: func(ctx context.Context, addr string) (bool, uint64, error) {
			return false, 0, nil // ATA doesn't exist
		},
	}

	svc := NewSOLConsolidationService(ks, mock, nil, "testnet", nil)

	addresses := []models.AddressWithBalance{{
		AddressIndex:  0,
		Address:       "3Cy3YNTFywCmxoxt8n7UH6hg6dLo5uACowX3CFceaSnx",
		NativeBalance: "100000000",
		TokenBalances: []models.TokenBalanceItem{{
			Symbol:  models.TokenUSDC,
			Balance: "20000000",
		}},
	}}

	preview, err := svc.PreviewTokenSweep(
		context.Background(),
		addresses,
		"5frqxtii9LeGq2bz3dSNokvZcEooF483MzeU24JrhcTA",
		models.TokenUSDC,
		config.SOLDevnetUSDCMint,
	)
	if err != nil {
		t.Fatalf("PreviewTokenSweep error = %v", err)
	}

	if !preview.NeedATACreation {
		t.Error("expected NeedATACreation = true")
	}

	if preview.ATARentCost != strconv.FormatUint(config.SOLATARentLamports, 10) {
		t.Errorf("ataRentCost = %s, want %d", preview.ATARentCost, config.SOLATARentLamports)
	}

	if preview.InputCount != 1 {
		t.Errorf("inputCount = %d, want 1", preview.InputCount)
	}
}

// --- Helper Tests ---

func TestFindTokenBalance(t *testing.T) {
	addr := models.AddressWithBalance{
		TokenBalances: []models.TokenBalanceItem{
			{Symbol: models.TokenUSDC, Balance: "20000000"},
			{Symbol: models.TokenUSDT, Balance: "50000000"},
		},
	}

	if got := findTokenBalance(addr, models.TokenUSDC); got != 20_000_000 {
		t.Errorf("USDC balance = %d, want 20000000", got)
	}
	if got := findTokenBalance(addr, models.TokenUSDT); got != 50_000_000 {
		t.Errorf("USDT balance = %d, want 50000000", got)
	}
	if got := findTokenBalance(addr, models.TokenNative); got != 0 {
		t.Errorf("native balance = %d, want 0", got)
	}
}

// --- DefaultSOLRPCClient Tests (httptest-based) ---

// solRPCResponse builds a JSON-RPC 2.0 success response body.
func solRPCResponse(result interface{}) []byte {
	raw, _ := json.Marshal(result)
	resp := map[string]interface{}{
		"jsonrpc": "2.0",
		"id":      1,
		"result":  json.RawMessage(raw),
	}
	body, _ := json.Marshal(resp)
	return body
}

// solRPCErrorResponse builds a JSON-RPC 2.0 error response body.
func solRPCErrorResponse(code int, message string) []byte {
	resp := map[string]interface{}{
		"jsonrpc": "2.0",
		"id":      1,
		"error": map[string]interface{}{
			"code":    code,
			"message": message,
		},
	}
	body, _ := json.Marshal(resp)
	return body
}

func TestDefaultSOLRPCClient_doRPCAllURLs_Fallback(t *testing.T) {
	t.Run("first URL fails, second succeeds", func(t *testing.T) {
		var server1Calls atomic.Int32
		server1 := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			server1Calls.Add(1)
			w.WriteHeader(http.StatusInternalServerError)
			w.Write([]byte("internal server error"))
		}))
		defer server1.Close()

		server2 := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.Header().Set("Content-Type", "application/json")
			w.Write(solRPCResponse(map[string]uint64{"value": 42}))
		}))
		defer server2.Close()

		client := NewDefaultSOLRPCClient(server1.Client(), []string{server1.URL, server2.URL})

		result, err := client.doRPCAllURLs(context.Background(), "getBalance", []interface{}{"someAddr"})
		if err != nil {
			t.Fatalf("doRPCAllURLs() error = %v, want nil (should fallback)", err)
		}

		if server1Calls.Load() != 1 {
			t.Errorf("server1 calls = %d, want 1", server1Calls.Load())
		}

		// Verify the result is parseable.
		var parsed struct {
			Value uint64 `json:"value"`
		}
		if err := json.Unmarshal(result, &parsed); err != nil {
			t.Fatalf("failed to unmarshal result: %v", err)
		}
		if parsed.Value != 42 {
			t.Errorf("result value = %d, want 42", parsed.Value)
		}
	})

	t.Run("all URLs fail", func(t *testing.T) {
		server1 := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.WriteHeader(http.StatusServiceUnavailable)
			w.Write([]byte("service unavailable"))
		}))
		defer server1.Close()

		server2 := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.WriteHeader(http.StatusBadGateway)
			w.Write([]byte("bad gateway"))
		}))
		defer server2.Close()

		client := NewDefaultSOLRPCClient(server1.Client(), []string{server1.URL, server2.URL})

		_, err := client.doRPCAllURLs(context.Background(), "getBalance", []interface{}{"someAddr"})
		if err == nil {
			t.Fatal("doRPCAllURLs() error = nil, want error when all URLs fail")
		}

		// The error should be from the first URL that failed.
		if !contains(err.Error(), "503") && !contains(err.Error(), "502") {
			t.Errorf("error = %v, want HTTP status code in error message", err)
		}
	})
}

func TestDefaultSOLRPCClient_GetLatestBlockhash(t *testing.T) {
	// Build a known 32-byte blockhash and its base58 encoding.
	var wantHash [32]byte
	for i := range 32 {
		wantHash[i] = byte(i + 1)
	}
	blockhashB58 := base58.Encode(wantHash[:])
	wantHeight := uint64(285_000_000)

	t.Run("valid response", func(t *testing.T) {
		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			// Verify the method is getLatestBlockhash.
			var req solRPCRequest
			if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
				t.Errorf("failed to decode request: %v", err)
			}
			if req.Method != "getLatestBlockhash" {
				t.Errorf("method = %s, want getLatestBlockhash", req.Method)
			}

			w.Header().Set("Content-Type", "application/json")
			w.Write(solRPCResponse(map[string]interface{}{
				"value": map[string]interface{}{
					"blockhash":            blockhashB58,
					"lastValidBlockHeight": wantHeight,
				},
			}))
		}))
		defer server.Close()

		client := NewDefaultSOLRPCClient(server.Client(), []string{server.URL})

		gotHash, gotHeight, err := client.GetLatestBlockhash(context.Background())
		if err != nil {
			t.Fatalf("GetLatestBlockhash() error = %v", err)
		}
		if gotHash != wantHash {
			t.Errorf("blockhash = %x, want %x", gotHash, wantHash)
		}
		if gotHeight != wantHeight {
			t.Errorf("lastValidBlockHeight = %d, want %d", gotHeight, wantHeight)
		}
	})

	t.Run("RPC error response", func(t *testing.T) {
		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.Header().Set("Content-Type", "application/json")
			w.Write(solRPCErrorResponse(-32005, "Node is behind by 42 slots"))
		}))
		defer server.Close()

		client := NewDefaultSOLRPCClient(server.Client(), []string{server.URL})

		_, _, err := client.GetLatestBlockhash(context.Background())
		if err == nil {
			t.Fatal("GetLatestBlockhash() error = nil, want error for RPC error response")
		}
		if !contains(err.Error(), "Node is behind") {
			t.Errorf("error = %v, want error containing 'Node is behind'", err)
		}
	})
}

func TestDefaultSOLRPCClient_SendTransaction(t *testing.T) {
	wantSignature := "5wHu1qwD7q3gMPz3e9YP9bYjMUdLGpVbSh3EDAx3pmMFELSbDWfLqkhJbcvxLwkYWvSNfwBxLgR4HMbDE7u3fVn"

	t.Run("valid response", func(t *testing.T) {
		var receivedTxBase64 string
		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			var req solRPCRequest
			if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
				t.Errorf("failed to decode request: %v", err)
			}
			if req.Method != "sendTransaction" {
				t.Errorf("method = %s, want sendTransaction", req.Method)
			}
			// First param is the base64-encoded transaction.
			if len(req.Params) > 0 {
				receivedTxBase64 = fmt.Sprintf("%v", req.Params[0])
			}

			w.Header().Set("Content-Type", "application/json")
			// sendTransaction returns a signature string directly as result.
			w.Write(solRPCResponse(wantSignature))
		}))
		defer server.Close()

		client := NewDefaultSOLRPCClient(server.Client(), []string{server.URL})

		gotSig, err := client.SendTransaction(context.Background(), "dGVzdHR4ZGF0YQ==")
		if err != nil {
			t.Fatalf("SendTransaction() error = %v", err)
		}
		if gotSig != wantSignature {
			t.Errorf("signature = %s, want %s", gotSig, wantSignature)
		}
		if receivedTxBase64 != "dGVzdHR4ZGF0YQ==" {
			t.Errorf("received tx = %s, want dGVzdHR4ZGF0YQ==", receivedTxBase64)
		}
	})

	t.Run("RPC error response", func(t *testing.T) {
		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.Header().Set("Content-Type", "application/json")
			w.Write(solRPCErrorResponse(-32002, "Transaction simulation failed"))
		}))
		defer server.Close()

		client := NewDefaultSOLRPCClient(server.Client(), []string{server.URL})

		_, err := client.SendTransaction(context.Background(), "dGVzdHR4ZGF0YQ==")
		if err == nil {
			t.Fatal("SendTransaction() error = nil, want error for RPC error response")
		}
		if !contains(err.Error(), "Transaction simulation failed") {
			t.Errorf("error = %v, want error containing 'Transaction simulation failed'", err)
		}
	})
}

func TestDefaultSOLRPCClient_GetSignatureStatuses(t *testing.T) {
	t.Run("mix of confirmed and pending", func(t *testing.T) {
		confirmed := "confirmed"
		var confirmations10 uint64 = 10

		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			var req solRPCRequest
			if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
				t.Errorf("failed to decode request: %v", err)
			}
			if req.Method != "getSignatureStatuses" {
				t.Errorf("method = %s, want getSignatureStatuses", req.Method)
			}

			w.Header().Set("Content-Type", "application/json")
			// Return a mix: first is confirmed, second is null (pending/unknown).
			w.Write(solRPCResponse(map[string]interface{}{
				"value": []interface{}{
					map[string]interface{}{
						"slot":               200,
						"confirmations":      confirmations10,
						"confirmationStatus": confirmed,
						"err":                nil,
					},
					nil, // Pending/unknown signature
				},
			}))
		}))
		defer server.Close()

		client := NewDefaultSOLRPCClient(server.Client(), []string{server.URL})

		statuses, err := client.GetSignatureStatuses(context.Background(), []string{"sig1", "sig2"})
		if err != nil {
			t.Fatalf("GetSignatureStatuses() error = %v", err)
		}
		if len(statuses) != 2 {
			t.Fatalf("statuses count = %d, want 2", len(statuses))
		}

		// First should be confirmed.
		if statuses[0].Slot != 200 {
			t.Errorf("statuses[0].Slot = %d, want 200", statuses[0].Slot)
		}
		if statuses[0].ConfirmationStatus == nil || *statuses[0].ConfirmationStatus != "confirmed" {
			t.Errorf("statuses[0].ConfirmationStatus = %v, want 'confirmed'", statuses[0].ConfirmationStatus)
		}
		if statuses[0].Confirmations == nil || *statuses[0].Confirmations != 10 {
			t.Errorf("statuses[0].Confirmations = %v, want 10", statuses[0].Confirmations)
		}

		// Second should be zero-value (null in the response maps to empty struct).
		if statuses[1].Slot != 0 {
			t.Errorf("statuses[1].Slot = %d, want 0 (null entry)", statuses[1].Slot)
		}
		if statuses[1].ConfirmationStatus != nil {
			t.Errorf("statuses[1].ConfirmationStatus = %v, want nil", statuses[1].ConfirmationStatus)
		}
	})

	t.Run("empty signatures list", func(t *testing.T) {
		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.Header().Set("Content-Type", "application/json")
			w.Write(solRPCResponse(map[string]interface{}{
				"value": []interface{}{},
			}))
		}))
		defer server.Close()

		client := NewDefaultSOLRPCClient(server.Client(), []string{server.URL})

		statuses, err := client.GetSignatureStatuses(context.Background(), []string{})
		if err != nil {
			t.Fatalf("GetSignatureStatuses() error = %v", err)
		}
		if len(statuses) != 0 {
			t.Errorf("statuses count = %d, want 0", len(statuses))
		}
	})
}

func TestDefaultSOLRPCClient_GetBalance(t *testing.T) {
	t.Run("valid response", func(t *testing.T) {
		wantLamports := uint64(2_500_000_000) // 2.5 SOL

		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			var req solRPCRequest
			if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
				t.Errorf("failed to decode request: %v", err)
			}
			if req.Method != "getBalance" {
				t.Errorf("method = %s, want getBalance", req.Method)
			}
			// Verify the address param is passed through.
			if len(req.Params) < 1 {
				t.Error("expected at least 1 param (address)")
			} else if fmt.Sprintf("%v", req.Params[0]) != "SomeSOLAddress123" {
				t.Errorf("address param = %v, want SomeSOLAddress123", req.Params[0])
			}

			w.Header().Set("Content-Type", "application/json")
			w.Write(solRPCResponse(map[string]interface{}{
				"value": wantLamports,
			}))
		}))
		defer server.Close()

		client := NewDefaultSOLRPCClient(server.Client(), []string{server.URL})

		got, err := client.GetBalance(context.Background(), "SomeSOLAddress123")
		if err != nil {
			t.Fatalf("GetBalance() error = %v", err)
		}
		if got != wantLamports {
			t.Errorf("balance = %d, want %d", got, wantLamports)
		}
	})

	t.Run("RPC error", func(t *testing.T) {
		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.Header().Set("Content-Type", "application/json")
			w.Write(solRPCErrorResponse(-32600, "Invalid request"))
		}))
		defer server.Close()

		client := NewDefaultSOLRPCClient(server.Client(), []string{server.URL})

		_, err := client.GetBalance(context.Background(), "SomeSOLAddress123")
		if err == nil {
			t.Fatal("GetBalance() error = nil, want error")
		}
		if !contains(err.Error(), "Invalid request") {
			t.Errorf("error = %v, want error containing 'Invalid request'", err)
		}
	})
}

func TestDefaultSOLRPCClient_GetAccountInfo(t *testing.T) {
	t.Run("account exists", func(t *testing.T) {
		wantLamports := uint64(5_000_000) // 0.005 SOL

		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			var req solRPCRequest
			if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
				t.Errorf("failed to decode request: %v", err)
			}
			if req.Method != "getAccountInfo" {
				t.Errorf("method = %s, want getAccountInfo", req.Method)
			}

			w.Header().Set("Content-Type", "application/json")
			w.Write(solRPCResponse(map[string]interface{}{
				"value": map[string]interface{}{
					"lamports":   wantLamports,
					"data":       []string{"", "base64"},
					"owner":      "11111111111111111111111111111111",
					"executable": false,
					"rentEpoch":  0,
				},
			}))
		}))
		defer server.Close()

		client := NewDefaultSOLRPCClient(server.Client(), []string{server.URL})

		exists, lamports, err := client.GetAccountInfo(context.Background(), "SomeSOLAddress123")
		if err != nil {
			t.Fatalf("GetAccountInfo() error = %v", err)
		}
		if !exists {
			t.Error("exists = false, want true")
		}
		if lamports != wantLamports {
			t.Errorf("lamports = %d, want %d", lamports, wantLamports)
		}
	})

	t.Run("account does not exist (null value)", func(t *testing.T) {
		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.Header().Set("Content-Type", "application/json")
			// Solana returns {"value": null} for non-existent accounts.
			w.Write(solRPCResponse(map[string]interface{}{
				"value": nil,
			}))
		}))
		defer server.Close()

		client := NewDefaultSOLRPCClient(server.Client(), []string{server.URL})

		exists, lamports, err := client.GetAccountInfo(context.Background(), "NonExistentAddress")
		if err != nil {
			t.Fatalf("GetAccountInfo() error = %v", err)
		}
		if exists {
			t.Error("exists = true, want false")
		}
		if lamports != 0 {
			t.Errorf("lamports = %d, want 0", lamports)
		}
	})
}

// contains is a simple substring check helper for test assertions.
func contains(s, substr string) bool {
	return len(s) >= len(substr) && searchSubstring(s, substr)
}

func searchSubstring(s, substr string) bool {
	for i := 0; i <= len(s)-len(substr); i++ {
		if s[i:i+len(substr)] == substr {
			return true
		}
	}
	return false
}
