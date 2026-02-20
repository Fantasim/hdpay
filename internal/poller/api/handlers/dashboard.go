package handlers

import (
	"fmt"
	"log/slog"
	"net/http"
	"strconv"
	"time"

	"github.com/Fantasim/hdpay/internal/poller/httputil"
	pollerconfig "github.com/Fantasim/hdpay/internal/poller/config"
	"github.com/Fantasim/hdpay/internal/poller/models"
	"github.com/Fantasim/hdpay/internal/poller/pollerdb"
	"github.com/Fantasim/hdpay/internal/poller/watcher"
)

// DashboardStatsHandler returns a handler for GET /api/dashboard/stats?range=week.
func DashboardStatsHandler(db *pollerdb.DB, w *watcher.Watcher) http.HandlerFunc {
	return func(rw http.ResponseWriter, r *http.Request) {
		timeRange := r.URL.Query().Get("range")
		if timeRange == "" {
			timeRange = "all"
		}

		dateFrom, dateTo, err := resolveDateRange(timeRange)
		if err != nil {
			httputil.Error(rw, http.StatusBadRequest, pollerconfig.ErrorInvalidRequest, err.Error())
			return
		}

		slog.Debug("dashboard stats requested", "range", timeRange, "dateFrom", dateFrom, "dateTo", dateTo)

		// Run aggregate queries.
		txStats, err := db.DashboardStats(dateFrom, dateTo)
		if err != nil {
			slog.Error("dashboard stats failed", "error", err)
			httputil.Error(rw, http.StatusInternalServerError, pollerconfig.ErrorDatabase, "Failed to query stats")
			return
		}

		watchStats, err := db.WatchStats(dateFrom, dateTo)
		if err != nil {
			slog.Error("dashboard watch stats failed", "error", err)
			httputil.Error(rw, http.StatusInternalServerError, pollerconfig.ErrorDatabase, "Failed to query watch stats")
			return
		}

		pendingAccounts, pendingTotal, err := db.PendingPointsSummary()
		if err != nil {
			slog.Error("dashboard pending summary failed", "error", err)
			httputil.Error(rw, http.StatusInternalServerError, pollerconfig.ErrorDatabase, "Failed to query pending points")
			return
		}

		byDay, err := db.DailyStats(dateFrom, dateTo)
		if err != nil {
			slog.Error("dashboard daily stats failed", "error", err)
			httputil.Error(rw, http.StatusInternalServerError, pollerconfig.ErrorDatabase, "Failed to query daily stats")
			return
		}

		if byDay == nil {
			byDay = []models.DailyStatRow{}
		}

		httputil.JSON(rw, http.StatusOK, map[string]interface{}{
			"range":            timeRange,
			"active_watches":   w.ActiveCount(),
			"total_watches":    watchStats.Total,
			"watches_completed": watchStats.Completed,
			"watches_expired":  watchStats.Expired,
			"usd_received":     txStats.USDReceived,
			"points_awarded":   txStats.PointsAwarded,
			"pending_points": map[string]int{
				"accounts": pendingAccounts,
				"total":    pendingTotal,
			},
			"unique_addresses": txStats.UniqueAddresses,
			"avg_tx_usd":       txStats.AvgTxUSD,
			"largest_tx_usd":   txStats.LargestTxUSD,
			"by_day":           byDay,
		})
	}
}

