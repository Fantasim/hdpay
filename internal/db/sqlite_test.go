package db

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/Fantasim/hdpay/internal/config"
	"github.com/Fantasim/hdpay/internal/models"
)

func TestNewDB(t *testing.T) {
	tmpDir := t.TempDir()
	dbPath := filepath.Join(tmpDir, "test.sqlite")

	d, err := New(dbPath, "testnet")
	if err != nil {
		t.Fatalf("New() error = %v", err)
	}
	defer d.Close()

	if _, err := os.Stat(dbPath); os.IsNotExist(err) {
		t.Error("expected database file to be created")
	}

	// Verify WAL mode
	var mode string
	if err := d.Conn().QueryRow("PRAGMA journal_mode").Scan(&mode); err != nil {
		t.Fatalf("failed to query journal_mode: %v", err)
	}
	if mode != "wal" {
		t.Errorf("expected journal_mode=wal, got %q", mode)
	}
}

func TestRunMigrations(t *testing.T) {
	tmpDir := t.TempDir()
	dbPath := filepath.Join(tmpDir, "test.sqlite")

	d, err := New(dbPath, "testnet")
	if err != nil {
		t.Fatalf("New() error = %v", err)
	}
	defer d.Close()

	if err := d.RunMigrations(); err != nil {
		t.Fatalf("RunMigrations() error = %v", err)
	}

	// Verify tables exist
	tables := []string{"addresses", "balances", "scan_state", "transactions", "settings", "schema_migrations"}
	for _, table := range tables {
		var name string
		err := d.Conn().QueryRow("SELECT name FROM sqlite_master WHERE type='table' AND name=?", table).Scan(&name)
		if err != nil {
			t.Errorf("table %q not found: %v", table, err)
		}
	}
}

func TestRunMigrationsIdempotent(t *testing.T) {
	tmpDir := t.TempDir()
	dbPath := filepath.Join(tmpDir, "test.sqlite")

	d, err := New(dbPath, "testnet")
	if err != nil {
		t.Fatalf("New() error = %v", err)
	}
	defer d.Close()

	if err := d.RunMigrations(); err != nil {
		t.Fatalf("first RunMigrations() error = %v", err)
	}

	if err := d.RunMigrations(); err != nil {
		t.Fatalf("second RunMigrations() error = %v", err)
	}

	// Verify each migration recorded exactly once (no duplicates from second run)
	var count int
	if err := d.Conn().QueryRow("SELECT COUNT(*) FROM schema_migrations").Scan(&count); err != nil {
		t.Fatalf("failed to count migrations: %v", err)
	}
	// Count all embedded migration files to verify idempotency
	entries, _ := migrationsFS.ReadDir("migrations")
	expectedCount := 0
	for _, e := range entries {
		if !e.IsDir() {
			expectedCount++
		}
	}
	if count != expectedCount {
		t.Errorf("expected %d migration records, got %d", expectedCount, count)
	}
}

func TestValidateNetworkConsistency_EmptyDB(t *testing.T) {
	tmpDir := t.TempDir()
	dbPath := filepath.Join(tmpDir, "test.sqlite")

	d, err := New(dbPath, "testnet")
	if err != nil {
		t.Fatalf("New() error = %v", err)
	}
	defer d.Close()

	if err := d.RunMigrations(); err != nil {
		t.Fatalf("RunMigrations() error = %v", err)
	}

	// Empty DB should pass — no addresses to compare against.
	if err := d.ValidateNetworkConsistency(); err != nil {
		t.Errorf("ValidateNetworkConsistency() should pass on empty DB, got: %v", err)
	}
}

func TestValidateNetworkConsistency_Match(t *testing.T) {
	tmpDir := t.TempDir()
	dbPath := filepath.Join(tmpDir, "test.sqlite")

	d, err := New(dbPath, "testnet")
	if err != nil {
		t.Fatalf("New() error = %v", err)
	}
	defer d.Close()

	if err := d.RunMigrations(); err != nil {
		t.Fatalf("RunMigrations() error = %v", err)
	}

	// Seed testnet addresses.
	addrs := []models.Address{
		{Chain: models.ChainBTC, AddressIndex: 0, Address: "tb1qtest0"},
	}
	if err := d.InsertAddressBatch(models.ChainBTC, addrs); err != nil {
		t.Fatalf("InsertAddressBatch() error = %v", err)
	}

	// Same network should pass.
	if err := d.ValidateNetworkConsistency(); err != nil {
		t.Errorf("ValidateNetworkConsistency() should pass for matching network, got: %v", err)
	}
}

