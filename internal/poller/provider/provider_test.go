package provider

import (
	"context"
	"errors"
	"testing"
	"time"

	hdconfig "github.com/Fantasim/hdpay/internal/config"
	"github.com/Fantasim/hdpay/internal/scanner"
)

// mockProvider implements Provider for testing round-robin and circuit breaker logic.
type mockProvider struct {
	name        string
	chain       string
	fetchErr    error
	fetchResult []RawTransaction
	callCount   int
}

func (m *mockProvider) Name() string  { return m.name }
func (m *mockProvider) Chain() string { return m.chain }

func (m *mockProvider) FetchTransactions(_ context.Context, _ string, _ int64) ([]RawTransaction, error) {
	m.callCount++
	if m.fetchErr != nil {
		return nil, m.fetchErr
	}
	return m.fetchResult, nil
}

func (m *mockProvider) CheckConfirmation(_ context.Context, _ string, _ int64) (bool, int, error) {
	m.callCount++
	return true, 1, nil
}

func (m *mockProvider) GetCurrentBlock(_ context.Context) (uint64, error) {
	m.callCount++
	return 100, nil
}

func TestNewProviderSet(t *testing.T) {
	p1 := &mockProvider{name: "mock1", chain: "BTC"}
	p2 := &mockProvider{name: "mock2", chain: "BTC"}

	ps := NewProviderSet("BTC", []Provider{p1, p2}, []int{10, 10})

	if ps.Chain() != "BTC" {
		t.Errorf("Chain() = %q, want %q", ps.Chain(), "BTC")
	}
	if ps.ProviderCount() != 2 {
		t.Errorf("ProviderCount() = %d, want 2", ps.ProviderCount())
	}
}

func TestProviderSet_ExecuteFetch_Success(t *testing.T) {
	expectedTxs := []RawTransaction{
		{TxHash: "tx1", Token: "BTC", AmountRaw: "100000", AmountHuman: "0.00100000"},
	}
	p1 := &mockProvider{name: "mock1", chain: "BTC", fetchResult: expectedTxs}

	ps := NewProviderSet("BTC", []Provider{p1}, []int{100})

	ctx := context.Background()
	txs, err := ps.ExecuteFetch(ctx, "addr1", 0)
	if err != nil {
		t.Fatalf("ExecuteFetch() error = %v", err)
	}
	if len(txs) != 1 {
		t.Fatalf("ExecuteFetch() returned %d txs, want 1", len(txs))
	}
	if txs[0].TxHash != "tx1" {
		t.Errorf("TxHash = %q, want %q", txs[0].TxHash, "tx1")
	}
	if p1.callCount != 1 {
		t.Errorf("provider called %d times, want 1", p1.callCount)
	}
}

func TestProviderSet_ExecuteFetch_Rotation(t *testing.T) {
	// First provider fails, second succeeds
	expectedTxs := []RawTransaction{
		{TxHash: "tx2", Token: "BTC"},
	}
	p1 := &mockProvider{name: "failing", chain: "BTC", fetchErr: errors.New("provider down")}
	p2 := &mockProvider{name: "working", chain: "BTC", fetchResult: expectedTxs}

	ps := NewProviderSet("BTC", []Provider{p1, p2}, []int{100, 100})

	ctx := context.Background()
	txs, err := ps.ExecuteFetch(ctx, "addr1", 0)
	if err != nil {
		t.Fatalf("ExecuteFetch() error = %v", err)
	}
	if len(txs) != 1 || txs[0].TxHash != "tx2" {
		t.Errorf("Expected tx2 from second provider")
	}
	if p1.callCount != 1 {
		t.Errorf("failing provider called %d times, want 1", p1.callCount)
	}
	if p2.callCount != 1 {
		t.Errorf("working provider called %d times, want 1", p2.callCount)
	}
}

func TestProviderSet_ExecuteFetch_AllFail(t *testing.T) {
	p1 := &mockProvider{name: "fail1", chain: "BTC", fetchErr: errors.New("down")}
	p2 := &mockProvider{name: "fail2", chain: "BTC", fetchErr: errors.New("also down")}

	ps := NewProviderSet("BTC", []Provider{p1, p2}, []int{100, 100})

	ctx := context.Background()
	_, err := ps.ExecuteFetch(ctx, "addr1", 0)
	if err == nil {
		t.Fatal("ExecuteFetch() expected error when all providers fail")
	}
	if !errors.Is(err, ErrAllProvidersFailed) {
		t.Errorf("error = %v, want ErrAllProvidersFailed", err)
	}
}

func TestProviderSet_ExecuteFetch_NoProviders(t *testing.T) {
	ps := NewProviderSet("BTC", []Provider{}, []int{})

	ctx := context.Background()
	_, err := ps.ExecuteFetch(ctx, "addr1", 0)
	if err == nil {
		t.Fatal("ExecuteFetch() expected error with no providers")
	}
	if !errors.Is(err, ErrNoProviders) {
		t.Errorf("error = %v, want ErrNoProviders", err)
	}
}

