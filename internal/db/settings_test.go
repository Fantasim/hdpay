package db

import (
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
