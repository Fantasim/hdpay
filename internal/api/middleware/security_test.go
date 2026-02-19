package middleware

import (
	"net/http"
	"net/http/httptest"
	"testing"
)

// okHandler is a simple handler that returns 200 OK for testing middleware.
var okHandler = http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
	w.WriteHeader(http.StatusOK)
})

// --- HostCheck Tests ---

func TestHostCheck_AllowLocalhost(t *testing.T) {
	handler := HostCheck(okHandler)

	req := httptest.NewRequest(http.MethodGet, "/", nil)
	req.Host = "localhost"
	rec := httptest.NewRecorder()

	handler.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Errorf("expected 200 for localhost, got %d", rec.Code)
	}
}

func TestHostCheck_Allow127(t *testing.T) {
	handler := HostCheck(okHandler)

	req := httptest.NewRequest(http.MethodGet, "/", nil)
	req.Host = "127.0.0.1"
	rec := httptest.NewRecorder()

	handler.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Errorf("expected 200 for 127.0.0.1, got %d", rec.Code)
	}
}

func TestHostCheck_AllowLocalhostWithPort(t *testing.T) {
	handler := HostCheck(okHandler)

	req := httptest.NewRequest(http.MethodGet, "/", nil)
	req.Host = "localhost:8080"
	rec := httptest.NewRecorder()

	handler.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Errorf("expected 200 for localhost:8080, got %d", rec.Code)
	}
}

func TestHostCheck_Allow127WithPort(t *testing.T) {
	handler := HostCheck(okHandler)

	req := httptest.NewRequest(http.MethodGet, "/", nil)
	req.Host = "127.0.0.1:8080"
	rec := httptest.NewRecorder()

	handler.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Errorf("expected 200 for 127.0.0.1:8080, got %d", rec.Code)
	}
}

func TestHostCheck_BlockExternalHost(t *testing.T) {
	handler := HostCheck(okHandler)

	req := httptest.NewRequest(http.MethodGet, "/", nil)
	req.Host = "evil.com"
	rec := httptest.NewRecorder()

	handler.ServeHTTP(rec, req)

	if rec.Code != http.StatusForbidden {
		t.Errorf("expected 403 for evil.com, got %d", rec.Code)
	}
}

func TestHostCheck_BlockPrivateIP(t *testing.T) {
	handler := HostCheck(okHandler)

	req := httptest.NewRequest(http.MethodGet, "/", nil)
	req.Host = "192.168.1.1"
	rec := httptest.NewRecorder()

	handler.ServeHTTP(rec, req)

	if rec.Code != http.StatusForbidden {
		t.Errorf("expected 403 for 192.168.1.1, got %d", rec.Code)
	}
}

func TestHostCheck_BlockEmptyHost(t *testing.T) {
	handler := HostCheck(okHandler)

	req := httptest.NewRequest(http.MethodGet, "/", nil)
	req.Host = ""
	rec := httptest.NewRecorder()

	handler.ServeHTTP(rec, req)

	if rec.Code != http.StatusForbidden {
		t.Errorf("expected 403 for empty host, got %d", rec.Code)
	}
}

// --- CORS Tests ---

func TestCORS_AllowLocalhostOrigin(t *testing.T) {
	handler := CORS(okHandler)

	req := httptest.NewRequest(http.MethodGet, "/", nil)
	req.Header.Set("Origin", "http://localhost:8080")
	rec := httptest.NewRecorder()

	handler.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Errorf("expected 200, got %d", rec.Code)
	}

	acao := rec.Header().Get("Access-Control-Allow-Origin")
	if acao != "http://localhost:8080" {
		t.Errorf("expected Access-Control-Allow-Origin http://localhost:8080, got %q", acao)
	}
}

func TestCORS_Allow127Origin(t *testing.T) {
	handler := CORS(okHandler)

	req := httptest.NewRequest(http.MethodGet, "/", nil)
	req.Header.Set("Origin", "http://127.0.0.1:3000")
	rec := httptest.NewRecorder()

	handler.ServeHTTP(rec, req)

	acao := rec.Header().Get("Access-Control-Allow-Origin")
	if acao != "http://127.0.0.1:3000" {
		t.Errorf("expected Access-Control-Allow-Origin http://127.0.0.1:3000, got %q", acao)
	}

	acac := rec.Header().Get("Access-Control-Allow-Credentials")
	if acac != "true" {
		t.Errorf("expected Access-Control-Allow-Credentials true, got %q", acac)
	}
}

