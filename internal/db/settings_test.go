package db

import (
	"path/filepath"
	"testing"

	"github.com/Fantasim/hdpay/internal/models"
)

// testTransaction creates a test transaction for the given chain.
func testTransaction(chain models.Chain) models.Transaction {
	return models.Transaction{
		Chain:        chain,
		AddressIndex: 0,
		TxHash:       "deadbeef00000000000000000000000000000000000000000000000000000000",
		Direction:    "send",
		Token:        models.TokenNative,
		Amount:       "50000",
		FromAddress:  "from_address",
		ToAddress:    "to_address",
		Status:       "pending",
	}
}

func TestGetSetting_Default(t *testing.T) {
	d := setupTestDB(t)

	// No settings in DB — should return defaults.
	val, err := d.GetSetting("max_scan_id")
	if err != nil {
		t.Fatalf("GetSetting() error = %v", err)
	}
	if val != "5000" {
		t.Errorf("default max_scan_id = %q, want %q", val, "5000")
	}
}

func TestGetSetting_UnknownKey(t *testing.T) {
	d := setupTestDB(t)

	_, err := d.GetSetting("nonexistent_key")
	if err == nil {
		t.Fatal("expected error for unknown key, got nil")
	}
}

func TestSetSetting_AndGet(t *testing.T) {
	d := setupTestDB(t)

	if err := d.SetSetting("max_scan_id", "10000"); err != nil {
		t.Fatalf("SetSetting() error = %v", err)
	}

	val, err := d.GetSetting("max_scan_id")
	if err != nil {
		t.Fatalf("GetSetting() error = %v", err)
	}
	if val != "10000" {
		t.Errorf("max_scan_id = %q, want %q", val, "10000")
	}
}

func TestSetSetting_Upsert(t *testing.T) {
	d := setupTestDB(t)

	if err := d.SetSetting("log_level", "debug"); err != nil {
		t.Fatalf("first SetSetting() error = %v", err)
	}
	if err := d.SetSetting("log_level", "warn"); err != nil {
		t.Fatalf("second SetSetting() error = %v", err)
	}

	val, err := d.GetSetting("log_level")
	if err != nil {
		t.Fatalf("GetSetting() error = %v", err)
	}
	if val != "warn" {
		t.Errorf("log_level = %q, want %q", val, "warn")
	}
}

func TestGetAllSettings_Defaults(t *testing.T) {
	d := setupTestDB(t)

	settings, err := d.GetAllSettings()
	if err != nil {
		t.Fatalf("GetAllSettings() error = %v", err)
	}

	expectedKeys := []string{"max_scan_id", "auto_resume_scans", "resume_threshold_hours", "btc_fee_rate", "bsc_gas_preseed_bnb", "log_level"}
	for _, key := range expectedKeys {
		if _, ok := settings[key]; !ok {
			t.Errorf("missing default key %q", key)
		}
	}
}

func TestGetAllSettings_OverridesDefaults(t *testing.T) {
	d := setupTestDB(t)

	if err := d.SetSetting("max_scan_id", "99999"); err != nil {
		t.Fatal(err)
	}

	settings, err := d.GetAllSettings()
	if err != nil {
		t.Fatalf("GetAllSettings() error = %v", err)
	}

	if settings["max_scan_id"] != "99999" {
		t.Errorf("max_scan_id = %q, want %q", settings["max_scan_id"], "99999")
	}
	// Other defaults should still be present.
	if settings["log_level"] != "info" {
		t.Errorf("log_level = %q, want %q", settings["log_level"], "info")
	}
}

