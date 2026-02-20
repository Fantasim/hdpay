package pollerdb

import (
	"testing"

	"github.com/Fantasim/hdpay/internal/poller/models"
)

// seedConfirmedTx inserts a confirmed transaction for dashboard testing.
func seedConfirmedTx(t *testing.T, db *DB, hash, chain, address, token string, usdValue float64, points int, tier int, confirmedAt string) {
	t.Helper()
	db.InsertTransaction(&models.Transaction{
		WatchID: "w1", TxHash: hash, Chain: chain, Address: address, Token: token,
		AmountRaw: "100", AmountHuman: "0.001", Decimals: 8,
		USDValue: usdValue, USDPrice: 100.0, Tier: tier, Multiplier: 1.0, Points: points,
		Status: models.TxStatusConfirmed, DetectedAt: confirmedAt, ConfirmedAt: ptr(confirmedAt),
	})
}

func TestDashboardStats_EmptyDB(t *testing.T) {
	db := newTestDB(t)

	stats, err := db.DashboardStats("", "")
	if err != nil {
		t.Fatalf("DashboardStats() error = %v", err)
	}
	if stats.TxCount != 0 {
		t.Errorf("TxCount = %d, want 0", stats.TxCount)
	}
	if stats.USDReceived != 0 {
		t.Errorf("USDReceived = %f, want 0", stats.USDReceived)
	}
}

func TestDashboardStats_WithTransactions(t *testing.T) {
	db := newTestDB(t)
	seedWatch(t, db, "w1")

	seedConfirmedTx(t, db, "tx1", "BTC", "a1", "BTC", 100.0, 10000, 4, "2026-01-15T12:00:00Z")
	seedConfirmedTx(t, db, "tx2", "BSC", "a2", "USDC", 50.0, 6000, 3, "2026-01-16T12:00:00Z")

	stats, err := db.DashboardStats("", "")
	if err != nil {
		t.Fatalf("DashboardStats() error = %v", err)
	}
	if stats.TxCount != 2 {
		t.Errorf("TxCount = %d, want 2", stats.TxCount)
	}
	if stats.USDReceived != 150.0 {
		t.Errorf("USDReceived = %f, want 150.0", stats.USDReceived)
	}
	if stats.PointsAwarded != 16000 {
		t.Errorf("PointsAwarded = %d, want 16000", stats.PointsAwarded)
	}
	if stats.UniqueAddresses != 2 {
		t.Errorf("UniqueAddresses = %d, want 2", stats.UniqueAddresses)
	}
	if stats.LargestTxUSD != 100.0 {
		t.Errorf("LargestTxUSD = %f, want 100.0", stats.LargestTxUSD)
	}
}

func TestDashboardStats_DateRange(t *testing.T) {
	db := newTestDB(t)
	seedWatch(t, db, "w1")

	seedConfirmedTx(t, db, "tx1", "BTC", "a1", "BTC", 100.0, 10000, 4, "2026-01-10T12:00:00Z")
	seedConfirmedTx(t, db, "tx2", "BTC", "a1", "BTC", 50.0, 5000, 3, "2026-01-20T12:00:00Z")

	stats, err := db.DashboardStats("2026-01-15", "2026-01-31")
	if err != nil {
		t.Fatalf("DashboardStats() error = %v", err)
	}
	if stats.TxCount != 1 {
		t.Errorf("TxCount = %d, want 1 (filtered by date)", stats.TxCount)
	}
	if stats.USDReceived != 50.0 {
		t.Errorf("USDReceived = %f, want 50.0", stats.USDReceived)
	}
}

func TestDailyStats_GroupsByDay(t *testing.T) {
	db := newTestDB(t)
	seedWatch(t, db, "w1")

	seedConfirmedTx(t, db, "tx1", "BTC", "a1", "BTC", 100.0, 10000, 4, "2026-01-15T10:00:00Z")
	seedConfirmedTx(t, db, "tx2", "BTC", "a1", "BTC", 50.0, 5000, 3, "2026-01-15T14:00:00Z")
	seedConfirmedTx(t, db, "tx3", "BSC", "a2", "USDC", 25.0, 2750, 2, "2026-01-16T12:00:00Z")

	rows, err := db.DailyStats("", "")
	if err != nil {
		t.Fatalf("DailyStats() error = %v", err)
	}
	if len(rows) != 2 {
		t.Fatalf("len = %d, want 2 days", len(rows))
	}
	if rows[0].TxCount != 2 {
		t.Errorf("day1 TxCount = %d, want 2", rows[0].TxCount)
	}
	if rows[1].TxCount != 1 {
		t.Errorf("day2 TxCount = %d, want 1", rows[1].TxCount)
	}
}

func TestWatchStats(t *testing.T) {
	db := newTestDB(t)

	db.CreateWatch(&models.Watch{ID: "w1", Chain: "BTC", Address: "a1", Status: models.WatchStatusCompleted, StartedAt: "2026-01-15", ExpiresAt: "2026-01-16"})
	db.CreateWatch(&models.Watch{ID: "w2", Chain: "BSC", Address: "a2", Status: models.WatchStatusExpired, StartedAt: "2026-01-15", ExpiresAt: "2026-01-16"})
	db.CreateWatch(&models.Watch{ID: "w3", Chain: "SOL", Address: "a3", Status: models.WatchStatusCompleted, StartedAt: "2026-01-20", ExpiresAt: "2026-01-21"})

	stats, err := db.WatchStats("", "")
	if err != nil {
		t.Fatalf("WatchStats() error = %v", err)
	}
	if stats.Total != 3 {
		t.Errorf("Total = %d, want 3", stats.Total)
	}
	if stats.Completed != 2 {
		t.Errorf("Completed = %d, want 2", stats.Completed)
	}
	if stats.Expired != 1 {
		t.Errorf("Expired = %d, want 1", stats.Expired)
	}

	// With date filter.
	stats2, err := db.WatchStats("2026-01-16", "")
	if err != nil {
		t.Fatalf("WatchStats(dateFrom) error = %v", err)
	}
	if stats2.Total != 1 {
		t.Errorf("filtered Total = %d, want 1", stats2.Total)
	}
}

