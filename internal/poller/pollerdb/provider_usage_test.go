package pollerdb

import (
	"testing"
	"time"

	"github.com/Fantasim/hdpay/internal/poller/config"
)

func TestIncrementUsage_Success(t *testing.T) {
	db := newTestDB(t)

	if err := db.IncrementUsage("SOL", "helius", true, false); err != nil {
		t.Fatalf("IncrementUsage() error = %v", err)
	}

	today := time.Now().UTC().Format(config.ProviderUsageDateFormat)
	rows, err := db.GetDailyUsage(today)
	if err != nil {
		t.Fatalf("GetDailyUsage() error = %v", err)
	}

	if len(rows) != 1 {
		t.Fatalf("expected 1 row, got %d", len(rows))
	}
	r := rows[0]
	if r.Requests != 1 || r.Successes != 1 || r.Failures != 0 || r.Hits429 != 0 {
		t.Errorf("got requests=%d successes=%d failures=%d hits429=%d, want 1/1/0/0",
			r.Requests, r.Successes, r.Failures, r.Hits429)
	}
}

func TestIncrementUsage_Failure(t *testing.T) {
	db := newTestDB(t)

	if err := db.IncrementUsage("BTC", "blockstream", false, false); err != nil {
		t.Fatalf("IncrementUsage() error = %v", err)
	}

	today := time.Now().UTC().Format(config.ProviderUsageDateFormat)
	rows, err := db.GetDailyUsage(today)
	if err != nil {
		t.Fatalf("GetDailyUsage() error = %v", err)
	}

	if len(rows) != 1 {
		t.Fatalf("expected 1 row, got %d", len(rows))
	}
	r := rows[0]
	if r.Requests != 1 || r.Successes != 0 || r.Failures != 1 || r.Hits429 != 0 {
		t.Errorf("got requests=%d successes=%d failures=%d hits429=%d, want 1/0/1/0",
			r.Requests, r.Successes, r.Failures, r.Hits429)
	}
}

func TestIncrementUsage_429(t *testing.T) {
	db := newTestDB(t)

	if err := db.IncrementUsage("BSC", "bscrpc", false, true); err != nil {
		t.Fatalf("IncrementUsage() error = %v", err)
	}

	today := time.Now().UTC().Format(config.ProviderUsageDateFormat)
	rows, err := db.GetDailyUsage(today)
	if err != nil {
		t.Fatalf("GetDailyUsage() error = %v", err)
	}

	r := rows[0]
	if r.Requests != 1 || r.Failures != 1 || r.Hits429 != 1 {
		t.Errorf("got requests=%d failures=%d hits429=%d, want 1/1/1",
			r.Requests, r.Failures, r.Hits429)
	}
}

func TestIncrementUsage_Accumulates(t *testing.T) {
	db := newTestDB(t)

	for i := 0; i < 5; i++ {
		if err := db.IncrementUsage("SOL", "helius", true, false); err != nil {
			t.Fatalf("IncrementUsage() iteration %d error = %v", i, err)
		}
	}
	for i := 0; i < 3; i++ {
		if err := db.IncrementUsage("SOL", "helius", false, false); err != nil {
			t.Fatalf("IncrementUsage() failure iteration %d error = %v", i, err)
		}
	}

	today := time.Now().UTC().Format(config.ProviderUsageDateFormat)
	rows, err := db.GetDailyUsage(today)
	if err != nil {
		t.Fatalf("GetDailyUsage() error = %v", err)
	}

	if len(rows) != 1 {
		t.Fatalf("expected 1 row, got %d", len(rows))
	}
	r := rows[0]
	if r.Requests != 8 || r.Successes != 5 || r.Failures != 3 {
		t.Errorf("got requests=%d successes=%d failures=%d, want 8/5/3",
			r.Requests, r.Successes, r.Failures)
	}
}

