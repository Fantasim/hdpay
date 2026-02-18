package handlers

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"testing"

	"github.com/Fantasim/hdpay/internal/db"
	"github.com/Fantasim/hdpay/internal/models"
	"github.com/Fantasim/hdpay/internal/price"
	"github.com/go-chi/chi/v5"
)

// setupDashboardTestDB creates a temporary database with addresses and balances.
func setupDashboardTestDB(t *testing.T, withBalances bool) *db.DB {
	t.Helper()
	tmpDir := t.TempDir()
	dbPath := filepath.Join(tmpDir, "test_dashboard.sqlite")

	database, err := db.New(dbPath)
	if err != nil {
		t.Fatalf("New() error = %v", err)
	}
	if err := database.RunMigrations(); err != nil {
		t.Fatalf("RunMigrations() error = %v", err)
	}

	// Seed 10 addresses per chain.
	for _, chain := range models.AllChains {
		addrs := make([]models.Address, 10)
		for i := 0; i < 10; i++ {
			addrs[i] = models.Address{
				Chain:        chain,
				AddressIndex: i,
				Address:      string(chain) + "_addr_" + string(rune('A'+i)),
			}
		}
		if err := database.InsertAddressBatch(chain, addrs); err != nil {
			t.Fatalf("InsertAddressBatch(%s) error = %v", chain, err)
		}
	}

	if withBalances {
		// BTC: 2 funded addresses with native balance.
		if err := database.UpsertBalance(models.ChainBTC, 0, models.TokenNative, "0.5"); err != nil {
			t.Fatalf("UpsertBalance error = %v", err)
		}
		if err := database.UpsertBalance(models.ChainBTC, 1, models.TokenNative, "1.0"); err != nil {
			t.Fatalf("UpsertBalance error = %v", err)
		}

		// BSC: 1 BNB + 1 USDC.
		if err := database.UpsertBalance(models.ChainBSC, 0, models.TokenNative, "10"); err != nil {
			t.Fatalf("UpsertBalance error = %v", err)
		}
		if err := database.UpsertBalance(models.ChainBSC, 2, models.TokenUSDC, "5000"); err != nil {
			t.Fatalf("UpsertBalance error = %v", err)
		}

		// SOL: 1 funded.
		if err := database.UpsertBalance(models.ChainSOL, 0, models.TokenNative, "100"); err != nil {
			t.Fatalf("UpsertBalance error = %v", err)
		}

		// Add a scan state so GetLatestScanTime returns something.
		if err := database.UpsertScanState(models.ScanState{
			Chain:            models.ChainBTC,
			LastScannedIndex: 10,
			MaxScanID:        10,
			Status:           "completed",
			StartedAt:        "2026-02-18T10:00:00Z",
		}); err != nil {
			t.Fatalf("UpsertScanState error = %v", err)
		}
	}

	t.Cleanup(func() { database.Close() })
	return database
}

// mockPriceServer returns an httptest.Server that returns mock prices.
func mockPriceServer(t *testing.T) *httptest.Server {
	t.Helper()
	return httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(map[string]map[string]float64{
			"bitcoin":     {"usd": 100000},
			"binancecoin": {"usd": 600},
			"solana":      {"usd": 200},
			"usd-coin":    {"usd": 1},
			"tether":      {"usd": 1},
		})
	}))
}

func setupDashboardRouter(t *testing.T, database *db.DB, ps *price.PriceService) http.Handler {
	t.Helper()
	r := chi.NewRouter()
	r.Get("/api/dashboard/prices", GetPrices(ps))
	r.Get("/api/dashboard/portfolio", GetPortfolio(database, ps))
	return r
}