// DashboardTransactionsHandler returns a handler for GET /api/dashboard/transactions.
func DashboardTransactionsHandler(db *pollerdb.DB) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		q := r.URL.Query()

		filters := models.TransactionFilters{}
		if v := q.Get("chain"); v != "" {
			filters.Chain = &v
		}
		if v := q.Get("token"); v != "" {
			filters.Token = &v
		}
		if v := q.Get("status"); v != "" {
			s := models.TxStatus(v)
			filters.Status = &s
		}
		if v := q.Get("tier"); v != "" {
			if t, err := strconv.Atoi(v); err == nil {
				filters.Tier = &t
			}
		}
		if v := q.Get("min_usd"); v != "" {
			if f, err := strconv.ParseFloat(v, 64); err == nil {
				filters.MinUSD = &f
			}
		}
		if v := q.Get("max_usd"); v != "" {
			if f, err := strconv.ParseFloat(v, 64); err == nil {
				filters.MaxUSD = &f
			}
		}
		if v := q.Get("date_from"); v != "" {
			filters.DateFrom = &v
		}
		if v := q.Get("date_to"); v != "" {
			filters.DateTo = &v
		}

		page, pageSize := parsePagination(q.Get("page"), q.Get("page_size"))

		pag := models.Pagination{
			Page:     page,
			PageSize: pageSize,
		}

		txs, total, err := db.ListAll(filters, pag)
		if err != nil {
			slog.Error("dashboard transactions failed", "error", err)
			httputil.Error(w, http.StatusInternalServerError, pollerconfig.ErrorDatabase, "Failed to query transactions")
			return
		}

		if txs == nil {
			txs = []models.Transaction{}
		}

		slog.Debug("dashboard transactions listed", "count", len(txs), "total", total, "page", page)
		httputil.JSONList(w, txs, page, pageSize, total)
	}
}

// DashboardChartsHandler returns a handler for GET /api/dashboard/charts?range=week.
func DashboardChartsHandler(db *pollerdb.DB) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		timeRange := r.URL.Query().Get("range")
		if timeRange == "" {
			timeRange = "all"
		}

		dateFrom, dateTo, err := resolveDateRange(timeRange)
		if err != nil {
			httputil.Error(w, http.StatusBadRequest, pollerconfig.ErrorInvalidRequest, err.Error())
			return
		}

		slog.Debug("dashboard charts requested", "range", timeRange)

		// All chart data runs in sequence (same DB, WAL mode handles concurrent reads).
		dailyStats, err := db.DailyStats(dateFrom, dateTo)
		if err != nil {
			slog.Error("chart daily stats failed", "error", err)
			httputil.Error(w, http.StatusInternalServerError, pollerconfig.ErrorDatabase, "Failed to query chart data")
			return
		}

		byChain, err := db.ChartByChain(dateFrom, dateTo)
		if err != nil {
			slog.Error("chart by chain failed", "error", err)
			httputil.Error(w, http.StatusInternalServerError, pollerconfig.ErrorDatabase, "Failed to query chart data")
			return
		}

		byToken, err := db.ChartByToken(dateFrom, dateTo)
		if err != nil {
			slog.Error("chart by token failed", "error", err)
			httputil.Error(w, http.StatusInternalServerError, pollerconfig.ErrorDatabase, "Failed to query chart data")
			return
		}

		byTier, err := db.ChartByTier(dateFrom, dateTo)
		if err != nil {
			slog.Error("chart by tier failed", "error", err)
			httputil.Error(w, http.StatusInternalServerError, pollerconfig.ErrorDatabase, "Failed to query chart data")
			return
		}

		watchesByDay, err := db.ChartWatchesByDay(dateFrom, dateTo)
		if err != nil {
			slog.Error("chart watches by day failed", "error", err)
			httputil.Error(w, http.StatusInternalServerError, pollerconfig.ErrorDatabase, "Failed to query chart data")
			return
		}

		// Build the 7 datasets from daily stats.
		usdOverTime := make([]map[string]interface{}, 0, len(dailyStats))
		pointsOverTime := make([]map[string]interface{}, 0, len(dailyStats))
		txCountOverTime := make([]map[string]interface{}, 0, len(dailyStats))
		for _, d := range dailyStats {
			usdOverTime = append(usdOverTime, map[string]interface{}{"date": d.Date, "usd": d.USD})
			pointsOverTime = append(pointsOverTime, map[string]interface{}{"date": d.Date, "points": d.Points})
			txCountOverTime = append(txCountOverTime, map[string]interface{}{"date": d.Date, "count": d.TxCount})
		}

		if byChain == nil {
			byChain = []models.ChainBreakdown{}
		}
		if byToken == nil {
			byToken = []models.TokenBreakdown{}
		}
		if byTier == nil {
			byTier = []models.TierBreakdown{}
		}
		if watchesByDay == nil {
			watchesByDay = []models.DailyWatchStat{}
		}

		httputil.JSON(w, http.StatusOK, map[string]interface{}{
			"usd_over_time":      usdOverTime,
			"points_over_time":   pointsOverTime,
			"tx_count_over_time": txCountOverTime,
			"by_chain":           byChain,
			"by_token":           byToken,
			"by_tier":            byTier,
			"watches_over_time":  watchesByDay,
		})
	}
}

