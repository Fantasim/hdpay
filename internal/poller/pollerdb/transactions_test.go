package pollerdb

import (
	"fmt"
	"testing"

	"github.com/Fantasim/hdpay/internal/poller/models"
)

func seedWatch(t *testing.T, db *DB, id string) {
	t.Helper()
	db.CreateWatch(&models.Watch{
		ID: id, Chain: "BTC", Address: "addr1",
		Status: models.WatchStatusActive, StartedAt: "2026-01-01", ExpiresAt: "2026-01-02",
	})
}

func TestInsertTransaction_And_GetByTxHash(t *testing.T) {
	db := newTestDB(t)
	seedWatch(t, db, "w1")

	tx := &models.Transaction{
		WatchID:     "w1",
		TxHash:      "abc123",
		Chain:       "BTC",
		Address:     "addr1",
		Token:       "BTC",
		AmountRaw:   "168841",
		AmountHuman: "0.00168841",
		Decimals:    8,
		USDValue:    100.0,
		USDPrice:    59250.0,
		Tier:        4,
		Multiplier:  1.3,
		Points:      13000,
		Status:      models.TxStatusPending,
		DetectedAt:  "2026-01-01T12:00:00Z",
	}

	id, err := db.InsertTransaction(tx)
	if err != nil {
		t.Fatalf("InsertTransaction() error = %v", err)
	}
	if id == 0 {
		t.Error("InsertTransaction() should return non-zero ID")
	}

	got, err := db.GetByTxHash("abc123")
	if err != nil {
		t.Fatalf("GetByTxHash() error = %v", err)
	}
	if got == nil {
		t.Fatal("GetByTxHash() returned nil")
	}
	if got.TxHash != "abc123" {
		t.Errorf("TxHash = %q, want abc123", got.TxHash)
	}
	if got.Points != 13000 {
		t.Errorf("Points = %d, want 13000", got.Points)
	}
	if got.Status != models.TxStatusPending {
		t.Errorf("Status = %q, want PENDING", got.Status)
	}
}

func TestInsertTransaction_DuplicateHash(t *testing.T) {
	db := newTestDB(t)
	seedWatch(t, db, "w1")

	tx := &models.Transaction{
		WatchID: "w1", TxHash: "dup1", Chain: "BTC", Address: "addr1", Token: "BTC",
		AmountRaw: "100", AmountHuman: "0.000001", Decimals: 8,
		USDValue: 1.0, USDPrice: 100000.0, Tier: 1, Multiplier: 1.0, Points: 100,
		Status: models.TxStatusPending, DetectedAt: "2026-01-01",
	}

	if _, err := db.InsertTransaction(tx); err != nil {
		t.Fatalf("first insert error = %v", err)
	}

	_, err := db.InsertTransaction(tx)
	if err == nil {
		t.Error("duplicate tx_hash should error (UNIQUE constraint)")
	}
}

func TestGetByTxHash_NotFound(t *testing.T) {
	db := newTestDB(t)

	got, err := db.GetByTxHash("nonexistent")
	if err != nil {
		t.Fatalf("GetByTxHash() error = %v", err)
	}
	if got != nil {
		t.Error("should return nil for nonexistent hash")
	}
}

func TestUpdateToConfirmed(t *testing.T) {
	db := newTestDB(t)
	seedWatch(t, db, "w1")

	tx := &models.Transaction{
		WatchID: "w1", TxHash: "conf1", Chain: "BSC", Address: "addr1", Token: "BNB",
		AmountRaw: "5000000000000000", AmountHuman: "0.005", Decimals: 18,
		USDValue: 0.0, USDPrice: 0.0, Tier: 0, Multiplier: 0.0, Points: 0,
		Status: models.TxStatusPending, DetectedAt: "2026-01-01",
	}
	db.InsertTransaction(tx)

	blockNum := int64(12345678)
	err := db.UpdateToConfirmed("conf1", 12, &blockNum, "2026-01-01T12:05:00Z", 3.25, 650.0, 2, 1.1, 358)
	if err != nil {
		t.Fatalf("UpdateToConfirmed() error = %v", err)
	}

	got, _ := db.GetByTxHash("conf1")
	if got.Status != models.TxStatusConfirmed {
		t.Errorf("Status = %q, want CONFIRMED", got.Status)
	}
	if got.Confirmations != 12 {
		t.Errorf("Confirmations = %d, want 12", got.Confirmations)
	}
	if got.Points != 358 {
		t.Errorf("Points = %d, want 358", got.Points)
	}
	if got.BlockNumber == nil || *got.BlockNumber != 12345678 {
		t.Errorf("BlockNumber = %v, want 12345678", got.BlockNumber)
	}
}

