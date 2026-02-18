package handlers

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/Fantasim/hdpay/internal/config"
	"github.com/Fantasim/hdpay/internal/db"
	"github.com/Fantasim/hdpay/internal/models"
	"github.com/Fantasim/hdpay/internal/scanner"
	"github.com/go-chi/chi/v5"
)

// setupScanTestDB creates a temporary database for scan handler tests.
func setupScanTestDB(t *testing.T) *db.DB {
	t.Helper()
	tmpDir := t.TempDir()
	dbPath := filepath.Join(tmpDir, "test_scan.sqlite")

	database, err := db.New(dbPath)
	if err != nil {
		t.Fatalf("New() error = %v", err)
	}
	if err := database.RunMigrations(); err != nil {
		t.Fatalf("RunMigrations() error = %v", err)
	}

	// Seed 10 addresses per chain for scanning.
	for _, chain := range models.AllChains {
		addrs := make([]models.Address, 10)
		for i := 0; i < 10; i++ {
			addrs[i] = models.Address{
				Chain:        chain,
				AddressIndex: i,
				Address:      string(chain) + "_addr_" + string(rune('0'+i)),
			}
		}
		if err := database.InsertAddressBatch(chain, addrs); err != nil {
			t.Fatalf("InsertAddressBatch(%s) error = %v", chain, err)
		}
	}

	t.Cleanup(func() { database.Close() })
	return database
}

// setupScanRouter creates a chi router with scan handlers wired to a test scanner.
func setupScanRouter(t *testing.T, sc *scanner.Scanner, hub *scanner.SSEHub, database *db.DB) http.Handler {
	t.Helper()
	r := chi.NewRouter()
	r.Post("/api/scan/start", StartScan(sc))
	r.Post("/api/scan/stop", StopScan(sc))
	r.Get("/api/scan/status", GetScanStatus(sc, database))
	r.Get("/api/scan/sse", ScanSSE(hub))
	return r
}

func TestStartScan_Success(t *testing.T) {
	database := setupScanTestDB(t)
	hub := scanner.NewSSEHub()
	sc := scanner.SetupScannerForTest(database, hub, map[models.Chain]*scanner.Pool{
		models.ChainBTC: scanner.NewPoolForTest(models.ChainBTC),
	})
	router := setupScanRouter(t, sc, hub, database)

	body := `{"chain":"BTC","maxId":10}`
	req := httptest.NewRequest("POST", "/api/scan/start", strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200. body: %s", w.Code, w.Body.String())
	}

	var resp models.APIResponse
	if err := json.Unmarshal(w.Body.Bytes(), &resp); err != nil {
		t.Fatalf("unmarshal error: %v", err)
	}

	data, ok := resp.Data.(map[string]interface{})
	if !ok {
		t.Fatal("data is not a map")
	}
	if data["message"] != "scan started" {
		t.Errorf("message = %v, want 'scan started'", data["message"])
	}

	// Clean up: stop the scan so the goroutine exits.
	sc.StopScan(models.ChainBTC)
	time.Sleep(100 * time.Millisecond)
}

func TestStartScan_InvalidChain(t *testing.T) {
	database := setupScanTestDB(t)
	hub := scanner.NewSSEHub()
	sc := scanner.SetupScannerForTest(database, hub, map[models.Chain]*scanner.Pool{})
	router := setupScanRouter(t, sc, hub, database)

	body := `{"chain":"INVALID","maxId":10}`
	req := httptest.NewRequest("POST", "/api/scan/start", strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	if w.Code != http.StatusBadRequest {
		t.Fatalf("status = %d, want 400", w.Code)
	}

	var resp models.APIError
	if err := json.Unmarshal(w.Body.Bytes(), &resp); err != nil {
		t.Fatalf("unmarshal error: %v", err)
	}
	if resp.Error.Code != config.ErrorInvalidChain {
		t.Errorf("error code = %q, want %q", resp.Error.Code, config.ErrorInvalidChain)
	}
}

func TestStartScan_InvalidMaxId(t *testing.T) {
	database := setupScanTestDB(t)
	hub := scanner.NewSSEHub()
	sc := scanner.SetupScannerForTest(database, hub, map[models.Chain]*scanner.Pool{})
	router := setupScanRouter(t, sc, hub, database)

	tests := []struct {
		name string
		body string
	}{
		{"zero", `{"chain":"BTC","maxId":0}`},
		{"negative", `{"chain":"BTC","maxId":-5}`},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			req := httptest.NewRequest("POST", "/api/scan/start", strings.NewReader(tt.body))
			req.Header.Set("Content-Type", "application/json")
			w := httptest.NewRecorder()
			router.ServeHTTP(w, req)

			if w.Code != http.StatusBadRequest {
				t.Fatalf("status = %d, want 400", w.Code)
			}
		})
	}
}

func TestStartScan_AlreadyRunning(t *testing.T) {
	database := setupScanTestDB(t)
	hub := scanner.NewSSEHub()
	sc := scanner.SetupScannerForTest(database, hub, map[models.Chain]*scanner.Pool{
		models.ChainBTC: scanner.NewPoolForTest(models.ChainBTC),
	})
	router := setupScanRouter(t, sc, hub, database)

	// Start first scan.
	body := `{"chain":"BTC","maxId":10}`
	req := httptest.NewRequest("POST", "/api/scan/start", strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("first start: status = %d, want 200", w.Code)
	}

	// Try to start again — should get 409.
	req2 := httptest.NewRequest("POST", "/api/scan/start", strings.NewReader(body))
	req2.Header.Set("Content-Type", "application/json")
	w2 := httptest.NewRecorder()
	router.ServeHTTP(w2, req2)

	if w2.Code != http.StatusConflict {
		t.Fatalf("second start: status = %d, want 409", w2.Code)
	}

	sc.StopScan(models.ChainBTC)
	time.Sleep(100 * time.Millisecond)
}

