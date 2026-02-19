package scanner

import (
	"context"
	"fmt"
	"log/slog"
	"net/http"
	"strings"
	"sync"
	"time"

	"github.com/Fantasim/hdpay/internal/config"
	"github.com/Fantasim/hdpay/internal/models"
)

// ProviderCheck defines a single provider connectivity check.
type ProviderCheck struct {
	Name    string
	Chain   models.Chain
	URL     string       // Full URL to probe (GET).
	CheckFn func() error // Optional custom check (overrides URL probe if set).
}

// HealthCheckResult holds the outcome of a single provider check.
type HealthCheckResult struct {
	Name    string
	Chain   models.Chain
	OK      bool
	Latency time.Duration
	Error   error
}

// RunStartupHealthChecks probes all configured provider endpoints and logs results.
// This is non-blocking — failures emit WARN logs but don't prevent startup.
func RunStartupHealthChecks(cfg *config.Config) []HealthCheckResult {
	slog.Info("running startup provider health checks",
		"network", cfg.Network,
	)

	checks := buildProviderChecks(cfg)

	client := &http.Client{
		Timeout: config.HealthCheckTimeout,
	}

	var (
		results []HealthCheckResult
		mu      sync.Mutex
		wg      sync.WaitGroup
	)

	for _, check := range checks {
		wg.Add(1)
		go func(c ProviderCheck) {
			defer wg.Done()

			start := time.Now()
			var err error

			if c.CheckFn != nil {
				err = c.CheckFn()
			} else {
				err = probeURL(client, c.URL)
			}

			latency := time.Since(start)
			result := HealthCheckResult{
				Name:    c.Name,
				Chain:   c.Chain,
				OK:      err == nil,
				Latency: latency,
				Error:   err,
			}

			mu.Lock()
			results = append(results, result)
			mu.Unlock()

			if err != nil {
				slog.Warn("provider health check FAILED",
					"provider", c.Name,
					"chain", c.Chain,
					"url", c.URL,
					"latency", latency.Round(time.Millisecond),
					"error", err,
				)
			} else {
				slog.Info("provider health check OK",
					"provider", c.Name,
					"chain", c.Chain,
					"latency", latency.Round(time.Millisecond),
				)
			}
		}(check)
	}

	wg.Wait()

	// Summary.
	okCount := 0
	failCount := 0
	for _, r := range results {
		if r.OK {
			okCount++
		} else {
			failCount++
		}
	}

	slog.Info("startup health checks complete",
		"total", len(results),
		"ok", okCount,
		"failed", failCount,
	)

	return results
}

// buildProviderChecks returns the list of providers to probe based on network config.
func buildProviderChecks(cfg *config.Config) []ProviderCheck {
	isTestnet := cfg.Network == string(models.NetworkTestnet)

	var checks []ProviderCheck

	// BTC providers — lightweight GET that returns block height.
	if isTestnet {
		checks = append(checks,
			ProviderCheck{Name: "Blockstream-Testnet", Chain: models.ChainBTC, URL: config.BlockstreamTestnetURL + "/blocks/tip/height"},
			ProviderCheck{Name: "Mempool-Testnet", Chain: models.ChainBTC, URL: config.MempoolTestnetURL + "/blocks/tip/height"},
		)
	} else {
		checks = append(checks,
			ProviderCheck{Name: "Blockstream", Chain: models.ChainBTC, URL: config.BlockstreamMainnetURL + "/blocks/tip/height"},
			ProviderCheck{Name: "Mempool", Chain: models.ChainBTC, URL: config.MempoolMainnetURL + "/blocks/tip/height"},
		)
	}

	// BSC providers.
	if isTestnet {
		checks = append(checks,
			ProviderCheck{Name: "BscScan-Testnet", Chain: models.ChainBSC, URL: config.BscScanTestnetURL + "?module=proxy&action=eth_blockNumber"},
			ProviderCheck{Name: "BSC-RPC-Testnet", Chain: models.ChainBSC, CheckFn: makeEVMRPCCheck(config.BscRPCTestnetURL)},
		)
	} else {
		checks = append(checks,
			ProviderCheck{Name: "BscScan", Chain: models.ChainBSC, URL: config.BscScanAPIURL + "?module=proxy&action=eth_blockNumber"},
			ProviderCheck{Name: "BSC-RPC-Primary", Chain: models.ChainBSC, CheckFn: makeEVMRPCCheck(config.BscRPCMainnetURL)},
			ProviderCheck{Name: "BSC-RPC-Ankr", Chain: models.ChainBSC, CheckFn: makeEVMRPCCheck(config.BscRPCMainnetURL2)},
		)
	}

	// SOL providers.
	if isTestnet {
		checks = append(checks,
			ProviderCheck{Name: "Solana-Devnet", Chain: models.ChainSOL, CheckFn: makeSolanaRPCCheck(config.SolanaDevnetRPCURL)},
		)
	} else {
		checks = append(checks,
			ProviderCheck{Name: "Solana-Mainnet", Chain: models.ChainSOL, CheckFn: makeSolanaRPCCheck(config.SolanaMainnetRPCURL)},
		)
		if cfg.HeliusAPIKey != "" {
			checks = append(checks,
				ProviderCheck{Name: "Helius", Chain: models.ChainSOL, CheckFn: makeSolanaRPCCheck(config.HeliusMainnetRPCURL + "/?api-key=" + cfg.HeliusAPIKey)},
			)
		}
	}

	// Price provider.
	checks = append(checks,
		ProviderCheck{Name: "CoinGecko", Chain: "", URL: config.CoinGeckoBaseURL + "/ping"},
	)

	return checks
}

// probeURL does a simple GET request and checks for a non-error status.
func probeURL(client *http.Client, url string) error {
	ctx, cancel := context.WithTimeout(context.Background(), config.HealthCheckTimeout)
	defer cancel()

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return fmt.Errorf("create request: %w", err)
	}
	req.Header.Set("User-Agent", "hdpay-healthcheck")

	resp, err := client.Do(req)
	if err != nil {
		return fmt.Errorf("request failed: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode >= 400 {
		return fmt.Errorf("unexpected status %d", resp.StatusCode)
	}

	return nil
}

// makeEVMRPCCheck returns a function that sends eth_blockNumber to an EVM JSON-RPC endpoint.
func makeEVMRPCCheck(rpcURL string) func() error {
	return func() error {
		body := `{"jsonrpc":"2.0","method":"eth_blockNumber","params":[],"id":1}`
		return doJSONRPCCheck(rpcURL, body)
	}
}

// makeSolanaRPCCheck returns a function that sends getHealth to a Solana JSON-RPC endpoint.
func makeSolanaRPCCheck(rpcURL string) func() error {
	return func() error {
		body := `{"jsonrpc":"2.0","method":"getHealth","id":1}`
		return doJSONRPCCheck(rpcURL, body)
	}
}

// doJSONRPCCheck sends a JSON-RPC POST request and verifies a successful response.
func doJSONRPCCheck(rpcURL, body string) error {
	client := &http.Client{Timeout: config.HealthCheckTimeout}

	ctx, cancel := context.WithTimeout(context.Background(), config.HealthCheckTimeout)
	defer cancel()

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, rpcURL, strings.NewReader(body))
	if err != nil {
		return fmt.Errorf("create request: %w", err)
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("User-Agent", "hdpay-healthcheck")

	resp, err := client.Do(req)
	if err != nil {
		return fmt.Errorf("request failed: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode >= 400 {
		return fmt.Errorf("unexpected status %d", resp.StatusCode)
	}

	return nil
}
