package handlers

import (
	"encoding/json"
	"log/slog"
	"net/http"

	"github.com/Fantasim/hdpay/internal/config"
)

// HealthHandler returns a handler for the GET /api/health endpoint.
func HealthHandler(cfg *config.Config, version string) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		slog.Debug("health check requested", "remoteAddr", r.RemoteAddr)

		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(map[string]string{
			"status":  "ok",
			"version": version,
			"network": cfg.Network,
			"dbPath":  cfg.DBPath,
		})
	}
}
