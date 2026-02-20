package handlers_test

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"testing"

	pollerapi "github.com/Fantasim/hdpay/internal/poller/api"
	pollermw "github.com/Fantasim/hdpay/internal/poller/api/middleware"
	pollerconfig "github.com/Fantasim/hdpay/internal/poller/config"
	"github.com/Fantasim/hdpay/internal/poller/models"
	"github.com/Fantasim/hdpay/internal/poller/points"
	"github.com/Fantasim/hdpay/internal/poller/pollerdb"
	"github.com/Fantasim/hdpay/internal/poller/watcher"
)

// testDeps holds the shared test dependencies.
type testDeps struct {
	db        *pollerdb.DB
	sessions  *pollermw.SessionStore
	allowlist *pollermw.IPAllowlist
	calc      *points.PointsCalculator
	cfg       *pollerconfig.Config
	router    http.Handler
}

func setupTestDeps(t *testing.T) *testDeps {
	t.Helper()

	// Create temp DB.
	tmpDB := t.TempDir() + "/test.sqlite"
	db, err := pollerdb.New(tmpDB)
	if err != nil {
		t.Fatalf("failed to create test DB: %v", err)
	}
	if err := db.RunMigrations(); err != nil {
		t.Fatalf("failed to run migrations: %v", err)
	}

	t.Cleanup(func() { db.Close() })

	// Create tiers file.
	tiersFile := t.TempDir() + "/tiers.json"
	tiers, err := points.LoadOrCreateTiers(tiersFile)
	if err != nil {
		t.Fatalf("failed to create tiers: %v", err)
	}
	calc := points.NewPointsCalculator(tiers)

	cfg := &pollerconfig.Config{
		DBPath:              tmpDB,
		Port:                8081,
		LogLevel:            "error",
		LogDir:              os.TempDir(),
		Network:             "testnet",
		StartDate:           1700000000,
		AdminUsername:       "admin",
		AdminPassword:       "testpass",
		MaxActiveWatches:    10,
		DefaultWatchTimeout: 30,
		TiersFile:           tiersFile,
	}

	allowlist := pollermw.NewIPAllowlist(map[string]bool{})
	sessions, err := pollermw.NewSessionStore("admin", "testpass")
	if err != nil {
		t.Fatalf("failed to create session store: %v", err)
	}

	w := watcher.NewWatcher(db, nil, nil, calc, cfg)

	deps := &pollerapi.Dependencies{
		DB:         db,
		Watcher:    w,
		Calculator: calc,
		Allowlist:  allowlist,
		Sessions:   sessions,
		Config:     cfg,
	}
	router := pollerapi.NewRouter(deps)

	return &testDeps{
		db:        db,
		sessions:  sessions,
		allowlist: allowlist,
		calc:      calc,
		cfg:       cfg,
		router:    router,
	}
}

// loginAndGetCookie logs in and returns the session cookie.
func loginAndGetCookie(t *testing.T, td *testDeps) *http.Cookie {
	t.Helper()

	body := `{"username":"admin","password":"testpass"}`
	req := httptest.NewRequest("POST", "/api/admin/login", bytes.NewBufferString(body))
	req.Header.Set("Content-Type", "application/json")
	req.RemoteAddr = "127.0.0.1:12345"
	rec := httptest.NewRecorder()

	td.router.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("login failed: status %d, body: %s", rec.Code, rec.Body.String())
	}

	cookies := rec.Result().Cookies()
	for _, c := range cookies {
		if c.Name == pollerconfig.SessionCookieName {
			return c
		}
	}
	t.Fatal("no session cookie returned")
	return nil
}

func TestHealthNoAuth(t *testing.T) {
	td := setupTestDeps(t)

	req := httptest.NewRequest("GET", "/api/health", nil)
	req.RemoteAddr = "8.8.8.8:12345" // Non-local IP — should still work
	rec := httptest.NewRecorder()

	td.router.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Errorf("expected 200, got %d", rec.Code)
	}

	var resp map[string]interface{}
	json.NewDecoder(rec.Body).Decode(&resp)
	data, ok := resp["data"].(map[string]interface{})
	if !ok {
		t.Fatal("expected data object in response")
	}
	if data["status"] != "ok" {
		t.Errorf("expected status ok, got %v", data["status"])
	}
}

