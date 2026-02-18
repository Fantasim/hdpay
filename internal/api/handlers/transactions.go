package handlers

import (
	"log/slog"
	"net/http"
	"strings"
	"time"

	"github.com/Fantasim/hdpay/internal/config"
	"github.com/Fantasim/hdpay/internal/db"
	"github.com/Fantasim/hdpay/internal/models"
	"github.com/go-chi/chi/v5"
)

// ListTransactions handles GET /api/transactions and GET /api/transactions/{chain}.
func ListTransactions(database *db.DB) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		start := time.Now()

		chainParam := strings.ToUpper(chi.URLParam(r, "chain"))

		slog.Info("list transactions requested",
			"chain", chainParam,
			"query", r.URL.RawQuery,
			"remoteAddr", r.RemoteAddr,
		)

		// Build filter from query params.
		filter := db.TransactionFilter{
			Page:     parseIntParam(r, "page", config.DefaultPage),
			PageSize: parseIntParam(r, "pageSize", config.DefaultPageSize),
		}

		// Clamp page size.
		if filter.PageSize > config.MaxPageSize {
			filter.PageSize = config.MaxPageSize
		}
		if filter.PageSize < 1 {
			filter.PageSize = config.DefaultPageSize
		}
		if filter.Page < 1 {
			filter.Page = config.DefaultPage
		}

		// Chain filter — from path param or query param.
		if chainParam == "" {
			chainParam = strings.ToUpper(r.URL.Query().Get("chain"))
		}
		if chainParam != "" {
			chain := models.Chain(chainParam)
			if !isValidChain(chain) {
				slog.Warn("invalid chain parameter", "chain", chainParam)
				writeError(w, http.StatusBadRequest, config.ErrorInvalidChain, "invalid chain: "+chainParam+", must be BTC, BSC, or SOL")
				return
			}
			filter.Chain = &chain
		}

		// Direction filter.
		direction := strings.ToLower(r.URL.Query().Get("direction"))
		if direction != "" {
			if direction != "in" && direction != "out" {
				slog.Warn("invalid direction parameter", "direction", direction)
				writeError(w, http.StatusBadRequest, config.ErrorInvalidConfig, "invalid direction: "+direction+", must be in or out")
				return
			}
			filter.Direction = &direction
		}

		// Token filter.
		token := strings.ToUpper(r.URL.Query().Get("token"))
		if token != "" {
			if token != "NATIVE" && token != "USDC" && token != "USDT" {
				slog.Warn("invalid token parameter", "token", token)
				writeError(w, http.StatusBadRequest, config.ErrorInvalidToken, "invalid token: "+token+", must be NATIVE, USDC, or USDT")
				return
			}
			t := models.Token(token)
			filter.Token = &t
		}

		// Status filter.
		status := strings.ToLower(r.URL.Query().Get("status"))
		if status != "" {
			if status != "pending" && status != "confirmed" && status != "failed" {
				slog.Warn("invalid status parameter", "status", status)
				writeError(w, http.StatusBadRequest, config.ErrorInvalidConfig, "invalid status: "+status+", must be pending, confirmed, or failed")
				return
			}
			filter.Status = &status
		}

		slog.Debug("parsed transaction list params",
			"chain", filter.Chain,
			"direction", filter.Direction,
			"token", filter.Token,
			"status", filter.Status,
			"page", filter.Page,
			"pageSize", filter.PageSize,
		)

		txs, total, err := database.ListTransactionsFiltered(filter)
		if err != nil {
			slog.Error("failed to fetch transactions",
				"error", err,
			)
			writeError(w, http.StatusInternalServerError, config.ErrorDatabase, "failed to fetch transactions")
			return
		}

		elapsed := time.Since(start).Milliseconds()

		slog.Info("transactions fetched",
			"chain", filter.Chain,
			"page", filter.Page,
			"pageSize", filter.PageSize,
			"returned", len(txs),
			"total", total,
			"elapsed_ms", elapsed,
		)

		writeJSON(w, http.StatusOK, models.APIResponse{
			Data: txs,
			Meta: &models.APIMeta{
				Page:          filter.Page,
				PageSize:      filter.PageSize,
				Total:         total,
				ExecutionTime: elapsed,
			},
		})
	}
}
