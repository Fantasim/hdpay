package handlers

import (
	"encoding/json"
	"log/slog"
	"net/http"

	"github.com/Fantasim/hdpay/internal/poller/httputil"
	"github.com/Fantasim/hdpay/internal/poller/api/middleware"
	"github.com/Fantasim/hdpay/internal/poller/config"
)

type loginRequest struct {
	Username string `json:"username"`
	Password string `json:"password"`
}

// LoginHandler returns a handler for POST /api/admin/login.
// No session required (this IS the login endpoint). IP check is also exempt.
func LoginHandler(sessions *middleware.SessionStore) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		var req loginRequest
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			slog.Debug("login: invalid request body", "error", err)
			httputil.Error(w, http.StatusBadRequest, config.ErrorInvalidRequest, "Invalid request body")
			return
		}

		if req.Username == "" || req.Password == "" {
			httputil.Error(w, http.StatusBadRequest, config.ErrorInvalidRequest, "Username and password are required")
			return
		}

		token, err := sessions.Login(req.Username, req.Password)
		if err != nil {
			httputil.Error(w, http.StatusUnauthorized, config.ErrorInvalidCredentials, "Invalid username or password")
			return
		}

		http.SetCookie(w, &http.Cookie{
			Name:     config.SessionCookieName,
			Value:    token,
			Path:     "/",
			HttpOnly: true,
			SameSite: http.SameSiteStrictMode,
		})

		slog.Info("admin login via API", "remoteAddr", r.RemoteAddr)
		httputil.JSON(w, http.StatusOK, map[string]string{"status": "logged_in"})
	}
}

// LogoutHandler returns a handler for POST /api/admin/logout.
func LogoutHandler(sessions *middleware.SessionStore) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		cookie, err := r.Cookie(config.SessionCookieName)
		if err == nil {
			sessions.Logout(cookie.Value)
		}

		// Clear the cookie.
		http.SetCookie(w, &http.Cookie{
			Name:     config.SessionCookieName,
			Value:    "",
			Path:     "/",
			MaxAge:   -1,
			HttpOnly: true,
			SameSite: http.SameSiteStrictMode,
		})

		slog.Info("admin logout via API", "remoteAddr", r.RemoteAddr)
		httputil.JSON(w, http.StatusOK, map[string]string{"status": "logged_out"})
	}
}
