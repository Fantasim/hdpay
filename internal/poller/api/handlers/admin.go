package handlers

import (
	"encoding/json"
	"fmt"
	"log/slog"
	"net"
	"net/http"
	"os"
	"strconv"
	"time"

	"github.com/Fantasim/hdpay/internal/poller/httputil"
	"github.com/Fantasim/hdpay/internal/poller/api/middleware"
	pollerconfig "github.com/Fantasim/hdpay/internal/poller/config"
	"github.com/Fantasim/hdpay/internal/poller/models"
	"github.com/Fantasim/hdpay/internal/poller/points"
	"github.com/Fantasim/hdpay/internal/poller/pollerdb"
	"github.com/Fantasim/hdpay/internal/poller/watcher"
	"github.com/go-chi/chi/v5"
)

type addIPRequest struct {
	IP          string `json:"ip"`
	Description string `json:"description"`
}

type updateWatchDefaultsRequest struct {
	MaxActiveWatches    *int `json:"max_active_watches"`
	DefaultWatchTimeout *int `json:"default_watch_timeout"`
}

// GetAllowlistHandler returns a handler for GET /api/admin/allowlist.
func GetAllowlistHandler(db *pollerdb.DB) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		entries, err := db.ListAllowedIPs()
		if err != nil {
			slog.Error("get allowlist failed", "error", err)
			httputil.Error(w, http.StatusInternalServerError, pollerconfig.ErrorDatabase, "Failed to query allowlist")
			return
		}
		slog.Debug("allowlist listed", "count", len(entries))
		httputil.JSON(w, http.StatusOK, entries)
	}
}

// AddAllowlistHandler returns a handler for POST /api/admin/allowlist.
func AddAllowlistHandler(db *pollerdb.DB, allowlist *middleware.IPAllowlist) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		var req addIPRequest
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			httputil.Error(w, http.StatusBadRequest, pollerconfig.ErrorInvalidRequest, "Invalid request body")
			return
		}

		if req.IP == "" {
			httputil.Error(w, http.StatusBadRequest, pollerconfig.ErrorInvalidRequest, "IP address is required")
			return
		}

		// Validate IP format.
		if net.ParseIP(req.IP) == nil {
			httputil.Error(w, http.StatusBadRequest, pollerconfig.ErrorInvalidRequest,
				fmt.Sprintf("Invalid IP address format: %s", req.IP))
			return
		}

		id, err := db.AddIP(req.IP, req.Description)
		if err != nil {
			slog.Error("add IP failed", "ip", req.IP, "error", err)
			httputil.Error(w, http.StatusInternalServerError, pollerconfig.ErrorDatabase, "Failed to add IP")
			return
		}

		// Refresh in-memory cache.
		refreshAllowlistCache(db, allowlist)

		slog.Info("IP added to allowlist", "ip", req.IP, "description", req.Description, "id", id)
		httputil.JSON(w, http.StatusCreated, models.IPAllowEntry{
			ID:          int(id),
			IP:          req.IP,
			Description: req.Description,
			AddedAt:     time.Now().UTC().Format(time.RFC3339),
		})
	}
}

// RemoveAllowlistHandler returns a handler for DELETE /api/admin/allowlist/{id}.
func RemoveAllowlistHandler(db *pollerdb.DB, allowlist *middleware.IPAllowlist) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		idStr := chi.URLParam(r, "id")
		id, err := strconv.Atoi(idStr)
		if err != nil {
			httputil.Error(w, http.StatusBadRequest, pollerconfig.ErrorInvalidRequest, "Invalid ID")
			return
		}

		if err := db.RemoveIP(id); err != nil {
			slog.Error("remove IP failed", "id", id, "error", err)
			httputil.Error(w, http.StatusInternalServerError, pollerconfig.ErrorDatabase, "Failed to remove IP")
			return
		}

		// Refresh in-memory cache.
		refreshAllowlistCache(db, allowlist)

		slog.Info("IP removed from allowlist", "id", id)
		httputil.JSON(w, http.StatusOK, map[string]string{"status": "removed"})
	}
}

