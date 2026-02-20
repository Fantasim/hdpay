package middleware

import (
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/Fantasim/hdpay/internal/poller/config"
)

func TestSessionStore_LoginSuccess(t *testing.T) {
	store, err := NewSessionStore("admin", "password123")
	if err != nil {
		t.Fatalf("NewSessionStore failed: %v", err)
	}

	token, err := store.Login("admin", "password123")
	if err != nil {
		t.Fatalf("Login failed: %v", err)
	}
	if token == "" {
		t.Error("expected non-empty token")
	}
	if len(token) != config.SessionTokenLength*2 { // hex-encoded
		t.Errorf("token length = %d, want %d", len(token), config.SessionTokenLength*2)
	}
}

func TestSessionStore_LoginWrongPassword(t *testing.T) {
	store, err := NewSessionStore("admin", "password123")
	if err != nil {
		t.Fatalf("NewSessionStore failed: %v", err)
	}

	_, err = store.Login("admin", "wrongpassword")
	if err == nil {
		t.Error("expected error for wrong password")
	}
}

func TestSessionStore_LoginWrongUsername(t *testing.T) {
	store, err := NewSessionStore("admin", "password123")
	if err != nil {
		t.Fatalf("NewSessionStore failed: %v", err)
	}

	_, err = store.Login("wronguser", "password123")
	if err == nil {
		t.Error("expected error for wrong username")
	}
}

func TestSessionStore_Validate(t *testing.T) {
	store, err := NewSessionStore("admin", "password123")
	if err != nil {
		t.Fatalf("NewSessionStore failed: %v", err)
	}

	token, err := store.Login("admin", "password123")
	if err != nil {
		t.Fatalf("Login failed: %v", err)
	}

	if !store.Validate(token) {
		t.Error("valid token should be validated")
	}
	if store.Validate("nonexistent-token") {
		t.Error("nonexistent token should NOT be validated")
	}
}

func TestSessionStore_Expiry(t *testing.T) {
	store, err := NewSessionStore("admin", "password123")
	if err != nil {
		t.Fatalf("NewSessionStore failed: %v", err)
	}

	token, err := store.Login("admin", "password123")
	if err != nil {
		t.Fatalf("Login failed: %v", err)
	}

	// Manually expire the session.
	store.mu.Lock()
	store.sessions[token].ExpiresAt = time.Now().Add(-1 * time.Second)
	store.mu.Unlock()

	if store.Validate(token) {
		t.Error("expired token should NOT be validated")
	}
}

func TestSessionStore_Logout(t *testing.T) {
	store, err := NewSessionStore("admin", "password123")
	if err != nil {
		t.Fatalf("NewSessionStore failed: %v", err)
	}

	token, err := store.Login("admin", "password123")
	if err != nil {
		t.Fatalf("Login failed: %v", err)
	}

	store.Logout(token)

	if store.Validate(token) {
		t.Error("logged-out token should NOT be validated")
	}
}

func TestSessionStore_Middleware_NoCookie(t *testing.T) {
	store, err := NewSessionStore("admin", "password123")
	if err != nil {
		t.Fatalf("NewSessionStore failed: %v", err)
	}

	handler := store.Middleware(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))

	req := httptest.NewRequest("GET", "/api/admin/settings", nil)
	rec := httptest.NewRecorder()

	handler.ServeHTTP(rec, req)

	if rec.Code != http.StatusUnauthorized {
		t.Errorf("expected 401, got %d", rec.Code)
	}
}

func TestSessionStore_Middleware_ValidCookie(t *testing.T) {
	store, err := NewSessionStore("admin", "password123")
	if err != nil {
		t.Fatalf("NewSessionStore failed: %v", err)
	}

	token, err := store.Login("admin", "password123")
	if err != nil {
		t.Fatalf("Login failed: %v", err)
	}

	handler := store.Middleware(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))

	req := httptest.NewRequest("GET", "/api/admin/settings", nil)
	req.AddCookie(&http.Cookie{Name: config.SessionCookieName, Value: token})
	rec := httptest.NewRecorder()

	handler.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Errorf("expected 200, got %d", rec.Code)
	}
}

func TestSessionStore_Middleware_ExpiredCookie(t *testing.T) {
	store, err := NewSessionStore("admin", "password123")
	if err != nil {
		t.Fatalf("NewSessionStore failed: %v", err)
	}

	token, err := store.Login("admin", "password123")
	if err != nil {
		t.Fatalf("Login failed: %v", err)
	}

	// Expire the session.
	store.mu.Lock()
	store.sessions[token].ExpiresAt = time.Now().Add(-1 * time.Second)
	store.mu.Unlock()

	handler := store.Middleware(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))

	req := httptest.NewRequest("GET", "/api/admin/settings", nil)
	req.AddCookie(&http.Cookie{Name: config.SessionCookieName, Value: token})
	rec := httptest.NewRecorder()

	handler.ServeHTTP(rec, req)

	if rec.Code != http.StatusUnauthorized {
		t.Errorf("expected 401, got %d", rec.Code)
	}
}
