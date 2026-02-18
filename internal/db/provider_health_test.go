package db

import (
	"testing"

	"github.com/Fantasim/hdpay/internal/config"
)

func TestUpsertProviderHealth(t *testing.T) {
	d := setupTestDB(t)

	ph := ProviderHealthRow{
		ProviderName: "Blockstream",
		Chain:        "BTC",
		ProviderType: config.ProviderTypeScan,
		Status:       config.ProviderStatusHealthy,
		CircuitState: config.CircuitClosed,
	}

	if err := d.UpsertProviderHealth(ph); err != nil {
		t.Fatalf("UpsertProviderHealth() error = %v", err)
	}

	// Retrieve
	got, err := d.GetProviderHealth("Blockstream")
	if err != nil {
		t.Fatalf("GetProviderHealth() error = %v", err)
	}
	if got == nil {
		t.Fatal("expected provider, got nil")
	}
	if got.Status != config.ProviderStatusHealthy {
		t.Errorf("expected healthy, got %s", got.Status)
	}

	// Update via upsert
	ph.Status = config.ProviderStatusDegraded
	ph.ConsecutiveFails = 2
	if err := d.UpsertProviderHealth(ph); err != nil {
		t.Fatalf("UpsertProviderHealth() update error = %v", err)
	}

	got, _ = d.GetProviderHealth("Blockstream")
	if got.Status != config.ProviderStatusDegraded {
		t.Errorf("expected degraded after update, got %s", got.Status)
	}
	if got.ConsecutiveFails != 2 {
		t.Errorf("expected 2 fails, got %d", got.ConsecutiveFails)
	}
}

func TestGetProviderHealthByChain(t *testing.T) {
	d := setupTestDB(t)

	providers := []ProviderHealthRow{
		{ProviderName: "Blockstream", Chain: "BTC", ProviderType: config.ProviderTypeScan, Status: config.ProviderStatusHealthy, CircuitState: config.CircuitClosed},
		{ProviderName: "Mempool", Chain: "BTC", ProviderType: config.ProviderTypeScan, Status: config.ProviderStatusHealthy, CircuitState: config.CircuitClosed},
		{ProviderName: "BscScan", Chain: "BSC", ProviderType: config.ProviderTypeScan, Status: config.ProviderStatusHealthy, CircuitState: config.CircuitClosed},
	}

	for _, p := range providers {
		if err := d.UpsertProviderHealth(p); err != nil {
			t.Fatalf("UpsertProviderHealth(%s) error = %v", p.ProviderName, err)
		}
	}

	btcProviders, err := d.GetProviderHealthByChain("BTC")
	if err != nil {
		t.Fatalf("GetProviderHealthByChain(BTC) error = %v", err)
	}
	if len(btcProviders) != 2 {
		t.Errorf("expected 2 BTC providers, got %d", len(btcProviders))
	}

	bscProviders, err := d.GetProviderHealthByChain("BSC")
	if err != nil {
		t.Fatalf("GetProviderHealthByChain(BSC) error = %v", err)
	}
	if len(bscProviders) != 1 {
		t.Errorf("expected 1 BSC provider, got %d", len(bscProviders))
	}
}

func TestGetAllProviderHealth(t *testing.T) {
	d := setupTestDB(t)

	providers := []ProviderHealthRow{
		{ProviderName: "Blockstream", Chain: "BTC", ProviderType: config.ProviderTypeScan, Status: config.ProviderStatusHealthy, CircuitState: config.CircuitClosed},
		{ProviderName: "BscScan", Chain: "BSC", ProviderType: config.ProviderTypeScan, Status: config.ProviderStatusDegraded, CircuitState: config.CircuitHalfOpen},
		{ProviderName: "SolanaRPC", Chain: "SOL", ProviderType: config.ProviderTypeScan, Status: config.ProviderStatusDown, CircuitState: config.CircuitOpen},
	}

	for _, p := range providers {
		if err := d.UpsertProviderHealth(p); err != nil {
			t.Fatalf("UpsertProviderHealth(%s) error = %v", p.ProviderName, err)
		}
	}

	all, err := d.GetAllProviderHealth()
	if err != nil {
		t.Fatalf("GetAllProviderHealth() error = %v", err)
	}
	if len(all) != 3 {
		t.Errorf("expected 3 providers, got %d", len(all))
	}
}

func TestRecordProviderSuccess(t *testing.T) {
	d := setupTestDB(t)

	ph := ProviderHealthRow{
		ProviderName:     "Blockstream",
		Chain:            "BTC",
		ProviderType:     config.ProviderTypeScan,
		Status:           config.ProviderStatusDegraded,
		ConsecutiveFails: 3,
		CircuitState:     config.CircuitOpen,
	}
	if err := d.UpsertProviderHealth(ph); err != nil {
		t.Fatalf("UpsertProviderHealth() error = %v", err)
	}

	if err := d.RecordProviderSuccess("Blockstream"); err != nil {
		t.Fatalf("RecordProviderSuccess() error = %v", err)
	}

	got, _ := d.GetProviderHealth("Blockstream")
	if got.ConsecutiveFails != 0 {
		t.Errorf("expected 0 consecutive fails, got %d", got.ConsecutiveFails)
	}
	if got.Status != config.ProviderStatusHealthy {
		t.Errorf("expected healthy, got %s", got.Status)
	}
	if got.CircuitState != config.CircuitClosed {
		t.Errorf("expected circuit closed, got %s", got.CircuitState)
	}
	if got.LastSuccess == "" {
		t.Error("expected last_success to be set")
	}
}

func TestRecordProviderFailure(t *testing.T) {
	d := setupTestDB(t)

	ph := ProviderHealthRow{
		ProviderName:     "BscScan",
		Chain:            "BSC",
		ProviderType:     config.ProviderTypeScan,
		Status:           config.ProviderStatusHealthy,
		ConsecutiveFails: 0,
		CircuitState:     config.CircuitClosed,
	}
	if err := d.UpsertProviderHealth(ph); err != nil {
		t.Fatalf("UpsertProviderHealth() error = %v", err)
	}

	if err := d.RecordProviderFailure("BscScan", "connection refused"); err != nil {
		t.Fatalf("RecordProviderFailure() error = %v", err)
	}

	got, _ := d.GetProviderHealth("BscScan")
	if got.ConsecutiveFails != 1 {
		t.Errorf("expected 1 consecutive fail, got %d", got.ConsecutiveFails)
	}
	if got.LastErrorMsg != "connection refused" {
		t.Errorf("expected error msg 'connection refused', got %q", got.LastErrorMsg)
	}
	if got.LastError == "" {
		t.Error("expected last_error to be set")
	}

	// Second failure
	if err := d.RecordProviderFailure("BscScan", "timeout"); err != nil {
		t.Fatalf("RecordProviderFailure() error = %v", err)
	}

	got, _ = d.GetProviderHealth("BscScan")
	if got.ConsecutiveFails != 2 {
		t.Errorf("expected 2 consecutive fails, got %d", got.ConsecutiveFails)
	}
}

func TestProviderHealthNotFound(t *testing.T) {
	d := setupTestDB(t)

	got, err := d.GetProviderHealth("nonexistent")
	if err != nil {
		t.Fatalf("GetProviderHealth() error = %v", err)
	}
	if got != nil {
		t.Errorf("expected nil, got %+v", got)
	}
}
