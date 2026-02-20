package handlers

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/Fantasim/hdpay/internal/config"
	"github.com/Fantasim/hdpay/internal/models"
	"github.com/go-chi/chi/v5"
)

func setupSettingsRouter(t *testing.T) (http.Handler, func()) {
	t.Helper()
	database := setupTestDB(t)

	cfg := &config.Config{Network: "testnet"}

	r := chi.NewRouter()
	r.Get("/api/settings", GetSettings(database, cfg))
	r.Put("/api/settings", UpdateSettings(database))
	r.Post("/api/settings/reset-balances", ResetBalancesHandler(database))

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

	// Should have all default keys plus read-only config fields.
	expectedKeys := []string{"max_scan_id", "auto_resume_scans", "resume_threshold_hours", "btc_fee_rate", "bsc_gas_preseed_bnb", "log_level", "network"}
	for _, key := range expectedKeys {
		if _, ok := data[key]; !ok {
			t.Errorf("missing key %q in settings response", key)
		}
	}

	if data["network"] != "testnet" {
		t.Errorf("network = %v, want %q", data["network"], "testnet")
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

// --- validateSettingValue tests ---

func TestValidateSettingValue(t *testing.T) {
	tests := []struct {
		name    string
		key     string
		value   string
		wantErr bool
	}{
		// max_scan_id
		{"max_scan_id valid", "max_scan_id", "5000", false},
		{"max_scan_id min", "max_scan_id", "1", false},
		{"max_scan_id max", "max_scan_id", "500000", false},
		{"max_scan_id zero", "max_scan_id", "0", true},
		{"max_scan_id negative", "max_scan_id", "-1", true},
		{"max_scan_id too large", "max_scan_id", "999999", true},
		{"max_scan_id not a number", "max_scan_id", "abc", true},

		// btc_fee_rate
		{"btc_fee_rate valid", "btc_fee_rate", "10", false},
		{"btc_fee_rate zero", "btc_fee_rate", "0", false},
		{"btc_fee_rate negative", "btc_fee_rate", "-1", true},
		{"btc_fee_rate not a number", "btc_fee_rate", "abc", true},

		// resume_threshold_hours
		{"resume_threshold_hours valid", "resume_threshold_hours", "24", false},
		{"resume_threshold_hours min", "resume_threshold_hours", "1", false},
		{"resume_threshold_hours zero", "resume_threshold_hours", "0", true},
		{"resume_threshold_hours negative", "resume_threshold_hours", "-1", true},
		{"resume_threshold_hours not a number", "resume_threshold_hours", "xyz", true},

		// keys without validation pass through
		{"log_level any value", "log_level", "debug", false},
		{"auto_resume_scans", "auto_resume_scans", "true", false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := validateSettingValue(tt.key, tt.value)
			if (err != nil) != tt.wantErr {
				t.Errorf("validateSettingValue(%q, %q) error = %v, wantErr %v",
					tt.key, tt.value, err, tt.wantErr)
			}
		})
	}
}

func TestUpdateSettings_ValidationRejectsInvalid(t *testing.T) {
	router, cleanup := setupSettingsRouter(t)
	defer cleanup()

	tests := []struct {
		name string
		body string
	}{
		{"max_scan_id zero", `{"max_scan_id": "0"}`},
		{"max_scan_id negative", `{"max_scan_id": "-5"}`},
		{"max_scan_id too large", `{"max_scan_id": "999999"}`},
		{"btc_fee_rate negative", `{"btc_fee_rate": "-1"}`},
		{"resume_threshold_hours zero", `{"resume_threshold_hours": "0"}`},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			req := httptest.NewRequest("PUT", "/api/settings", strings.NewReader(tt.body))
			req.Header.Set("Content-Type", "application/json")
			w := httptest.NewRecorder()
			router.ServeHTTP(w, req)

			if w.Code != http.StatusBadRequest {
				t.Errorf("status = %d, want 400. body: %s", w.Code, w.Body.String())
			}
		})
	}
}
