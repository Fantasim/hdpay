package handlers

import (
	"encoding/json"
	"log/slog"
	"net/http"
	"strings"

	"github.com/Fantasim/hdpay/internal/poller/httputil"
	pollerconfig "github.com/Fantasim/hdpay/internal/poller/config"
	"github.com/Fantasim/hdpay/internal/poller/models"
	"github.com/Fantasim/hdpay/internal/poller/pollerdb"
	"github.com/Fantasim/hdpay/internal/poller/validate"
	"github.com/Fantasim/hdpay/internal/poller/watcher"
	"github.com/go-chi/chi/v5"
)

type createWatchRequest struct {
	Chain          string `json:"chain"`
	Address        string `json:"address"`
	TimeoutMinutes int    `json:"timeout_minutes"`
}

type createWatchResponse struct {
	WatchID             string `json:"watch_id"`
	Chain               string `json:"chain"`
	Address             string `json:"address"`
	Status              string `json:"status"`
	StartedAt           string `json:"started_at"`
	ExpiresAt           string `json:"expires_at"`
	PollIntervalSeconds int    `json:"poll_interval_seconds"`
}

// CreateWatchHandler returns a handler for POST /api/watch.
func CreateWatchHandler(w *watcher.Watcher, cfg *pollerconfig.Config) http.HandlerFunc {
	return func(rw http.ResponseWriter, r *http.Request) {
		var req createWatchRequest
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			slog.Debug("create watch: invalid request body", "error", err)
			httputil.Error(rw, http.StatusBadRequest, pollerconfig.ErrorInvalidRequest, "Invalid request body")
			return
		}

		// Validate chain.
		if req.Chain != "BTC" && req.Chain != "BSC" && req.Chain != "SOL" {
			httputil.Error(rw, http.StatusBadRequest, pollerconfig.ErrorInvalidChain,
				"Invalid chain: "+req.Chain+". Must be BTC, BSC, or SOL")
			return
		}

		// Validate address format.
		if err := validate.Address(req.Chain, req.Address, cfg.Network); err != nil {
			httputil.Error(rw, http.StatusBadRequest, pollerconfig.ErrorAddressInvalid, err.Error())
			return
		}

		// Default timeout.
		timeoutMin := req.TimeoutMinutes
		if timeoutMin <= 0 {
			timeoutMin = w.DefaultWatchTimeout()
		}
		if timeoutMin > pollerconfig.MaxWatchTimeoutMinutes {
			httputil.Error(rw, http.StatusBadRequest, pollerconfig.ErrorInvalidTimeout,
				"Timeout must be 1-120 minutes")
			return
		}

		watch, err := w.CreateWatch(req.Chain, req.Address, timeoutMin)
		if err != nil {
			errMsg := err.Error()
			switch {
			case strings.Contains(errMsg, pollerconfig.ErrorAlreadyWatching):
				httputil.Error(rw, http.StatusConflict, pollerconfig.ErrorAlreadyWatching, errMsg)
			case strings.Contains(errMsg, pollerconfig.ErrorMaxWatches):
				httputil.Error(rw, http.StatusTooManyRequests, pollerconfig.ErrorMaxWatches, errMsg)
			case strings.Contains(errMsg, pollerconfig.ErrorProviderUnavailable):
				httputil.Error(rw, http.StatusInternalServerError, pollerconfig.ErrorProviderUnavailable, errMsg)
			default:
				slog.Error("create watch failed", "error", err)
				httputil.Error(rw, http.StatusInternalServerError, pollerconfig.ErrorInternal, "Failed to create watch")
			}
			return
		}

		slog.Info("watch created via API",
			"watchID", watch.ID,
			"chain", watch.Chain,
			"address", watch.Address,
			"remoteAddr", r.RemoteAddr,
		)

		resp := createWatchResponse{
			WatchID:             watch.ID,
			Chain:               watch.Chain,
			Address:             watch.Address,
			Status:              string(watch.Status),
			StartedAt:           watch.StartedAt,
			ExpiresAt:           watch.ExpiresAt,
			PollIntervalSeconds: pollIntervalSeconds(watch.Chain),
		}
		httputil.JSON(rw, http.StatusCreated, resp)
	}
}

// CancelWatchHandler returns a handler for DELETE /api/watch/{id}.
func CancelWatchHandler(w *watcher.Watcher) http.HandlerFunc {
	return func(rw http.ResponseWriter, r *http.Request) {
		watchID := chi.URLParam(r, "id")
		if watchID == "" {
			httputil.Error(rw, http.StatusBadRequest, pollerconfig.ErrorInvalidRequest, "Watch ID is required")
			return
		}

		if err := w.CancelWatch(watchID); err != nil {
			errMsg := err.Error()
			switch {
			case strings.Contains(errMsg, pollerconfig.ErrorWatchNotFound):
				httputil.Error(rw, http.StatusNotFound, pollerconfig.ErrorWatchNotFound, errMsg)
			case strings.Contains(errMsg, pollerconfig.ErrorWatchExpired):
				httputil.Error(rw, http.StatusConflict, pollerconfig.ErrorWatchExpired, errMsg)
			default:
				slog.Error("cancel watch failed", "watchID", watchID, "error", err)
				httputil.Error(rw, http.StatusInternalServerError, pollerconfig.ErrorInternal, "Failed to cancel watch")
			}
			return
		}

		slog.Info("watch cancelled via API", "watchID", watchID, "remoteAddr", r.RemoteAddr)
		httputil.JSON(rw, http.StatusOK, map[string]string{
			"watch_id": watchID,
			"status":   string(models.WatchStatusCancelled),
		})
	}
}

// ListWatchesHandler returns a handler for GET /api/watches.
func ListWatchesHandler(db *pollerdb.DB) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		filters := models.WatchFilters{}

		if status := r.URL.Query().Get("status"); status != "" {
			s := models.WatchStatus(status)
			filters.Status = &s
		}
		if chain := r.URL.Query().Get("chain"); chain != "" {
			filters.Chain = &chain
		}

		watches, err := db.ListWatches(filters)
		if err != nil {
			slog.Error("list watches failed", "error", err)
			httputil.Error(w, http.StatusInternalServerError, pollerconfig.ErrorDatabase, "Failed to list watches")
			return
		}

		slog.Debug("watches listed", "count", len(watches), "filters", filters)
		httputil.JSON(w, http.StatusOK, watches)
	}
}

// pollIntervalSeconds returns the poll interval for a chain in seconds.
func pollIntervalSeconds(chain string) int {
	switch chain {
	case "BTC":
		return int(pollerconfig.PollIntervalBTC.Seconds())
	case "BSC":
		return int(pollerconfig.PollIntervalBSC.Seconds())
	case "SOL":
		return int(pollerconfig.PollIntervalSOL.Seconds())
	default:
		return 60
	}
}