func TestLoginSuccess(t *testing.T) {
	td := setupTestDeps(t)

	body := `{"username":"admin","password":"testpass"}`
	req := httptest.NewRequest("POST", "/api/admin/login", bytes.NewBufferString(body))
	req.Header.Set("Content-Type", "application/json")
	req.RemoteAddr = "8.8.8.8:12345" // Login exempt from IP check
	rec := httptest.NewRecorder()

	td.router.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Errorf("expected 200, got %d: %s", rec.Code, rec.Body.String())
	}

	// Check cookie was set.
	cookies := rec.Result().Cookies()
	found := false
	for _, c := range cookies {
		if c.Name == pollerconfig.SessionCookieName {
			found = true
			if !c.HttpOnly {
				t.Error("cookie should be HttpOnly")
			}
		}
	}
	if !found {
		t.Error("expected session cookie to be set")
	}
}

func TestLoginWrongCredentials(t *testing.T) {
	td := setupTestDeps(t)

	body := `{"username":"admin","password":"wrongpass"}`
	req := httptest.NewRequest("POST", "/api/admin/login", bytes.NewBufferString(body))
	req.Header.Set("Content-Type", "application/json")
	req.RemoteAddr = "127.0.0.1:12345"
	rec := httptest.NewRecorder()

	td.router.ServeHTTP(rec, req)

	if rec.Code != http.StatusUnauthorized {
		t.Errorf("expected 401, got %d", rec.Code)
	}
}

func TestLogout(t *testing.T) {
	td := setupTestDeps(t)
	cookie := loginAndGetCookie(t, td)

	// Logout.
	req := httptest.NewRequest("POST", "/api/admin/logout", nil)
	req.RemoteAddr = "127.0.0.1:12345"
	req.AddCookie(cookie)
	rec := httptest.NewRecorder()

	td.router.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Errorf("expected 200, got %d", rec.Code)
	}

	// Verify session is invalidated — try accessing admin endpoint.
	req2 := httptest.NewRequest("GET", "/api/admin/settings", nil)
	req2.RemoteAddr = "127.0.0.1:12345"
	req2.AddCookie(cookie)
	rec2 := httptest.NewRecorder()

	td.router.ServeHTTP(rec2, req2)

	if rec2.Code != http.StatusUnauthorized {
		t.Errorf("expected 401 after logout, got %d", rec2.Code)
	}
}

func TestIPRestriction(t *testing.T) {
	td := setupTestDeps(t)

	// Non-local IP should be blocked on /api/watches.
	req := httptest.NewRequest("GET", "/api/watches", nil)
	req.RemoteAddr = "8.8.8.8:12345"
	rec := httptest.NewRecorder()

	td.router.ServeHTTP(rec, req)

	if rec.Code != http.StatusForbidden {
		t.Errorf("expected 403, got %d", rec.Code)
	}

	// Same request from localhost should work.
	req2 := httptest.NewRequest("GET", "/api/watches", nil)
	req2.RemoteAddr = "127.0.0.1:12345"
	rec2 := httptest.NewRecorder()

	td.router.ServeHTTP(rec2, req2)

	if rec2.Code != http.StatusOK {
		t.Errorf("expected 200 from localhost, got %d", rec2.Code)
	}
}

func TestSessionRequired(t *testing.T) {
	td := setupTestDeps(t)

	// Admin endpoint without session → 401.
	req := httptest.NewRequest("GET", "/api/admin/settings", nil)
	req.RemoteAddr = "127.0.0.1:12345"
	rec := httptest.NewRecorder()

	td.router.ServeHTTP(rec, req)

	if rec.Code != http.StatusUnauthorized {
		t.Errorf("expected 401 without session, got %d", rec.Code)
	}

	// Dashboard endpoint without session → 401.
	req2 := httptest.NewRequest("GET", "/api/dashboard/stats", nil)
	req2.RemoteAddr = "127.0.0.1:12345"
	rec2 := httptest.NewRecorder()

	td.router.ServeHTTP(rec2, req2)

	if rec2.Code != http.StatusUnauthorized {
		t.Errorf("expected 401 without session for dashboard, got %d", rec2.Code)
	}
}

