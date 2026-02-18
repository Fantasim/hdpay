package scanner

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/Fantasim/hdpay/internal/config"
	"github.com/Fantasim/hdpay/internal/models"
)

func TestBlockstreamProvider_FetchNativeBalances(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		resp := blockstreamResponse{
			Address: "bc1qtest",
		}
		resp.ChainStats.FundedTxoSum = 100000
		resp.ChainStats.SpentTxoSum = 50000
		resp.MempoolStats.FundedTxoSum = 10000
		resp.MempoolStats.SpentTxoSum = 0
		json.NewEncoder(w).Encode(resp)
	}))
	defer server.Close()

	rl := NewRateLimiter("test", 100)
	provider := &BlockstreamProvider{
		client:  server.Client(),
		rl:      rl,
		baseURL: server.URL,
	}

	addresses := []models.Address{
		{Chain: models.ChainBTC, AddressIndex: 0, Address: "bc1qtest"},
	}

	results, err := provider.FetchNativeBalances(context.Background(), addresses)
	if err != nil {
		t.Fatalf("FetchNativeBalances() error = %v", err)
	}

	if len(results) != 1 {
		t.Fatalf("expected 1 result, got %d", len(results))
	}

	// Balance = (100000 - 50000) + (10000 - 0) = 60000
	if results[0].Balance != "60000" {
		t.Errorf("expected balance 60000, got %s", results[0].Balance)
	}

	if results[0].AddressIndex != 0 {
		t.Errorf("expected index 0, got %d", results[0].AddressIndex)
	}
}

func TestBlockstreamProvider_RateLimit(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusTooManyRequests)
	}))
	defer server.Close()

	rl := NewRateLimiter("test", 100)
	provider := &BlockstreamProvider{
		client:  server.Client(),
		rl:      rl,
		baseURL: server.URL,
	}

	addresses := []models.Address{
		{Chain: models.ChainBTC, AddressIndex: 0, Address: "bc1qtest"},
	}

	_, err := provider.FetchNativeBalances(context.Background(), addresses)
	if err == nil {
		t.Fatal("expected error for rate limit")
	}
	if err.Error() != "fetch balance for bc1qtest (index 0): provider rate limit exceeded" {
		// Just check it contains the sentinel error.
		if !contains(err.Error(), "rate limit") {
			t.Errorf("expected rate limit error, got: %v", err)
		}
	}
}

func TestBlockstreamProvider_ServerError(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
	}))
	defer server.Close()

	rl := NewRateLimiter("test", 100)
	provider := &BlockstreamProvider{
		client:  server.Client(),
		rl:      rl,
		baseURL: server.URL,
	}

	addresses := []models.Address{
		{Chain: models.ChainBTC, AddressIndex: 0, Address: "bc1qtest"},
	}

	_, err := provider.FetchNativeBalances(context.Background(), addresses)
	if err == nil {
		t.Fatal("expected error for server error")
	}
}

func TestBlockstreamProvider_TokensNotSupported(t *testing.T) {
	provider := &BlockstreamProvider{}
	_, err := provider.FetchTokenBalances(context.Background(), nil, models.TokenUSDC, "")
	if err != config.ErrTokensNotSupported {
		t.Errorf("expected ErrTokensNotSupported, got %v", err)
	}
}

func TestBlockstreamProvider_Metadata(t *testing.T) {
	provider := &BlockstreamProvider{}
	if provider.Name() != "Blockstream" {
		t.Errorf("expected name Blockstream, got %s", provider.Name())
	}
	if provider.Chain() != models.ChainBTC {
		t.Errorf("expected chain BTC, got %s", provider.Chain())
	}
	if provider.MaxBatchSize() != 1 {
		t.Errorf("expected batch size 1, got %d", provider.MaxBatchSize())
	}
}

func contains(s, substr string) bool {
	return len(s) >= len(substr) && (s == substr || len(s) > 0 && containsHelper(s, substr))
}

func containsHelper(s, substr string) bool {
	for i := 0; i <= len(s)-len(substr); i++ {
		if s[i:i+len(substr)] == substr {
			return true
		}
	}
	return false
}
