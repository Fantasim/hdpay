package config

import (
	"os"
	"testing"
)

func TestLoad_RequiredFields(t *testing.T) {
	// Clear all POLLER_ env vars to test required field validation.
	os.Clearenv()

	_, err := Load()
	if err == nil {
		t.Fatal("expected error when required fields are missing")
	}
}

func TestLoad_ValidConfig(t *testing.T) {
	os.Clearenv()
	t.Setenv("POLLER_START_DATE", "1740000000")
	t.Setenv("POLLER_ADMIN_USERNAME", "admin")
	t.Setenv("POLLER_ADMIN_PASSWORD", "secret")

	cfg, err := Load()
	if err != nil {
		t.Fatalf("Load() error = %v", err)
	}

	if cfg.Port != 8081 {
		t.Errorf("Port = %d, want 8081", cfg.Port)
	}
	if cfg.Network != "mainnet" {
		t.Errorf("Network = %q, want mainnet", cfg.Network)
	}
	if cfg.StartDate != 1740000000 {
		t.Errorf("StartDate = %d, want 1740000000", cfg.StartDate)
	}
	if cfg.MaxActiveWatches != 100 {
		t.Errorf("MaxActiveWatches = %d, want 100", cfg.MaxActiveWatches)
	}
	if cfg.DefaultWatchTimeout != 30 {
		t.Errorf("DefaultWatchTimeout = %d, want 30", cfg.DefaultWatchTimeout)
	}
	if cfg.DBPath != "./data/poller.sqlite" {
		t.Errorf("DBPath = %q, want ./data/poller.sqlite", cfg.DBPath)
	}
}

func TestValidate_InvalidNetwork(t *testing.T) {
	cfg := &Config{
		Network:             "invalid",
		Port:                8081,
		StartDate:           1740000000,
		MaxActiveWatches:    100,
		DefaultWatchTimeout: 30,
	}
	if err := cfg.Validate(); err == nil {
		t.Error("expected error for invalid network")
	}
}

func TestValidate_InvalidPort(t *testing.T) {
	cfg := &Config{
		Network:             "mainnet",
		Port:                0,
		StartDate:           1740000000,
		MaxActiveWatches:    100,
		DefaultWatchTimeout: 30,
	}
	if err := cfg.Validate(); err == nil {
		t.Error("expected error for port 0")
	}
}

func TestValidate_InvalidStartDate(t *testing.T) {
	cfg := &Config{
		Network:             "mainnet",
		Port:                8081,
		StartDate:           0,
		MaxActiveWatches:    100,
		DefaultWatchTimeout: 30,
	}
	if err := cfg.Validate(); err == nil {
		t.Error("expected error for start date 0")
	}
}

func TestValidate_InvalidMaxActiveWatches(t *testing.T) {
	cfg := &Config{
		Network:             "mainnet",
		Port:                8081,
		StartDate:           1740000000,
		MaxActiveWatches:    0,
		DefaultWatchTimeout: 30,
	}
	if err := cfg.Validate(); err == nil {
		t.Error("expected error for max active watches 0")
	}
}

func TestValidate_InvalidWatchTimeout(t *testing.T) {
	cfg := &Config{
		Network:             "mainnet",
		Port:                8081,
		StartDate:           1740000000,
		MaxActiveWatches:    100,
		DefaultWatchTimeout: 999,
	}
	if err := cfg.Validate(); err == nil {
		t.Error("expected error for watch timeout > max")
	}
}

func TestValidate_ValidConfig(t *testing.T) {
	cfg := &Config{
		Network:             "testnet",
		Port:                9090,
		StartDate:           1740000000,
		MaxActiveWatches:    50,
		DefaultWatchTimeout: 60,
	}
	if err := cfg.Validate(); err != nil {
		t.Errorf("Validate() error = %v, want nil", err)
	}
}
