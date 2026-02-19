package tx

import (
	"context"
	"math/big"
	"net/http"
	"path/filepath"
	"testing"

	"github.com/ethereum/go-ethereum/core/types"

	"github.com/Fantasim/hdpay/internal/config"
	"github.com/Fantasim/hdpay/internal/db"
	"github.com/Fantasim/hdpay/internal/models"
)

// setupReconcilerTestDB creates a fresh test database with migrations applied.
func setupReconcilerTestDB(t *testing.T) *db.DB {
	t.Helper()
	tmpDir := t.TempDir()
	dbPath := filepath.Join(tmpDir, "reconciler_test.sqlite")

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

func TestReconcilePending_ConfirmedSOL(t *testing.T) {
	database := setupReconcilerTestDB(t)

	// Insert a tx_state row in "confirming" status with a txHash.
	txStateRow := db.TxStateRow{
		ID:          "test-sol-001",
		SweepID:     "sweep-001",
		Chain:       "SOL",
		Token:       "SOL",
		AddressIndex: 0,
		FromAddress: "SolFrom",
		ToAddress:   "SolTo",
		Amount:      "1000000000",
		TxHash:      "5MockSigAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAA",
		Status:      config.TxStateConfirming,
	}
	if err := database.CreateTxState(txStateRow); err != nil {
		t.Fatalf("CreateTxState() error = %v", err)
	}

	// Also insert a matching transactions row (as would happen during normal broadcast).
	if _, err := database.InsertTransaction(models.Transaction{
		Chain:        models.ChainSOL,
		AddressIndex: 0,
		TxHash:       txStateRow.TxHash,
		Direction:    "send",
		Token:        models.TokenNative,
		Amount:       "1000000000",
		FromAddress:  "SolFrom",
		ToAddress:    "SolTo",
		Status:       "pending",
	}); err != nil {
		t.Fatalf("InsertTransaction() error = %v", err)
	}

	// Mock SOL client that returns confirmed.
	confirmed := "confirmed"
	solClient := &mockSOLRPCClient{
		getSignatureStatusesFn: func(ctx context.Context, sigs []string) ([]SOLSignatureStatus, error) {
			return []SOLSignatureStatus{{
				Slot:               100,
				ConfirmationStatus: &confirmed,
			}}, nil
		},
	}

	reconciler := NewTxReconciler(database, &http.Client{}, nil, nil, solClient)
	reconciler.ReconcilePending(context.Background())

	// Verify tx_state is confirmed.
	states, err := database.GetTxStatesBySweepID("sweep-001")
	if err != nil {
		t.Fatalf("GetTxStatesBySweepID() error = %v", err)
	}
	if len(states) != 1 {
		t.Fatalf("expected 1 tx_state, got %d", len(states))
	}
	if states[0].Status != config.TxStateConfirmed {
		t.Errorf("tx_state status = %q, want %q", states[0].Status, config.TxStateConfirmed)
	}

	// Verify transactions table is also confirmed.
	tx, err := database.GetTransactionByHash(models.ChainSOL, txStateRow.TxHash)
	if err != nil {
		t.Fatalf("GetTransactionByHash() error = %v", err)
	}
	if tx.Status != "confirmed" {
		t.Errorf("transaction status = %q, want %q", tx.Status, "confirmed")
	}
	if tx.ConfirmedAt == "" {
		t.Error("transaction ConfirmedAt should be set")
	}
}

func TestReconcilePending_ConfirmedBSC(t *testing.T) {
	database := setupReconcilerTestDB(t)

	txHash := "0xaabbccdd00000000000000000000000000000000000000000000000000000001"

	// Insert tx_state in confirming status.
	if err := database.CreateTxState(db.TxStateRow{
		ID:          "test-bsc-001",
		SweepID:     "sweep-002",
		Chain:       "BSC",
		Token:       "BNB",
		AddressIndex: 0,
		FromAddress: "0xFrom",
		ToAddress:   "0xTo",
		Amount:      "1000000000000000000",
		TxHash:      txHash,
		Status:      config.TxStateConfirming,
	}); err != nil {
		t.Fatal(err)
	}

	if _, err := database.InsertTransaction(models.Transaction{
		Chain: models.ChainBSC, AddressIndex: 0,
		TxHash: txHash, Direction: "send",
		Token: models.TokenNative, Amount: "1000000000000000000",
		FromAddress: "0xFrom", ToAddress: "0xTo", Status: "pending",
	}); err != nil {
		t.Fatal(err)
	}

	// Mock BSC client returns a confirmed receipt.
	ethClient := &mockEthClient{
		receipt: &types.Receipt{
			Status:      types.ReceiptStatusSuccessful,
			BlockNumber: big.NewInt(12345),
		},
	}

	reconciler := NewTxReconciler(database, &http.Client{}, nil, ethClient, nil)
	reconciler.ReconcilePending(context.Background())

	// Verify.
	tx, err := database.GetTransactionByHash(models.ChainBSC, txHash)
	if err != nil {
		t.Fatal(err)
	}
	if tx.Status != "confirmed" {
		t.Errorf("transaction status = %q, want confirmed", tx.Status)
	}
}

func TestReconcilePending_NoTxHash_MarkedFailed(t *testing.T) {
	database := setupReconcilerTestDB(t)

	// Insert tx_state without a txHash (was never broadcast).
	if err := database.CreateTxState(db.TxStateRow{
		ID:          "test-no-hash-001",
		SweepID:     "sweep-003",
		Chain:       "BTC",
		Token:       "BTC",
		AddressIndex: 0,
		FromAddress: "bc1qfrom",
		ToAddress:   "bc1qto",
		Amount:      "50000",
		TxHash:      "",
		Status:      config.TxStatePending,
	}); err != nil {
		t.Fatal(err)
	}

	reconciler := NewTxReconciler(database, &http.Client{}, nil, nil, nil)
	reconciler.ReconcilePending(context.Background())

	// Verify tx_state is marked failed.
	states, err := database.GetTxStatesBySweepID("sweep-003")
	if err != nil {
		t.Fatal(err)
	}
	if len(states) != 1 {
		t.Fatalf("expected 1 state, got %d", len(states))
	}
	if states[0].Status != config.TxStateFailed {
		t.Errorf("status = %q, want %q", states[0].Status, config.TxStateFailed)
	}
}

func TestReconcilePending_NoPendingTransactions(t *testing.T) {
	database := setupReconcilerTestDB(t)

	// No tx_state rows at all — should complete without error.
	reconciler := NewTxReconciler(database, &http.Client{}, nil, nil, nil)
	reconciler.ReconcilePending(context.Background())
	// No panic, no error = success.
}