func TestStopScan_Success(t *testing.T) {
	database := setupScanTestDB(t)
	hub := scanner.NewSSEHub()
	sc := scanner.SetupScannerForTest(database, hub, map[models.Chain]*scanner.Pool{
		models.ChainBTC: scanner.NewPoolForTest(models.ChainBTC),
	})
	router := setupScanRouter(t, sc, hub, database)

	body := `{"chain":"BTC"}`
	req := httptest.NewRequest("POST", "/api/scan/stop", strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200", w.Code)
	}
}

func TestStopScan_InvalidChain(t *testing.T) {
	database := setupScanTestDB(t)
	hub := scanner.NewSSEHub()
	sc := scanner.SetupScannerForTest(database, hub, map[models.Chain]*scanner.Pool{})
	router := setupScanRouter(t, sc, hub, database)

	body := `{"chain":"NOPE"}`
	req := httptest.NewRequest("POST", "/api/scan/stop", strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	if w.Code != http.StatusBadRequest {
		t.Fatalf("status = %d, want 400", w.Code)
	}
}

func TestGetScanStatus_AllChains(t *testing.T) {
	database := setupScanTestDB(t)
	hub := scanner.NewSSEHub()
	sc := scanner.SetupScannerForTest(database, hub, map[models.Chain]*scanner.Pool{})
	router := setupScanRouter(t, sc, hub, database)

	req := httptest.NewRequest("GET", "/api/scan/status", nil)
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
		t.Fatal("data is not a map")
	}

	// Should have all 3 chains.
	for _, chain := range []string{"BTC", "BSC", "SOL"} {
		if _, exists := data[chain]; !exists {
			t.Errorf("missing chain %s in status response", chain)
		}
	}
}

func TestGetScanStatus_SingleChain(t *testing.T) {
	database := setupScanTestDB(t)
	hub := scanner.NewSSEHub()
	sc := scanner.SetupScannerForTest(database, hub, map[models.Chain]*scanner.Pool{})
	router := setupScanRouter(t, sc, hub, database)

	req := httptest.NewRequest("GET", "/api/scan/status?chain=BTC", nil)
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
		t.Fatal("data is not a map")
	}

	if data["chain"] != "BTC" {
		t.Errorf("chain = %v, want BTC", data["chain"])
	}
	if data["status"] != "idle" {
		t.Errorf("status = %v, want idle", data["status"])
	}
}

func TestGetScanStatus_InvalidChain(t *testing.T) {
	database := setupScanTestDB(t)
	hub := scanner.NewSSEHub()
	sc := scanner.SetupScannerForTest(database, hub, map[models.Chain]*scanner.Pool{})
	router := setupScanRouter(t, sc, hub, database)

	req := httptest.NewRequest("GET", "/api/scan/status?chain=INVALID", nil)
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	if w.Code != http.StatusBadRequest {
		t.Fatalf("status = %d, want 400", w.Code)
	}
}

func TestScanSSE_Connect(t *testing.T) {
	hub := scanner.NewSSEHub()
	ctx, cancel := context.WithCancel(context.Background())
	go hub.Run(ctx)
	defer cancel()

	database := setupScanTestDB(t)
	sc := scanner.SetupScannerForTest(database, hub, map[models.Chain]*scanner.Pool{})
	router := setupScanRouter(t, sc, hub, database)

	// Create a context that cancels quickly so the SSE handler exits.
	reqCtx, reqCancel := context.WithTimeout(context.Background(), 200*time.Millisecond)
	defer reqCancel()

	req := httptest.NewRequest("GET", "/api/scan/sse", nil).WithContext(reqCtx)
	w := httptest.NewRecorder()

	// Run handler in goroutine since it blocks.
	done := make(chan struct{})
	go func() {
		router.ServeHTTP(w, req)
		close(done)
	}()

	// Broadcast an event while client is connected.
	time.Sleep(50 * time.Millisecond)
	hub.Broadcast(scanner.Event{
		Type: "scan_progress",
		Data: scanner.ScanProgressData{
			Chain:   "BTC",
			Scanned: 100,
			Total:   1000,
			Found:   2,
			Elapsed: "5s",
		},
	})

	<-done

	// Verify SSE headers.
	contentType := w.Header().Get("Content-Type")
	if contentType != "text/event-stream" {
		t.Errorf("content-type = %q, want text/event-stream", contentType)
	}

	cacheControl := w.Header().Get("Cache-Control")
	if cacheControl != "no-cache" {
		t.Errorf("cache-control = %q, want no-cache", cacheControl)
	}

	// Verify the event appears in the body.
	body := w.Body.String()
	if !strings.Contains(body, "event: scan_progress") {
		t.Error("response body missing 'event: scan_progress'")
	}
	if !strings.Contains(body, `"scanned":100`) {
		t.Error("response body missing scanned:100 in data")
	}
}

func TestStartScan_InvalidBody(t *testing.T) {
	database := setupScanTestDB(t)
	hub := scanner.NewSSEHub()
	sc := scanner.SetupScannerForTest(database, hub, map[models.Chain]*scanner.Pool{})
	router := setupScanRouter(t, sc, hub, database)

	req := httptest.NewRequest("POST", "/api/scan/start", bytes.NewReader([]byte("not json")))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	if w.Code != http.StatusBadRequest {
		t.Fatalf("status = %d, want 400", w.Code)
	}
}
