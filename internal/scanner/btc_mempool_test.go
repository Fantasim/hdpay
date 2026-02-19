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

func TestMempoolProvider_FetchNativeBalances(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		resp := mempoolResponse{
			Address: "bc1qtest",
		}
		resp.ChainStats.FundedTxoSum = 200000
		resp.ChainStats.SpentTxoSum = 80000
		resp.MempoolStats.FundedTxoSum = 5000
		resp.MempoolStats.SpentTxoSum = 1000
		json.NewEncoder(w).Encode(resp)
	}))
	defer server.Close()

	rl := NewRateLimiter("test", 100)
	provider := &MempoolProvider{
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

	// Balance = (200000 - 80000) + (5000 - 1000) = 124000
	if results[0].Balance != "124000" {
		t.Errorf("expected balance 124000, got %s", results[0].Balance)
	}

	if results[0].AddressIndex != 0 {
		t.Errorf("expected index 0, got %d", results[0].AddressIndex)
	}

	if results[0].Error != "" {
		t.Errorf("expected no error, got %s", results[0].Error)
	}

	if results[0].Source != "Mempool" {
		t.Errorf("expected source Mempool, got %s", results[0].Source)
	}
}

func TestMempoolProvider_RateLimit(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Retry-After", "15")
		w.WriteHeader(http.StatusTooManyRequests)
	}))
	defer server.Close()

	rl := NewRateLimiter("test", 100)
	provider := &MempoolProvider{
		client:  server.Client(),
		rl:      rl,
		baseURL: server.URL,
	}

	addresses := []models.Address{
		{Chain: models.ChainBTC, AddressIndex: 0, Address: "bc1qtest"},
	}

	results, err := provider.FetchNativeBalances(context.Background(), addresses)
	if err == nil {
		t.Fatal("expected error when all addresses fail with 429")
	}

	if len(results) != 1 {
		t.Fatalf("expected 1 result, got %d", len(results))
	}

	// Per-address error annotation should reference rate limit.
	if !strings.Contains(results[0].Error, "rate limit") {
		t.Errorf("expected rate limit in error annotation, got %q", results[0].Error)
	}
	if results[0].Balance != "0" {
		t.Errorf("expected balance 0 for rate limited address, got %s", results[0].Balance)
	}
}

func TestMempoolProvider_ServerError(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
	}))
	defer server.Close()

	rl := NewRateLimiter("test", 100)
	provider := &MempoolProvider{
		client:  server.Client(),
		rl:      rl,
		baseURL: server.URL,
	}

	addresses := []models.Address{
		{Chain: models.ChainBTC, AddressIndex: 0, Address: "bc1qtest"},
	}

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

func TestMempoolProvider_MalformedJSON(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.Write([]byte(`{invalid json`))
	}))
	defer server.Close()

	rl := NewRateLimiter("test", 100)
	provider := &MempoolProvider{
		client:  server.Client(),
		rl:      rl,
		baseURL: server.URL,
	}

	addresses := []models.Address{
		{Chain: models.ChainBTC, AddressIndex: 0, Address: "bc1qtest"},
	}

	results, err := provider.FetchNativeBalances(context.Background(), addresses)
	if err == nil {
		t.Fatal("expected error for malformed JSON")
	}

	if len(results) != 1 {
		t.Fatalf("expected 1 result, got %d", len(results))
	}
	if results[0].Error == "" {
		t.Error("expected error annotation for malformed JSON")
	}
}

func TestMempoolProvider_PartialFailure(t *testing.T) {
	var callCount atomic.Int32
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		n := callCount.Add(1)
		if n == 2 {
			w.WriteHeader(http.StatusInternalServerError)
			return
		}
		resp := mempoolResponse{}
		resp.ChainStats.FundedTxoSum = 75000
		json.NewEncoder(w).Encode(resp)
	}))
	defer server.Close()

	rl := NewRateLimiter("test", 100)
	provider := &MempoolProvider{
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

	// Partial failure: no top-level error.
	if err != nil {
		t.Fatalf("expected no error for partial failure, got %v", err)
	}

	if len(results) != 3 {
		t.Fatalf("expected 3 results, got %d", len(results))
	}

	// addr0: success
	if results[0].Error != "" {
		t.Errorf("addr0: expected no error, got %s", results[0].Error)
	}
	if results[0].Balance != "75000" {
		t.Errorf("addr0: expected balance 75000, got %s", results[0].Balance)
	}

	// addr1: failure
	if results[1].Error == "" {
		t.Error("addr1: expected error annotation")
	}
	if results[1].Balance != "0" {
		t.Errorf("addr1: expected balance 0, got %s", results[1].Balance)
	}

	// addr2: success (continued after failure)
	if results[2].Error != "" {
		t.Errorf("addr2: expected no error, got %s", results[2].Error)
	}
	if results[2].Balance != "75000" {
		t.Errorf("addr2: expected balance 75000, got %s", results[2].Balance)
	}
}

func TestMempoolProvider_AllFail(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
	}))
	defer server.Close()

	rl := NewRateLimiter("test", 100)
	provider := &MempoolProvider{
		client:  server.Client(),
		rl:      rl,
		baseURL: server.URL,
	}

	addresses := []models.Address{
		{Chain: models.ChainBTC, AddressIndex: 0, Address: "addr0"},
		{Chain: models.ChainBTC, AddressIndex: 1, Address: "addr1"},
	}

	results, err := provider.FetchNativeBalances(context.Background(), addresses)
	if err == nil {
		t.Fatal("expected error when all addresses fail")
	}
	if !strings.Contains(err.Error(), "all 2 addresses failed") {
		t.Errorf("expected 'all 2 addresses failed' error, got: %v", err)
	}

	if len(results) != 2 {
		t.Fatalf("expected 2 results, got %d", len(results))
	}
	for i, r := range results {
		if r.Error == "" {
			t.Errorf("result %d: expected error annotation", i)
		}
	}
}

func TestMempoolProvider_ContextCancellation(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	provider := &MempoolProvider{
		client:  http.DefaultClient,
		rl:      NewRateLimiter("test", 100),
		baseURL: "http://localhost:1",
	}

	addresses := []models.Address{
		{Chain: models.ChainBTC, AddressIndex: 0, Address: "addr0"},
	}

	_, err := provider.FetchNativeBalances(ctx, addresses)
	if err == nil {
		t.Fatal("expected error on context cancellation")
	}
}

func TestMempoolProvider_TokenNotSupported(t *testing.T) {
	provider := &MempoolProvider{}
	_, err := provider.FetchTokenBalances(context.Background(), nil, models.TokenUSDC, "")
	if err != config.ErrTokensNotSupported {
		t.Errorf("expected ErrTokensNotSupported, got %v", err)
	}
}

func TestMempoolProvider_Metadata(t *testing.T) {
	provider := &MempoolProvider{}
	if provider.Name() != "Mempool" {
		t.Errorf("expected name Mempool, got %s", provider.Name())
	}
	if provider.Chain() != models.ChainBTC {
		t.Errorf("expected chain BTC, got %s", provider.Chain())
	}
	if provider.MaxBatchSize() != 1 {
		t.Errorf("expected batch size 1, got %d", provider.MaxBatchSize())
	}
}