// DashboardErrorsHandler returns a handler for GET /api/dashboard/errors.
// Runs the 5 discrepancy checks on demand.
func DashboardErrorsHandler(db *pollerdb.DB) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		slog.Debug("dashboard errors requested")

		// Collect all discrepancies.
		var allDiscrepancies []models.DiscrepancyRow

		mismatch, err := db.CheckPointsMismatch()
		if err != nil {
			slog.Error("discrepancy check failed: points mismatch", "error", err)
		} else {
			allDiscrepancies = append(allDiscrepancies, mismatch...)
		}

		unclaimed, err := db.CheckUnclaimedExceedsTotal()
		if err != nil {
			slog.Error("discrepancy check failed: unclaimed exceeds total", "error", err)
		} else {
			allDiscrepancies = append(allDiscrepancies, unclaimed...)
		}

		orphaned, err := db.CheckOrphanedTransactions()
		if err != nil {
			slog.Error("discrepancy check failed: orphaned transactions", "error", err)
		} else {
			allDiscrepancies = append(allDiscrepancies, orphaned...)
		}

		// Stale pending.
		stalePending, err := db.CheckStalePending()
		if err != nil {
			slog.Error("discrepancy check failed: stale pending", "error", err)
			stalePending = []models.StalePendingRow{}
		}

		// System errors (unresolved).
		sysErrors, err := db.ListUnresolved()
		if err != nil {
			slog.Error("failed to list system errors", "error", err)
			sysErrors = []models.SystemError{}
		}

		if allDiscrepancies == nil {
			allDiscrepancies = []models.DiscrepancyRow{}
		}
		if sysErrors == nil {
			sysErrors = []models.SystemError{}
		}
		if stalePending == nil {
			stalePending = []models.StalePendingRow{}
		}

		slog.Info("dashboard errors check completed",
			"discrepancies", len(allDiscrepancies),
			"stalePending", len(stalePending),
			"systemErrors", len(sysErrors),
		)

		httputil.JSON(w, http.StatusOK, map[string]interface{}{
			"discrepancies":  allDiscrepancies,
			"errors":         sysErrors,
			"stale_pending":  stalePending,
		})
	}
}

// resolveDateRange converts a time range label to dateFrom/dateTo in "YYYY-MM-DD" format.
func resolveDateRange(timeRange string) (dateFrom, dateTo string, err error) {
	now := time.Now().UTC()
	dateTo = now.Format("2006-01-02")

	switch timeRange {
	case "today":
		dateFrom = dateTo
	case "week":
		dateFrom = now.AddDate(0, 0, -7).Format("2006-01-02")
	case "month":
		dateFrom = now.AddDate(0, 0, -30).Format("2006-01-02")
	case "quarter":
		dateFrom = now.AddDate(0, 0, -90).Format("2006-01-02")
	case "all":
		dateFrom = ""
		dateTo = ""
	default:
		return "", "", fmt.Errorf("invalid range: %s. Must be today, week, month, quarter, or all", timeRange)
	}

	return dateFrom, dateTo, nil
}

// parsePagination extracts page and page_size from query params with defaults.
func parsePagination(pageStr, pageSizeStr string) (int, int) {
	page := 1
	if v, err := strconv.Atoi(pageStr); err == nil && v > 0 {
		page = v
	}

	pageSize := pollerconfig.DefaultPageSize
	if v, err := strconv.Atoi(pageSizeStr); err == nil && v > 0 {
		if v > pollerconfig.MaxPageSize {
			v = pollerconfig.MaxPageSize
		}
		pageSize = v
	}

	return page, pageSize
}
