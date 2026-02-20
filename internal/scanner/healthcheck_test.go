package scanner

import (
	"net/http"
	"net/http/httptest"
	"sync"
	"testing"
	"time"

	"github.com/Fantasim/hdpay/internal/config"
	"github.com/Fantasim/hdpay/internal/models"
)

func TestProbeURL_Success(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		w.Write([]byte("12345")) //nolint:errcheck
	}))
	defer server.Close()

	client := &http.Client{Timeout: config.HealthCheckTimeout}
	err := probeURL(client, server.URL)
	if err != nil {
		t.Fatalf("probeURL() error = %v, want nil", err)
	}
}

func TestProbeURL_Non200Status(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
	}))
	defer server.Close()

	client := &http.Client{Timeout: config.HealthCheckTimeout}
	err := probeURL(client, server.URL)
	if err == nil {
		t.Fatal("probeURL() expected error for HTTP 500, got nil")
	}
}

func TestProbeURL_404(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusNotFound)
	}))
	defer server.Close()

	client := &http.Client{Timeout: config.HealthCheckTimeout}
	err := probeURL(client, server.URL)
	if err == nil {
		t.Fatal("probeURL() expected error for HTTP 404, got nil")
	}
}

func TestProbeURL_ConnectionRefused(t *testing.T) {
	client := &http.Client{Timeout: config.HealthCheckTimeout}
	err := probeURL(client, "http://127.0.0.1:1") // port 1 is almost certainly closed
	if err == nil {
		t.Fatal("probeURL() expected error for connection refused, got nil")
	}
}

func TestDoJSONRPCCheck_Success(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			t.Errorf("expected POST, got %s", r.Method)
		}
		if ct := r.Header.Get("Content-Type"); ct != "application/json" {
			t.Errorf("expected Content-Type application/json, got %s", ct)
		}
		w.WriteHeader(http.StatusOK)
		w.Write([]byte(`{"jsonrpc":"2.0","id":1,"result":"0x1234"}`)) //nolint:errcheck
	}))
	defer server.Close()

	err := doJSONRPCCheck(server.URL, `{"jsonrpc":"2.0","method":"eth_blockNumber","params":[],"id":1}`)
	if err != nil {
		t.Fatalf("doJSONRPCCheck() error = %v, want nil", err)
	}
}

func TestDoJSONRPCCheck_ServerError(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
	}))
	defer server.Close()

	err := doJSONRPCCheck(server.URL, `{"jsonrpc":"2.0","method":"eth_blockNumber","params":[],"id":1}`)
	if err == nil {
		t.Fatal("doJSONRPCCheck() expected error for HTTP 500, got nil")
	}
}

func TestDoJSONRPCCheck_RateLimit(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusTooManyRequests)
	}))
	defer server.Close()

	err := doJSONRPCCheck(server.URL, `{"jsonrpc":"2.0","method":"getHealth","id":1}`)
	if err == nil {
		t.Fatal("doJSONRPCCheck() expected error for HTTP 429, got nil")
	}
}

func TestMakeEVMRPCCheck_Success(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		w.Write([]byte(`{"jsonrpc":"2.0","id":1,"result":"0xabc123"}`)) //nolint:errcheck
	}))
	defer server.Close()

	checkFn := makeEVMRPCCheck(server.URL)
	if err := checkFn(); err != nil {
		t.Fatalf("EVM RPC check error = %v, want nil", err)
	}
}

func TestMakeSolanaRPCCheck_Success(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		w.Write([]byte(`{"jsonrpc":"2.0","id":1,"result":"ok"}`)) //nolint:errcheck
	}))
	defer server.Close()

	checkFn := makeSolanaRPCCheck(server.URL)
	if err := checkFn(); err != nil {
		t.Fatalf("Solana RPC check error = %v, want nil", err)
	}
}

func TestRunStartupHealthChecks_AllPass(t *testing.T) {
	// Create mock servers for all provider types.
	btcServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		w.Write([]byte("800000")) //nolint:errcheck
	}))
	defer btcServer.Close()

	rpcServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		w.Write([]byte(`{"jsonrpc":"2.0","id":1,"result":"ok"}`)) //nolint:errcheck
	}))
	defer rpcServer.Close()

	// Inject test URLs via a custom config.
	// We test buildProviderChecks indirectly through RunStartupHealthChecks.
	// Since buildProviderChecks uses hardcoded config constants, we test the
	// individual functions (probeURL, doJSONRPCCheck) directly above,
	// and verify RunStartupHealthChecks doesn't panic with real checks.
	// For a focused test, we directly create ProviderCheck entries.
	checks := []ProviderCheck{
		{Name: "BTC-Test", Chain: models.ChainBTC, URL: btcServer.URL + "/blocks/tip/height"},
		{Name: "BSC-Test", Chain: models.ChainBSC, CheckFn: makeEVMRPCCheck(rpcServer.URL)},
		{Name: "SOL-Test", Chain: models.ChainSOL, CheckFn: makeSolanaRPCCheck(rpcServer.URL)},
		{Name: "CoinGecko-Test", Chain: "", URL: btcServer.URL + "/ping"},
	}

	// Run the checks manually (like RunStartupHealthChecks does internally).
	results := runChecks(checks)

	for _, r := range results {
		if !r.OK {
			t.Errorf("provider %s failed: %v", r.Name, r.Error)
		}
		if r.Latency <= 0 {
			t.Errorf("provider %s has zero latency", r.Name)
		}
	}

	if len(results) != len(checks) {
		t.Errorf("got %d results, want %d", len(results), len(checks))
	}
}

