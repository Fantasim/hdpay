package handlers

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/Fantasim/hdpay/internal/models"
	"github.com/go-chi/chi/v5"
)

func setupSettingsRouter(t *testing.T) (http.Handler, func()) {
	t.Helper()
	database := setupTestDB(t)

	r := chi.NewRouter()
	r.Get("/api/settings", GetSettings(database))
	r.Put("/api/settings", UpdateSettings(database))
	r.Post("/api/settings/reset-balances", ResetBalancesHandler(database))
	r.Post("/api/settings/reset-all", ResetAllHandler(database))

	return r, func() { database.Close() }
}

func TestGetSettings(t *testing.T) {
	router, cleanup := setupSettingsRouter(t)
	defer cleanup()

	req := httptest.NewRequest("GET", "/api/settings", nil)
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200", w.Code)
	}

	var resp models.APIResponse
	if err := json.Unmarshal(w.Body.Bytes(), &resp); err != nil {
		t.Fatalf("unmarshal error: %v", err)
	}

	data, ok := resp.Data.(map[string]interface{})
	if !ok {
		t.Fatalf("data is not a map")
	}

	// Should have all default keys.
	expectedKeys := []string{"max_scan_id", "auto_resume_scans", "resume_threshold_hours", "btc_fee_rate", "bsc_gas_preseed_bnb", "log_level"}
	for _, key := range expectedKeys {
		if _, ok := data[key]; !ok {
			t.Errorf("missing key %q in settings response", key)
		}
	}
}

func TestUpdateSettings(t *testing.T) {
	router, cleanup := setupSettingsRouter(t)
	defer cleanup()

	body := `{"max_scan_id": "10000", "log_level": "debug"}`
	req := httptest.NewRequest("PUT", "/api/settings", strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200, body = %s", w.Code, w.Body.String())
	}

	var resp models.APIResponse
	if err := json.Unmarshal(w.Body.Bytes(), &resp); err != nil {
		t.Fatalf("unmarshal error: %v", err)
	}

	data, ok := resp.Data.(map[string]interface{})
	if !ok {
		t.Fatalf("data is not a map")
	}

	if data["max_scan_id"] != "10000" {
		t.Errorf("max_scan_id = %v, want %q", data["max_scan_id"], "10000")
	}
	if data["log_level"] != "debug" {
		t.Errorf("log_level = %v, want %q", data["log_level"], "debug")
	}
}

func TestUpdateSettings_InvalidKey(t *testing.T) {
	router, cleanup := setupSettingsRouter(t)
	defer cleanup()

	body := `{"unknown_key": "value"}`
	req := httptest.NewRequest("PUT", "/api/settings", strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	if w.Code != http.StatusBadRequest {
		t.Fatalf("status = %d, want 400", w.Code)
	}
}

func TestUpdateSettings_InvalidBody(t *testing.T) {
	router, cleanup := setupSettingsRouter(t)
	defer cleanup()

	req := httptest.NewRequest("PUT", "/api/settings", strings.NewReader("not json"))
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	if w.Code != http.StatusBadRequest {
		t.Fatalf("status = %d, want 400", w.Code)
	}
}

func TestResetBalances_RequiresConfirmation(t *testing.T) {
	router, cleanup := setupSettingsRouter(t)
	defer cleanup()

	// Without confirm.
	req := httptest.NewRequest("POST", "/api/settings/reset-balances", strings.NewReader(`{}`))
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	if w.Code != http.StatusBadRequest {
		t.Fatalf("status = %d, want 400 without confirmation", w.Code)
	}

	// With confirm = false.
	req2 := httptest.NewRequest("POST", "/api/settings/reset-balances", strings.NewReader(`{"confirm": false}`))
	w2 := httptest.NewRecorder()
	router.ServeHTTP(w2, req2)

	if w2.Code != http.StatusBadRequest {
		t.Fatalf("status = %d, want 400 with confirm=false", w2.Code)
	}
}

func TestResetBalances_Success(t *testing.T) {
	router, cleanup := setupSettingsRouter(t)
	defer cleanup()

	req := httptest.NewRequest("POST", "/api/settings/reset-balances", strings.NewReader(`{"confirm": true}`))
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200, body = %s", w.Code, w.Body.String())
	}
}

func TestResetAll_RequiresConfirmation(t *testing.T) {
	router, cleanup := setupSettingsRouter(t)
	defer cleanup()

	req := httptest.NewRequest("POST", "/api/settings/reset-all", strings.NewReader(`{}`))
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	if w.Code != http.StatusBadRequest {
		t.Fatalf("status = %d, want 400 without confirmation", w.Code)
	}
}

func TestResetAll_Success(t *testing.T) {
	router, cleanup := setupSettingsRouter(t)
	defer cleanup()

	req := httptest.NewRequest("POST", "/api/settings/reset-all", strings.NewReader(`{"confirm": true}`))
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200, body = %s", w.Code, w.Body.String())
	}
}
