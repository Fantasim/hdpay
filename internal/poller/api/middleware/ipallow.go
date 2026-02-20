package middleware

import (
	"log/slog"
	"net"
	"net/http"
	"sync"

	"github.com/Fantasim/hdpay/internal/poller/httputil"
	"github.com/Fantasim/hdpay/internal/poller/config"
)

// IPAllowlist enforces IP-based access control.
// Localhost and private-network IPs are always allowed.
// Internet IPs must be in the allowlist (loaded from DB, cached in memory).
type IPAllowlist struct {
	mu    sync.RWMutex
	cache map[string]bool
}

// NewIPAllowlist creates an IPAllowlist pre-populated with the given IPs.
func NewIPAllowlist(initial map[string]bool) *IPAllowlist {
	al := &IPAllowlist{
		cache: make(map[string]bool),
	}
	for ip := range initial {
		al.cache[ip] = true
	}
	slog.Info("IP allowlist initialized", "cachedIPs", len(al.cache))
	return al
}

// Middleware returns an HTTP middleware that checks the client IP against the allowlist.
// Expects Chi's RealIP middleware to have already resolved X-Forwarded-For into RemoteAddr.
func (al *IPAllowlist) Middleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		clientIP := extractIP(r.RemoteAddr)

		if al.IsAllowed(clientIP) {
			next.ServeHTTP(w, r)
			return
		}

		slog.Warn("IP not allowed",
			"ip", clientIP,
			"method", r.Method,
			"path", r.URL.Path,
		)
		httputil.Error(w, http.StatusForbidden, config.ErrorIPNotAllowed,
			"IP address "+clientIP+" is not in the allowlist")
	})
}

// IsAllowed checks whether the given IP should be granted access.
func (al *IPAllowlist) IsAllowed(ip string) bool {
	// Localhost is always allowed.
	if ip == "127.0.0.1" || ip == "::1" {
		return true
	}

	// Private networks are always allowed.
	if isPrivateIP(ip) {
		return true
	}

	// Check the cache.
	al.mu.RLock()
	allowed := al.cache[ip]
	al.mu.RUnlock()

	return allowed
}

// Refresh replaces the entire allowlist cache.
// Called when IPs are added or removed from the dashboard.
func (al *IPAllowlist) Refresh(ips map[string]bool) {
	al.mu.Lock()
	al.cache = make(map[string]bool, len(ips))
	for ip := range ips {
		al.cache[ip] = true
	}
	al.mu.Unlock()
	slog.Info("IP allowlist refreshed", "cachedIPs", len(ips))
}

// extractIP extracts the IP address from a host:port string.
// If there's no port, returns the string as-is.
func extractIP(remoteAddr string) string {
	host, _, err := net.SplitHostPort(remoteAddr)
	if err != nil {
		// No port — return as-is.
		return remoteAddr
	}
	return host
}

// isPrivateIP checks whether an IP is in a private/reserved range (RFC 1918 + loopback).
func isPrivateIP(ipStr string) bool {
	ip := net.ParseIP(ipStr)
	if ip == nil {
		return false
	}

	// Check against standard private ranges.
	privateRanges := []struct {
		network string
	}{
		{"10.0.0.0/8"},
		{"172.16.0.0/12"},
		{"192.168.0.0/16"},
		{"127.0.0.0/8"},
		{"::1/128"},
		{"fc00::/7"},
	}

	for _, r := range privateRanges {
		_, cidr, err := net.ParseCIDR(r.network)
		if err != nil {
			continue
		}
		if cidr.Contains(ip) {
			return true
		}
	}

	return false
}
