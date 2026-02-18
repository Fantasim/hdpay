package config

import (
	"fmt"

	"github.com/kelseyhightower/envconfig"
)

// Config holds all application configuration loaded from environment variables.
type Config struct {
	MnemonicFile string `envconfig:"HDPAY_MNEMONIC_FILE"`
	DBPath       string `envconfig:"HDPAY_DB_PATH" default:"./data/hdpay.sqlite"`
	Port         int    `envconfig:"HDPAY_PORT" default:"8080"`
	LogLevel     string `envconfig:"HDPAY_LOG_LEVEL" default:"info"`
	LogDir       string `envconfig:"HDPAY_LOG_DIR" default:"./logs"`
	Network      string `envconfig:"HDPAY_NETWORK" default:"mainnet"`

	BscScanAPIKey string `envconfig:"HDPAY_BSCSCAN_API_KEY"`
	HeliusAPIKey  string `envconfig:"HDPAY_HELIUS_API_KEY"`

	BTCFeeRate       int    `envconfig:"HDPAY_BTC_FEE_RATE" default:"10"`
	BSCGasPreSeedWei string `envconfig:"HDPAY_BSC_GAS_PRESEED_WEI" default:"5000000000000000"`
}

// Load reads configuration from environment variables prefixed with HDPAY_.
func Load() (*Config, error) {
	var cfg Config
	if err := envconfig.Process("", &cfg); err != nil {
		return nil, fmt.Errorf("failed to process env config: %w", err)
	}
	return &cfg, nil
}
