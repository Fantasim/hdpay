package pollerdb

import (
	"os"
	"path/filepath"
	"testing"
)

// newTestDB creates a temporary SQLite database for testing.
func newTestDB(t *testing.T) *DB {
	t.Helper()
	tmpDir := t.TempDir()
	dbPath := filepath.Join(tmpDir, "test.sqlite")

	db, err := New(dbPath)
	if err != nil {
		t.Fatalf("New() error = %v", err)
	}

	if err := db.RunMigrations(); err != nil {
		t.Fatalf("RunMigrations() error = %v", err)
	}

	t.Cleanup(func() { db.Close() })
	return db
}

func TestNew_CreatesDatabaseFile(t *testing.T) {
	tmpDir := t.TempDir()
	dbPath := filepath.Join(tmpDir, "sub", "dir", "test.sqlite")

	db, err := New(dbPath)
	if err != nil {
		t.Fatalf("New() error = %v", err)
	}
	defer db.Close()

	if _, err := os.Stat(dbPath); os.IsNotExist(err) {
		t.Error("database file should exist after New()")
	}
}

func TestRunMigrations_CreatesAllTables(t *testing.T) {
	db := newTestDB(t)

	tables := []string{"watches", "points", "transactions", "ip_allowlist", "system_errors"}
	for _, table := range tables {
		var name string
		err := db.Conn().QueryRow(
			"SELECT name FROM sqlite_master WHERE type='table' AND name=?", table,
		).Scan(&name)
		if err != nil {
			t.Errorf("table %q should exist: %v", table, err)
		}
	}
}

func TestRunMigrations_Idempotent(t *testing.T) {
	db := newTestDB(t)

	// Run migrations again — should not error.
	if err := db.RunMigrations(); err != nil {
		t.Errorf("RunMigrations() second call error = %v", err)
	}
}

func TestRunMigrations_TransactionsHasBlockNumber(t *testing.T) {
	db := newTestDB(t)

	// Verify block_number column exists by inserting a row with it.
	_, err := db.Conn().Exec(`
		INSERT INTO watches (id, chain, address, status, started_at, expires_at)
		VALUES ('w1', 'BTC', 'addr1', 'ACTIVE', '2026-01-01', '2026-01-02')`)
	if err != nil {
		t.Fatalf("insert watch: %v", err)
	}

	_, err = db.Conn().Exec(`
		INSERT INTO transactions (
			watch_id, tx_hash, chain, address, token,
			amount_raw, amount_human, decimals,
			usd_value, usd_price, tier, multiplier, points,
			status, confirmations, block_number, detected_at
		) VALUES ('w1', 'tx1', 'BSC', 'addr1', 'BNB',
			'1000', '0.001', 18,
			0.50, 500.0, 1, 1.0, 50,
			'PENDING', 3, 12345678, '2026-01-01')`)
	if err != nil {
		t.Fatalf("insert transaction with block_number: %v", err)
	}

	var blockNum int64
	err = db.Conn().QueryRow("SELECT block_number FROM transactions WHERE tx_hash = 'tx1'").Scan(&blockNum)
	if err != nil {
		t.Fatalf("select block_number: %v", err)
	}
	if blockNum != 12345678 {
		t.Errorf("block_number = %d, want 12345678", blockNum)
	}
}
