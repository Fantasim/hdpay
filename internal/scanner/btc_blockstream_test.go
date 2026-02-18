package scanner

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync/atomic"
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

	if results[0].Error != "" {
		t.Errorf("expected no error, got %s", results[0].Error)
	}

	if results[0].Source != "Blockstream" {
		t.Errorf("expected source Blockstream, got %s", results[0].Source)
	}
}

func TestBlockstreamProvider_RateLimit(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Retry-After", "30")
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

	// Single address rate limited → all addresses failed → returns error.
	results, err := provider.FetchNativeBalances(context.Background(), addresses)
	if err == nil {
		t.Fatal("expected error when all addresses fail")
	}

	// Results should still contain the failed address with error annotation.
	if len(results) != 1 {
		t.Fatalf("expected 1 result, got %d", len(results))
	}
	if results[0].Error == "" {
		t.Error("expected error annotation on failed result")
	}
	if results[0].Balance != "0" {
		t.Errorf("expected balance 0 for failed address, got %s", results[0].Balance)
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

	// Single address server error → all addresses failed → returns error.
	results, err := provider.FetchNativeBalances(context.Background(), addresses)
	if err == nil {
		t.Fatal("expected error for server error")
	}

	if len(results) != 1 {
		t.Fatalf("expected 1 result, got %d", len(results))
	}
	if results[0].Error == "" {
		t.Error("expected error annotation on failed result")
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

// TestBlockstreamProvider_ErrorCollection_PartialFailure tests that partial failures
// don't abort the remaining addresses — the key B1 fix.
func TestBlockstreamProvider_ErrorCollection_PartialFailure(t *testing.T) {
	var callCount atomic.Int32
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		n := callCount.Add(1)
		if n == 2 {
			// Second call fails.
			w.WriteHeader(http.StatusInternalServerError)
			return
		}
		resp := blockstreamResponse{}
		resp.ChainStats.FundedTxoSum = 50000
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
		{Chain: models.ChainBTC, AddressIndex: 0, Address: "addr0"},
		{Chain: models.ChainBTC, AddressIndex: 1, Address: "addr1"},
		{Chain: models.ChainBTC, AddressIndex: 2, Address: "addr2"},
	}

	results, err := provider.FetchNativeBalances(context.Background(), addresses)

	// Partial failure → no error returned (only all-fail returns error).
	if err != nil {
		t.Fatalf("expected no error for partial failure, got %v", err)
	}

	// All 3 addresses should have results.
	if len(results) != 3 {
		t.Fatalf("expected 3 results, got %d", len(results))
	}

	// Address 0: success.
	if results[0].Error != "" {
		t.Errorf("addr0: expected no error, got %s", results[0].Error)
	}
	if results[0].Balance != "50000" {
		t.Errorf("addr0: expected balance 50000, got %s", results[0].Balance)
	}

	// Address 1: failure — annotated.
	if results[1].Error == "" {
		t.Error("addr1: expected error annotation")
	}
	if results[1].Balance != "0" {
		t.Errorf("addr1: expected balance 0, got %s", results[1].Balance)
	}

	// Address 2: success (continued after addr1 failure).
	if results[2].Error != "" {
		t.Errorf("addr2: expected no error, got %s", results[2].Error)
	}
	if results[2].Balance != "50000" {
		t.Errorf("addr2: expected balance 50000, got %s", results[2].Balance)
	}
}

// TestBlockstreamProvider_ErrorCollection_AllFail tests that when all addresses fail,
// an error is returned so the pool tries the next provider.
func TestBlockstreamProvider_ErrorCollection_AllFail(t *testing.T) {
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
		{Chain: models.ChainBTC, AddressIndex: 0, Address: "addr0"},
		{Chain: models.ChainBTC, AddressIndex: 1, Address: "addr1"},
	}

	results, err := provider.FetchNativeBalances(context.Background(), addresses)

	// All fail → error returned.
	if err == nil {
		t.Fatal("expected error when all addresses fail")
	}
	if !strings.Contains(err.Error(), "all 2 addresses failed") {
		t.Errorf("expected 'all 2 addresses failed' error, got: %v", err)
	}

	// Results should still have all addresses annotated.
	if len(results) != 2 {
		t.Fatalf("expected 2 results, got %d", len(results))
	}
	for i, r := range results {
		if r.Error == "" {
			t.Errorf("result %d: expected error annotation", i)
		}
	}
}

// TestBlockstreamProvider_ContextCancellation tests that context cancellation stops iteration.
func TestBlockstreamProvider_ContextCancellation(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	cancel() // Cancel immediately.

	provider := &BlockstreamProvider{
		client:  http.DefaultClient,
		rl:      NewRateLimiter("test", 100),
		baseURL: "http://localhost:1", // won't be called
	}

	addresses := []models.Address{
		{Chain: models.ChainBTC, AddressIndex: 0, Address: "addr0"},
	}

	_, err := provider.FetchNativeBalances(ctx, addresses)
	if err == nil {
		t.Fatal("expected error on context cancellation")
	}
}
