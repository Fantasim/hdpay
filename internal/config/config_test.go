package config

import (
	"testing"
)

func TestValidate_ValidMainnet(t *testing.T) {
	cfg := &Config{
		Network: "mainnet",
		Port:    8080,
	}
	if err := cfg.Validate(); err != nil {
		t.Fatalf("Validate() error = %v, want nil", err)
	}
}

func TestValidate_ValidTestnet(t *testing.T) {
	cfg := &Config{
		Network: "testnet",
		Port:    8080,
	}
	if err := cfg.Validate(); err != nil {
		t.Fatalf("Validate() error = %v, want nil", err)
	}
}

func TestValidate_InvalidNetwork(t *testing.T) {
	tests := []struct {
		name    string
		network string
	}{
		{"empty", ""},
		{"foobar", "foobar"},
		{"Mainnet case sensitive", "Mainnet"},
		{"devnet", "devnet"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			cfg := &Config{
				Network: tt.network,
				Port:    8080,
			}
			err := cfg.Validate()
			if err == nil {
				t.Fatalf("Validate() expected error for network=%q, got nil", tt.network)
			}
		})
	}
}

func TestValidate_InvalidPort(t *testing.T) {
	tests := []struct {
		name string
		port int
	}{
		{"zero", 0},
		{"negative", -1},
		{"too high", 65536},
		{"way too high", 100000},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			cfg := &Config{
				Network: "testnet",
				Port:    tt.port,
			}
			err := cfg.Validate()
			if err == nil {
				t.Fatalf("Validate() expected error for port=%d, got nil", tt.port)
			}
		})
	}
}

func TestValidate_ValidPortBoundaries(t *testing.T) {
	tests := []struct {
		name string
		port int
	}{
		{"minimum valid", 1},
		{"maximum valid", 65535},
		{"common port", 3000},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			cfg := &Config{
				Network: "testnet",
				Port:    tt.port,
			}
			if err := cfg.Validate(); err != nil {
				t.Fatalf("Validate() error = %v for port=%d, want nil", err, tt.port)
			}
		})
	}
}

func TestConfig_DefaultValues(t *testing.T) {
	// Verify that the struct tags define the expected defaults.
	// This test documents the expected defaults without calling Load()
	// (which depends on the environment).
	cfg := Config{}

	// When Load() is not called, fields are zero values.
	// This test validates the Validate() interaction with defaults.
	// The actual default application is done by envconfig via struct tags.
	// We test that a properly configured Config validates correctly.
	cfg.Network = "testnet"
	cfg.Port = 8080
	cfg.DBPath = "./data/hdpay.sqlite"
	cfg.LogLevel = "info"
	cfg.LogDir = "./logs"

	if err := cfg.Validate(); err != nil {
		t.Fatalf("Validate() on default-like config: %v", err)
	}
}