func TestRunStartupHealthChecks_AllFail_NoPanic(t *testing.T) {
	// All checks point to a closed server — should log warnings but not panic.
	checks := []ProviderCheck{
		{Name: "BTC-Dead", Chain: models.ChainBTC, URL: "http://127.0.0.1:1/blocks/tip/height"},
		{Name: "BSC-Dead", Chain: models.ChainBSC, CheckFn: makeEVMRPCCheck("http://127.0.0.1:1")},
		{Name: "SOL-Dead", Chain: models.ChainSOL, CheckFn: makeSolanaRPCCheck("http://127.0.0.1:1")},
	}

	results := runChecks(checks)

	for _, r := range results {
		if r.OK {
			t.Errorf("provider %s should have failed", r.Name)
		}
		if r.Error == nil {
			t.Errorf("provider %s should have an error", r.Name)
		}
	}
}

func TestRunStartupHealthChecks_MixedResults(t *testing.T) {
	goodServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		w.Write([]byte("ok")) //nolint:errcheck
	}))
	defer goodServer.Close()

	checks := []ProviderCheck{
		{Name: "Good", Chain: models.ChainBTC, URL: goodServer.URL},
		{Name: "Bad", Chain: models.ChainBSC, URL: "http://127.0.0.1:1"},
	}

	results := runChecks(checks)

	var okCount, failCount int
	for _, r := range results {
		if r.OK {
			okCount++
		} else {
			failCount++
		}
	}

	if okCount != 1 || failCount != 1 {
		t.Errorf("expected 1 ok + 1 fail, got %d ok + %d fail", okCount, failCount)
	}
}

func TestBuildProviderChecks_Testnet(t *testing.T) {
	cfg := &config.Config{
		Network: "testnet",
	}
	checks := buildProviderChecks(cfg)

	// Should have testnet BTC providers, BSC providers, SOL devnet, and CoinGecko.
	if len(checks) < 4 {
		t.Errorf("expected at least 4 checks for testnet, got %d", len(checks))
	}

	// Verify all checks have a name and chain (except CoinGecko which has empty chain).
	for _, c := range checks {
		if c.Name == "" {
			t.Error("check has empty name")
		}
		// Either URL or CheckFn must be set.
		if c.URL == "" && c.CheckFn == nil {
			t.Errorf("check %s has neither URL nor CheckFn", c.Name)
		}
	}
}

func TestBuildProviderChecks_Mainnet(t *testing.T) {
	cfg := &config.Config{
		Network: "mainnet",
	}
	checks := buildProviderChecks(cfg)

	// Mainnet has more providers (including Ankr for BSC).
	if len(checks) < 5 {
		t.Errorf("expected at least 5 checks for mainnet, got %d", len(checks))
	}
}

func TestBuildProviderChecks_HeliusKey(t *testing.T) {
	cfg := &config.Config{
		Network:      "mainnet",
		HeliusAPIKey: "test-helius-key",
	}
	checks := buildProviderChecks(cfg)

	hasHelius := false
	for _, c := range checks {
		if c.Name == "Helius" {
			hasHelius = true
		}
	}
	if !hasHelius {
		t.Error("expected Helius provider check when HeliusAPIKey is set")
	}
}

// runChecks runs a set of ProviderChecks concurrently and returns results.
// This mirrors the core logic of RunStartupHealthChecks without depending on config URLs.
func runChecks(checks []ProviderCheck) []HealthCheckResult {
	httpClient := &http.Client{Timeout: config.HealthCheckTimeout}

	var (
		results []HealthCheckResult
		mu      = &sync.Mutex{}
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
				err = probeURL(httpClient, c.URL)
			}

			mu.Lock()
			results = append(results, HealthCheckResult{
				Name:    c.Name,
				Chain:   c.Chain,
				OK:      err == nil,
				Latency: time.Since(start),
				Error:   err,
			})
			mu.Unlock()
		}(check)
	}
	wg.Wait()
	return results
}
