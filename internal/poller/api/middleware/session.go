package middleware

import (
	"crypto/rand"
	"encoding/hex"
	"fmt"
	"log/slog"
	"net/http"
	"sync"
	"time"

	"github.com/Fantasim/hdpay/internal/poller/httputil"
	"github.com/Fantasim/hdpay/internal/poller/config"
	"golang.org/x/crypto/bcrypt"
)

// Session represents an active admin session.
type Session struct {
	Token     string
	CreatedAt time.Time
	ExpiresAt time.Time
}

// SessionStore manages in-memory sessions for dashboard authentication.
// Sessions are lost on restart (acceptable per design).
type SessionStore struct {
	mu       sync.RWMutex
	sessions map[string]*Session // token -> session
	passHash []byte
	username string
}

// NewSessionStore creates a session store with the admin credentials.
// The plaintext password is bcrypt-hashed immediately and discarded.
func NewSessionStore(username, password string) (*SessionStore, error) {
	hash, err := bcrypt.GenerateFromPassword([]byte(password), bcrypt.DefaultCost)
	if err != nil {
		return nil, fmt.Errorf("failed to hash admin password: %w", err)
	}

	slog.Info("session store initialized", "username", username)
	return &SessionStore{
		sessions: make(map[string]*Session),
		passHash: hash,
		username: username,
	}, nil
}

// Login validates credentials and returns a session token on success.
func (s *SessionStore) Login(username, password string) (string, error) {
	if username != s.username {
		slog.Warn("login attempt with wrong username", "attempted", username)
		return "", fmt.Errorf(config.ErrorInvalidCredentials)
	}

	if err := bcrypt.CompareHashAndPassword(s.passHash, []byte(password)); err != nil {
		slog.Warn("login attempt with wrong password", "username", username)
		return "", fmt.Errorf(config.ErrorInvalidCredentials)
	}

	// Generate random token.
	tokenBytes := make([]byte, config.SessionTokenLength)
	if _, err := rand.Read(tokenBytes); err != nil {
		return "", fmt.Errorf("failed to generate session token: %w", err)
	}
	token := hex.EncodeToString(tokenBytes)

	now := time.Now()
	session := &Session{
		Token:     token,
		CreatedAt: now,
		ExpiresAt: now.Add(config.SessionTimeout),
	}

	s.mu.Lock()
	s.sessions[token] = session
	s.mu.Unlock()

	slog.Info("admin login successful", "username", username, "expiresAt", session.ExpiresAt.UTC().Format(time.RFC3339))
	return token, nil
}

// Validate checks whether a token corresponds to a valid, non-expired session.
// Expired sessions are lazily removed.
func (s *SessionStore) Validate(token string) bool {
	s.mu.RLock()
	session, exists := s.sessions[token]
	s.mu.RUnlock()

	if !exists {
		return false
	}

	if time.Now().After(session.ExpiresAt) {
		// Lazily remove expired session.
		s.mu.Lock()
		delete(s.sessions, token)
		s.mu.Unlock()
		slog.Debug("expired session removed", "token", token[:8]+"...")
		return false
	}

	return true
}

// Logout invalidates a session by removing it from the store.
func (s *SessionStore) Logout(token string) {
	s.mu.Lock()
	delete(s.sessions, token)
	s.mu.Unlock()
	slog.Info("admin logged out")
}

// Middleware returns an HTTP middleware that requires a valid session cookie.
// Returns 401 if the cookie is missing, invalid, or expired.
func (s *SessionStore) Middleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		cookie, err := r.Cookie(config.SessionCookieName)
		if err != nil {
			slog.Debug("session middleware: no cookie",
				"path", r.URL.Path,
				"remoteAddr", r.RemoteAddr,
			)
			httputil.Error(w, http.StatusUnauthorized, config.ErrorSessionExpired, "Session required — please log in")
			return
		}

		if !s.Validate(cookie.Value) {
			slog.Debug("session middleware: invalid or expired token",
				"path", r.URL.Path,
				"remoteAddr", r.RemoteAddr,
			)
			httputil.Error(w, http.StatusUnauthorized, config.ErrorSessionExpired, "Session expired — please log in again")
			return
		}

		next.ServeHTTP(w, r)
	})
}
