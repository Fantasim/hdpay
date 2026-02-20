package middleware

import (
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestIPAllowlist_Localhost(t *testing.T) {
	al := NewIPAllowlist(map[string]bool{})

	if !al.IsAllowed("127.0.0.1") {
		t.Error("127.0.0.1 should be allowed")
	}
	if !al.IsAllowed("::1") {
		t.Error("::1 should be allowed")
	}
}

func TestIPAllowlist_PrivateIPs(t *testing.T) {
	al := NewIPAllowlist(map[string]bool{})

	privateIPs := []string{
		"10.0.0.1",
		"10.255.255.255",
		"172.16.0.1",
		"172.31.255.255",
		"192.168.0.1",
		"192.168.1.100",
	}

	for _, ip := range privateIPs {
		if !al.IsAllowed(ip) {
			t.Errorf("private IP %s should be allowed", ip)
		}
	}
}

func TestIPAllowlist_UnknownIP(t *testing.T) {
	al := NewIPAllowlist(map[string]bool{})

	if al.IsAllowed("8.8.8.8") {
		t.Error("8.8.8.8 should NOT be allowed when not in allowlist")
	}
	if al.IsAllowed("1.2.3.4") {
		t.Error("1.2.3.4 should NOT be allowed when not in allowlist")
	}
}

func TestIPAllowlist_AllowlistedIP(t *testing.T) {
	al := NewIPAllowlist(map[string]bool{
		"8.8.8.8":  true,
		"1.2.3.4":  true,
	})

	if !al.IsAllowed("8.8.8.8") {
		t.Error("8.8.8.8 should be allowed when in allowlist")
	}
	if !al.IsAllowed("1.2.3.4") {
		t.Error("1.2.3.4 should be allowed when in allowlist")
	}
	if al.IsAllowed("5.6.7.8") {
		t.Error("5.6.7.8 should NOT be allowed")
	}
}

func TestIPAllowlist_Refresh(t *testing.T) {
	al := NewIPAllowlist(map[string]bool{})

	if al.IsAllowed("8.8.8.8") {
		t.Error("8.8.8.8 should NOT be allowed before refresh")
	}

	al.Refresh(map[string]bool{"8.8.8.8": true})

	if !al.IsAllowed("8.8.8.8") {
		t.Error("8.8.8.8 should be allowed after refresh")
	}

	// Previous entries removed on refresh.
	al.Refresh(map[string]bool{"9.9.9.9": true})

	if al.IsAllowed("8.8.8.8") {
		t.Error("8.8.8.8 should NOT be allowed after second refresh")
	}
	if !al.IsAllowed("9.9.9.9") {
		t.Error("9.9.9.9 should be allowed after second refresh")
	}
}

func TestIPAllowlist_Middleware_Blocked(t *testing.T) {
	al := NewIPAllowlist(map[string]bool{})

	handler := al.Middleware(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))

	req := httptest.NewRequest("GET", "/api/test", nil)
	req.RemoteAddr = "8.8.8.8:12345"
	rec := httptest.NewRecorder()

	handler.ServeHTTP(rec, req)

	if rec.Code != http.StatusForbidden {
		t.Errorf("expected 403, got %d", rec.Code)
	}
}

func TestIPAllowlist_Middleware_Allowed(t *testing.T) {
	al := NewIPAllowlist(map[string]bool{})

	handler := al.Middleware(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))

	req := httptest.NewRequest("GET", "/api/test", nil)
	req.RemoteAddr = "127.0.0.1:12345"
	rec := httptest.NewRecorder()

	handler.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Errorf("expected 200, got %d", rec.Code)
	}
}

func TestExtractIP(t *testing.T) {
	tests := []struct {
		input string
		want  string
	}{
		{"127.0.0.1:8080", "127.0.0.1"},
		{"8.8.8.8:443", "8.8.8.8"},
		{"[::1]:8080", "::1"},
		{"127.0.0.1", "127.0.0.1"},
	}

	for _, tt := range tests {
		got := extractIP(tt.input)
		if got != tt.want {
			t.Errorf("extractIP(%q) = %q, want %q", tt.input, got, tt.want)
		}
	}
}

func TestIsPrivateIP(t *testing.T) {
	tests := []struct {
		ip   string
		want bool
	}{
		{"10.0.0.1", true},
		{"172.16.0.1", true},
		{"172.32.0.1", false},
		{"192.168.1.1", true},
		{"8.8.8.8", false},
		{"127.0.0.1", true},
		{"invalid", false},
	}

	for _, tt := range tests {
		got := isPrivateIP(tt.ip)
		if got != tt.want {
			t.Errorf("isPrivateIP(%q) = %v, want %v", tt.ip, got, tt.want)
		}
	}
}
