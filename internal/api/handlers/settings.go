package handlers

import (
	"encoding/json"
	"fmt"
	"log/slog"
	"net/http"
	"strconv"
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
}

// validateSettingValue validates a setting value for a given key.
func validateSettingValue(key, value string) error {
	switch key {
	case "max_scan_id":
		n, err := strconv.Atoi(value)
		if err != nil {
			return fmt.Errorf("max_scan_id must be a number, got %q", value)
		}
		if n < 1 || n > config.MaxAddressesPerChain {
			return fmt.Errorf("max_scan_id must be between 1 and %d, got %d", config.MaxAddressesPerChain, n)
		}
	case "btc_fee_rate":
		n, err := strconv.Atoi(value)
		if err != nil {
			return fmt.Errorf("btc_fee_rate must be a number, got %q", value)
		}
		if n < 0 {
			return fmt.Errorf("btc_fee_rate must be non-negative, got %d", n)
		}
	case "resume_threshold_hours":
		n, err := strconv.Atoi(value)
		if err != nil {
			return fmt.Errorf("resume_threshold_hours must be a number, got %q", value)
		}
		if n < 1 {
			return fmt.Errorf("resume_threshold_hours must be at least 1, got %d", n)
		}
	}
	return nil
}

// GetSettings handles GET /api/settings.
// Includes the read-only "network" field from server config.
func GetSettings(database *db.DB, cfg *config.Config) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		start := time.Now()

		slog.Info("get settings requested", "remoteAddr", r.RemoteAddr)

		settings, err := database.GetAllSettings()
		if err != nil {
			slog.Error("failed to get settings", "error", err)
			writeError(w, http.StatusInternalServerError, config.ErrorDatabase, "failed to get settings")
			return
		}

		// Read-only: network comes from env config, not DB.
		settings["network"] = cfg.Network

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

			// Value validation.
			if err := validateSettingValue(key, value); err != nil {
				slog.Warn("invalid setting value", "key", key, "value", value, "error", err)
				writeError(w, http.StatusBadRequest, config.ErrorInvalidConfig, err.Error())
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

