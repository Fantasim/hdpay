package pollerdb

import (
	"testing"

	"github.com/Fantasim/hdpay/internal/poller/models"
)

func TestCheckPointsMismatch_NoMismatch(t *testing.T) {
	db := newTestDB(t)
	seedWatch(t, db, "w1")

	// Insert confirmed tx with 100 points.
	db.InsertTransaction(&models.Transaction{
		WatchID: "w1", TxHash: "tx1", Chain: "BTC", Address: "a1", Token: "BTC",
		AmountRaw: "100", AmountHuman: "0.000001", Decimals: 8,
		USDValue: 1.0, USDPrice: 100000.0, Tier: 1, Multiplier: 1.0, Points: 100,
		Status: models.TxStatusConfirmed, DetectedAt: "2026-01-01", ConfirmedAt: ptr("2026-01-01"),
	})
	db.GetOrCreatePoints("a1", "BTC")
	db.AddUnclaimed("a1", "BTC", 100)

	rows, err := db.CheckPointsMismatch()
	if err != nil {
		t.Fatalf("CheckPointsMismatch() error = %v", err)
	}
	if len(rows) != 0 {
		t.Errorf("expected no mismatches, got %d", len(rows))
	}
}

func TestCheckPointsMismatch_WithMismatch(t *testing.T) {
	db := newTestDB(t)
	seedWatch(t, db, "w1")

	db.InsertTransaction(&models.Transaction{
		WatchID: "w1", TxHash: "tx1", Chain: "BTC", Address: "a1", Token: "BTC",
		AmountRaw: "100", AmountHuman: "0.000001", Decimals: 8,
		USDValue: 1.0, USDPrice: 100000.0, Tier: 1, Multiplier: 1.0, Points: 100,
		Status: models.TxStatusConfirmed, DetectedAt: "2026-01-01", ConfirmedAt: ptr("2026-01-01"),
	})
	// Intentionally set wrong total (200 instead of 100).
	db.GetOrCreatePoints("a1", "BTC")
	db.AddUnclaimed("a1", "BTC", 200)

	rows, err := db.CheckPointsMismatch()
	if err != nil {
		t.Fatalf("CheckPointsMismatch() error = %v", err)
	}
	if len(rows) != 1 {
		t.Fatalf("expected 1 mismatch, got %d", len(rows))
	}
	if rows[0].Type != "POINTS_MISMATCH" {
		t.Errorf("type = %q, want POINTS_MISMATCH", rows[0].Type)
	}
}

func TestCheckUnclaimedExceedsTotal_NoIssues(t *testing.T) {
	db := newTestDB(t)

	db.GetOrCreatePoints("a1", "BTC")
	db.AddUnclaimed("a1", "BTC", 100)

	rows, err := db.CheckUnclaimedExceedsTotal()
	if err != nil {
		t.Fatalf("CheckUnclaimedExceedsTotal() error = %v", err)
	}
	if len(rows) != 0 {
		t.Errorf("expected no issues, got %d", len(rows))
	}
}

func TestCheckUnclaimedExceedsTotal_WithIssue(t *testing.T) {
	db := newTestDB(t)

	// Manually insert a row where unclaimed > total (shouldn't happen normally).
	db.conn.Exec(`INSERT INTO points (address, chain, unclaimed, pending, total) VALUES ('a1', 'BTC', 200, 0, 100)`)

	rows, err := db.CheckUnclaimedExceedsTotal()
	if err != nil {
		t.Fatalf("CheckUnclaimedExceedsTotal() error = %v", err)
	}
	if len(rows) != 1 {
		t.Fatalf("expected 1 issue, got %d", len(rows))
	}
	if rows[0].Type != "UNCLAIMED_EXCEEDS_TOTAL" {
		t.Errorf("type = %q, want UNCLAIMED_EXCEEDS_TOTAL", rows[0].Type)
	}
}