func TestListPending(t *testing.T) {
	db := newTestDB(t)
	seedWatch(t, db, "w1")

	db.InsertTransaction(&models.Transaction{
		WatchID: "w1", TxHash: "p1", Chain: "BTC", Address: "a1", Token: "BTC",
		AmountRaw: "100", AmountHuman: "0.000001", Decimals: 8,
		USDValue: 1.0, USDPrice: 100000.0, Tier: 1, Multiplier: 1.0, Points: 100,
		Status: models.TxStatusPending, DetectedAt: "2026-01-01",
	})
	db.InsertTransaction(&models.Transaction{
		WatchID: "w1", TxHash: "c1", Chain: "BTC", Address: "a1", Token: "BTC",
		AmountRaw: "200", AmountHuman: "0.000002", Decimals: 8,
		USDValue: 2.0, USDPrice: 100000.0, Tier: 1, Multiplier: 1.0, Points: 200,
		Status: models.TxStatusConfirmed, DetectedAt: "2026-01-01", ConfirmedAt: ptr("2026-01-01T01:00:00Z"),
	})

	pending, err := db.ListPending()
	if err != nil {
		t.Fatalf("ListPending() error = %v", err)
	}
	if len(pending) != 1 {
		t.Errorf("len = %d, want 1", len(pending))
	}
	if pending[0].TxHash != "p1" {
		t.Errorf("TxHash = %q, want p1", pending[0].TxHash)
	}
}

func TestListAll_WithPagination(t *testing.T) {
	db := newTestDB(t)
	seedWatch(t, db, "w1")

	for i := 0; i < 10; i++ {
		db.InsertTransaction(&models.Transaction{
			WatchID: "w1", TxHash: fmt.Sprintf("tx%d", i), Chain: "BTC", Address: "a1", Token: "BTC",
			AmountRaw: "100", AmountHuman: "0.000001", Decimals: 8,
			USDValue: 1.0, USDPrice: 100000.0, Tier: 1, Multiplier: 1.0, Points: 100,
			Status: models.TxStatusConfirmed, DetectedAt: "2026-01-01", ConfirmedAt: ptr("2026-01-01"),
		})
	}

	txs, total, err := db.ListAll(models.TransactionFilters{}, models.Pagination{Page: 1, PageSize: 3})
	if err != nil {
		t.Fatalf("ListAll() error = %v", err)
	}
	if total != 10 {
		t.Errorf("total = %d, want 10", total)
	}
	if len(txs) != 3 {
		t.Errorf("page len = %d, want 3", len(txs))
	}
}

func TestListAll_WithFilters(t *testing.T) {
	db := newTestDB(t)
	seedWatch(t, db, "w1")

	db.InsertTransaction(&models.Transaction{
		WatchID: "w1", TxHash: "btc1", Chain: "BTC", Address: "a1", Token: "BTC",
		AmountRaw: "100", AmountHuman: "0.000001", Decimals: 8,
		USDValue: 50.0, USDPrice: 100000.0, Tier: 3, Multiplier: 1.2, Points: 6000,
		Status: models.TxStatusConfirmed, DetectedAt: "2026-01-01", ConfirmedAt: ptr("2026-01-01"),
	})
	db.InsertTransaction(&models.Transaction{
		WatchID: "w1", TxHash: "bsc1", Chain: "BSC", Address: "a2", Token: "USDC",
		AmountRaw: "50000000000000000000", AmountHuman: "50.00", Decimals: 18,
		USDValue: 50.0, USDPrice: 1.0, Tier: 3, Multiplier: 1.2, Points: 6000,
		Status: models.TxStatusPending, DetectedAt: "2026-01-02",
	})

	chain := "BTC"
	txs, total, err := db.ListAll(models.TransactionFilters{Chain: &chain}, models.Pagination{Page: 1, PageSize: 50})
	if err != nil {
		t.Fatalf("ListAll(chain=BTC) error = %v", err)
	}
	if total != 1 {
		t.Errorf("total = %d, want 1", total)
	}
	if len(txs) != 1 {
		t.Errorf("len = %d, want 1", len(txs))
	}
}

func TestLastDetectedAt(t *testing.T) {
	db := newTestDB(t)
	seedWatch(t, db, "w1")

	// No transactions yet.
	ts, err := db.LastDetectedAt("addr1")
	if err != nil {
		t.Fatalf("LastDetectedAt() error = %v", err)
	}
	if ts != "" {
		t.Errorf("expected empty string, got %q", ts)
	}

	// Add transactions.
	db.InsertTransaction(&models.Transaction{
		WatchID: "w1", TxHash: "early", Chain: "BTC", Address: "addr1", Token: "BTC",
		AmountRaw: "100", AmountHuman: "0.000001", Decimals: 8,
		USDValue: 1.0, USDPrice: 100000.0, Tier: 1, Multiplier: 1.0, Points: 100,
		Status: models.TxStatusConfirmed, DetectedAt: "2026-01-01T10:00:00Z", ConfirmedAt: ptr("2026-01-01T10:10:00Z"),
	})
	db.InsertTransaction(&models.Transaction{
		WatchID: "w1", TxHash: "later", Chain: "BTC", Address: "addr1", Token: "BTC",
		AmountRaw: "200", AmountHuman: "0.000002", Decimals: 8,
		USDValue: 2.0, USDPrice: 100000.0, Tier: 1, Multiplier: 1.0, Points: 200,
		Status: models.TxStatusPending, DetectedAt: "2026-01-02T15:00:00Z",
	})

	ts, err = db.LastDetectedAt("addr1")
	if err != nil {
		t.Fatalf("LastDetectedAt() error = %v", err)
	}
	if ts != "2026-01-02T15:00:00Z" {
		t.Errorf("LastDetectedAt = %q, want 2026-01-02T15:00:00Z", ts)
	}
}

func ptr(s string) *string { return &s }
