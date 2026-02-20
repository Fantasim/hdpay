package points

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"sync/atomic"
	"testing"
	"time"

	"github.com/Fantasim/hdpay/internal/poller/config"
	"github.com/Fantasim/hdpay/internal/price"
)

func TestGetTokenPrice_Stablecoins(t *testing.T) {
	// Stablecoins should return $1.00 without any API call.
	pricer := NewPricer(price.NewPriceServiceWithURL("http://should-not-be-called"))

	ctx := context.Background()

	for _, token := range []string{"USDC", "USDT"} {
		p, err := pricer.GetTokenPrice(ctx, token)
		if err != nil {
			t.Errorf("GetTokenPrice(%s) error = %v", token, err)
		}
		if p != config.StablecoinPrice {
			t.Errorf("GetTokenPrice(%s) = %.2f, want %.2f", token, p, config.StablecoinPrice)
		}
	}
}

func TestGetTokenPrice_NativeTokens(t *testing.T) {
	// Mock CoinGecko server returning known prices.
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		resp := map[string]map[string]float64{
			"bitcoin":     {"usd": 45000.0},
			"binancecoin": {"usd": 320.0},
			"solana":      {"usd": 105.0},
			"usd-coin":    {"usd": 1.0},
			"tether":      {"usd": 1.0},
		}
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(resp)
	}))
	defer srv.Close()

	ps := price.NewPriceServiceWithURL(srv.URL)
	pricer := NewPricer(ps)
	ctx := context.Background()

	tests := []struct {
		token string
		want  float64
	}{
		{"BTC", 45000.0},
		{"BNB", 320.0},
		{"SOL", 105.0},
	}

	for _, tt := range tests {
		p, err := pricer.GetTokenPrice(ctx, tt.token)
		if err != nil {
			t.Errorf("GetTokenPrice(%s) error = %v", tt.token, err)
		}
		if p != tt.want {
			t.Errorf("GetTokenPrice(%s) = %.2f, want %.2f", tt.token, p, tt.want)
		}
	}
}

func TestGetTokenPrice_UnknownToken(t *testing.T) {
	pricer := NewPricer(price.NewPriceServiceWithURL("http://irrelevant"))
	ctx := context.Background()

	_, err := pricer.GetTokenPrice(ctx, "DOGE")
	if err == nil {
		t.Error("GetTokenPrice(DOGE) should error for unknown token")
	}
}

func TestGetTokenPrice_RetryOnFailure(t *testing.T) {
	var callCount atomic.Int32

	// Fail first 2 calls, succeed on 3rd.
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		n := callCount.Add(1)
		if n < 3 {
			w.WriteHeader(http.StatusServiceUnavailable)
			return
		}
		resp := map[string]map[string]float64{
			"bitcoin":     {"usd": 50000.0},
			"binancecoin": {"usd": 350.0},
			"solana":      {"usd": 110.0},
			"usd-coin":    {"usd": 1.0},
			"tether":      {"usd": 1.0},
		}
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(resp)
	}))
	defer srv.Close()

	ps := price.NewPriceServiceWithURL(srv.URL)
	pricer := NewPricer(ps)

	ctx := context.Background()
	p, err := pricer.GetTokenPrice(ctx, "BTC")
	if err != nil {
		t.Fatalf("GetTokenPrice(BTC) error = %v (expected retry success)", err)
	}
	if p != 50000.0 {
		t.Errorf("GetTokenPrice(BTC) = %.2f, want 50000.0", p)
	}
}

func TestGetTokenPrice_AllRetriesFail(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusServiceUnavailable)
	}))
	defer srv.Close()

	ps := price.NewPriceServiceWithURL(srv.URL)
	pricer := NewPricer(ps)

	ctx := context.Background()
	_, err := pricer.GetTokenPrice(ctx, "BTC")
	if err == nil {
		t.Error("GetTokenPrice(BTC) should error when all retries fail")
	}
}

func TestGetTokenPrice_ContextCancelled(t *testing.T) {
	// Server that always fails (to trigger retry loop).
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusServiceUnavailable)
	}))
	defer srv.Close()

	ps := price.NewPriceServiceWithURL(srv.URL)
	pricer := NewPricer(ps)

	ctx, cancel := context.WithTimeout(context.Background(), 100*time.Millisecond)
	defer cancel()

	_, err := pricer.GetTokenPrice(ctx, "BTC")
	if err == nil {
		t.Error("GetTokenPrice should error when context is cancelled")
	}
}