func TestIncrementUsage_MultipleProviders(t *testing.T) {
	db := newTestDB(t)

	if err := db.IncrementUsage("SOL", "helius", true, false); err != nil {
		t.Fatalf("IncrementUsage() error = %v", err)
	}
	if err := db.IncrementUsage("SOL", "solana-rpc", true, false); err != nil {
		t.Fatalf("IncrementUsage() error = %v", err)
	}
	if err := db.IncrementUsage("BTC", "blockstream", true, false); err != nil {
		t.Fatalf("IncrementUsage() error = %v", err)
	}

	today := time.Now().UTC().Format(config.ProviderUsageDateFormat)
	rows, err := db.GetDailyUsage(today)
	if err != nil {
		t.Fatalf("GetDailyUsage() error = %v", err)
	}

	if len(rows) != 3 {
		t.Fatalf("expected 3 rows, got %d", len(rows))
	}
}

func TestGetMonthlyUsage(t *testing.T) {
	db := newTestDB(t)

	// Insert rows for multiple days manually.
	for i := 0; i < 5; i++ {
		date := time.Now().UTC().AddDate(0, 0, -i).Format(config.ProviderUsageDateFormat)
		_, err := db.conn.Exec(`
			INSERT INTO provider_usage (chain, provider, date, requests, successes, failures, hits_429)
			VALUES ('SOL', 'helius', ?, 10, 9, 1, 0)`, date)
		if err != nil {
			t.Fatalf("insert test data day %d: %v", i, err)
		}
	}

	row, err := db.GetMonthlyUsage("SOL", "helius", 30)
	if err != nil {
		t.Fatalf("GetMonthlyUsage() error = %v", err)
	}

	if row.Requests != 50 || row.Successes != 45 || row.Failures != 5 {
		t.Errorf("got requests=%d successes=%d failures=%d, want 50/45/5",
			row.Requests, row.Successes, row.Failures)
	}
}

func TestGetAllMonthlyUsage(t *testing.T) {
	db := newTestDB(t)

	// Insert data for two providers across multiple days.
	for i := 0; i < 3; i++ {
		date := time.Now().UTC().AddDate(0, 0, -i).Format(config.ProviderUsageDateFormat)
		_, err := db.conn.Exec(`
			INSERT INTO provider_usage (chain, provider, date, requests, successes, failures, hits_429)
			VALUES ('SOL', 'helius', ?, 10, 10, 0, 0)`, date)
		if err != nil {
			t.Fatalf("insert helius day %d: %v", i, err)
		}
		_, err = db.conn.Exec(`
			INSERT INTO provider_usage (chain, provider, date, requests, successes, failures, hits_429)
			VALUES ('BTC', 'blockstream', ?, 5, 4, 1, 0)`, date)
		if err != nil {
			t.Fatalf("insert blockstream day %d: %v", i, err)
		}
	}

	rows, err := db.GetAllMonthlyUsage()
	if err != nil {
		t.Fatalf("GetAllMonthlyUsage() error = %v", err)
	}

	if len(rows) != 2 {
		t.Fatalf("expected 2 rows, got %d", len(rows))
	}

	// Rows are ordered by chain, provider — BTC/blockstream first, SOL/helius second.
	if rows[0].Chain != "BTC" || rows[0].Requests != 15 {
		t.Errorf("BTC row: chain=%s requests=%d, want BTC/15", rows[0].Chain, rows[0].Requests)
	}
	if rows[1].Chain != "SOL" || rows[1].Requests != 30 {
		t.Errorf("SOL row: chain=%s requests=%d, want SOL/30", rows[1].Chain, rows[1].Requests)
	}
}

func TestGetDailyUsage_EmptyDate(t *testing.T) {
	db := newTestDB(t)

	rows, err := db.GetDailyUsage("2020-01-01")
	if err != nil {
		t.Fatalf("GetDailyUsage() error = %v", err)
	}
	if len(rows) != 0 {
		t.Errorf("expected 0 rows, got %d", len(rows))
	}
}
