package scanner

import (
	"context"
	"fmt"
	"log/slog"
	"net/http"
	"time"

	"github.com/Fantasim/hdpay/internal/shared/config"
	"github.com/Fantasim/hdpay/internal/wallet/db"
	"github.com/Fantasim/hdpay/internal/shared/models"
)

// SetupScanner creates a fully wired scanner with all provider pools.
func SetupScanner(database *db.DB, cfg *config.Config, hub *SSEHub) (*Scanner, error) {
	slog.Info("setting up scanner",
		"network", cfg.Network,
	)

	httpClient := &http.Client{
		Timeout: config.ProviderRequestTimeout,
		Transport: &http.Transport{
			MaxConnsPerHost:     config.HTTPMaxConnsPerHost,
			MaxIdleConnsPerHost: config.HTTPMaxIdleConnsPerHost,
			MaxIdleConns:        config.HTTPMaxIdleConns,
		},
	}

	scanner := New(database, cfg, hub)

	// ── BTC providers ────────────────────────────────────────────────────────
	// All use the Esplora-compatible REST API (Blockstream, Mempool) or
	// the Bitaps REST API. Round-robin for redundancy and rate-limit spreading.
	btcRL1 := NewRateLimiter("Blockstream", config.RateLimitBlockstream, config.KnownMonthlyLimitBlockstream)
	btcRL2 := NewRateLimiter("Mempool", config.RateLimitMempool, config.KnownMonthlyLimitMempool)
	btcRL3 := NewRateLimiter("Bitaps", config.RateLimitBitaps, config.KnownMonthlyLimitBitaps)
	btcPool := NewPool(models.ChainBTC,
		NewBlockstreamProvider(httpClient, btcRL1, cfg.Network),
		NewMempoolProvider(httpClient, btcRL2, cfg.Network),
		NewBitapsProvider(httpClient, btcRL3, cfg.Network),
	)
	btcPool.SetDB(database)
	scanner.RegisterPool(models.ChainBTC, btcPool)

	// ── BSC providers ─────────────────────────────────────────────────────────
	// BscScan API was shut down Dec 18, 2025. We use public JSON-RPC nodes with
	// round-robin rotation. NodeReal BSCTrace is added if an API key is set —
	// it restores 20-address batch native balance queries (same as old BscScan).
	bscURLs := []string{
		config.BscRPCMainnetURL,  // bsc-dataseed.binance.org
		config.BscRPCMainnetURL2, // rpc.ankr.com/bsc (30 req/s)
		config.LlamaNodesBSCURL,  // bsc.llamarpc.com (50 req/s)
		config.DRPCBSCURL,        // bsc.drpc.org
		config.BscRPCMainnetURL3, // bsc-dataseed.nariox.org
		config.BscRPCMainnetURL4, // bsc-dataseed.defibit.io
		config.BscRPCMainnetURL5, // bsc-dataseed.ninicoin.io
		config.BscRPCMainnetURL6, // bsc-dataseed-public.bnbchain.org
	}
	if cfg.Network == string(models.NetworkTestnet) {
		bscURLs = []string{config.BscRPCTestnetURL}
	}

	var bscProviders []Provider

	// Multicall3 providers first — highest batch efficiency (200 addrs/call).
	// One Multicall3 eth_call reads native + token balances for up to 200 addresses.
	// These are tried before plain RPC providers; circuit breaker handles failover.
	for _, rpcURL := range bscURLs {
		mcName := "Multicall3-" + bscProviderName(rpcURL)
		mcRL := NewRateLimiter(mcName, config.RateLimitBSCRPC, config.KnownMonthlyLimitBSCRPC)
		mcProvider, err := NewBSCMulticallProvider(mcRL, mcName, rpcURL)
		if err != nil {
			slog.Warn("BSC Multicall3 provider failed to connect, skipping",
				"name", mcName,
				"rpcURL", rpcURL,
				"error", err,
			)
			continue
		}
		bscProviders = append(bscProviders, mcProvider)
	}
	slog.Info("BSC Multicall3 providers initialized", "count", len(bscProviders))

	// Plain RPC providers as fallback (1→20 addrs/call with JSON-RPC batch).
	for _, rpcURL := range bscURLs {
		name := bscProviderName(rpcURL)
		rl := NewRateLimiter(name, config.RateLimitBSCRPC, config.KnownMonthlyLimitBSCRPC)
		provider, err := NewBSCRPCProvider(rl, name, rpcURL)
		if err != nil {
			slog.Warn("BSC RPC provider failed to connect, skipping",
				"name", name,
				"rpcURL", rpcURL,
				"error", err,
			)
			continue
		}
		bscProviders = append(bscProviders, provider)
	}

	// NodeReal BSCTrace: optional batch provider (requires free API key from nodereal.io).
	if cfg.NodeRealAPIKey != "" && cfg.Network != string(models.NetworkTestnet) {
		nrRL := NewRateLimiter("NodeRealBSCTrace", config.RateLimitNodeReal, config.KnownMonthlyLimitNodeReal)
		bscProviders = append(bscProviders, NewNodeRealBSCTraceProvider(httpClient, nrRL, cfg.NodeRealAPIKey))
		slog.Info("nodereal bsctrace provider enabled", "batchSize", config.ScanBatchSizeBscScan)
	}

	if len(bscProviders) == 0 {
		slog.Error("all BSC RPC providers failed to connect — BSC scanning disabled")
	} else {
		bscPool := NewPool(models.ChainBSC, bscProviders...)
		bscPool.SetDB(database)
		scanner.RegisterPool(models.ChainBSC, bscPool)
		slog.Info("BSC scanner pool ready", "providerCount", len(bscProviders))
	}

	// ── SOL providers ─────────────────────────────────────────────────────────
	// All use the standard Solana JSON-RPC interface with getMultipleAccounts (100 batch).
	// Round-robin across public and optional key-based providers.
	solanaRPCURL := config.SolanaMainnetRPCURL
	if cfg.Network == string(models.NetworkTestnet) {
		solanaRPCURL = config.SolanaDevnetRPCURL
	}

	solProviders := []Provider{
		NewSolanaRPCProvider(httpClient,
			NewRateLimiter("SolanaPublicRPC", config.RateLimitSolanaRPC, config.KnownMonthlyLimitSolanaRPC),
			solanaRPCURL, "SolanaPublicRPC"),
		NewSolanaRPCProvider(httpClient,
			NewRateLimiter("AnkrSOL", config.RateLimitAnkrSOL, config.KnownMonthlyLimitAnkrSOL),
			ankrSolanaURL(cfg.Network), "AnkrSOL"),
		NewSolanaRPCProvider(httpClient,
			NewRateLimiter("DRPCSOL", config.RateLimitDRPC, config.KnownMonthlyLimitDRPC),
			drpcSolanaURL(cfg.Network), "DRPCSOL"),
		NewSolanaRPCProvider(httpClient,
			NewRateLimiter("OnFinalitySOL", config.RateLimitOnFinality, config.KnownMonthlyLimitOnFinality),
			onFinalitySolanaURL(cfg.Network), "OnFinalitySOL"),
	}

	// Helius: optional key-based (1M credits/month, 10 req/s).
	if cfg.HeliusAPIKey != "" && cfg.Network != string(models.NetworkTestnet) {
		heliusURL := config.HeliusMainnetRPCURL + "?api-key=" + cfg.HeliusAPIKey
		solProviders = append(solProviders,
			NewSolanaRPCProvider(httpClient,
				NewRateLimiter("Helius", config.RateLimitHelius, config.KnownMonthlyLimitHelius),
				heliusURL, "Helius"),
		)
		slog.Info("helius solana provider enabled")
	}

	// Alchemy: optional key-based (30M CU/month, 25 req/s).
	if cfg.AlchemyAPIKey != "" {
		alchemyURL := alchemySolanaURL(cfg.Network, cfg.AlchemyAPIKey)
		solProviders = append(solProviders,
			NewSolanaRPCProvider(httpClient,
				NewRateLimiter("AlchemySOL", config.RateLimitAlchemy, config.KnownMonthlyLimitAlchemy),
				alchemyURL, "AlchemySOL"),
		)
		slog.Info("alchemy solana provider enabled")
	}

	solPool := NewPool(models.ChainSOL, solProviders...)
	solPool.SetDB(database)
	scanner.RegisterPool(models.ChainSOL, solPool)
	slog.Info("SOL scanner pool ready", "providerCount", len(solProviders))

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
			Source:       p.Name(),
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

// bscProviderName derives a short display name from a BSC RPC URL for logging/metrics.
func bscProviderName(rpcURL string) string {
	switch rpcURL {
	case config.BscRPCMainnetURL:
		return "BSCDataseed"
	case config.BscRPCMainnetURL2:
		return "AnkrBSC"
	case config.BscRPCMainnetURL3:
		return "NarioBSC"
	case config.BscRPCMainnetURL4:
		return "DefibitBSC"
	case config.BscRPCMainnetURL5:
		return "NinicoinBSC"
	case config.BscRPCMainnetURL6:
		return "BSCPublic"
	case config.LlamaNodesBSCURL:
		return "LlamaNodesBSC"
	case config.DRPCBSCURL:
		return "DRPCBSC"
	case config.NodeRealBSCRPCURL:
		return "NodeRealBSC"
	case config.BscRPCTestnetURL:
		return "BSCTestnet"
	default:
		return "BSCRPC"
	}
}

// ankrSolanaURL returns the Ankr Solana RPC URL for the given network.
func ankrSolanaURL(network string) string {
	if network == string(models.NetworkTestnet) {
		return config.AnkrSolanaDevnetURL
	}
	return config.AnkrSolanaMainnetURL
}

// drpcSolanaURL returns the dRPC Solana RPC URL for the given network.
// dRPC does not have a documented devnet endpoint; falls back to Solana devnet.
func drpcSolanaURL(network string) string {
	if network == string(models.NetworkTestnet) {
		return config.SolanaDevnetRPCURL
	}
	return config.DRPCSolanaURL
}

// onFinalitySolanaURL returns the OnFinality Solana RPC URL for the given network.
// OnFinality does not have a documented devnet endpoint; falls back to Solana devnet.
func onFinalitySolanaURL(network string) string {
	if network == string(models.NetworkTestnet) {
		return config.SolanaDevnetRPCURL
	}
	return config.OnFinalitySolanaURL
}

// alchemySolanaURL returns the Alchemy Solana RPC URL with the API key embedded in the path.
func alchemySolanaURL(network, apiKey string) string {
	if network == string(models.NetworkTestnet) {
		return fmt.Sprintf(config.AlchemySolanaDevnetURLFmt, apiKey)
	}
	return fmt.Sprintf(config.AlchemySolanaMainnetURLFmt, apiKey)
}