func TestCheckOrphanedTransactions_NoOrphans(t *testing.T) {
	db := newTestDB(t)
	seedWatch(t, db, "w1")

	db.InsertTransaction(&models.Transaction{
		WatchID: "w1", TxHash: "tx1", Chain: "BTC", Address: "a1", Token: "BTC",
		AmountRaw: "100", AmountHuman: "0.000001", Decimals: 8,
		USDValue: 1.0, USDPrice: 100000.0, Tier: 1, Multiplier: 1.0, Points: 100,
		Status: models.TxStatusConfirmed, DetectedAt: "2026-01-01", ConfirmedAt: ptr("2026-01-01"),
	})

	rows, err := db.CheckOrphanedTransactions()
	if err != nil {
		t.Fatalf("CheckOrphanedTransactions() error = %v", err)
	}
	if len(rows) != 0 {
		t.Errorf("expected no orphans, got %d", len(rows))
	}
}

func TestCheckOrphanedTransactions_WithOrphan(t *testing.T) {
	db := newTestDB(t)

	// Insert tx referencing a watch that doesn't exist (bypass FK since SQLite FKs are off by default).
	db.conn.Exec(`INSERT INTO transactions (
		watch_id, tx_hash, chain, address, token,
		amount_raw, amount_human, decimals,
		usd_value, usd_price, tier, multiplier, points,
		status, confirmations, detected_at
	) VALUES ('nonexistent', 'orphan1', 'BTC', 'a1', 'BTC',
		'100', '0.001', 8, 1.0, 100000.0, 1, 1.0, 100,
		'CONFIRMED', 1, '2026-01-01')`)

	rows, err := db.CheckOrphanedTransactions()
	if err != nil {
		t.Fatalf("CheckOrphanedTransactions() error = %v", err)
	}
	if len(rows) != 1 {
		t.Fatalf("expected 1 orphan, got %d", len(rows))
	}
	if rows[0].Type != "ORPHANED_TRANSACTION" {
		t.Errorf("type = %q, want ORPHANED_TRANSACTION", rows[0].Type)
	}
}

func TestCheckStalePending_NoStale(t *testing.T) {
	db := newTestDB(t)
	seedWatch(t, db, "w1")

	// Recent pending tx (not stale).
	db.InsertTransaction(&models.Transaction{
		WatchID: "w1", TxHash: "recent1", Chain: "BTC", Address: "a1", Token: "BTC",
		AmountRaw: "100", AmountHuman: "0.000001", Decimals: 8,
		USDValue: 1.0, USDPrice: 100000.0, Tier: 1, Multiplier: 1.0, Points: 100,
		Status: models.TxStatusPending, DetectedAt: "2099-01-01T12:00:00Z",
	})

	rows, err := db.CheckStalePending()
	if err != nil {
		t.Fatalf("CheckStalePending() error = %v", err)
	}
	if len(rows) != 0 {
		t.Errorf("expected no stale, got %d", len(rows))
	}
}

func TestCheckStalePending_WithStale(t *testing.T) {
	db := newTestDB(t)
	seedWatch(t, db, "w1")

	// Old pending tx (stale — detected 2 years ago).
	db.InsertTransaction(&models.Transaction{
		WatchID: "w1", TxHash: "stale1", Chain: "BSC", Address: "a1", Token: "BNB",
		AmountRaw: "100", AmountHuman: "0.001", Decimals: 18,
		USDValue: 1.0, USDPrice: 500.0, Tier: 1, Multiplier: 1.0, Points: 100,
		Status: models.TxStatusPending, DetectedAt: "2024-01-01T12:00:00Z",
	})

	rows, err := db.CheckStalePending()
	if err != nil {
		t.Fatalf("CheckStalePending() error = %v", err)
	}
	if len(rows) != 1 {
		t.Fatalf("expected 1 stale, got %d", len(rows))
	}
	if rows[0].TxHash != "stale1" {
		t.Errorf("TxHash = %q, want stale1", rows[0].TxHash)
	}
}