// GetSettingsHandler returns a handler for GET /api/admin/settings.
func GetSettingsHandler(cfg *pollerconfig.Config, w *watcher.Watcher, calc *points.PointsCalculator) http.HandlerFunc {
	return func(rw http.ResponseWriter, r *http.Request) {
		slog.Debug("settings requested")
		httputil.JSON(rw, http.StatusOK, map[string]interface{}{
			"max_active_watches":    w.MaxActiveWatches(),
			"default_watch_timeout": w.DefaultWatchTimeout(),
			"tiers":                 calc.Tiers(),
			"network":              cfg.Network,
			"start_date":           time.Unix(cfg.StartDate, 0).UTC().Format(time.RFC3339),
			"active_watches":       w.ActiveCount(),
		})
	}
}

// UpdateTiersHandler returns a handler for PUT /api/admin/tiers.
func UpdateTiersHandler(cfg *pollerconfig.Config, calc *points.PointsCalculator) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		var tiers []models.Tier
		if err := json.NewDecoder(r.Body).Decode(&tiers); err != nil {
			httputil.Error(w, http.StatusBadRequest, pollerconfig.ErrorInvalidRequest, "Invalid request body")
			return
		}

		if err := points.ValidateTiers(tiers); err != nil {
			httputil.Error(w, http.StatusBadRequest, pollerconfig.ErrorTiersInvalid, err.Error())
			return
		}

		// Write to tiers.json.
		data, err := json.MarshalIndent(tiers, "", "  ")
		if err != nil {
			slog.Error("marshal tiers failed", "error", err)
			httputil.Error(w, http.StatusInternalServerError, pollerconfig.ErrorInternal, "Failed to encode tiers")
			return
		}

		if err := os.WriteFile(cfg.TiersFile, data, 0644); err != nil {
			slog.Error("write tiers file failed", "error", err)
			httputil.Error(w, http.StatusInternalServerError, pollerconfig.ErrorTiersFile, "Failed to write tiers file")
			return
		}

		// Reload calculator.
		if err := calc.Reload(tiers); err != nil {
			slog.Error("reload calculator failed", "error", err)
			httputil.Error(w, http.StatusInternalServerError, pollerconfig.ErrorInternal, "Failed to reload tiers")
			return
		}

		slog.Info("tiers updated via API", "tierCount", len(tiers))
		httputil.JSON(w, http.StatusOK, map[string]interface{}{
			"status": "updated",
			"tiers":  len(tiers),
		})
	}
}

// UpdateWatchDefaultsHandler returns a handler for PUT /api/admin/watch-defaults.
func UpdateWatchDefaultsHandler(w *watcher.Watcher) http.HandlerFunc {
	return func(rw http.ResponseWriter, r *http.Request) {
		var req updateWatchDefaultsRequest
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			httputil.Error(rw, http.StatusBadRequest, pollerconfig.ErrorInvalidRequest, "Invalid request body")
			return
		}

		if req.MaxActiveWatches != nil {
			if *req.MaxActiveWatches < 1 {
				httputil.Error(rw, http.StatusBadRequest, pollerconfig.ErrorInvalidRequest, "max_active_watches must be >= 1")
				return
			}
			w.SetMaxActiveWatches(*req.MaxActiveWatches)
		}

		if req.DefaultWatchTimeout != nil {
			if *req.DefaultWatchTimeout < 1 || *req.DefaultWatchTimeout > pollerconfig.MaxWatchTimeoutMinutes {
				httputil.Error(rw, http.StatusBadRequest, pollerconfig.ErrorInvalidRequest,
					fmt.Sprintf("default_watch_timeout must be 1-%d", pollerconfig.MaxWatchTimeoutMinutes))
				return
			}
			w.SetDefaultWatchTimeout(*req.DefaultWatchTimeout)
		}

		slog.Info("watch defaults updated via API",
			"maxActiveWatches", w.MaxActiveWatches(),
			"defaultWatchTimeout", w.DefaultWatchTimeout(),
		)
		httputil.JSON(rw, http.StatusOK, map[string]interface{}{
			"max_active_watches":    w.MaxActiveWatches(),
			"default_watch_timeout": w.DefaultWatchTimeout(),
		})
	}
}

// refreshAllowlistCache loads all IPs from DB and refreshes the in-memory cache.
func refreshAllowlistCache(db *pollerdb.DB, allowlist *middleware.IPAllowlist) {
	ips, err := db.LoadAllIPsIntoMap()
	if err != nil {
		slog.Error("failed to refresh allowlist cache", "error", err)
		return
	}
	allowlist.Refresh(ips)
}