func TestPendingPointsSummary(t *testing.T) {
	db := newTestDB(t)

	// No points yet.
	accounts, total, err := db.PendingPointsSummary()
	if err != nil {
		t.Fatalf("PendingPointsSummary() error = %v", err)
	}
	if accounts != 0 || total != 0 {
		t.Errorf("empty: accounts=%d total=%d, want 0,0", accounts, total)
	}

	// Seed points with pending.
	db.GetOrCreatePoints("a1", "BTC")
	db.AddPending("a1", "BTC", 500)
	db.GetOrCreatePoints("a2", "BSC")
	db.AddPending("a2", "BSC", 300)

	accounts, total, err = db.PendingPointsSummary()
	if err != nil {
		t.Fatalf("PendingPointsSummary() error = %v", err)
	}
	if accounts != 2 {
		t.Errorf("accounts = %d, want 2", accounts)
	}
	if total != 800 {
		t.Errorf("total = %d, want 800", total)
	}
}

func TestChartByChain(t *testing.T) {
	db := newTestDB(t)
	seedWatch(t, db, "w1")

	seedConfirmedTx(t, db, "tx1", "BTC", "a1", "BTC", 100.0, 10000, 4, "2026-01-15T12:00:00Z")
	seedConfirmedTx(t, db, "tx2", "BSC", "a2", "USDC", 50.0, 6000, 3, "2026-01-15T12:00:00Z")
	seedConfirmedTx(t, db, "tx3", "BSC", "a3", "BNB", 25.0, 2750, 2, "2026-01-15T12:00:00Z")

	rows, err := db.ChartByChain("", "")
	if err != nil {
		t.Fatalf("ChartByChain() error = %v", err)
	}
	if len(rows) != 2 {
		t.Fatalf("len = %d, want 2 chains", len(rows))
	}
	// BSC first (alphabetical).
	if rows[0].Chain != "BSC" || rows[0].Count != 2 {
		t.Errorf("BSC: chain=%s count=%d, want BSC,2", rows[0].Chain, rows[0].Count)
	}
	if rows[1].Chain != "BTC" || rows[1].Count != 1 {
		t.Errorf("BTC: chain=%s count=%d, want BTC,1", rows[1].Chain, rows[1].Count)
	}
}

func TestChartByToken(t *testing.T) {
	db := newTestDB(t)
	seedWatch(t, db, "w1")

	seedConfirmedTx(t, db, "tx1", "BTC", "a1", "BTC", 100.0, 10000, 4, "2026-01-15T12:00:00Z")
	seedConfirmedTx(t, db, "tx2", "BSC", "a2", "USDC", 50.0, 6000, 3, "2026-01-15T12:00:00Z")

	rows, err := db.ChartByToken("", "")
	if err != nil {
		t.Fatalf("ChartByToken() error = %v", err)
	}
	if len(rows) != 2 {
		t.Fatalf("len = %d, want 2 tokens", len(rows))
	}
}

func TestChartByTier(t *testing.T) {
	db := newTestDB(t)
	seedWatch(t, db, "w1")

	seedConfirmedTx(t, db, "tx1", "BTC", "a1", "BTC", 100.0, 10000, 4, "2026-01-15T12:00:00Z")
	seedConfirmedTx(t, db, "tx2", "BSC", "a2", "USDC", 50.0, 6000, 3, "2026-01-15T12:00:00Z")
	seedConfirmedTx(t, db, "tx3", "BSC", "a3", "BNB", 50.0, 6000, 3, "2026-01-15T12:00:00Z")

	rows, err := db.ChartByTier("", "")
	if err != nil {
		t.Fatalf("ChartByTier() error = %v", err)
	}
	if len(rows) != 2 {
		t.Fatalf("len = %d, want 2 tiers", len(rows))
	}
	// Tier 3 should have 2 txs.
	for _, r := range rows {
		if r.Tier == 3 && r.Count != 2 {
			t.Errorf("tier 3 count = %d, want 2", r.Count)
		}
	}
}

func TestChartWatchesByDay(t *testing.T) {
	db := newTestDB(t)

	db.CreateWatch(&models.Watch{ID: "w1", Chain: "BTC", Address: "a1", Status: models.WatchStatusCompleted, StartedAt: "2026-01-15T10:00:00Z", ExpiresAt: "2026-01-16"})
	db.CreateWatch(&models.Watch{ID: "w2", Chain: "BSC", Address: "a2", Status: models.WatchStatusExpired, StartedAt: "2026-01-15T14:00:00Z", ExpiresAt: "2026-01-16"})
	db.CreateWatch(&models.Watch{ID: "w3", Chain: "SOL", Address: "a3", Status: models.WatchStatusCompleted, StartedAt: "2026-01-16T12:00:00Z", ExpiresAt: "2026-01-17"})

	rows, err := db.ChartWatchesByDay("", "")
	if err != nil {
		t.Fatalf("ChartWatchesByDay() error = %v", err)
	}
	if len(rows) != 2 {
		t.Fatalf("len = %d, want 2 days", len(rows))
	}
	if rows[0].Completed != 1 || rows[0].Expired != 1 {
		t.Errorf("day1: completed=%d expired=%d, want 1,1", rows[0].Completed, rows[0].Expired)
	}
}
