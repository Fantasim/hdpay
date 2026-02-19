package config

import (
	"fmt"
	"log/slog"
	"os"

	"github.com/joho/godotenv"
	"github.com/kelseyhightower/envconfig"
)

// Config holds all application configuration loaded from environment variables.
type Config struct {
	MnemonicFile string `envconfig:"HDPAY_MNEMONIC_FILE"`
	DBPath       string `envconfig:"HDPAY_DB_PATH" default:"./data/hdpay.sqlite"`
	Port         int    `envconfig:"HDPAY_PORT" default:"8080"`
	LogLevel     string `envconfig:"HDPAY_LOG_LEVEL" default:"info"`
	LogDir       string `envconfig:"HDPAY_LOG_DIR" default:"./logs"`
	Network      string `envconfig:"HDPAY_NETWORK" default:"testnet"`

	BscScanAPIKey string `envconfig:"HDPAY_BSCSCAN_API_KEY"`
	HeliusAPIKey  string `envconfig:"HDPAY_HELIUS_API_KEY"`

	BTCFeeRate       int    `envconfig:"HDPAY_BTC_FEE_RATE" default:"10"`
	BSCGasPreSeedWei string `envconfig:"HDPAY_BSC_GAS_PRESEED_WEI" default:"5000000000000000"`
}

// Load reads configuration from .env file (if present) then from environment variables.
// Environment variables override .env values.
func Load() (*Config, error) {
	// Load .env file if it exists. godotenv does NOT override already-set env vars,
	// so real environment variables take precedence over .env values.
	envFiles := []string{".env"}
	for _, f := range envFiles {
		if _, err := os.Stat(f); err == nil {
			if err := godotenv.Load(f); err != nil {
				slog.Warn("failed to load .env file", "file", f, "error", err)
			} else {
				slog.Info("loaded .env file", "file", f)
			}
		}
	}

	var cfg Config
	if err := envconfig.Process("", &cfg); err != nil {
		return nil, fmt.Errorf("failed to process env config: %w", err)
	}

	if err := cfg.Validate(); err != nil {
		return nil, err
	}

	return &cfg, nil
}

// Validate checks configuration values for correctness.
func (c *Config) Validate() error {
	if c.Network != "mainnet" && c.Network != "testnet" {
		return fmt.Errorf("%w: network must be \"mainnet\" or \"testnet\", got %q", ErrInvalidConfig, c.Network)
	}
	if c.Port < 1 || c.Port > 65535 {
		return fmt.Errorf("%w: port must be 1-65535, got %d", ErrInvalidConfig, c.Port)
	}
	return nil
}
