package handlers

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"testing"

	"github.com/Fantasim/hdpay/internal/db"
	"github.com/Fantasim/hdpay/internal/models"
	"github.com/go-chi/chi/v5"
)

func setupTestDB(t *testing.T) *db.DB {
	t.Helper()
	tmpDir := t.TempDir()
	dbPath := filepath.Join(tmpDir, "test.sqlite")

	database, err := db.New(dbPath)
	if err != nil {
		t.Fatalf("New() error = %v", err)
	}
	if err := database.RunMigrations(); err != nil {
		t.Fatalf("RunMigrations() error = %v", err)
	}

	// Seed 25 BTC addresses
	addrs := make([]models.Address, 25)
	for i := 0; i < 25; i++ {
		addrs[i] = models.Address{
			Chain:        models.ChainBTC,
			AddressIndex: i,
			Address:      "bc1qtest" + string(rune('a'+i)),
		}
	}
	if err := database.InsertAddressBatch(models.ChainBTC, addrs); err != nil {
		t.Fatalf("InsertAddressBatch() error = %v", err)
	}

	t.Cleanup(func() { database.Close() })
	return database
}

func setupRouter(database *db.DB) http.Handler {
	r := chi.NewRouter()
	r.Get("/api/addresses/{chain}", ListAddresses(database))
	r.Get("/api/addresses/{chain}/export", ExportAddresses(database))
	return r
}

func TestListAddresses_BasicPagination(t *testing.T) {
	database := setupTestDB(t)
	router := setupRouter(database)

	req := httptest.NewRequest("GET", "/api/addresses/BTC?page=1&pageSize=10", nil)
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200", w.Code)
	}

	var resp models.APIResponse
	if err := json.Unmarshal(w.Body.Bytes(), &resp); err != nil {
		t.Fatalf("unmarshal error: %v", err)
	}

	if resp.Meta == nil {
		t.Fatal("meta is nil")
	}
	if resp.Meta.Total != 25 {
		t.Errorf("total = %d, want 25", resp.Meta.Total)
	}
	if resp.Meta.Page != 1 {
		t.Errorf("page = %d, want 1", resp.Meta.Page)
	}
	if resp.Meta.PageSize != 10 {
		t.Errorf("pageSize = %d, want 10", resp.Meta.PageSize)
	}

	data, ok := resp.Data.([]interface{})
	if !ok {
		t.Fatalf("data is not a slice")
	}
	if len(data) != 10 {
		t.Errorf("len(data) = %d, want 10", len(data))
	}
}

func TestListAddresses_InvalidChain(t *testing.T) {
	database := setupTestDB(t)
	router := setupRouter(database)

	req := httptest.NewRequest("GET", "/api/addresses/INVALID", nil)
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	if w.Code != http.StatusBadRequest {
		t.Fatalf("status = %d, want 400", w.Code)
	}

	var resp models.APIError
	if err := json.Unmarshal(w.Body.Bytes(), &resp); err != nil {
		t.Fatalf("unmarshal error: %v", err)
	}

	if resp.Error.Code != "ERROR_INVALID_CHAIN" {
		t.Errorf("error code = %q, want ERROR_INVALID_CHAIN", resp.Error.Code)
	}
}

func TestListAddresses_DefaultPagination(t *testing.T) {
	database := setupTestDB(t)
	router := setupRouter(database)

	req := httptest.NewRequest("GET", "/api/addresses/BTC", nil)
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200", w.Code)
	}

	var resp models.APIResponse
	if err := json.Unmarshal(w.Body.Bytes(), &resp); err != nil {
		t.Fatalf("unmarshal error: %v", err)
	}

	if resp.Meta.Page != 1 {
		t.Errorf("default page = %d, want 1", resp.Meta.Page)
	}
	if resp.Meta.PageSize != 100 {
		t.Errorf("default pageSize = %d, want 100", resp.Meta.PageSize)
	}
}

func TestListAddresses_CaseInsensitiveChain(t *testing.T) {
	database := setupTestDB(t)
	router := setupRouter(database)

	req := httptest.NewRequest("GET", "/api/addresses/btc", nil)
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200 (lowercase chain should work)", w.Code)
	}
}

func TestExportAddresses(t *testing.T) {
	database := setupTestDB(t)
	router := setupRouter(database)

	req := httptest.NewRequest("GET", "/api/addresses/BTC/export", nil)
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200", w.Code)
	}

	contentType := w.Header().Get("Content-Type")
	if contentType != "application/json" {
		t.Errorf("content-type = %q, want application/json", contentType)
	}

	disposition := w.Header().Get("Content-Disposition")
	if disposition != "attachment; filename=BTC-addresses.json" {
		t.Errorf("content-disposition = %q, want attachment", disposition)
	}

	// Parse as JSON array
	var items []models.AddressExportItem
	if err := json.Unmarshal(w.Body.Bytes(), &items); err != nil {
		t.Fatalf("unmarshal error: %v", err)
	}

	if len(items) != 25 {
		t.Errorf("exported %d items, want 25", len(items))
	}
}

func TestExportAddresses_InvalidChain(t *testing.T) {
	database := setupTestDB(t)
	router := setupRouter(database)

	req := httptest.NewRequest("GET", "/api/addresses/INVALID/export", nil)
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	if w.Code != http.StatusBadRequest {
		t.Fatalf("status = %d, want 400", w.Code)
	}
}
