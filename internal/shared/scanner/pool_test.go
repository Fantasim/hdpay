package scanner

import (
	"context"
	"fmt"
	"sort"
	"sync/atomic"
	"testing"

	"github.com/Fantasim/hdpay/internal/shared/config"
	"github.com/Fantasim/hdpay/internal/shared/models"
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

func (m *mockProvider) Name() string       { return m.name }
func (m *mockProvider) Chain() models.Chain { return m.chain }
func (m *mockProvider) MaxBatchSize() int   { return m.batchSize }

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

// makeAddresses creates N test addresses.
func makeAddresses(n int) []models.Address {
	addrs := make([]models.Address, n)
	for i := range addrs {
		addrs[i] = models.Address{
			Chain:        models.ChainBSC,
			AddressIndex: i,
			Address:      fmt.Sprintf("0x%040d", i),
		}
	}
	return addrs
}

// sortResults sorts BalanceResults by AddressIndex for deterministic comparison.
func sortResults(results []BalanceResult) {
	sort.Slice(results, func(i, j int) bool {
		return results[i].AddressIndex < results[j].AddressIndex
	})
}

// ── Distribution Tests ───────────────────────────────────────────────────────

func TestDistributeAddresses_Proportional(t *testing.T) {
	addrs := makeAddresses(100)

	healthy := []providerAssignment{
		{provider: &mockProvider{batchSize: 200}},
		{provider: &mockProvider{batchSize: 20}},
	}

	distributeAddresses(addrs, healthy)

	// Provider with batch=200 should get ~91 addresses (200/220 * 100).
	// Provider with batch=20 should get ~9 addresses (20/220 * 100).
	total := len(healthy[0].addrs) + len(healthy[1].addrs)
	if total != 100 {
		t.Fatalf("total distributed = %d, want 100", total)
	}

	if len(healthy[0].addrs) < 80 {
		t.Errorf("big provider got %d addresses, expected >= 80", len(healthy[0].addrs))
	}
	if len(healthy[1].addrs) < 5 {
		t.Errorf("small provider got %d addresses, expected >= 5", len(healthy[1].addrs))
	}
}

func TestDistributeAddresses_SingleProvider(t *testing.T) {
	addrs := makeAddresses(50)
	healthy := []providerAssignment{
		{provider: &mockProvider{batchSize: 10}},
	}

	distributeAddresses(addrs, healthy)

	if len(healthy[0].addrs) != 50 {
		t.Errorf("single provider should get all 50 addresses, got %d", len(healthy[0].addrs))
	}
}

func TestDistributeAddresses_Empty(t *testing.T) {
	healthy := []providerAssignment{
		{provider: &mockProvider{batchSize: 10}},
	}
	distributeAddresses(nil, healthy)
	if len(healthy[0].addrs) != 0 {
		t.Error("expected no addresses distributed for nil input")
	}
}

// ── Pool Parallel Fan-Out Tests ──────────────────────────────────────────────

func TestPool_ParallelDistribution(t *testing.T) {
	// Both providers called concurrently; verify all addresses are covered.
	var p1Count, p2Count atomic.Int32

	p1 := &mockProvider{name: "P1", chain: models.ChainBSC, batchSize: 50}
	p1.nativeFunc = func(_ context.Context, addrs []models.Address) ([]BalanceResult, error) {
		p1Count.Add(int32(len(addrs)))
		results := make([]BalanceResult, len(addrs))
		for i, a := range addrs {
			results[i] = BalanceResult{Address: a.Address, AddressIndex: a.AddressIndex, Balance: "1", Source: "P1"}
		}
		return results, nil
	}

	p2 := &mockProvider{name: "P2", chain: models.ChainBSC, batchSize: 50}
	p2.nativeFunc = func(_ context.Context, addrs []models.Address) ([]BalanceResult, error) {
		p2Count.Add(int32(len(addrs)))
		results := make([]BalanceResult, len(addrs))
		for i, a := range addrs {
			results[i] = BalanceResult{Address: a.Address, AddressIndex: a.AddressIndex, Balance: "2", Source: "P2"}
		}
		return results, nil
	}

	pool := NewPool(models.ChainBSC, p1, p2)
	addrs := makeAddresses(100)

	results, err := pool.FetchNativeBalances(context.Background(), addrs)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if len(results) != 100 {
		t.Fatalf("expected 100 results, got %d", len(results))
	}

	totalHandled := int(p1Count.Load()) + int(p2Count.Load())
	if totalHandled != 100 {
		t.Errorf("total handled = %d, want 100", totalHandled)
	}

	// Both providers should have been called (parallel fan-out).
	if p1Count.Load() == 0 || p2Count.Load() == 0 {
		t.Errorf("expected both providers called, P1=%d P2=%d", p1Count.Load(), p2Count.Load())
	}
}

func TestPool_ParallelWeightedDistribution(t *testing.T) {
	// Provider with batchSize=200 should get more addresses than batchSize=20.
	var bigCount, smallCount atomic.Int32

	big := &mockProvider{name: "Big", chain: models.ChainBSC, batchSize: 200}
	big.nativeFunc = func(_ context.Context, addrs []models.Address) ([]BalanceResult, error) {
		bigCount.Add(int32(len(addrs)))
		results := make([]BalanceResult, len(addrs))
		for i, a := range addrs {
			results[i] = BalanceResult{Address: a.Address, AddressIndex: a.AddressIndex, Balance: "1", Source: "Big"}
		}
		return results, nil
	}

	small := &mockProvider{name: "Small", chain: models.ChainBSC, batchSize: 20}
	small.nativeFunc = func(_ context.Context, addrs []models.Address) ([]BalanceResult, error) {
		smallCount.Add(int32(len(addrs)))
		results := make([]BalanceResult, len(addrs))
		for i, a := range addrs {
			results[i] = BalanceResult{Address: a.Address, AddressIndex: a.AddressIndex, Balance: "2", Source: "Small"}
		}
		return results, nil
	}

	pool := NewPool(models.ChainBSC, big, small)
	addrs := makeAddresses(220)

	results, err := pool.FetchNativeBalances(context.Background(), addrs)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if len(results) != 220 {
		t.Fatalf("expected 220 results, got %d", len(results))
	}

	// Big should have ~200 (200/220 * 220), Small ~20.
	if bigCount.Load() < 150 {
		t.Errorf("big provider got %d, expected >= 150", bigCount.Load())
	}
	if smallCount.Load() < 10 {
		t.Errorf("small provider got %d, expected >= 10", smallCount.Load())
	}
}

func TestPool_ParallelPartialFailure(t *testing.T) {
	// P1 succeeds, P2 fails → P2's addresses retried on P1.
	p1 := &mockProvider{name: "P1", chain: models.ChainBSC, batchSize: 50}
	p2 := &mockProvider{name: "P2", chain: models.ChainBSC, batchSize: 50,
		nativeErr: config.NewTransientError(config.ErrProviderUnavailable)}

	pool := NewPool(models.ChainBSC, p1, p2)
	addrs := makeAddresses(10)

	results, err := pool.FetchNativeBalances(context.Background(), addrs)
	if err != nil {
		t.Fatalf("expected success with retry, got error: %v", err)
	}

	if len(results) != 10 {
		t.Fatalf("expected 10 results, got %d", len(results))
	}

	// All results should be successful (retried addresses go to P1).
	for _, r := range results {
		if r.Error != "" {
			t.Errorf("address %d: unexpected error: %s", r.AddressIndex, r.Error)
		}
	}
}

func TestPool_SingleProviderFastPath(t *testing.T) {
	p1 := &mockProvider{name: "P1", chain: models.ChainBTC, batchSize: 10}

	pool := NewPool(models.ChainBTC, p1)
	addrs := makeAddresses(5)

	results, err := pool.FetchNativeBalances(context.Background(), addrs)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if len(results) != 5 {
		t.Fatalf("expected 5 results, got %d", len(results))
	}

	for _, r := range results {
		if r.Source != "P1" {
			t.Errorf("expected source P1, got %s", r.Source)
		}
	}
}

func TestPool_SubBatching(t *testing.T) {
	// Provider with MaxBatchSize=3 gets 10 addresses → should make 4 calls (3+3+3+1).
	var callCount atomic.Int32

	p1 := &mockProvider{name: "P1", chain: models.ChainBSC, batchSize: 3}
	p1.nativeFunc = func(_ context.Context, addrs []models.Address) ([]BalanceResult, error) {
		callCount.Add(1)
		if len(addrs) > 3 {
			t.Errorf("sub-batch too large: %d > MaxBatchSize 3", len(addrs))
		}
		results := make([]BalanceResult, len(addrs))
		for i, a := range addrs {
			results[i] = BalanceResult{Address: a.Address, AddressIndex: a.AddressIndex, Balance: "1", Source: "P1"}
		}
		return results, nil
	}

	pool := NewPool(models.ChainBSC, p1)
	addrs := makeAddresses(10)

	results, err := pool.FetchNativeBalances(context.Background(), addrs)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if len(results) != 10 {
		t.Fatalf("expected 10 results, got %d", len(results))
	}

	if callCount.Load() != 4 {
		t.Errorf("expected 4 sub-batch calls, got %d", callCount.Load())
	}
}

func TestPool_Failover(t *testing.T) {
	// P1 fails → retry sends addresses to P2.
	p1 := &mockProvider{name: "P1", chain: models.ChainBTC, batchSize: 1,
		nativeErr: config.ErrProviderRateLimit}
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
	p1 := &mockProvider{name: "P1", chain: models.ChainBTC, batchSize: 1,
		nativeErr: config.ErrProviderUnavailable}
	p2 := &mockProvider{name: "P2", chain: models.ChainBTC, batchSize: 1,
		nativeErr: config.ErrProviderRateLimit}

	pool := NewPool(models.ChainBTC, p1, p2)
	addrs := []models.Address{{Chain: models.ChainBTC, AddressIndex: 0, Address: "addr1"}}

	_, err := pool.FetchNativeBalances(context.Background(), addrs)
	if err == nil {
		t.Fatal("expected error when all providers fail")
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

func TestPool_CircuitBreakerSkipsOpenProvider(t *testing.T) {
	p1 := &mockProvider{name: "P1", chain: models.ChainBTC, batchSize: 1,
		nativeErr: config.ErrProviderUnavailable}
	p2 := &mockProvider{name: "P2", chain: models.ChainBTC, batchSize: 1}

	pool := NewPool(models.ChainBTC, p1, p2)

	// Trip P1's circuit breaker.
	for i := 0; i < config.CircuitBreakerThreshold; i++ {
		pool.breakers[0].RecordFailure()
	}

	if pool.breakers[0].State() != config.CircuitOpen {
		t.Fatalf("expected P1 circuit to be open, got %s", pool.breakers[0].State())
	}

	addrs := []models.Address{{Chain: models.ChainBTC, AddressIndex: 0, Address: "addr1"}}

	results, err := pool.FetchNativeBalances(context.Background(), addrs)
	if err != nil {
		t.Fatalf("expected success via P2, got error: %v", err)
	}

	if len(results) != 1 || results[0].Source != "P2" {
		t.Errorf("expected result from P2, got: %+v", results)
	}
}

func TestPool_CircuitBreakerRecordsFailure(t *testing.T) {
	p1 := &mockProvider{name: "P1", chain: models.ChainBTC, batchSize: 1,
		nativeErr: config.ErrProviderUnavailable}
	p2 := &mockProvider{name: "P2", chain: models.ChainBTC, batchSize: 1,
		nativeErr: config.ErrProviderRateLimit}

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

func TestPool_CircuitBreakerResetsOnSuccess(t *testing.T) {
	callCount := 0
	p1 := &mockProvider{name: "P1", chain: models.ChainBTC, batchSize: 1}
	p1.nativeFunc = func(_ context.Context, addrs []models.Address) ([]BalanceResult, error) {
		callCount++
		if callCount <= 2 {
			return nil, config.ErrProviderUnavailable
		}
		return []BalanceResult{{Balance: "100", Source: "P1", AddressIndex: addrs[0].AddressIndex, Address: addrs[0].Address}}, nil
	}

	pool := NewPool(models.ChainBTC, p1)
	addrs := []models.Address{{Chain: models.ChainBTC, AddressIndex: 0, Address: "addr1"}}

	pool.FetchNativeBalances(context.Background(), addrs)
	pool.FetchNativeBalances(context.Background(), addrs)

	if pool.breakers[0].ConsecutiveFailures() != 2 {
		t.Fatalf("expected 2 failures, got %d", pool.breakers[0].ConsecutiveFailures())
	}

	_, err := pool.FetchNativeBalances(context.Background(), addrs)
	if err != nil {
		t.Fatalf("expected success, got: %v", err)
	}

	if pool.breakers[0].ConsecutiveFailures() != 0 {
		t.Errorf("expected 0 failures after success, got %d", pool.breakers[0].ConsecutiveFailures())
	}
}

func TestPool_TransientErrorRetried(t *testing.T) {
	p1 := &mockProvider{name: "P1", chain: models.ChainBTC, batchSize: 1}
	p1.nativeFunc = func(_ context.Context, _ []models.Address) ([]BalanceResult, error) {
		return nil, config.NewTransientError(config.ErrProviderUnavailable)
	}
	p2 := &mockProvider{name: "P2", chain: models.ChainBTC, batchSize: 1}

	pool := NewPool(models.ChainBTC, p1, p2)
	addrs := []models.Address{{Chain: models.ChainBTC, AddressIndex: 0, Address: "addr1"}}

	results, err := pool.FetchNativeBalances(context.Background(), addrs)
	if err != nil {
		t.Fatalf("expected failover success, got: %v", err)
	}
	if len(results) != 1 || results[0].Balance != "100" {
		t.Errorf("expected result from P2, got: %+v", results)
	}
}

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
		{6, "30s"},
		{10, "30s"},
	}

	for _, tt := range tests {
		got := pool.SuggestBackoff(tt.failures)
		if got.String() != tt.want {
			t.Errorf("SuggestBackoff(%d) = %s, want %s", tt.failures, got, tt.want)
		}
	}
}

func TestPool_TokenFailover(t *testing.T) {
	p1 := &mockProvider{name: "P1", chain: models.ChainBSC, batchSize: 1}
	p1.tokenFunc = func(_ context.Context, _ []models.Address, _ models.Token, _ string) ([]BalanceResult, error) {
		return nil, config.NewTransientError(config.ErrProviderRateLimit)
	}
	p2 := &mockProvider{name: "P2", chain: models.ChainBSC, batchSize: 1}
	p2.tokenFunc = func(_ context.Context, addrs []models.Address, _ models.Token, _ string) ([]BalanceResult, error) {
		results := make([]BalanceResult, len(addrs))
		for i, a := range addrs {
			results[i] = BalanceResult{Balance: "500", Source: "P2", Address: a.Address, AddressIndex: a.AddressIndex}
		}
		return results, nil
	}

	pool := NewPool(models.ChainBSC, p1, p2)
	addrs := []models.Address{{Chain: models.ChainBSC, AddressIndex: 0, Address: "0xAddr"}}

	results, err := pool.FetchTokenBalances(context.Background(), addrs, models.TokenUSDC, "0xContract")
	if err != nil {
		t.Fatalf("expected token failover success, got: %v", err)
	}

	if len(results) != 1 || results[0].Balance != "500" {
		t.Errorf("expected 500 from P2, got: %+v", results)
	}
}

func TestPool_TokenAllUnsupported(t *testing.T) {
	p1 := &mockProvider{name: "P1", chain: models.ChainBTC, batchSize: 1}
	p2 := &mockProvider{name: "P2", chain: models.ChainBTC, batchSize: 1}
	// Both return ErrTokensNotSupported (default mockProvider behavior).

	pool := NewPool(models.ChainBTC, p1, p2)
	addrs := []models.Address{{Chain: models.ChainBTC, AddressIndex: 0, Address: "addr1"}}

	_, err := pool.FetchTokenBalances(context.Background(), addrs, models.TokenUSDC, "0xContract")
	if err != config.ErrTokensNotSupported {
		t.Errorf("expected ErrTokensNotSupported, got: %v", err)
	}
}

func TestPool_EmptyAddresses(t *testing.T) {
	p1 := &mockProvider{name: "P1", chain: models.ChainBTC, batchSize: 1}
	pool := NewPool(models.ChainBTC, p1)

	results, err := pool.FetchNativeBalances(context.Background(), nil)
	if err != nil {
		t.Fatalf("unexpected error for empty addresses: %v", err)
	}
	if results != nil {
		t.Errorf("expected nil results for empty addresses, got %v", results)
	}
}

func TestPool_AllCircuitBreakersOpen(t *testing.T) {
	p1 := &mockProvider{name: "P1", chain: models.ChainBTC, batchSize: 1}
	p2 := &mockProvider{name: "P2", chain: models.ChainBTC, batchSize: 1}

	pool := NewPool(models.ChainBTC, p1, p2)

	// Trip both circuit breakers.
	for i := 0; i < config.CircuitBreakerThreshold; i++ {
		pool.breakers[0].RecordFailure()
		pool.breakers[1].RecordFailure()
	}

	addrs := []models.Address{{Chain: models.ChainBTC, AddressIndex: 0, Address: "addr1"}}

	_, err := pool.FetchNativeBalances(context.Background(), addrs)
	if err == nil {
		t.Fatal("expected error when all circuit breakers are open")
	}
}
