package scanner

import (
	"log/slog"
	"net/http"
	"time"

	"github.com/Fantasim/hdpay/internal/config"
	"github.com/Fantasim/hdpay/internal/db"
	"github.com/Fantasim/hdpay/internal/models"
)

// SetupScanner creates a fully wired scanner with all provider pools.
func SetupScanner(database *db.DB, cfg *config.Config, hub *SSEHub) (*Scanner, error) {
	slog.Info("setting up scanner",
		"network", cfg.Network,
	)

	httpClient := &http.Client{
		Timeout: config.ProviderRequestTimeout,
	}

	scanner := New(database, cfg, hub)

	// BTC providers.
	btcRL1 := NewRateLimiter("Blockstream", config.RateLimitBlockstream)
	btcRL2 := NewRateLimiter("Mempool", config.RateLimitMempool)
	btcPool := NewPool(models.ChainBTC,
		NewBlockstreamProvider(httpClient, btcRL1, cfg.Network),
		NewMempoolProvider(httpClient, btcRL2, cfg.Network),
	)
	scanner.RegisterPool(models.ChainBTC, btcPool)

	// BSC providers.
	bscRL1 := NewRateLimiter("BscScan", config.RateLimitBscScan)
	bscRL2 := NewRateLimiter("BSCRPC", config.RateLimitBlockstream) // ~10 rps for public RPC

	bscScanProvider := NewBscScanProvider(httpClient, bscRL1, cfg.BscScanAPIKey, cfg.Network)

	bscRPCProvider, err := NewBSCRPCProvider(bscRL2, cfg.Network)
	if err != nil {
		slog.Warn("BSC RPC provider failed to connect, using BscScan only",
			"error", err,
		)
		// Fallback: use only BscScan.
		bscPool := NewPool(models.ChainBSC, bscScanProvider)
		scanner.RegisterPool(models.ChainBSC, bscPool)
	} else {
		bscPool := NewPool(models.ChainBSC, bscScanProvider, bscRPCProvider)
		scanner.RegisterPool(models.ChainBSC, bscPool)
	}

	// SOL providers.
	solRL1 := NewRateLimiter("SolanaPublicRPC", config.RateLimitSolanaRPC)
	solRL2 := NewRateLimiter("Helius", config.RateLimitHelius)

	solanaRPCURL := config.SolanaMainnetRPCURL
	heliusRPCURL := config.HeliusMainnetRPCURL
	if cfg.Network == string(models.NetworkTestnet) {
		solanaRPCURL = config.SolanaDevnetRPCURL
		heliusRPCURL = "" // No Helius devnet
	}

	solProviders := []Provider{
		NewSolanaRPCProvider(httpClient, solRL1, solanaRPCURL, "SolanaPublicRPC"),
	}

	if heliusRPCURL != "" {
		apiKeyParam := ""
		if cfg.HeliusAPIKey != "" {
			apiKeyParam = "?api-key=" + cfg.HeliusAPIKey
		}
		solProviders = append(solProviders,
			NewSolanaRPCProvider(httpClient, solRL2, heliusRPCURL+apiKeyParam, "Helius"),
		)
	}

	solPool := NewPool(models.ChainSOL, solProviders...)
	scanner.RegisterPool(models.ChainSOL, solPool)

	slog.Info("scanner setup complete",
		"chains", len(scanner.pools),
	)

	return scanner, nil
}

// SetupScannerForTest creates a scanner with custom providers (for testing).
func SetupScannerForTest(database *db.DB, hub *SSEHub, pools map[models.Chain]*Pool) *Scanner {
	cfg := &config.Config{Network: "testnet"}
	scanner := New(database, cfg, hub)
	for chain, pool := range pools {
		scanner.RegisterPool(chain, pool)
	}
	return scanner
}

// NewHTTPClient creates an HTTP client with the standard provider timeout.
func NewHTTPClient() *http.Client {
	return &http.Client{
		Timeout: time.Duration(config.ProviderRequestTimeout),
	}
}
