package scanner

import (
	"context"
	"strings"
	"testing"

	"github.com/Fantasim/hdpay/internal/config"
	"github.com/Fantasim/hdpay/internal/models"
)

// mockProvider implements Provider for testing.
type mockProvider struct {
	name       string
	chain      models.Chain
	batchSize  int
	nativeErr  error
	tokenErr   error
	nativeFunc func(ctx context.Context, addresses []models.Address) ([]BalanceResult, error)
	tokenFunc  func(ctx context.Context, addresses []models.Address, token models.Token, contract string) ([]BalanceResult, error)
}

func (m *mockProvider) Name() string         { return m.name }
func (m *mockProvider) Chain() models.Chain   { return m.chain }
func (m *mockProvider) MaxBatchSize() int     { return m.batchSize }

func (m *mockProvider) FetchNativeBalances(ctx context.Context, addresses []models.Address) ([]BalanceResult, error) {
	if m.nativeFunc != nil {
		return m.nativeFunc(ctx, addresses)
	}
	if m.nativeErr != nil {
		return nil, m.nativeErr
	}
	results := make([]BalanceResult, len(addresses))
	for i, a := range addresses {
		results[i] = BalanceResult{
			Address:      a.Address,
			AddressIndex: a.AddressIndex,
			Balance:      "100",
			Source:       m.name,
		}
	}
	return results, nil
}

func (m *mockProvider) FetchTokenBalances(ctx context.Context, addresses []models.Address, token models.Token, contract string) ([]BalanceResult, error) {
	if m.tokenFunc != nil {
		return m.tokenFunc(ctx, addresses, token, contract)
	}
	if m.tokenErr != nil {
		return nil, m.tokenErr
	}
	return nil, config.ErrTokensNotSupported
}

func TestPool_RoundRobin(t *testing.T) {
	p1 := &mockProvider{name: "P1", chain: models.ChainBTC, batchSize: 1}
	p2 := &mockProvider{name: "P2", chain: models.ChainBTC, batchSize: 1}

	pool := NewPool(models.ChainBTC, p1, p2)

	addrs := []models.Address{{Chain: models.ChainBTC, AddressIndex: 0, Address: "addr1"}}

	// Track which providers are called.
	calls := map[string]int{}
	p1.nativeFunc = func(_ context.Context, _ []models.Address) ([]BalanceResult, error) {
		calls["P1"]++
		return []BalanceResult{{Balance: "1"}}, nil
	}
	p2.nativeFunc = func(_ context.Context, _ []models.Address) ([]BalanceResult, error) {
		calls["P2"]++
		return []BalanceResult{{Balance: "2"}}, nil
	}

	for range 4 {
		pool.FetchNativeBalances(context.Background(), addrs)
	}

	if calls["P1"] != 2 || calls["P2"] != 2 {
		t.Errorf("expected even distribution, got P1=%d P2=%d", calls["P1"], calls["P2"])
	}
}

func TestPool_Failover(t *testing.T) {
	p1 := &mockProvider{name: "P1", chain: models.ChainBTC, batchSize: 1, nativeErr: config.ErrProviderRateLimit}
	p2 := &mockProvider{name: "P2", chain: models.ChainBTC, batchSize: 1}

	pool := NewPool(models.ChainBTC, p1, p2)

	addrs := []models.Address{{Chain: models.ChainBTC, AddressIndex: 0, Address: "addr1"}}

	results, err := pool.FetchNativeBalances(context.Background(), addrs)
	if err != nil {
		t.Fatalf("expected failover to succeed, got error: %v", err)
	}

	if len(results) != 1 {
		t.Fatalf("expected 1 result, got %d", len(results))
	}

	if results[0].Balance != "100" {
		t.Errorf("expected balance 100 from P2, got %s", results[0].Balance)
	}
}

