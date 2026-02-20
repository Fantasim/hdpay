package handlers

import (
	"net/http"

	"github.com/Fantasim/hdpay/internal/poller/httputil"
	pollerconfig "github.com/Fantasim/hdpay/internal/poller/config"
	"github.com/Fantasim/hdpay/internal/poller/watcher"
)

// HealthHandler returns a handler for GET /api/health.
// No auth, no IP check — always open.
func HealthHandler(cfg *pollerconfig.Config, w *watcher.Watcher) http.HandlerFunc {
	return func(rw http.ResponseWriter, r *http.Request) {
		httputil.JSON(rw, http.StatusOK, map[string]interface{}{
			"status":         "ok",
			"network":        cfg.Network,
			"active_watches": w.ActiveCount(),
		})
	}
}
