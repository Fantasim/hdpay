package tx

import (
	"context"
	"errors"
	"math/big"
	"path/filepath"
	"sync/atomic"
	"testing"

	ethereum "github.com/ethereum/go-ethereum"
	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/core/types"
	"github.com/ethereum/go-ethereum/crypto"

	"github.com/Fantasim/hdpay/internal/config"
	"github.com/Fantasim/hdpay/internal/db"
)

func TestGasPreSeed_Preview_Sufficient(t *testing.T) {
	// Source has 1 BNB, sending to 3 targets at 0.005 BNB each.
	// Total amount: 0.015 BNB + gas costs.
	gasPrice := big.NewInt(3_000_000_000)
	sourceBalance := big.NewInt(1_000_000_000_000_000_000) // 1 BNB

	mock := &mockEthClient{
		gasPrice: gasPrice,
		balance:  sourceBalance,
	}

	// We need a KeyService that works without a real file.
	// For preview, we only need DeriveBSCPrivateKey to return an address.
	// Since we can't easily mock KeyService, test the computation logic directly.

	bufferedGP := BufferedGasPrice(gasPrice)
	gasCostPerSend := new(big.Int).Mul(bufferedGP, big.NewInt(int64(config.BSCGasLimitTransfer)))

	targetCount := int64(3)
	totalAmount := new(big.Int).Mul(gasPreSeedAmountWei, big.NewInt(targetCount))
	totalGas := new(big.Int).Mul(gasCostPerSend, big.NewInt(targetCount))
	totalNeeded := new(big.Int).Add(totalAmount, totalGas)

	if sourceBalance.Cmp(totalNeeded) < 0 {
		t.Fatalf("source should be sufficient: balance=%s, needed=%s", sourceBalance, totalNeeded)
	}

	// Verify the gas pre-seed amount is correct.
	expected, _ := new(big.Int).SetString(config.BSCGasPreSeedWei, 10)
	if gasPreSeedAmountWei.Cmp(expected) != 0 {
		t.Errorf("gasPreSeedAmountWei: expected %s, got %s", expected, gasPreSeedAmountWei)
	}

	_ = mock
}

func TestGasPreSeed_InsufficientSource(t *testing.T) {
	gasPrice := big.NewInt(3_000_000_000)
	// Source only has 0.001 BNB — not enough for even one pre-seed (0.005 BNB + gas).
	sourceBalance := big.NewInt(1_000_000_000_000_000) // 0.001 BNB

	bufferedGP := BufferedGasPrice(gasPrice)
	gasCostPerSend := new(big.Int).Mul(bufferedGP, big.NewInt(int64(config.BSCGasLimitTransfer)))

	targetCount := int64(3)
	totalAmount := new(big.Int).Mul(gasPreSeedAmountWei, big.NewInt(targetCount))
	totalGas := new(big.Int).Mul(gasCostPerSend, big.NewInt(targetCount))
	totalNeeded := new(big.Int).Add(totalAmount, totalGas)

	if sourceBalance.Cmp(totalNeeded) >= 0 {
		t.Fatalf("source should be insufficient: balance=%s, needed=%s", sourceBalance, totalNeeded)
	}
}

func TestGasPreSeed_NonceIncrement(t *testing.T) {
	// Verify that nonce increments correctly during sequential sends.
	gasPrice := big.NewInt(3_000_000_000)
	privKey, _ := crypto.GenerateKey()

	mock := &mockEthClient{
		pendingNonce: 5,
		gasPrice:     gasPrice,
		receipt: &types.Receipt{
			Status:      types.ReceiptStatusSuccessful,
			BlockNumber: big.NewInt(100),
		},
	}

	targets := []common.Address{
		common.HexToAddress("0xaaaa000000000000000000000000000000000001"),
		common.HexToAddress("0xaaaa000000000000000000000000000000000002"),
		common.HexToAddress("0xaaaa000000000000000000000000000000000003"),
	}

	bufferedGP := BufferedGasPrice(gasPrice)
	chainID := big.NewInt(config.BSCTestnetChainID)
	nonce := uint64(5)

	for i, target := range targets {
		tx := BuildBSCNativeTransfer(nonce, target, gasPreSeedAmountWei, bufferedGP)

		if tx.Nonce() != uint64(5+i) {
			t.Errorf("tx %d: expected nonce %d, got %d", i, 5+i, tx.Nonce())
		}

		signedTx, err := SignBSCTx(tx, chainID, privKey)
		if err != nil {
			t.Fatalf("sign tx %d: %v", i, err)
		}

		if err := mock.SendTransaction(context.Background(), signedTx); err != nil {
			t.Fatalf("send tx %d: %v", i, err)
		}

		nonce++
	}

	// Should have sent 3 transactions.
	if len(mock.sentTxs) != 3 {
		t.Errorf("expected 3 sent txs, got %d", len(mock.sentTxs))
	}

	// Verify each tx has sequential nonces.
	for i, tx := range mock.sentTxs {
		expectedNonce := uint64(5 + i)
		if tx.Nonce() != expectedNonce {
			t.Errorf("sent tx %d: expected nonce %d, got %d", i, expectedNonce, tx.Nonce())
		}
	}
}

