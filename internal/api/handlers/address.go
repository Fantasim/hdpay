package handlers

import (
	"encoding/json"
	"log/slog"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/Fantasim/hdpay/internal/config"
	"github.com/Fantasim/hdpay/internal/db"
	"github.com/Fantasim/hdpay/internal/models"
	"github.com/go-chi/chi/v5"
)

// ListAddresses handles GET /api/addresses/{chain}
func ListAddresses(database *db.DB) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		start := time.Now()
		chainParam := strings.ToUpper(chi.URLParam(r, "chain"))

		slog.Info("list addresses requested",
			"chain", chainParam,
			"query", r.URL.RawQuery,
			"remoteAddr", r.RemoteAddr,
		)

		// Validate chain
		chain := models.Chain(chainParam)
		if !isValidChain(chain) {
			slog.Warn("invalid chain parameter", "chain", chainParam)
			writeError(w, http.StatusBadRequest, config.ErrorInvalidChain, "invalid chain: "+chainParam+", must be BTC, BSC, or SOL")
			return
		}

		// Parse query params
		page := parseIntParam(r, "page", config.DefaultPage)
		pageSize := parseIntParam(r, "pageSize", config.DefaultPageSize)
		hasBalance := r.URL.Query().Get("hasBalance") == "true"
		token := strings.ToUpper(r.URL.Query().Get("token"))

		// Clamp page size
		if pageSize > config.MaxPageSize {
			pageSize = config.MaxPageSize
		}
		if pageSize < 1 {
			pageSize = config.DefaultPageSize
		}
		if page < 1 {
			page = config.DefaultPage
		}

		// Validate token filter
		if token != "" && token != "NATIVE" && token != "USDC" && token != "USDT" {
			slog.Warn("invalid token parameter", "token", token)
			writeError(w, http.StatusBadRequest, config.ErrorInvalidToken, "invalid token: "+token+", must be NATIVE, USDC, or USDT")
			return
		}

		slog.Debug("parsed address list params",
			"chain", chain,
			"page", page,
			"pageSize", pageSize,
			"hasBalance", hasBalance,
			"token", token,
		)

		filter := db.AddressFilter{
			Chain:      chain,
			Page:       page,
			PageSize:   pageSize,
			HasBalance: hasBalance,
			Token:      token,
		}

		addresses, total, err := database.GetAddressesWithBalances(filter)
		if err != nil {
			slog.Error("failed to fetch addresses",
				"chain", chain,
				"error", err,
			)
			writeError(w, http.StatusInternalServerError, config.ErrorDatabase, "failed to fetch addresses")
			return
		}

		elapsed := time.Since(start).Milliseconds()

		slog.Info("addresses fetched",
			"chain", chain,
			"page", page,
			"pageSize", pageSize,
			"returned", len(addresses),
			"total", total,
			"elapsed_ms", elapsed,
		)

		writeJSON(w, http.StatusOK, models.APIResponse{
			Data: addresses,
			Meta: &models.APIMeta{
				Page:          page,
				PageSize:      pageSize,
				Total:         total,
				ExecutionTime: elapsed,
			},
		})
	}
}

// ExportAddresses handles GET /api/addresses/{chain}/export
func ExportAddresses(database *db.DB) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		chainParam := strings.ToUpper(chi.URLParam(r, "chain"))

		slog.Info("export addresses requested",
			"chain", chainParam,
			"remoteAddr", r.RemoteAddr,
		)

		chain := models.Chain(chainParam)
		if !isValidChain(chain) {
			slog.Warn("invalid chain for export", "chain", chainParam)
			writeError(w, http.StatusBadRequest, config.ErrorInvalidChain, "invalid chain: "+chainParam)
			return
		}

		w.Header().Set("Content-Type", "application/json")
		w.Header().Set("Content-Disposition", "attachment; filename="+string(chain)+"-addresses.json")

		// Stream addresses as JSON array
		first := true

		w.Write([]byte("["))

		err := database.StreamAddresses(chain, func(addr models.Address) error {
			if !first {
				w.Write([]byte(","))
			}
			first = false
			item := models.AddressExportItem{
				Index:   addr.AddressIndex,
				Address: addr.Address,
			}
			data, err := json.Marshal(item)
			if err != nil {
				return err
			}
			_, err = w.Write(data)
			return err
		})

		if err != nil {
			slog.Error("export stream error",
				"chain", chain,
				"error", err,
			)
			// Can't change status code mid-stream, just log
			return
		}

		w.Write([]byte("]"))

		slog.Info("address export complete", "chain", chain)
	}
}

// isValidChain checks if the chain is one of the supported chains.
func isValidChain(chain models.Chain) bool {
	for _, c := range models.AllChains {
		if c == chain {
			return true
		}
	}
	return false
}

// parseIntParam extracts an integer query parameter with a default value.
func parseIntParam(r *http.Request, key string, defaultVal int) int {
	val := r.URL.Query().Get(key)
	if val == "" {
		return defaultVal
	}
	n, err := strconv.Atoi(val)
	if err != nil {
		slog.Debug("invalid int param, using default",
			"key", key,
			"value", val,
			"default", defaultVal,
		)
		return defaultVal
	}
	return n
}

// writeJSON writes a JSON response with the given status code.
func writeJSON(w http.ResponseWriter, status int, data interface{}) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	if err := json.NewEncoder(w).Encode(data); err != nil {
		slog.Error("failed to write JSON response", "error", err)
	}
}

// writeError writes a JSON error response.
func writeError(w http.ResponseWriter, status int, code, message string) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	json.NewEncoder(w).Encode(models.APIError{
		Error: models.APIErrorDetail{
			Code:    code,
			Message: message,
		},
	})
}