func TestCORS_BlockExternalOrigin(t *testing.T) {
	handler := CORS(okHandler)

	req := httptest.NewRequest(http.MethodGet, "/", nil)
	req.Header.Set("Origin", "http://evil.com")
	rec := httptest.NewRecorder()

	handler.ServeHTTP(rec, req)

	acao := rec.Header().Get("Access-Control-Allow-Origin")
	if acao != "" {
		t.Errorf("expected no Access-Control-Allow-Origin for evil.com, got %q", acao)
	}
}

func TestCORS_BlockNullOrigin(t *testing.T) {
	handler := CORS(okHandler)

	req := httptest.NewRequest(http.MethodGet, "/", nil)
	req.Header.Set("Origin", "null")
	rec := httptest.NewRecorder()

	handler.ServeHTTP(rec, req)

	acao := rec.Header().Get("Access-Control-Allow-Origin")
	if acao != "" {
		t.Errorf("expected no Access-Control-Allow-Origin for null origin, got %q", acao)
	}
}

func TestCORS_BlockEmptyOrigin(t *testing.T) {
	handler := CORS(okHandler)

	req := httptest.NewRequest(http.MethodGet, "/", nil)
	// No Origin header set.
	rec := httptest.NewRecorder()

	handler.ServeHTTP(rec, req)

	acao := rec.Header().Get("Access-Control-Allow-Origin")
	if acao != "" {
		t.Errorf("expected no Access-Control-Allow-Origin for empty origin, got %q", acao)
	}
}

func TestCORS_PreflightOptions(t *testing.T) {
	handler := CORS(okHandler)

	req := httptest.NewRequest(http.MethodOptions, "/", nil)
	req.Header.Set("Origin", "http://localhost:8080")
	rec := httptest.NewRecorder()

	handler.ServeHTTP(rec, req)

	if rec.Code != http.StatusNoContent {
		t.Errorf("expected 204 for preflight, got %d", rec.Code)
	}

	acao := rec.Header().Get("Access-Control-Allow-Origin")
	if acao != "http://localhost:8080" {
		t.Errorf("expected ACAO header on preflight, got %q", acao)
	}

	acam := rec.Header().Get("Access-Control-Allow-Methods")
	if acam == "" {
		t.Error("expected Access-Control-Allow-Methods header on preflight")
	}

	acah := rec.Header().Get("Access-Control-Allow-Headers")
	if acah == "" {
		t.Error("expected Access-Control-Allow-Headers header on preflight")
	}

	maxAge := rec.Header().Get("Access-Control-Max-Age")
	if maxAge != "3600" {
		t.Errorf("expected Access-Control-Max-Age 3600, got %q", maxAge)
	}
}

func TestCORS_NonPreflightPassesThrough(t *testing.T) {
	called := false
	inner := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		called = true
		w.WriteHeader(http.StatusOK)
	})
	handler := CORS(inner)

	req := httptest.NewRequest(http.MethodGet, "/", nil)
	req.Header.Set("Origin", "http://localhost:5173")
	rec := httptest.NewRecorder()

	handler.ServeHTTP(rec, req)

	if !called {
		t.Error("expected inner handler to be called for non-OPTIONS request")
	}

	if rec.Code != http.StatusOK {
		t.Errorf("expected 200, got %d", rec.Code)
	}
}

// --- CSRF Tests ---