func TestGetPoints_Empty(t *testing.T) {
	td := setupTestDeps(t)

	req := httptest.NewRequest("GET", "/api/points", nil)
	req.RemoteAddr = "127.0.0.1:12345"
	rec := httptest.NewRecorder()

	td.router.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Errorf("expected 200, got %d", rec.Code)
	}

	var resp map[string]interface{}
	json.NewDecoder(rec.Body).Decode(&resp)
	data, ok := resp["data"].([]interface{})
	if !ok {
		t.Fatal("expected data array")
	}
	if len(data) != 0 {
		t.Errorf("expected empty array, got %d items", len(data))
	}
}

func TestClaimPoints_EmptyAddresses(t *testing.T) {
	td := setupTestDeps(t)

	body := `{"addresses":[]}`
	req := httptest.NewRequest("POST", "/api/points/claim", bytes.NewBufferString(body))
	req.Header.Set("Content-Type", "application/json")
	req.RemoteAddr = "127.0.0.1:12345"
	rec := httptest.NewRecorder()

	td.router.ServeHTTP(rec, req)

	if rec.Code != http.StatusBadRequest {
		t.Errorf("expected 400, got %d", rec.Code)
	}
}

func TestClaimPoints_UnknownAddress(t *testing.T) {
	td := setupTestDeps(t)

	body := `{"addresses":["tb1qunknownaddress123"]}`
	req := httptest.NewRequest("POST", "/api/points/claim", bytes.NewBufferString(body))
	req.Header.Set("Content-Type", "application/json")
	req.RemoteAddr = "127.0.0.1:12345"
	rec := httptest.NewRecorder()

	td.router.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Errorf("expected 200 (skip silently), got %d: %s", rec.Code, rec.Body.String())
	}

	var resp map[string]interface{}
	json.NewDecoder(rec.Body).Decode(&resp)
	data := resp["data"].(map[string]interface{})
	skipped := data["skipped"].([]interface{})
	if len(skipped) != 1 {
		t.Errorf("expected 1 skipped, got %d", len(skipped))
	}
	total := data["total_claimed"].(float64)
	if total != 0 {
		t.Errorf("expected 0 total claimed, got %v", total)
	}
}

func TestClaimPoints_WithUnclaimed(t *testing.T) {
	td := setupTestDeps(t)

	// Seed points data.
	_, err := td.db.GetOrCreatePoints("tb1qtest123", "BTC")
	if err != nil {
		t.Fatalf("failed to create points: %v", err)
	}
	if err := td.db.AddUnclaimed("tb1qtest123", "BTC", 5000); err != nil {
		t.Fatalf("failed to add unclaimed: %v", err)
	}

	body := `{"addresses":["tb1qtest123"]}`
	req := httptest.NewRequest("POST", "/api/points/claim", bytes.NewBufferString(body))
	req.Header.Set("Content-Type", "application/json")
	req.RemoteAddr = "127.0.0.1:12345"
	rec := httptest.NewRecorder()

	td.router.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Errorf("expected 200, got %d: %s", rec.Code, rec.Body.String())
	}

	var resp map[string]interface{}
	json.NewDecoder(rec.Body).Decode(&resp)
	data := resp["data"].(map[string]interface{})
	total := data["total_claimed"].(float64)
	if total != 5000 {
		t.Errorf("expected 5000 total claimed, got %v", total)
	}

	// Verify points are reset.
	acct, err := td.db.GetOrCreatePoints("tb1qtest123", "BTC")
	if err != nil {
		t.Fatalf("failed to get points: %v", err)
	}
	if acct.Unclaimed != 0 {
		t.Errorf("expected unclaimed 0 after claim, got %d", acct.Unclaimed)
	}
}

func TestListWatches_Empty(t *testing.T) {
	td := setupTestDeps(t)

	req := httptest.NewRequest("GET", "/api/watches", nil)
	req.RemoteAddr = "127.0.0.1:12345"
	rec := httptest.NewRecorder()

	td.router.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Errorf("expected 200, got %d", rec.Code)
	}
}