func TestGetPrices_Handler(t *testing.T) {
	srv := mockPriceServer(t)
	defer srv.Close()

	ps := price.NewPriceServiceWithURL(srv.URL)
	// Pre-fill the cache.
	_, err := ps.GetPrices(context.Background())
	if err != nil {
		t.Fatalf("pre-fill prices error = %v", err)
	}

	database := setupDashboardTestDB(t, false)
	router := setupDashboardRouter(t, database, ps)

	req := httptest.NewRequest("GET", "/api/dashboard/prices", nil)
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200. body: %s", w.Code, w.Body.String())
	}

	var resp models.APIResponse
	if err := json.Unmarshal(w.Body.Bytes(), &resp); err != nil {
		t.Fatalf("unmarshal error = %v", err)
	}

	data, ok := resp.Data.(map[string]interface{})
	if !ok {
		t.Fatalf("data is not a map: %T", resp.Data)
	}

	btcPrice, ok := data["BTC"].(float64)
	if !ok || btcPrice != 100000 {
		t.Errorf("BTC price = %v, want 100000", data["BTC"])
	}
}

func TestGetPortfolio_WithBalances(t *testing.T) {
	srv := mockPriceServer(t)
	defer srv.Close()

	ps := price.NewPriceServiceWithURL(srv.URL)
	_, err := ps.GetPrices(context.Background())
	if err != nil {
		t.Fatalf("pre-fill prices error = %v", err)
	}

	database := setupDashboardTestDB(t, true)
	router := setupDashboardRouter(t, database, ps)

	req := httptest.NewRequest("GET", "/api/dashboard/portfolio", nil)
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200. body: %s", w.Code, w.Body.String())
	}

	var resp models.APIResponse
	if err := json.Unmarshal(w.Body.Bytes(), &resp); err != nil {
		t.Fatalf("unmarshal error = %v", err)
	}

	data, ok := resp.Data.(map[string]interface{})
	if !ok {
		t.Fatalf("data is not a map: %T", resp.Data)
	}

	totalUsd, ok := data["totalUsd"].(float64)
	if !ok {
		t.Fatalf("totalUsd not found or not a number")
	}

	// BTC: 1.5 * 100000 = 150000, BSC BNB: 10 * 600 = 6000, BSC USDC: 5000 * 1 = 5000, SOL: 100 * 200 = 20000
	// Total = 181000
	if totalUsd < 180000 || totalUsd > 182000 {
		t.Errorf("totalUsd = %f, want ~181000", totalUsd)
	}

	chains, ok := data["chains"].([]interface{})
	if !ok {
		t.Fatalf("chains not found or not an array")
	}
	if len(chains) != 3 {
		t.Errorf("chains count = %d, want 3", len(chains))
	}

	// Verify lastScan is present.
	if data["lastScan"] == nil {
		t.Error("lastScan should not be nil when scan state exists")
	}
}

func TestGetPortfolio_EmptyBalances(t *testing.T) {
	srv := mockPriceServer(t)
	defer srv.Close()

	ps := price.NewPriceServiceWithURL(srv.URL)
	_, err := ps.GetPrices(context.Background())
	if err != nil {
		t.Fatalf("pre-fill prices error = %v", err)
	}

	database := setupDashboardTestDB(t, false)
	router := setupDashboardRouter(t, database, ps)

	req := httptest.NewRequest("GET", "/api/dashboard/portfolio", nil)
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200. body: %s", w.Code, w.Body.String())
	}

	var resp models.APIResponse
	if err := json.Unmarshal(w.Body.Bytes(), &resp); err != nil {
		t.Fatalf("unmarshal error = %v", err)
	}

	data, ok := resp.Data.(map[string]interface{})
	if !ok {
		t.Fatalf("data is not a map: %T", resp.Data)
	}

	totalUsd, ok := data["totalUsd"].(float64)
	if !ok {
		t.Fatalf("totalUsd not found")
	}
	if totalUsd != 0 {
		t.Errorf("totalUsd = %f, want 0 for empty balances", totalUsd)
	}

	// lastScan should be null with no scan state.
	if data["lastScan"] != nil {
		t.Errorf("lastScan = %v, want nil for no scan state", data["lastScan"])
	}
}