func TestCSRF_GetSetsCookie(t *testing.T) {
	handler := CSRF(okHandler)

	req := httptest.NewRequest(http.MethodGet, "/", nil)
	rec := httptest.NewRecorder()

	handler.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Errorf("expected 200, got %d", rec.Code)
	}

	cookies := rec.Result().Cookies()
	var csrfCookie *http.Cookie
	for _, c := range cookies {
		if c.Name == "csrf_token" {
			csrfCookie = c
			break
		}
	}

	if csrfCookie == nil {
		t.Fatal("expected csrf_token cookie to be set")
	}

	if len(csrfCookie.Value) != 64 {
		t.Errorf("expected 64-char hex token, got %d chars: %q", len(csrfCookie.Value), csrfCookie.Value)
	}

	if csrfCookie.SameSite != http.SameSiteStrictMode {
		t.Errorf("expected SameSite=Strict, got %v", csrfCookie.SameSite)
	}
}

func TestCSRF_GetPreservesExistingCookie(t *testing.T) {
	handler := CSRF(okHandler)

	req := httptest.NewRequest(http.MethodGet, "/", nil)
	req.AddCookie(&http.Cookie{Name: "csrf_token", Value: "existing_token_value_1234567890abcdef1234567890abcdef"})
	rec := httptest.NewRecorder()

	handler.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Errorf("expected 200, got %d", rec.Code)
	}

	// Should NOT set a new cookie since one already exists.
	cookies := rec.Result().Cookies()
	for _, c := range cookies {
		if c.Name == "csrf_token" {
			t.Error("expected no new csrf_token cookie when one already exists")
		}
	}
}

func TestCSRF_PostValidToken(t *testing.T) {
	handler := CSRF(okHandler)

	token := "abcdef1234567890abcdef1234567890abcdef1234567890abcdef1234567890"

	req := httptest.NewRequest(http.MethodPost, "/", nil)
	req.AddCookie(&http.Cookie{Name: "csrf_token", Value: token})
	req.Header.Set("X-CSRF-Token", token)
	rec := httptest.NewRecorder()

	handler.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Errorf("expected 200 for valid CSRF token, got %d", rec.Code)
	}
}

func TestCSRF_PostMissingCookie(t *testing.T) {
	handler := CSRF(okHandler)

	req := httptest.NewRequest(http.MethodPost, "/", nil)
	req.Header.Set("X-CSRF-Token", "sometoken")
	rec := httptest.NewRecorder()

	handler.ServeHTTP(rec, req)

	if rec.Code != http.StatusForbidden {
		t.Errorf("expected 403 for missing CSRF cookie, got %d", rec.Code)
	}
}

func TestCSRF_PostMissingHeader(t *testing.T) {
	handler := CSRF(okHandler)

	req := httptest.NewRequest(http.MethodPost, "/", nil)
	req.AddCookie(&http.Cookie{Name: "csrf_token", Value: "sometoken"})
	// No X-CSRF-Token header.
	rec := httptest.NewRecorder()

	handler.ServeHTTP(rec, req)

	if rec.Code != http.StatusForbidden {
		t.Errorf("expected 403 for missing CSRF header, got %d", rec.Code)
	}
}

func TestCSRF_PostMismatchedToken(t *testing.T) {
	handler := CSRF(okHandler)

	req := httptest.NewRequest(http.MethodPost, "/", nil)
	req.AddCookie(&http.Cookie{Name: "csrf_token", Value: "cookie_token"})
	req.Header.Set("X-CSRF-Token", "different_header_token")
	rec := httptest.NewRecorder()

	handler.ServeHTTP(rec, req)

	if rec.Code != http.StatusForbidden {
		t.Errorf("expected 403 for mismatched CSRF tokens, got %d", rec.Code)
	}
}

func TestCSRF_TokenFormat(t *testing.T) {
	token, err := generateCSRFToken()
	if err != nil {
		t.Fatalf("generateCSRFToken() error = %v", err)
	}

	if len(token) != 64 {
		t.Errorf("expected 64-char hex string, got %d chars: %q", len(token), token)
	}

	// Verify it's valid hex.
	for _, c := range token {
		if !((c >= '0' && c <= '9') || (c >= 'a' && c <= 'f')) {
			t.Errorf("expected only hex characters, found %q in %q", string(c), token)
			break
		}
	}

	// Verify two calls produce different tokens.
	token2, err := generateCSRFToken()
	if err != nil {
		t.Fatalf("generateCSRFToken() error = %v", err)
	}
	if token == token2 {
		t.Error("expected different tokens from two calls to generateCSRFToken()")
	}
}