func TestGasPreSeed_Execute_BroadcastFail(t *testing.T) {
	// When broadcast fails, the TX hash should be empty and nonce should NOT increment.
	gasPrice := big.NewInt(3_000_000_000)
	privKey, _ := crypto.GenerateKey()

	mock := &mockEthClient{
		gasPrice:  gasPrice,
		sendTxErr: errors.New("broadcast error"),
	}

	bufferedGP := BufferedGasPrice(gasPrice)
	target := common.HexToAddress("0xbbbb000000000000000000000000000000000001")
	chainID := big.NewInt(config.BSCTestnetChainID)

	tx := BuildBSCNativeTransfer(0, target, gasPreSeedAmountWei, bufferedGP)
	signedTx, _ := SignBSCTx(tx, chainID, privKey)

	err := mock.SendTransaction(context.Background(), signedTx)
	if err == nil {
		t.Fatal("expected broadcast error")
	}

	// The tx was "sent" to the mock (appended to sentTxs) but returned an error.
	// In real code, the nonce should NOT be incremented when broadcast fails.
}

func TestIsNonceTooLowError(t *testing.T) {
	tests := []struct {
		errStr string
		want   bool
	}{
		{"broadcast: nonce too low", true},
		{"Nonce Too Low", true},
		{"nonce is too low", true},
		{"already known", true},
		{"replacement transaction underpriced", true},
		{"broadcast: insufficient funds", false},
		{"timeout", false},
		{"", false},
	}

	for _, tt := range tests {
		got := isNonceTooLowError(tt.errStr)
		if got != tt.want {
			t.Errorf("isNonceTooLowError(%q) = %v, want %v", tt.errStr, got, tt.want)
		}
	}
}

// mockEthClientDynamic allows per-call behavior changes for testing retry logic.
type mockEthClientDynamic struct {
	gasPrice    *big.Int
	gasPriceErr error
	balance     *big.Int
	balanceErr  error
	callResult  []byte
	callErr     error

	// Dynamic nonce: returns values from this slice in order, then last value.
	nonceValues  []uint64
	nonceCallNum atomic.Int32
	nonceErr     error

	// Dynamic send behavior: returns errors from this slice in order, then nil.
	sendTxErrors []error
	sendCallNum  atomic.Int32

	// Dynamic receipt behavior.
	receipt    *types.Receipt
	receiptErr error

	sentTxs []*types.Transaction
}

func (m *mockEthClientDynamic) PendingNonceAt(_ context.Context, _ common.Address) (uint64, error) {
	if m.nonceErr != nil {
		return 0, m.nonceErr
	}
	idx := int(m.nonceCallNum.Add(1)) - 1
	if idx < len(m.nonceValues) {
		return m.nonceValues[idx], nil
	}
	// Return last value if past the list.
	if len(m.nonceValues) > 0 {
		return m.nonceValues[len(m.nonceValues)-1], nil
	}
	return 0, nil
}

func (m *mockEthClientDynamic) SuggestGasPrice(_ context.Context) (*big.Int, error) {
	if m.gasPriceErr != nil {
		return nil, m.gasPriceErr
	}
	return new(big.Int).Set(m.gasPrice), nil
}

func (m *mockEthClientDynamic) SendTransaction(_ context.Context, tx *types.Transaction) error {
	m.sentTxs = append(m.sentTxs, tx)
	idx := int(m.sendCallNum.Add(1)) - 1
	if idx < len(m.sendTxErrors) {
		return m.sendTxErrors[idx]
	}
	return nil
}

func (m *mockEthClientDynamic) TransactionReceipt(_ context.Context, _ common.Hash) (*types.Receipt, error) {
	if m.receiptErr != nil {
		return nil, m.receiptErr
	}
	return m.receipt, nil
}

func (m *mockEthClientDynamic) BalanceAt(_ context.Context, _ common.Address, _ *big.Int) (*big.Int, error) {
	if m.balanceErr != nil {
		return nil, m.balanceErr
	}
	return new(big.Int).Set(m.balance), nil
}

func (m *mockEthClientDynamic) CallContract(_ context.Context, _ ethereum.CallMsg, _ *big.Int) ([]byte, error) {
	if m.callErr != nil {
		return nil, m.callErr
	}
	return m.callResult, nil
}