func TestPool_AllProvidersFail(t *testing.T) {
	p1 := &mockProvider{name: "P1", chain: models.ChainBTC, batchSize: 1, nativeErr: config.ErrProviderUnavailable}
	p2 := &mockProvider{name: "P2", chain: models.ChainBTC, batchSize: 1, nativeErr: config.ErrProviderRateLimit}

	pool := NewPool(models.ChainBTC, p1, p2)

	addrs := []models.Address{{Chain: models.ChainBTC, AddressIndex: 0, Address: "addr1"}}

	_, err := pool.FetchNativeBalances(context.Background(), addrs)
	if err == nil {
		t.Fatal("expected error when all providers fail")
	}

	// B9 fix: Error should contain both provider errors, not just the last.
	errMsg := err.Error()
	if !strings.Contains(errMsg, "P1") || !strings.Contains(errMsg, "P2") {
		t.Errorf("expected error to mention both providers, got: %s", errMsg)
	}
}

func TestPool_MaxBatchSize(t *testing.T) {
	p1 := &mockProvider{name: "P1", chain: models.ChainBSC, batchSize: 20}
	p2 := &mockProvider{name: "P2", chain: models.ChainBSC, batchSize: 1}

	pool := NewPool(models.ChainBSC, p1, p2)

	if pool.MaxBatchSize() != 20 {
		t.Errorf("expected max batch size 20, got %d", pool.MaxBatchSize())
	}
}

// TestPool_CircuitBreakerSkipsOpenProvider tests that providers with open circuit
// breakers are skipped and the next provider is tried.
func TestPool_CircuitBreakerSkipsOpenProvider(t *testing.T) {
	p1 := &mockProvider{name: "P1", chain: models.ChainBTC, batchSize: 1, nativeErr: config.ErrProviderUnavailable}
	p2 := &mockProvider{name: "P2", chain: models.ChainBTC, batchSize: 1}

	pool := NewPool(models.ChainBTC, p1, p2)

	addrs := []models.Address{{Chain: models.ChainBTC, AddressIndex: 0, Address: "addr1"}}

	// Trip P1's circuit breaker by making it fail 3 times (threshold).
	for i := 0; i < config.CircuitBreakerThreshold; i++ {
		pool.breakers[0].RecordFailure()
	}

	// P1's circuit should now be open.
	if pool.breakers[0].State() != config.CircuitOpen {
		t.Fatalf("expected P1 circuit to be open, got %s", pool.breakers[0].State())
	}

	// Fetch should skip P1 (circuit open) and go to P2.
	results, err := pool.FetchNativeBalances(context.Background(), addrs)
	if err != nil {
		t.Fatalf("expected success via P2, got error: %v", err)
	}

	if len(results) != 1 || results[0].Source != "P2" {
		t.Errorf("expected result from P2, got: %+v", results)
	}
}

// TestPool_CircuitBreakerRecordsFailure tests that provider failures are recorded
// in the circuit breaker.
func TestPool_CircuitBreakerRecordsFailure(t *testing.T) {
	p1 := &mockProvider{name: "P1", chain: models.ChainBTC, batchSize: 1, nativeErr: config.ErrProviderUnavailable}
	p2 := &mockProvider{name: "P2", chain: models.ChainBTC, batchSize: 1, nativeErr: config.ErrProviderRateLimit}

	pool := NewPool(models.ChainBTC, p1, p2)

	addrs := []models.Address{{Chain: models.ChainBTC, AddressIndex: 0, Address: "addr1"}}

	// Call multiple times to accumulate failures.
	for range 3 {
		pool.FetchNativeBalances(context.Background(), addrs)
	}

	// Both providers should have recorded failures.
	if pool.breakers[0].ConsecutiveFailures() == 0 {
		t.Error("expected P1 to have recorded failures")
	}
	if pool.breakers[1].ConsecutiveFailures() == 0 {
		t.Error("expected P2 to have recorded failures")
	}
}

