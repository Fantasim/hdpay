package scanner

import (
	"context"
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
}

func (m *mockProvider) Name() string              { return m.name }
func (m *mockProvider) Chain() models.Chain        { return m.chain }
func (m *mockProvider) MaxBatchSize() int          { return m.batchSize }

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
		}
	}
	return results, nil
}

func (m *mockProvider) FetchTokenBalances(_ context.Context, _ []models.Address, _ models.Token, _ string) ([]BalanceResult, error) {
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
}

func TestPool_MaxBatchSize(t *testing.T) {
	p1 := &mockProvider{name: "P1", chain: models.ChainBSC, batchSize: 20}
	p2 := &mockProvider{name: "P2", chain: models.ChainBSC, batchSize: 1}

	pool := NewPool(models.ChainBSC, p1, p2)

	if pool.MaxBatchSize() != 20 {
		t.Errorf("expected max batch size 20, got %d", pool.MaxBatchSize())
	}
}
