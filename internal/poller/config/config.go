package config

import (
	"fmt"
	"log/slog"
	"os"

	"github.com/joho/godotenv"
	"github.com/kelseyhightower/envconfig"
)

// Config holds all Poller configuration loaded from environment variables.
type Config struct {
	DBPath              string `envconfig:"POLLER_DB_PATH" default:"./data/poller.sqlite"`
	Port                int    `envconfig:"POLLER_PORT" default:"8081"`
	LogLevel            string `envconfig:"POLLER_LOG_LEVEL" default:"info"`
	LogDir              string `envconfig:"POLLER_LOG_DIR" default:"./logs"`
	Network             string `envconfig:"POLLER_NETWORK" default:"mainnet"`
	StartDate           int64  `envconfig:"POLLER_START_DATE" required:"true"`
	AdminUsername       string `envconfig:"POLLER_ADMIN_USERNAME" required:"true"`
	AdminPassword       string `envconfig:"POLLER_ADMIN_PASSWORD" required:"true"`
	BscScanAPIKey       string `envconfig:"POLLER_BSCSCAN_API_KEY"`
	HeliusAPIKey        string `envconfig:"POLLER_HELIUS_API_KEY"`
	MaxActiveWatches    int    `envconfig:"POLLER_MAX_ACTIVE_WATCHES" default:"100"`
	DefaultWatchTimeout int    `envconfig:"POLLER_DEFAULT_WATCH_TIMEOUT_MIN" default:"30"`
	TiersFile           string `envconfig:"POLLER_TIERS_FILE" default:"./tiers.json"`
}

// Load reads configuration from .env file (if present) then from environment variables.
func Load() (*Config, error) {
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
		return fmt.Errorf("invalid config: network must be \"mainnet\" or \"testnet\", got %q", c.Network)
	}
	if c.Port < 1 || c.Port > 65535 {
		return fmt.Errorf("invalid config: port must be 1-65535, got %d", c.Port)
	}
	if c.StartDate <= 0 {
		return fmt.Errorf("invalid config: POLLER_START_DATE must be a positive unix timestamp")
	}
	if c.MaxActiveWatches < 1 {
		return fmt.Errorf("invalid config: POLLER_MAX_ACTIVE_WATCHES must be >= 1, got %d", c.MaxActiveWatches)
	}
	if c.DefaultWatchTimeout < 1 || c.DefaultWatchTimeout > MaxWatchTimeoutMinutes {
		return fmt.Errorf("invalid config: POLLER_DEFAULT_WATCH_TIMEOUT_MIN must be 1-%d, got %d", MaxWatchTimeoutMinutes, c.DefaultWatchTimeout)
	}
	return nil
}