func TestResetBalances(t *testing.T) {
	d := setupTestDB(t)

	// Insert some data.
	seedAddresses(t, d, "BTC", 3)

	// Insert a transaction.
	_, err := d.InsertTransaction(testTransaction("BTC"))
	if err != nil {
		t.Fatal(err)
	}

	// Reset balances (should keep addresses, remove transactions).
	if err := d.ResetBalances(); err != nil {
		t.Fatalf("ResetBalances() error = %v", err)
	}

	// Addresses should still exist.
	count, err := d.CountAddresses("BTC")
	if err != nil {
		t.Fatal(err)
	}
	if count != 3 {
		t.Errorf("address count = %d, want 3 (addresses should be preserved)", count)
	}

	// Transactions should be gone.
	txs, total, err := d.ListTransactions(nil, 1, 100)
	if err != nil {
		t.Fatal(err)
	}
	if total != 0 || len(txs) != 0 {
		t.Errorf("transactions total = %d, len = %d, want 0", total, len(txs))
	}
}

func TestResetAll(t *testing.T) {
	d := setupTestDB(t)

	seedAddresses(t, d, "BTC", 3)

	_, err := d.InsertTransaction(testTransaction("BTC"))
	if err != nil {
		t.Fatal(err)
	}

	if err := d.ResetAll(); err != nil {
		t.Fatalf("ResetAll() error = %v", err)
	}

	// Everything should be gone.
	count, err := d.CountAddresses("BTC")
	if err != nil {
		t.Fatal(err)
	}
	if count != 0 {
		t.Errorf("address count = %d, want 0", count)
	}

	txs, total, err := d.ListTransactions(nil, 1, 100)
	if err != nil {
		t.Fatal(err)
	}
	if total != 0 || len(txs) != 0 {
		t.Errorf("transactions total = %d, len = %d, want 0", total, len(txs))
	}
}

// setupDualNetworkDB creates testnet and mainnet DB instances on the same file,
// each seeded with addresses and a transaction.
func setupDualNetworkDB(t *testing.T) (testnetDB, mainnetDB *DB) {
	t.Helper()
	tmpDir := t.TempDir()
	dbPath := filepath.Join(tmpDir, "test.sqlite")

	testnetDB, err := New(dbPath, "testnet")
	if err != nil {
		t.Fatalf("New(testnet) error = %v", err)
	}
	if err := testnetDB.RunMigrations(); err != nil {
		t.Fatalf("RunMigrations() error = %v", err)
	}
	t.Cleanup(func() { testnetDB.Close() })

	mainnetDB, err = New(dbPath, "mainnet")
	if err != nil {
		t.Fatalf("New(mainnet) error = %v", err)
	}
	t.Cleanup(func() { mainnetDB.Close() })

	// Seed testnet: 3 addresses + 1 balance + 1 transaction.
	seedAddresses(t, testnetDB, models.ChainBTC, 3)
	testnetDB.UpsertBalance(models.ChainBTC, 0, models.TokenNative, "100000")
	testnetDB.InsertTransaction(models.Transaction{
		Chain: models.ChainBTC, TxHash: "testnet_tx_0000000000000000000000000000000000000000000000000000",
		Direction: "out", Token: models.TokenNative, Amount: "50000",
		FromAddress: "tb1qfrom", ToAddress: "tb1qto", Status: "pending",
	})

	// Seed mainnet: 2 addresses + 1 balance + 1 transaction.
	mainnetAddrs := []models.Address{
		{Chain: models.ChainBTC, AddressIndex: 0, Address: "bc1qmainnet0"},
		{Chain: models.ChainBTC, AddressIndex: 1, Address: "bc1qmainnet1"},
	}
	mainnetDB.InsertAddressBatch(models.ChainBTC, mainnetAddrs)
	mainnetDB.UpsertBalance(models.ChainBTC, 0, models.TokenNative, "200000")
	mainnetDB.InsertTransaction(models.Transaction{
		Chain: models.ChainBTC, TxHash: "mainnet_tx_0000000000000000000000000000000000000000000000000000",
		Direction: "out", Token: models.TokenNative, Amount: "75000",
		FromAddress: "bc1qfrom", ToAddress: "bc1qto", Status: "pending",
	})

	return testnetDB, mainnetDB
}

