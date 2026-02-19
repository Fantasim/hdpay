package price

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/Fantasim/hdpay/internal/config"
)

// mockCoinGeckoResponse returns a valid CoinGecko-style JSON response.
func mockCoinGeckoResponse() map[string]map[string]float64 {
	return map[string]map[string]float64{
		"bitcoin":     {"usd": 97500.00},
		"binancecoin": {"usd": 625.50},
		"solana":      {"usd": 185.20},
		"usd-coin":    {"usd": 1.00},
		"tether":      {"usd": 1.00},
	}
}

func TestGetPrices_Success(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/simple/price" {
			t.Errorf("unexpected path: %s", r.URL.Path)
		}
		ids := r.URL.Query().Get("ids")
		if ids != config.CoinGeckoIDs {
			t.Errorf("unexpected ids param: %s", ids)
		}
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(mockCoinGeckoResponse())
	}))
	defer srv.Close()

	ps := NewPriceServiceWithURL(srv.URL)
	prices, err := ps.GetPrices(context.Background())
	if err != nil {
		t.Fatalf("GetPrices() error = %v", err)
	}

	// Verify all 5 coins mapped correctly.
	expected := map[string]float64{
		"BTC":  97500.00,
		"BNB":  625.50,
		"SOL":  185.20,
		"USDC": 1.00,
		"USDT": 1.00,
	}

	for symbol, want := range expected {
		got, ok := prices[symbol]
		if !ok {
			t.Errorf("missing price for %s", symbol)
			continue
		}
		if got != want {
			t.Errorf("price[%s] = %f, want %f", symbol, got, want)
		}
	}
}

func TestGetPrices_CacheHit(t *testing.T) {
	callCount := 0
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		callCount++
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(mockCoinGeckoResponse())
	}))
	defer srv.Close()

	ps := NewPriceServiceWithURL(srv.URL)

	// First call — should fetch.
	_, err := ps.GetPrices(context.Background())
	if err != nil {
		t.Fatalf("first GetPrices() error = %v", err)
	}

	// Second call — should hit cache.
	_, err = ps.GetPrices(context.Background())
	if err != nil {
		t.Fatalf("second GetPrices() error = %v", err)
	}

	if callCount != 1 {
		t.Errorf("expected 1 HTTP call (cache hit), got %d", callCount)
	}
}

func TestGetPrices_CacheExpiry(t *testing.T) {
	callCount := 0
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		callCount++
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(mockCoinGeckoResponse())
	}))
	defer srv.Close()

	ps := NewPriceServiceWithURL(srv.URL)

	// First fetch.
	_, err := ps.GetPrices(context.Background())
	if err != nil {
		t.Fatalf("first GetPrices() error = %v", err)
	}

	// Manually expire the cache.
	ps.mu.Lock()
	ps.cachedAt = time.Now().Add(-config.PriceCacheDuration - time.Second)
	ps.mu.Unlock()

	// Second fetch — should call API again.
	_, err = ps.GetPrices(context.Background())
	if err != nil {
		t.Fatalf("second GetPrices() error = %v", err)
	}

	if callCount != 2 {
		t.Errorf("expected 2 HTTP calls after cache expiry, got %d", callCount)
	}
}

func TestGetPrices_HTTPError(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusTooManyRequests)
	}))
	defer srv.Close()

	ps := NewPriceServiceWithURL(srv.URL)
	_, err := ps.GetPrices(context.Background())
	if err == nil {
		t.Fatal("expected error for HTTP 429, got nil")
	}
}

func TestGetPrices_MalformedJSON(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.Write([]byte(`{invalid json`))
	}))
	defer srv.Close()

	ps := NewPriceServiceWithURL(srv.URL)
	_, err := ps.GetPrices(context.Background())
	if err == nil {
		t.Fatal("expected error for malformed JSON, got nil")
	}
}

func TestGetPrices_PartialResponse(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Return only BTC price.
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(map[string]map[string]float64{
			"bitcoin": {"usd": 50000},
		})
	}))
	defer srv.Close()

	ps := NewPriceServiceWithURL(srv.URL)
	prices, err := ps.GetPrices(context.Background())
	if err != nil {
		t.Fatalf("GetPrices() error = %v", err)
	}

	if prices["BTC"] != 50000 {
		t.Errorf("BTC = %f, want 50000", prices["BTC"])
	}

	// Missing coins should not be present.
	if _, ok := prices["SOL"]; ok {
		t.Error("expected SOL to be missing from partial response")
	}
}

func TestGetPrices_StaleCacheOnError(t *testing.T) {
	callCount := 0
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		callCount++
		if callCount == 1 {
			// First call succeeds.
			w.Header().Set("Content-Type", "application/json")
			json.NewEncoder(w).Encode(mockCoinGeckoResponse())
			return
		}
		// Subsequent calls fail.
		w.WriteHeader(http.StatusInternalServerError)
	}))
	defer srv.Close()

	ps := NewPriceServiceWithURL(srv.URL)

	// First call — populates cache.
	prices, err := ps.GetPrices(context.Background())
	if err != nil {
		t.Fatalf("first GetPrices() error = %v", err)
	}
	if ps.IsStale() {
		t.Error("expected not stale after successful fetch")
	}

	// Expire the cache (but still within stale tolerance).
	ps.mu.Lock()
	ps.cachedAt = time.Now().Add(-config.PriceCacheDuration - time.Second)
	ps.mu.Unlock()

	// Second call — fetch fails, should return stale cache.
	stalePrices, err := ps.GetPrices(context.Background())
	if err != nil {
		t.Fatalf("expected stale cache on error, got error: %v", err)
	}
	if !ps.IsStale() {
		t.Error("expected IsStale() to be true after stale cache return")
	}

	// Verify stale prices match original.
	if stalePrices["BTC"] != prices["BTC"] {
		t.Errorf("stale BTC = %f, want %f", stalePrices["BTC"], prices["BTC"])
	}
	if stalePrices["SOL"] != prices["SOL"] {
		t.Errorf("stale SOL = %f, want %f", stalePrices["SOL"], prices["SOL"])
	}
}

func TestGetPrices_NoCacheErrorReturnsError(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
	}))
	defer srv.Close()

	ps := NewPriceServiceWithURL(srv.URL)

	// No cache exists — error should be returned.
	_, err := ps.GetPrices(context.Background())
	if err == nil {
		t.Fatal("expected error when no cache and fetch fails")
	}
}

func TestGetPrices_StaleExpiredReturnsError(t *testing.T) {
	callCount := 0
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		callCount++
		if callCount == 1 {
			w.Header().Set("Content-Type", "application/json")
			json.NewEncoder(w).Encode(mockCoinGeckoResponse())
			return
		}
		w.WriteHeader(http.StatusInternalServerError)
	}))
	defer srv.Close()

	ps := NewPriceServiceWithURL(srv.URL)

	// Populate cache.
	_, err := ps.GetPrices(context.Background())
	if err != nil {
		t.Fatalf("first GetPrices() error = %v", err)
	}

	// Expire cache beyond stale tolerance.
	ps.mu.Lock()
	ps.cachedAt = time.Now().Add(-config.PriceStaleTolerance - time.Second)
	ps.mu.Unlock()

	// Second call — stale cache is too old, should return error.
	_, err = ps.GetPrices(context.Background())
	if err == nil {
		t.Fatal("expected error when stale cache is beyond tolerance")
	}
}