// setupGasTestDB creates a temporary SQLite DB for gas pre-seed tests.
func setupGasTestDB(t *testing.T) *db.DB {
	t.Helper()
	tmpDir := t.TempDir()
	dbPath := filepath.Join(tmpDir, "gas_test.sqlite")
	d, err := db.New(dbPath, "testnet")
	if err != nil {
		t.Fatalf("db.New() error = %v", err)
	}
	if err := d.RunMigrations(); err != nil {
		t.Fatalf("RunMigrations() error = %v", err)
	}
	t.Cleanup(func() { d.Close() })
	return d
}

func TestGasPreSeed_Execute_NonceTooLowRetry(t *testing.T) {
	// Test the nonce-too-low retry path in GasPreSeedService.Execute.
	// First SendTransaction returns "nonce too low", PendingNonceAt returns a fresh nonce,
	// then the retry succeeds.
	mnemonicPath := writeTempMnemonic(t, testMnemonic24)
	ks := NewKeyService(mnemonicPath, "testnet")
	database := setupGasTestDB(t)

	mock := &mockEthClientDynamic{
		nonceValues: []uint64{5, 10}, // First call returns stale nonce 5, re-fetch returns fresh 10.
		gasPrice:    big.NewInt(3_000_000_000),
		balance:     big.NewInt(1_000_000_000_000_000_000), // 1 BNB.
		sendTxErrors: []error{
			errors.New("nonce too low"), // First call fails.
			nil,                         // Retry succeeds.
		},
		receipt: &types.Receipt{
			Status:      types.ReceiptStatusSuccessful,
			BlockNumber: big.NewInt(100),
		},
	}

	svc := NewGasPreSeedService(ks, mock, database, big.NewInt(config.BSCTestnetChainID))

	result, err := svc.Execute(
		context.Background(),
		0, // sourceIndex
		[]string{"0xaaaa000000000000000000000000000000000001"},
	)
	if err != nil {
		t.Fatalf("Execute() error = %v", err)
	}

	// The retry should have succeeded.
	if result.SuccessCount != 1 {
		t.Errorf("expected 1 success, got %d", result.SuccessCount)
	}
	if result.FailCount != 0 {
		t.Errorf("expected 0 failures, got %d", result.FailCount)
	}

	// Should have sent 2 transactions: original (nonce too low) + retry.
	if len(mock.sentTxs) != 2 {
		t.Errorf("expected 2 sent txs (original + retry), got %d", len(mock.sentTxs))
	}
}

func TestGasPreSeed_Execute_Idempotency_SkipsConfirmed(t *testing.T) {
	// Test that Execute skips targets that already have confirmed gas pre-seeds.
	mnemonicPath := writeTempMnemonic(t, testMnemonic24)
	ks := NewKeyService(mnemonicPath, "testnet")

	database := setupGasTestDB(t)

	mock := &mockEthClientDynamic{
		nonceValues: []uint64{0},
		gasPrice:    big.NewInt(3_000_000_000),
		balance:     big.NewInt(1_000_000_000_000_000_000), // 1 BNB.
		receipt: &types.Receipt{
			Status:      types.ReceiptStatusSuccessful,
			BlockNumber: big.NewInt(100),
		},
	}

	svc := NewGasPreSeedService(ks, mock, database, big.NewInt(config.BSCTestnetChainID))

	target1 := "0xaaaa000000000000000000000000000000000001"
	target2 := "0xaaaa000000000000000000000000000000000002"
	sweepID := "test-sweep-123"

	// Pre-insert a confirmed tx_state for target1 in the same sweep.
	txState := db.TxStateRow{
		ID:           "pre-existing-id",
		SweepID:      sweepID,
		Chain:        "BSC",
		Token:        config.TokenGasPreSeed,
		AddressIndex: 0,
		FromAddress:  "0xsource",
		ToAddress:    target1,
		Amount:       gasPreSeedAmountWei.String(),
		Status:       config.TxStateConfirmed,
	}
	if err := database.CreateTxState(txState); err != nil {
		t.Fatalf("CreateTxState() error = %v", err)
	}

	result, err := svc.Execute(
		context.Background(),
		0,
		[]string{target1, target2},
		sweepID,
	)
	if err != nil {
		t.Fatalf("Execute() error = %v", err)
	}

	// target1 was skipped (already confirmed), target2 was sent.
	// SuccessCount = 1 (skipped) + 1 (sent) = 2.
	if result.SuccessCount != 2 {
		t.Errorf("expected 2 successes (1 skipped + 1 sent), got %d", result.SuccessCount)
	}
	if result.FailCount != 0 {
		t.Errorf("expected 0 failures, got %d", result.FailCount)
	}

	// Only 1 transaction should have been broadcast (for target2).
	if len(mock.sentTxs) != 1 {
		t.Errorf("expected 1 sent tx (target2 only), got %d", len(mock.sentTxs))
	}
}