func TestResetBalances_NetworkScoped(t *testing.T) {
	testnetDB, mainnetDB := setupDualNetworkDB(t)

	// Reset testnet balances.
	if err := testnetDB.ResetBalances(); err != nil {
		t.Fatalf("ResetBalances(testnet) error = %v", err)
	}

	// Testnet: addresses preserved, transactions + balances gone.
	count, err := testnetDB.CountAddresses(models.ChainBTC)
	if err != nil {
		t.Fatal(err)
	}
	if count != 3 {
		t.Errorf("testnet address count = %d, want 3 (preserved)", count)
	}

	testnetTxs, testnetTotal, err := testnetDB.ListTransactions(nil, 1, 100)
	if err != nil {
		t.Fatal(err)
	}
	if testnetTotal != 0 || len(testnetTxs) != 0 {
		t.Errorf("testnet transactions total=%d len=%d, want 0", testnetTotal, len(testnetTxs))
	}

	testnetFunded, err := testnetDB.GetFundedAddresses(models.ChainBTC, models.TokenNative)
	if err != nil {
		t.Fatal(err)
	}
	if len(testnetFunded) != 0 {
		t.Errorf("testnet funded = %d, want 0", len(testnetFunded))
	}

	// Mainnet: everything untouched.
	mainnetCount, err := mainnetDB.CountAddresses(models.ChainBTC)
	if err != nil {
		t.Fatal(err)
	}
	if mainnetCount != 2 {
		t.Errorf("mainnet address count = %d, want 2 (untouched)", mainnetCount)
	}

	mainnetTxs, mainnetTotal, err := mainnetDB.ListTransactions(nil, 1, 100)
	if err != nil {
		t.Fatal(err)
	}
	if mainnetTotal != 1 || len(mainnetTxs) != 1 {
		t.Errorf("mainnet transactions total=%d len=%d, want 1", mainnetTotal, len(mainnetTxs))
	}

	mainnetFunded, err := mainnetDB.GetFundedAddresses(models.ChainBTC, models.TokenNative)
	if err != nil {
		t.Fatal(err)
	}
	if len(mainnetFunded) != 1 {
		t.Errorf("mainnet funded = %d, want 1 (untouched)", len(mainnetFunded))
	}
}

func TestResetAll_NetworkScoped(t *testing.T) {
	testnetDB, mainnetDB := setupDualNetworkDB(t)

	// Reset ALL testnet data.
	if err := testnetDB.ResetAll(); err != nil {
		t.Fatalf("ResetAll(testnet) error = %v", err)
	}

	// Testnet: everything gone.
	testnetCount, err := testnetDB.CountAddresses(models.ChainBTC)
	if err != nil {
		t.Fatal(err)
	}
	if testnetCount != 0 {
		t.Errorf("testnet address count = %d, want 0", testnetCount)
	}

	testnetTxs, testnetTotal, err := testnetDB.ListTransactions(nil, 1, 100)
	if err != nil {
		t.Fatal(err)
	}
	if testnetTotal != 0 || len(testnetTxs) != 0 {
		t.Errorf("testnet transactions total=%d len=%d, want 0", testnetTotal, len(testnetTxs))
	}

	// Mainnet: everything untouched.
	mainnetCount, err := mainnetDB.CountAddresses(models.ChainBTC)
	if err != nil {
		t.Fatal(err)
	}
	if mainnetCount != 2 {
		t.Errorf("mainnet address count = %d, want 2 (untouched)", mainnetCount)
	}

	mainnetTxs, mainnetTotal, err := mainnetDB.ListTransactions(nil, 1, 100)
	if err != nil {
		t.Fatal(err)
	}
	if mainnetTotal != 1 || len(mainnetTxs) != 1 {
		t.Errorf("mainnet transactions total=%d len=%d, want 1", mainnetTotal, len(mainnetTxs))
	}

	mainnetFunded, err := mainnetDB.GetFundedAddresses(models.ChainBTC, models.TokenNative)
	if err != nil {
		t.Fatal(err)
	}
	if len(mainnetFunded) != 1 {
		t.Errorf("mainnet funded = %d, want 1 (untouched)", len(mainnetFunded))
	}
}
