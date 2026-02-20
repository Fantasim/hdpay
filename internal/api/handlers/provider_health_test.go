package handlers

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"testing"

	"github.com/Fantasim/hdpay/internal/db"
	"github.com/go-chi/chi/v5"
)

func setupProviderHealthDB(t *testing.T) *db.DB {
	t.Helper()
	tmpDir := t.TempDir()
	dbPath := filepath.Join(tmpDir, "test_health.sqlite")

	database, err := db.New(dbPath, "testnet")
	if err != nil {
		t.Fatalf("New() error = %v", err)
	}
	if err := database.RunMigrations(); err != nil {
		t.Fatalf("RunMigrations() error = %v", err)
	}

	t.Cleanup(func() { database.Close() })
	return database
}

func setupProviderHealthRouter(database *db.DB) http.Handler {
	r := chi.NewRouter()
	r.Get("/api/health/providers", GetProviderHealth(database))
	return r
}

func TestGetProviderHealth_EmptyDB(t *testing.T) {
	database := setupProviderHealthDB(t)
	router := setupProviderHealthRouter(database)

	req := httptest.NewRequest("GET", "/api/health/providers", nil)
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200. body: %s", w.Code, w.Body.String())
	}

	var resp map[string]interface{}
	if err := json.Unmarshal(w.Body.Bytes(), &resp); err != nil {
		t.Fatalf("failed to parse response: %v", err)
	}

	data, ok := resp["data"].(map[string]interface{})
	if !ok {
		t.Fatalf("expected data to be a map, got %T", resp["data"])
	}

	// All chains should have empty arrays.
	for _, chain := range []string{"BTC", "BSC", "SOL"} {
		arr, ok := data[chain].([]interface{})
		if !ok {
			t.Errorf("expected %s to be an array, got %T", chain, data[chain])
			continue
		}
		if len(arr) != 0 {
			t.Errorf("expected %s to have 0 providers, got %d", chain, len(arr))
		}
	}
}

func TestGetProviderHealth_WithProviders(t *testing.T) {
	database := setupProviderHealthDB(t)
	router := setupProviderHealthRouter(database)

	// Insert some provider health records.
	if err := database.UpsertProviderHealth(db.ProviderHealthRow{
		ProviderName: "Blockstream", Chain: "BTC", ProviderType: "http",
		Status: "healthy", CircuitState: "closed", ConsecutiveFails: 0,
	}); err != nil {
		t.Fatalf("UpsertProviderHealth() error = %v", err)
	}
	if err := database.UpsertProviderHealth(db.ProviderHealthRow{
		ProviderName: "Mempool", Chain: "BTC", ProviderType: "http",
		Status: "healthy", CircuitState: "closed", ConsecutiveFails: 0,
	}); err != nil {
		t.Fatalf("UpsertProviderHealth() error = %v", err)
	}
	if err := database.UpsertProviderHealth(db.ProviderHealthRow{
		ProviderName: "BscScan", Chain: "BSC", ProviderType: "http",
		Status: "degraded", CircuitState: "half_open", ConsecutiveFails: 3,
	}); err != nil {
		t.Fatalf("UpsertProviderHealth() error = %v", err)
	}

	req := httptest.NewRequest("GET", "/api/health/providers", nil)
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200. body: %s", w.Code, w.Body.String())
	}

	var resp map[string]interface{}
	if err := json.Unmarshal(w.Body.Bytes(), &resp); err != nil {
		t.Fatalf("failed to parse response: %v", err)
	}

	data := resp["data"].(map[string]interface{})

	// BTC should have 2 providers.
	btcProviders := data["BTC"].([]interface{})
	if len(btcProviders) != 2 {
		t.Errorf("BTC providers = %d, want 2", len(btcProviders))
	}

	// BSC should have 1 provider.
	bscProviders := data["BSC"].([]interface{})
	if len(bscProviders) != 1 {
		t.Errorf("BSC providers = %d, want 1", len(bscProviders))
	}

	// SOL should be empty.
	solProviders := data["SOL"].([]interface{})
	if len(solProviders) != 0 {
		t.Errorf("SOL providers = %d, want 0", len(solProviders))
	}

	// Verify BSC provider details.
	bscProvider := bscProviders[0].(map[string]interface{})
	if bscProvider["name"] != "BscScan" {
		t.Errorf("BSC provider name = %v, want BscScan", bscProvider["name"])
	}
	if bscProvider["circuitState"] != "half_open" {
		t.Errorf("BSC provider circuitState = %v, want half_open", bscProvider["circuitState"])
	}
	if bscProvider["consecutiveFails"].(float64) != 3 {
		t.Errorf("BSC provider consecutiveFails = %v, want 3", bscProvider["consecutiveFails"])
	}
}

func TestGetProviderHealth_ContentType(t *testing.T) {
	database := setupProviderHealthDB(t)
	router := setupProviderHealthRouter(database)

	req := httptest.NewRequest("GET", "/api/health/providers", nil)
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	ct := w.Header().Get("Content-Type")
	if ct != "application/json" {
		t.Errorf("Content-Type = %q, want application/json", ct)
	}
}
