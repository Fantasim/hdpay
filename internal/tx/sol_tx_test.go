package tx

import (
	"context"
	"errors"
	"strconv"
	"testing"
	"time"

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

	svc := NewSOLConsolidationService(ks, mock, nil, "testnet")

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

	svc := NewSOLConsolidationService(ks, mock, nil, "testnet")

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

	svc := NewSOLConsolidationService(ks, mock, nil, "testnet")

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

	svc := NewSOLConsolidationService(ks, mock, nil, "testnet")

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

	// Should have 1 result, failed due to confirmation timeout.
	if result.FailCount != 1 {
		t.Errorf("failCount = %d, want 1", result.FailCount)
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

	svc := NewSOLConsolidationService(ks, mock, nil, "testnet")

	addresses := []models.AddressWithBalance{{
		AddressIndex:  0,
		Address:       "3Cy3YNTFywCmxoxt8n7UH6hg6dLo5uACowX3CFceaSnx",
		NativeBalance: "1000000000",
	}}

	result, err := svc.ExecuteNativeSweep(context.Background(), addresses, "11111111111111111111111111111111", "test-sweep")
	if err != nil {
		t.Fatalf("ExecuteNativeSweep error = %v", err)
	}

	if result.FailCount != 1 {
		t.Errorf("failCount = %d, want 1", result.FailCount)
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

	svc := NewSOLConsolidationService(ks, mock, nil, "testnet")

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

	svc := NewSOLConsolidationService(ks, mock, nil, "testnet")

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

	svc := NewSOLConsolidationService(ks, mock, nil, "testnet")

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

	svc := NewSOLConsolidationService(ks, mock, nil, "testnet")

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

	svc := NewSOLConsolidationService(ks, mock, nil, "testnet")

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