// TestPool_CircuitBreakerResetsOnSuccess tests that a successful call resets
// the circuit breaker.
func TestPool_CircuitBreakerResetsOnSuccess(t *testing.T) {
	callCount := 0
	p1 := &mockProvider{name: "P1", chain: models.ChainBTC, batchSize: 1}
	p1.nativeFunc = func(_ context.Context, addrs []models.Address) ([]BalanceResult, error) {
		callCount++
		if callCount <= 2 {
			return nil, config.ErrProviderUnavailable
		}
		return []BalanceResult{{Balance: "100", Source: "P1"}}, nil
	}

	pool := NewPool(models.ChainBTC, p1)

	addrs := []models.Address{{Chain: models.ChainBTC, AddressIndex: 0, Address: "addr1"}}

	// First 2 calls fail, recording 2 consecutive failures.
	pool.FetchNativeBalances(context.Background(), addrs)
	pool.FetchNativeBalances(context.Background(), addrs)

	if pool.breakers[0].ConsecutiveFailures() != 2 {
		t.Fatalf("expected 2 failures, got %d", pool.breakers[0].ConsecutiveFailures())
	}

	// Third call succeeds, resetting the counter.
	_, err := pool.FetchNativeBalances(context.Background(), addrs)
	if err != nil {
		t.Fatalf("expected success, got: %v", err)
	}

	if pool.breakers[0].ConsecutiveFailures() != 0 {
		t.Errorf("expected 0 failures after success, got %d", pool.breakers[0].ConsecutiveFailures())
	}
}

// TestPool_TransientErrorIsRetriable tests that TransientError is treated
// as retriable by the pool.
func TestPool_TransientErrorIsRetriable(t *testing.T) {
	p1 := &mockProvider{name: "P1", chain: models.ChainBTC, batchSize: 1}
	p1.nativeFunc = func(_ context.Context, _ []models.Address) ([]BalanceResult, error) {
		return nil, config.NewTransientError(config.ErrProviderUnavailable)
	}
	p2 := &mockProvider{name: "P2", chain: models.ChainBTC, batchSize: 1}

	pool := NewPool(models.ChainBTC, p1, p2)

	addrs := []models.Address{{Chain: models.ChainBTC, AddressIndex: 0, Address: "addr1"}}

	// P1 returns TransientError → pool should fail over to P2.
	results, err := pool.FetchNativeBalances(context.Background(), addrs)
	if err != nil {
		t.Fatalf("expected failover success, got: %v", err)
	}
	if results[0].Source != "P2" {
		t.Errorf("expected result from P2, got source: %s", results[0].Source)
	}
}

// TestPool_SuggestBackoff tests exponential backoff calculation.
func TestPool_SuggestBackoff(t *testing.T) {
	pool := NewPool(models.ChainBTC)

	tests := []struct {
		failures int
		want     string
	}{
		{0, "0s"},
		{1, "1s"},
		{2, "2s"},
		{3, "4s"},
		{4, "8s"},
		{5, "16s"},
		{6, "30s"}, // capped at max
		{10, "30s"},
	}

	for _, tt := range tests {
		got := pool.SuggestBackoff(tt.failures)
		if got.String() != tt.want {
			t.Errorf("SuggestBackoff(%d) = %s, want %s", tt.failures, got, tt.want)
		}
	}
}

// TestPool_TokenFailover tests token failover with circuit breaker.
func TestPool_TokenFailover(t *testing.T) {
	p1 := &mockProvider{name: "P1", chain: models.ChainBSC, batchSize: 1}
	p1.tokenFunc = func(_ context.Context, _ []models.Address, _ models.Token, _ string) ([]BalanceResult, error) {
		return nil, config.NewTransientError(config.ErrProviderRateLimit)
	}
	p2 := &mockProvider{name: "P2", chain: models.ChainBSC, batchSize: 1}
	p2.tokenFunc = func(_ context.Context, addrs []models.Address, _ models.Token, _ string) ([]BalanceResult, error) {
		return []BalanceResult{{Balance: "500", Source: "P2"}}, nil
	}

	pool := NewPool(models.ChainBSC, p1, p2)
	addrs := []models.Address{{Chain: models.ChainBSC, AddressIndex: 0, Address: "0xAddr"}}

	results, err := pool.FetchTokenBalances(context.Background(), addrs, models.TokenUSDC, "0xContract")
	if err != nil {
		t.Fatalf("expected token failover success, got: %v", err)
	}
	if results[0].Source != "P2" {
		t.Errorf("expected result from P2, got source: %s", results[0].Source)
	}
}
