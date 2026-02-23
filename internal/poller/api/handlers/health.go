package handlers

import (
	"fmt"
	"net/http"
	"time"

	"github.com/Fantasim/hdpay/internal/poller/httputil"
	pollerconfig "github.com/Fantasim/hdpay/internal/poller/config"
	"github.com/Fantasim/hdpay/internal/poller/watcher"
)

// serverStartTime records when the process started (set at init).
var serverStartTime = time.Now()

// HealthHandler returns a handler for GET /api/health.
// No auth, no IP check — always open.
func HealthHandler(cfg *pollerconfig.Config, w *watcher.Watcher) http.HandlerFunc {
	return func(rw http.ResponseWriter, r *http.Request) {
		uptime := time.Since(serverStartTime)
		httputil.JSON(rw, http.StatusOK, map[string]interface{}{
			"status":         "ok",
			"network":        cfg.Network,
			"uptime":         formatUptime(uptime),
			"version":        pollerconfig.Version,
			"active_watches": w.ActiveCount(),
		})
	}
}

// formatUptime returns a human-readable uptime string.
func formatUptime(d time.Duration) string {
	days := int(d.Hours()) / 24
	hours := int(d.Hours()) % 24
	minutes := int(d.Minutes()) % 60

	if days > 0 {
		return fmt.Sprintf("%dd %dh %dm", days, hours, minutes)
	}
	if hours > 0 {
		return fmt.Sprintf("%dh %dm", hours, minutes)
	}
	return fmt.Sprintf("%dm", minutes)
}