func TestDashboardStats_NoAuth(t *testing.T) {
	td := setupTestDeps(t)

	// Without session → 401.
	req := httptest.NewRequest("GET", "/api/dashboard/stats", nil)
	req.RemoteAddr = "127.0.0.1:12345"
	rec := httptest.NewRecorder()

	td.router.ServeHTTP(rec, req)

	if rec.Code != http.StatusUnauthorized {
		t.Errorf("expected 401, got %d", rec.Code)
	}
}

func TestDashboardStats_WithAuth(t *testing.T) {
	td := setupTestDeps(t)
	cookie := loginAndGetCookie(t, td)

	req := httptest.NewRequest("GET", "/api/dashboard/stats?range=all", nil)
	req.RemoteAddr = "127.0.0.1:12345"
	req.AddCookie(cookie)
	rec := httptest.NewRecorder()

	td.router.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Errorf("expected 200, got %d: %s", rec.Code, rec.Body.String())
	}

	var resp map[string]interface{}
	json.NewDecoder(rec.Body).Decode(&resp)
	data := resp["data"].(map[string]interface{})
	if data["range"] != "all" {
		t.Errorf("expected range 'all', got %v", data["range"])
	}
}

func TestDashboardErrors(t *testing.T) {
	td := setupTestDeps(t)
	cookie := loginAndGetCookie(t, td)

	req := httptest.NewRequest("GET", "/api/dashboard/errors", nil)
	req.RemoteAddr = "127.0.0.1:12345"
	req.AddCookie(cookie)
	rec := httptest.NewRecorder()

	td.router.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Errorf("expected 200, got %d: %s", rec.Code, rec.Body.String())
	}

	var resp map[string]interface{}
	json.NewDecoder(rec.Body).Decode(&resp)
	data := resp["data"].(map[string]interface{})

	// All checks should return empty arrays.
	for _, key := range []string{"discrepancies", "errors", "stale_pending"} {
		arr, ok := data[key].([]interface{})
		if !ok {
			t.Errorf("expected %s to be array", key)
			continue
		}
		if len(arr) != 0 {
			t.Errorf("expected %s to be empty, got %d items", key, len(arr))
		}
	}
}

func TestDashboardCharts(t *testing.T) {
	td := setupTestDeps(t)
	cookie := loginAndGetCookie(t, td)

	req := httptest.NewRequest("GET", "/api/dashboard/charts?range=all", nil)
	req.RemoteAddr = "127.0.0.1:12345"
	req.AddCookie(cookie)
	rec := httptest.NewRecorder()

	td.router.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Errorf("expected 200, got %d: %s", rec.Code, rec.Body.String())
	}

	var resp map[string]interface{}
	json.NewDecoder(rec.Body).Decode(&resp)
	data := resp["data"].(map[string]interface{})

	// Verify all 7 datasets exist.
	for _, key := range []string{"usd_over_time", "points_over_time", "tx_count_over_time", "by_chain", "by_token", "by_tier", "watches_over_time"} {
		if _, ok := data[key]; !ok {
			t.Errorf("expected %s in chart data", key)
		}
	}
}

func TestAdminAllowlist_CRUD(t *testing.T) {
	td := setupTestDeps(t)
	cookie := loginAndGetCookie(t, td)

	// GET empty allowlist.
	req := httptest.NewRequest("GET", "/api/admin/allowlist", nil)
	req.RemoteAddr = "127.0.0.1:12345"
	req.AddCookie(cookie)
	rec := httptest.NewRecorder()

	td.router.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Errorf("expected 200, got %d", rec.Code)
	}

	// ADD an IP.
	body := `{"ip":"8.8.8.8","description":"test dns"}`
	req = httptest.NewRequest("POST", "/api/admin/allowlist", bytes.NewBufferString(body))
	req.Header.Set("Content-Type", "application/json")
	req.RemoteAddr = "127.0.0.1:12345"
	req.AddCookie(cookie)
	rec = httptest.NewRecorder()

	td.router.ServeHTTP(rec, req)

	if rec.Code != http.StatusCreated {
		t.Errorf("expected 201, got %d: %s", rec.Code, rec.Body.String())
	}

	// Verify the IP is now allowed.
	if !td.allowlist.IsAllowed("8.8.8.8") {
		t.Error("8.8.8.8 should be allowed after adding to allowlist")
	}

	// DELETE the IP.
	req = httptest.NewRequest("DELETE", "/api/admin/allowlist/1", nil)
	req.RemoteAddr = "127.0.0.1:12345"
	req.AddCookie(cookie)
	rec = httptest.NewRecorder()

	td.router.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Errorf("expected 200, got %d: %s", rec.Code, rec.Body.String())
	}
}