func TestValidateNetworkConsistency_Mismatch(t *testing.T) {
	tmpDir := t.TempDir()
	dbPath := filepath.Join(tmpDir, "test.sqlite")

	// Create DB as testnet and insert addresses.
	testnetDB, err := New(dbPath, "testnet")
	if err != nil {
		t.Fatalf("New(testnet) error = %v", err)
	}
	if err := testnetDB.RunMigrations(); err != nil {
		t.Fatalf("RunMigrations() error = %v", err)
	}
	addrs := []models.Address{
		{Chain: models.ChainBTC, AddressIndex: 0, Address: "tb1qtest0"},
	}
	if err := testnetDB.InsertAddressBatch(models.ChainBTC, addrs); err != nil {
		t.Fatalf("InsertAddressBatch() error = %v", err)
	}
	testnetDB.Close()

	// Re-open as mainnet — should fail validation.
	mainnetDB, err := New(dbPath, "mainnet")
	if err != nil {
		t.Fatalf("New(mainnet) error = %v", err)
	}
	defer mainnetDB.Close()

	err = mainnetDB.ValidateNetworkConsistency()
	if err == nil {
		t.Fatal("ValidateNetworkConsistency() should fail when network mismatches, got nil")
	}

	// Error message should mention both networks.
	errMsg := err.Error()
	if !strings.Contains(errMsg, "NETWORK MISMATCH") {
		t.Errorf("error should contain 'NETWORK MISMATCH', got: %s", errMsg)
	}
	if !strings.Contains(errMsg, "mainnet") || !strings.Contains(errMsg, "testnet") {
		t.Errorf("error should mention both networks, got: %s", errMsg)
	}
}

