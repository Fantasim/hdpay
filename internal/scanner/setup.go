package scanner

import (
	"context"
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

// NewPoolForTest creates a pool with a simple mock provider for handler-level testing.
// The mock returns zero balances instantly, allowing scans to run and complete quickly.
func NewPoolForTest(chain models.Chain) *Pool {
	return NewPool(chain, &testProvider{chain: chain})
}

// testProvider is a simple exported-package-safe provider that returns zero balances.
type testProvider struct {
	chain models.Chain
}

func (p *testProvider) Name() string       { return "TestProvider" }
func (p *testProvider) Chain() models.Chain { return p.chain }
func (p *testProvider) MaxBatchSize() int   { return 10 }

func (p *testProvider) FetchNativeBalances(_ context.Context, addresses []models.Address) ([]BalanceResult, error) {
	results := make([]BalanceResult, len(addresses))
	for i, a := range addresses {
		results[i] = BalanceResult{
			Address:      a.Address,
			AddressIndex: a.AddressIndex,
			Balance:      "0",
		}
	}
	return results, nil
}

func (p *testProvider) FetchTokenBalances(_ context.Context, _ []models.Address, _ models.Token, _ string) ([]BalanceResult, error) {
	return nil, config.ErrTokensNotSupported
}

// NewHTTPClient creates an HTTP client with the standard provider timeout.
func NewHTTPClient() *http.Client {
	return &http.Client{
		Timeout: time.Duration(config.ProviderRequestTimeout),
	}
}