func TestAdminSettings(t *testing.T) {
	td := setupTestDeps(t)
	cookie := loginAndGetCookie(t, td)

	req := httptest.NewRequest("GET", "/api/admin/settings", nil)
	req.RemoteAddr = "127.0.0.1:12345"
	req.AddCookie(cookie)
	rec := httptest.NewRecorder()

	td.router.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Errorf("expected 200, got %d: %s", rec.Code, rec.Body.String())
	}

	var resp map[string]interface{}
	json.NewDecoder(rec.Body).Decode(&resp)
	data := resp["data"].(map[string]interface{})

	if data["network"] != "testnet" {
		t.Errorf("expected network testnet, got %v", data["network"])
	}
}

func TestAdminUpdateTiers(t *testing.T) {
	td := setupTestDeps(t)
	cookie := loginAndGetCookie(t, td)

	// Valid update: 2 tiers.
	tiers := []models.Tier{
		{MinUSD: 0, MaxUSD: ptrFloat(10), Multiplier: 1.0},
		{MinUSD: 10, MaxUSD: nil, Multiplier: 2.0},
	}
	body, _ := json.Marshal(tiers)

	req := httptest.NewRequest("PUT", "/api/admin/tiers", bytes.NewBuffer(body))
	req.Header.Set("Content-Type", "application/json")
	req.RemoteAddr = "127.0.0.1:12345"
	req.AddCookie(cookie)
	rec := httptest.NewRecorder()

	td.router.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Errorf("expected 200, got %d: %s", rec.Code, rec.Body.String())
	}

	// Verify calculator reloaded.
	loaded := td.calc.Tiers()
	if len(loaded) != 2 {
		t.Errorf("expected 2 tiers after reload, got %d", len(loaded))
	}
}

func TestAdminUpdateTiers_Invalid(t *testing.T) {
	td := setupTestDeps(t)
	cookie := loginAndGetCookie(t, td)

	// Only 1 tier — should fail validation.
	body := `[{"min_usd":0,"max_usd":null,"multiplier":1.0}]`
	req := httptest.NewRequest("PUT", "/api/admin/tiers", bytes.NewBufferString(body))
	req.Header.Set("Content-Type", "application/json")
	req.RemoteAddr = "127.0.0.1:12345"
	req.AddCookie(cookie)
	rec := httptest.NewRecorder()

	td.router.ServeHTTP(rec, req)

	if rec.Code != http.StatusBadRequest {
		t.Errorf("expected 400, got %d: %s", rec.Code, rec.Body.String())
	}
}

func TestDashboardTransactions_Paginated(t *testing.T) {
	td := setupTestDeps(t)
	cookie := loginAndGetCookie(t, td)

	req := httptest.NewRequest("GET", "/api/dashboard/transactions?page=1&page_size=25", nil)
	req.RemoteAddr = "127.0.0.1:12345"
	req.AddCookie(cookie)
	rec := httptest.NewRecorder()

	td.router.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Errorf("expected 200, got %d: %s", rec.Code, rec.Body.String())
	}

	var resp map[string]interface{}
	json.NewDecoder(rec.Body).Decode(&resp)
	meta := resp["meta"].(map[string]interface{})
	if int(meta["page"].(float64)) != 1 {
		t.Errorf("expected page 1, got %v", meta["page"])
	}
	if int(meta["page_size"].(float64)) != 25 {
		t.Errorf("expected page_size 25, got %v", meta["page_size"])
	}
}

func TestCORSPreflight(t *testing.T) {
	td := setupTestDeps(t)

	req := httptest.NewRequest("OPTIONS", "/api/health", nil)
	req.RemoteAddr = "127.0.0.1:12345"
	rec := httptest.NewRecorder()

	td.router.ServeHTTP(rec, req)

	if rec.Header().Get("Access-Control-Allow-Origin") != "*" {
		t.Error("expected CORS header Access-Control-Allow-Origin: *")
	}
}

func ptrFloat(f float64) *float64 {
	return &f
}