func TestNetworkIsolation(t *testing.T) {
	tmpDir := t.TempDir()
	dbPath := filepath.Join(tmpDir, "test.sqlite")

	// Open testnet DB and run migrations.
	testnetDB, err := New(dbPath, "testnet")
	if err != nil {
		t.Fatalf("New(testnet) error = %v", err)
	}
	defer testnetDB.Close()

	if err := testnetDB.RunMigrations(); err != nil {
		t.Fatalf("RunMigrations() error = %v", err)
	}

	// Verify Network() accessor.
	if testnetDB.Network() != "testnet" {
		t.Errorf("Network() = %q, want %q", testnetDB.Network(), "testnet")
	}

	// Seed testnet data: 3 addresses, 1 balance, 1 scan state, 1 transaction, 1 tx_state.
	testnetAddrs := []models.Address{
		{Chain: models.ChainBTC, AddressIndex: 0, Address: "tb1qtestnet0"},
		{Chain: models.ChainBTC, AddressIndex: 1, Address: "tb1qtestnet1"},
		{Chain: models.ChainBTC, AddressIndex: 2, Address: "tb1qtestnet2"},
	}
	if err := testnetDB.InsertAddressBatch(models.ChainBTC, testnetAddrs); err != nil {
		t.Fatalf("InsertAddressBatch(testnet) error = %v", err)
	}

	if err := testnetDB.UpsertBalance(models.ChainBTC, 0, models.TokenNative, "100000"); err != nil {
		t.Fatalf("UpsertBalance(testnet) error = %v", err)
	}

	now := time.Now().UTC().Format(time.RFC3339)
	if err := testnetDB.UpsertScanState(models.ScanState{
		Chain:            models.ChainBTC,
		LastScannedIndex: 500,
		MaxScanID:        5000,
		Status:           ScanStatusScanning,
		StartedAt:        now,
	}); err != nil {
		t.Fatalf("UpsertScanState(testnet) error = %v", err)
	}

	if _, err := testnetDB.InsertTransaction(models.Transaction{
		Chain:       models.ChainBTC,
		TxHash:      "testnet_tx_hash_00000000000000000000000000000000000000000000000000",
		Direction:   "out",
		Token:       models.TokenNative,
		Amount:      "50000",
		FromAddress: "tb1qtestnet0",
		ToAddress:   "tb1qdest",
		Status:      "pending",
	}); err != nil {
		t.Fatalf("InsertTransaction(testnet) error = %v", err)
	}

	if err := testnetDB.CreateTxState(TxStateRow{
		ID:          "tx-testnet-001",
		SweepID:     "sweep-testnet",
		Chain:       "BTC",
		Token:       "NATIVE",
		FromAddress: "tb1qtestnet0",
		ToAddress:   "tb1qdest",
		Amount:      "50000",
		Status:      config.TxStatePending,
	}); err != nil {
		t.Fatalf("CreateTxState(testnet) error = %v", err)
	}

	// Open mainnet DB on the same file.
	mainnetDB, err := New(dbPath, "mainnet")
	if err != nil {
		t.Fatalf("New(mainnet) error = %v", err)
	}
	defer mainnetDB.Close()

	if mainnetDB.Network() != "mainnet" {
		t.Errorf("Network() = %q, want %q", mainnetDB.Network(), "mainnet")
	}

	// Mainnet should see ZERO of testnet's data.
	t.Run("mainnet sees no testnet addresses", func(t *testing.T) {
		count, err := mainnetDB.CountAddresses(models.ChainBTC)
		if err != nil {
			t.Fatalf("CountAddresses(mainnet) error = %v", err)
		}
		if count != 0 {
			t.Errorf("mainnet address count = %d, want 0", count)
		}
	})

	t.Run("mainnet sees no testnet balances", func(t *testing.T) {
		funded, err := mainnetDB.GetFundedAddresses(models.ChainBTC, models.TokenNative)
		if err != nil {
			t.Fatalf("GetFundedAddresses(mainnet) error = %v", err)
		}
		if len(funded) != 0 {
			t.Errorf("mainnet funded count = %d, want 0", len(funded))
		}
	})

	t.Run("mainnet sees no testnet scan state", func(t *testing.T) {
		state, err := mainnetDB.GetScanState(models.ChainBTC)
		if err != nil {
			t.Fatalf("GetScanState(mainnet) error = %v", err)
		}
		if state != nil {
			t.Errorf("mainnet scan state = %+v, want nil", state)
		}
	})

	t.Run("mainnet sees no testnet transactions", func(t *testing.T) {
		txs, total, err := mainnetDB.ListTransactions(nil, 1, 100)
		if err != nil {
			t.Fatalf("ListTransactions(mainnet) error = %v", err)
		}
		if total != 0 || len(txs) != 0 {
			t.Errorf("mainnet transactions total=%d len=%d, want 0", total, len(txs))
		}
	})

	t.Run("mainnet sees no testnet tx_states", func(t *testing.T) {
		pending, err := mainnetDB.GetPendingTxStates("BTC")
		if err != nil {
			t.Fatalf("GetPendingTxStates(mainnet) error = %v", err)
		}
		if len(pending) != 0 {
			t.Errorf("mainnet pending tx states = %d, want 0", len(pending))
		}
	})

	// Insert mainnet addresses — testnet count should be unchanged.
	mainnetAddrs := []models.Address{
		{Chain: models.ChainBTC, AddressIndex: 0, Address: "bc1qmainnet0"},
		{Chain: models.ChainBTC, AddressIndex: 1, Address: "bc1qmainnet1"},
	}
	if err := mainnetDB.InsertAddressBatch(models.ChainBTC, mainnetAddrs); err != nil {
		t.Fatalf("InsertAddressBatch(mainnet) error = %v", err)
	}

	t.Run("testnet still sees 3 addresses after mainnet insert", func(t *testing.T) {
		count, err := testnetDB.CountAddresses(models.ChainBTC)
		if err != nil {
			t.Fatalf("CountAddresses(testnet) error = %v", err)
		}
		if count != 3 {
			t.Errorf("testnet address count = %d, want 3", count)
		}
	})

	t.Run("mainnet sees 2 addresses", func(t *testing.T) {
		count, err := mainnetDB.CountAddresses(models.ChainBTC)
		if err != nil {
			t.Fatalf("CountAddresses(mainnet) error = %v", err)
		}
		if count != 2 {
			t.Errorf("mainnet address count = %d, want 2", count)
		}
	})
}