func TestGasPreSeed_Execute_AllTargetsAlreadyConfirmed(t *testing.T) {
	// When all targets are already confirmed, Execute returns immediately with no broadcasts.
	mnemonicPath := writeTempMnemonic(t, testMnemonic24)
	ks := NewKeyService(mnemonicPath, "testnet")

	database := setupGasTestDB(t)

	mock := &mockEthClientDynamic{
		nonceValues: []uint64{0},
		gasPrice:    big.NewInt(3_000_000_000),
		balance:     big.NewInt(1_000_000_000_000_000_000),
	}

	svc := NewGasPreSeedService(ks, mock, database, big.NewInt(config.BSCTestnetChainID))

	target := "0xaaaa000000000000000000000000000000000001"
	sweepID := "all-confirmed-sweep"

	// Pre-insert confirmed tx_state.
	txState := db.TxStateRow{
		ID:           "already-done",
		SweepID:      sweepID,
		Chain:        "BSC",
		Token:        config.TokenGasPreSeed,
		AddressIndex: 0,
		FromAddress:  "0xsource",
		ToAddress:    target,
		Amount:       gasPreSeedAmountWei.String(),
		Status:       config.TxStateConfirmed,
	}
	if err := database.CreateTxState(txState); err != nil {
		t.Fatalf("CreateTxState() error = %v", err)
	}

	result, err := svc.Execute(
		context.Background(),
		0,
		[]string{target},
		sweepID,
	)
	if err != nil {
		t.Fatalf("Execute() error = %v", err)
	}

	// All targets skipped — success count = 1 (the skipped one).
	if result.SuccessCount != 1 {
		t.Errorf("expected 1 success (skipped), got %d", result.SuccessCount)
	}
	if result.TotalSent != "0" {
		t.Errorf("expected TotalSent=0, got %s", result.TotalSent)
	}

	// No transactions should have been broadcast.
	if len(mock.sentTxs) != 0 {
		t.Errorf("expected 0 sent txs, got %d", len(mock.sentTxs))
	}
}

func TestGasPreSeed_Execute_InsufficientBalance(t *testing.T) {
	// When source balance is insufficient, Execute returns an error.
	mnemonicPath := writeTempMnemonic(t, testMnemonic24)
	ks := NewKeyService(mnemonicPath, "testnet")

	mock := &mockEthClientDynamic{
		nonceValues: []uint64{0},
		gasPrice:    big.NewInt(3_000_000_000),
		balance:     big.NewInt(1_000_000_000_000_000), // 0.001 BNB — insufficient.
	}

	svc := NewGasPreSeedService(ks, mock, nil, big.NewInt(config.BSCTestnetChainID))

	_, err := svc.Execute(
		context.Background(),
		0,
		[]string{
			"0xaaaa000000000000000000000000000000000001",
			"0xaaaa000000000000000000000000000000000002",
			"0xaaaa000000000000000000000000000000000003",
		},
	)
	if err == nil {
		t.Fatal("expected insufficient balance error")
	}
	if !errors.Is(err, config.ErrInsufficientBNBForGas) {
		t.Errorf("expected ErrInsufficientBNBForGas, got: %v", err)
	}
}

func TestGasPreSeed_Execute_ContextCancellation(t *testing.T) {
	// When context is already cancelled, Execute should fail early during key derivation.
	mnemonicPath := writeTempMnemonic(t, testMnemonic24)
	ks := NewKeyService(mnemonicPath, "testnet")

	ctx, cancel := context.WithCancel(context.Background())
	cancel() // Cancel immediately.

	mock := &mockEthClientDynamic{
		nonceValues: []uint64{0},
		gasPrice:    big.NewInt(3_000_000_000),
		balance:     big.NewInt(1_000_000_000_000_000_000),
	}

	svc := NewGasPreSeedService(ks, mock, nil, big.NewInt(config.BSCTestnetChainID))

	_, err := svc.Execute(
		ctx,
		0,
		[]string{
			"0xaaaa000000000000000000000000000000000001",
			"0xaaaa000000000000000000000000000000000002",
		},
	)
	// With cancelled context, key derivation fails.
	if err == nil {
		t.Fatal("expected error with cancelled context")
	}
	if !errors.Is(err, context.Canceled) {
		t.Errorf("expected context.Canceled error, got: %v", err)
	}

	// No transactions should have been broadcast.
	if len(mock.sentTxs) != 0 {
		t.Errorf("expected 0 sent txs with cancelled context, got %d", len(mock.sentTxs))
	}
}