func TestProviderSet_ExecuteFetch_ContextCancelled(t *testing.T) {
	p1 := &mockProvider{name: "mock1", chain: "BTC", fetchResult: []RawTransaction{}}

	ps := NewProviderSet("BTC", []Provider{p1}, []int{1}) // 1 rps to ensure rate limit

	ctx, cancel := context.WithCancel(context.Background())

	// First call should work
	_, err := ps.ExecuteFetch(ctx, "addr1", 0)
	if err != nil {
		t.Fatalf("first ExecuteFetch() error = %v", err)
	}

	// Cancel the context
	cancel()

	// Second call should fail due to cancelled context
	_, err = ps.ExecuteFetch(ctx, "addr1", 0)
	if err == nil {
		t.Fatal("ExecuteFetch() expected error with cancelled context")
	}
}

func TestProviderSet_ExecuteConfirmation(t *testing.T) {
	p1 := &mockProvider{name: "mock1", chain: "BTC"}
	ps := NewProviderSet("BTC", []Provider{p1}, []int{100})

	ctx := context.Background()
	confirmed, confirmations, err := ps.ExecuteConfirmation(ctx, "txhash1", 0)
	if err != nil {
		t.Fatalf("ExecuteConfirmation() error = %v", err)
	}
	if !confirmed {
		t.Error("expected confirmed=true")
	}
	if confirmations != 1 {
		t.Errorf("confirmations = %d, want 1", confirmations)
	}
}

func TestProviderSet_ExecuteGetBlock(t *testing.T) {
	p1 := &mockProvider{name: "mock1", chain: "BSC"}
	ps := NewProviderSet("BSC", []Provider{p1}, []int{100})

	ctx := context.Background()
	block, err := ps.ExecuteGetBlock(ctx)
	if err != nil {
		t.Fatalf("ExecuteGetBlock() error = %v", err)
	}
	if block != 100 {
		t.Errorf("block = %d, want 100", block)
	}
}

func TestProviderSet_CircuitBreaker_SkipsOpenCircuit(t *testing.T) {
	// Create a provider that always fails
	failProvider := &mockProvider{name: "failing", chain: "BTC", fetchErr: errors.New("down")}
	successProvider := &mockProvider{name: "success", chain: "BTC", fetchResult: []RawTransaction{
		{TxHash: "ok"},
	}}

	ps := NewProviderSet("BTC", []Provider{failProvider, successProvider}, []int{100, 100})

	ctx := context.Background()

	// Trip the circuit breaker on the first provider by making it fail repeatedly
	for i := 0; i < hdconfig.CircuitBreakerThreshold; i++ {
		ps.ExecuteFetch(ctx, "addr1", 0)
	}

	// Reset the success provider call count
	successProvider.callCount = 0

	// Now the failing provider's circuit should be open, so it should go directly to the success provider
	txs, err := ps.ExecuteFetch(ctx, "addr1", 0)
	if err != nil {
		t.Fatalf("ExecuteFetch() error = %v", err)
	}
	if len(txs) != 1 || txs[0].TxHash != "ok" {
		t.Error("expected success provider result")
	}
}

func TestProviderSet_RoundRobin_CyclesCorrectly(t *testing.T) {
	p1 := &mockProvider{name: "p1", chain: "BTC", fetchResult: []RawTransaction{{TxHash: "from-p1"}}}
	p2 := &mockProvider{name: "p2", chain: "BTC", fetchResult: []RawTransaction{{TxHash: "from-p2"}}}

	ps := NewProviderSet("BTC", []Provider{p1, p2}, []int{100, 100})

	ctx := context.Background()

	// First call should use p1 (index 0)
	txs1, _ := ps.ExecuteFetch(ctx, "addr1", 0)
	if len(txs1) != 1 || txs1[0].TxHash != "from-p1" {
		t.Error("first call should use p1")
	}

	// Current pointer should still be 0 since p1 succeeded
	// (rotation only happens on failure)
	txs2, _ := ps.ExecuteFetch(ctx, "addr1", 0)
	if len(txs2) != 1 || txs2[0].TxHash != "from-p1" {
		t.Error("second call should still use p1 (no rotation on success)")
	}
}

func TestNewHTTPClient(t *testing.T) {
	client := NewHTTPClient()
	if client == nil {
		t.Fatal("NewHTTPClient() returned nil")
	}
	if client.Timeout != hdconfig.ProviderRequestTimeout {
		t.Errorf("Timeout = %v, want %v", client.Timeout, hdconfig.ProviderRequestTimeout)
	}
}

// Ensure the rate limiter is from HDPay's scanner package (integration check)
func TestProviderSet_UsesHDPayRateLimiter(t *testing.T) {
	// Simply verify we can construct with the HDPay rate limiter constants
	rl := scanner.NewRateLimiter("test", hdconfig.RateLimitBlockstream)
	if rl == nil {
		t.Fatal("scanner.NewRateLimiter returned nil")
	}

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	if err := rl.Wait(ctx); err != nil {
		t.Fatalf("RateLimiter.Wait() error = %v", err)
	}
}