func TestListByAddress(t *testing.T) {
	db := newTestDB(t)
	seedWatch(t, db, "w1")

	db.InsertTransaction(&models.Transaction{
		WatchID: "w1", TxHash: "tx1", Chain: "BTC", Address: "a1", Token: "BTC",
		AmountRaw: "100", AmountHuman: "0.001", Decimals: 8,
		USDValue: 1.0, USDPrice: 100000.0, Tier: 1, Multiplier: 1.0, Points: 100,
		Status: models.TxStatusConfirmed, DetectedAt: "2026-01-01", ConfirmedAt: ptr("2026-01-01"),
	})
	db.InsertTransaction(&models.Transaction{
		WatchID: "w1", TxHash: "tx2", Chain: "BTC", Address: "a2", Token: "BTC",
		AmountRaw: "200", AmountHuman: "0.002", Decimals: 8,
		USDValue: 2.0, USDPrice: 100000.0, Tier: 1, Multiplier: 1.0, Points: 200,
		Status: models.TxStatusConfirmed, DetectedAt: "2026-01-01", ConfirmedAt: ptr("2026-01-01"),
	})

	txs, err := db.ListByAddress("a1")
	if err != nil {
		t.Fatalf("ListByAddress() error = %v", err)
	}
	if len(txs) != 1 {
		t.Errorf("len = %d, want 1", len(txs))
	}
}

func TestListPendingByWatchID(t *testing.T) {
	db := newTestDB(t)
	seedWatch(t, db, "w1")
	seedWatch(t, db, "w2")

	db.InsertTransaction(&models.Transaction{
		WatchID: "w1", TxHash: "p1", Chain: "BTC", Address: "a1", Token: "BTC",
		AmountRaw: "100", AmountHuman: "0.001", Decimals: 8,
		USDValue: 1.0, USDPrice: 100000.0, Tier: 1, Multiplier: 1.0, Points: 100,
		Status: models.TxStatusPending, DetectedAt: "2026-01-01",
	})
	db.InsertTransaction(&models.Transaction{
		WatchID: "w2", TxHash: "p2", Chain: "BTC", Address: "a1", Token: "BTC",
		AmountRaw: "200", AmountHuman: "0.002", Decimals: 8,
		USDValue: 2.0, USDPrice: 100000.0, Tier: 1, Multiplier: 1.0, Points: 200,
		Status: models.TxStatusPending, DetectedAt: "2026-01-01",
	})

	txs, err := db.ListPendingByWatchID("w1")
	if err != nil {
		t.Fatalf("ListPendingByWatchID() error = %v", err)
	}
	if len(txs) != 1 {
		t.Errorf("len = %d, want 1", len(txs))
	}
	if txs[0].TxHash != "p1" {
		t.Errorf("TxHash = %q, want p1", txs[0].TxHash)
	}
}

func TestCountByWatchID(t *testing.T) {
	db := newTestDB(t)
	seedWatch(t, db, "w1")

	db.InsertTransaction(&models.Transaction{
		WatchID: "w1", TxHash: "tx1", Chain: "BTC", Address: "a1", Token: "BTC",
		AmountRaw: "100", AmountHuman: "0.001", Decimals: 8,
		USDValue: 1.0, USDPrice: 100000.0, Tier: 1, Multiplier: 1.0, Points: 100,
		Status: models.TxStatusPending, DetectedAt: "2026-01-01",
	})
	db.InsertTransaction(&models.Transaction{
		WatchID: "w1", TxHash: "tx2", Chain: "BTC", Address: "a1", Token: "BTC",
		AmountRaw: "200", AmountHuman: "0.002", Decimals: 8,
		USDValue: 2.0, USDPrice: 100000.0, Tier: 1, Multiplier: 1.0, Points: 200,
		Status: models.TxStatusConfirmed, DetectedAt: "2026-01-01", ConfirmedAt: ptr("2026-01-01"),
	})

	total, pending, err := db.CountByWatchID("w1")
	if err != nil {
		t.Fatalf("CountByWatchID() error = %v", err)
	}
	if total != 2 {
		t.Errorf("total = %d, want 2", total)
	}
	if pending != 1 {
		t.Errorf("pending = %d, want 1", pending)
	}
}
