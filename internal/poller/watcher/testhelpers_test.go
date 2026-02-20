package watcher

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/Fantasim/hdpay/internal/price"
)

// testPriceServer is a mock CoinGecko server for tests.
var testPriceServer *httptest.Server

func init() {
	mux := http.NewServeMux()
	mux.HandleFunc("/simple/price", func(w http.ResponseWriter, r *http.Request) {
		resp := map[string]map[string]float64{
			"bitcoin":     {"usd": 50000.0},
			"binancecoin": {"usd": 300.0},
			"solana":      {"usd": 100.0},
		}
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(resp)
	})
	testPriceServer = httptest.NewServer(mux)
	// Server lives for the entire test process — no cleanup needed.
}

// newPriceMux creates a mock HTTP handler that returns fixed crypto prices.
func newPriceMux() *http.ServeMux {
	mux := http.NewServeMux()
	mux.HandleFunc("/simple/price", func(w http.ResponseWriter, r *http.Request) {
		resp := map[string]map[string]float64{
			"bitcoin":     {"usd": 50000.0},
			"binancecoin": {"usd": 300.0},
			"solana":      {"usd": 100.0},
		}
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(resp)
	})
	return mux
}

// startTestServer starts a httptest server with the given handler.
func startTestServer(handler http.Handler) *httptest.Server {
	return httptest.NewServer(handler)
}

// newPriceServiceWithURL creates a PriceService pointing to a test server URL.
func newPriceServiceWithURL(baseURL string) *price.PriceService {
	return price.NewPriceServiceWithURL(baseURL)
}

// newTestPricerForTest creates a pricer backed by the shared test price server.
func newTestPricerForTest(t *testing.T) *price.PriceService {
	t.Helper()
	return price.NewPriceServiceWithURL(testPriceServer.URL)
}
