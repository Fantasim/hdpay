package handlers

import (
	"encoding/json"
	"log/slog"
	"net/http"
	"time"

	"github.com/Fantasim/hdpay/internal/config"
	"github.com/Fantasim/hdpay/internal/db"
	"github.com/Fantasim/hdpay/internal/models"
)

// validSettingKeys defines the allowed setting keys for update.
var validSettingKeys = map[string]bool{
	"max_scan_id":            true,
	"auto_resume_scans":      true,
	"resume_threshold_hours": true,
	"btc_fee_rate":           true,
	"bsc_gas_preseed_bnb":    true,
	"log_level":              true,
	"network":                true,
}

// GetSettings handles GET /api/settings.
func GetSettings(database *db.DB) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		start := time.Now()

		slog.Info("get settings requested", "remoteAddr", r.RemoteAddr)

		settings, err := database.GetAllSettings()
		if err != nil {
			slog.Error("failed to get settings", "error", err)
			writeError(w, http.StatusInternalServerError, config.ErrorDatabase, "failed to get settings")
			return
		}

		elapsed := time.Since(start).Milliseconds()

		slog.Info("settings fetched", "count", len(settings), "elapsed_ms", elapsed)

		writeJSON(w, http.StatusOK, models.APIResponse{
			Data: settings,
			Meta: &models.APIMeta{ExecutionTime: elapsed},
		})
	}
}

// UpdateSettings handles PUT /api/settings.
func UpdateSettings(database *db.DB) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		start := time.Now()

		slog.Info("update settings requested", "remoteAddr", r.RemoteAddr)

		var updates map[string]string
		if err := json.NewDecoder(r.Body).Decode(&updates); err != nil {
			slog.Warn("invalid settings request body", "error", err)
			writeError(w, http.StatusBadRequest, config.ErrorInvalidConfig, "invalid request body")
			return
		}

		slog.Debug("settings update payload", "keys", len(updates))

		// Validate and apply each setting.
		for key, value := range updates {
			if !validSettingKeys[key] {
				slog.Warn("unknown setting key", "key", key)
				writeError(w, http.StatusBadRequest, config.ErrorInvalidConfig, "unknown setting key: "+key)
				return
			}

			if err := database.SetSetting(key, value); err != nil {
				slog.Error("failed to update setting", "key", key, "error", err)
				writeError(w, http.StatusInternalServerError, config.ErrorDatabase, "failed to update setting: "+key)
				return
			}
		}

		// Return updated settings.
		settings, err := database.GetAllSettings()
		if err != nil {
			slog.Error("failed to get updated settings", "error", err)
			writeError(w, http.StatusInternalServerError, config.ErrorDatabase, "failed to get settings after update")
			return
		}

		elapsed := time.Since(start).Milliseconds()

		slog.Info("settings updated", "keys", len(updates), "elapsed_ms", elapsed)

		writeJSON(w, http.StatusOK, models.APIResponse{
			Data: settings,
			Meta: &models.APIMeta{ExecutionTime: elapsed},
		})
	}
}

// resetConfirmation is the expected request body for reset operations.
type resetConfirmation struct {
	Confirm bool `json:"confirm"`
}

// ResetBalancesHandler handles POST /api/settings/reset-balances.
func ResetBalancesHandler(database *db.DB) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		slog.Warn("reset balances requested", "remoteAddr", r.RemoteAddr)

		var body resetConfirmation
		if err := json.NewDecoder(r.Body).Decode(&body); err != nil || !body.Confirm {
			slog.Warn("reset balances not confirmed")
			writeError(w, http.StatusBadRequest, config.ErrorInvalidConfig, "confirmation required: {\"confirm\": true}")
			return
		}

		if err := database.ResetBalances(); err != nil {
			slog.Error("failed to reset balances", "error", err)
			writeError(w, http.StatusInternalServerError, config.ErrorDatabase, "failed to reset balances")
			return
		}

		slog.Info("balances reset successfully")

		writeJSON(w, http.StatusOK, models.APIResponse{
			Data: map[string]string{"message": "Balances, scan state, and transaction history cleared"},
		})
	}
}

// ResetAllHandler handles POST /api/settings/reset-all.
func ResetAllHandler(database *db.DB) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		slog.Warn("reset ALL data requested", "remoteAddr", r.RemoteAddr)

		var body resetConfirmation
		if err := json.NewDecoder(r.Body).Decode(&body); err != nil || !body.Confirm {
			slog.Warn("reset all not confirmed")
			writeError(w, http.StatusBadRequest, config.ErrorInvalidConfig, "confirmation required: {\"confirm\": true}")
			return
		}

		if err := database.ResetAll(); err != nil {
			slog.Error("failed to reset all data", "error", err)
			writeError(w, http.StatusInternalServerError, config.ErrorDatabase, "failed to reset all data")
			return
		}

		slog.Info("full data reset successfully")

		writeJSON(w, http.StatusOK, models.APIResponse{
			Data: map[string]string{"message": "All data cleared — addresses, balances, scan state, and transactions"},
		})
	}
}
