package pollerdb

import (
	"fmt"
	"log/slog"

	"github.com/Fantasim/hdpay/internal/poller/models"
)

// DashboardAggregates holds the top-level stats for the dashboard.
type DashboardAggregates struct {
	USDReceived    float64
	PointsAwarded  int
	UniqueAddresses int
	AvgTxUSD       float64
	LargestTxUSD   float64
	TxCount        int
}

// DashboardStats returns aggregated transaction stats for a date range.
// dateFrom and dateTo are in "YYYY-MM-DD" format. Empty string means unbounded.
func (d *DB) DashboardStats(dateFrom, dateTo string) (*DashboardAggregates, error) {
	query := `
		SELECT
			COALESCE(SUM(usd_value), 0),
			COALESCE(SUM(points), 0),
			COUNT(DISTINCT address),
			COALESCE(AVG(usd_value), 0),
			COALESCE(MAX(usd_value), 0),
			COUNT(*)
		FROM transactions
		WHERE status = 'CONFIRMED'
	`
	args := []interface{}{}

	if dateFrom != "" {
		query += " AND date(confirmed_at) >= ?"
		args = append(args, dateFrom)
	}
	if dateTo != "" {
		query += " AND date(confirmed_at) <= ?"
		args = append(args, dateTo)
	}

	var stats DashboardAggregates
	err := d.conn.QueryRow(query, args...).Scan(
		&stats.USDReceived,
		&stats.PointsAwarded,
		&stats.UniqueAddresses,
		&stats.AvgTxUSD,
		&stats.LargestTxUSD,
		&stats.TxCount,
	)
	if err != nil {
		return nil, fmt.Errorf("dashboard stats query: %w", err)
	}

	slog.Debug("dashboard stats queried",
		"dateFrom", dateFrom,
		"dateTo", dateTo,
		"txCount", stats.TxCount,
		"usdReceived", stats.USDReceived,
	)
	return &stats, nil
}

// DailyStats returns per-day aggregates for confirmed transactions in a date range.
func (d *DB) DailyStats(dateFrom, dateTo string) ([]models.DailyStatRow, error) {
	query := `
		SELECT
			date(confirmed_at) as day,
			COALESCE(SUM(usd_value), 0),
			COALESCE(SUM(points), 0),
			COUNT(*)
		FROM transactions
		WHERE status = 'CONFIRMED'
	`
	args := []interface{}{}

	if dateFrom != "" {
		query += " AND date(confirmed_at) >= ?"
		args = append(args, dateFrom)
	}
	if dateTo != "" {
		query += " AND date(confirmed_at) <= ?"
		args = append(args, dateTo)
	}

	query += " GROUP BY day ORDER BY day ASC"

	rows, err := d.conn.Query(query, args...)
	if err != nil {
		return nil, fmt.Errorf("daily stats query: %w", err)
	}
	defer rows.Close()

	var result []models.DailyStatRow
	for rows.Next() {
		var row models.DailyStatRow
		if err := rows.Scan(&row.Date, &row.USD, &row.Points, &row.TxCount); err != nil {
			return nil, fmt.Errorf("daily stats scan: %w", err)
		}
		result = append(result, row)
	}

	slog.Debug("daily stats queried", "dateFrom", dateFrom, "dateTo", dateTo, "days", len(result))
	return result, rows.Err()
}

// WatchStatsResult holds watch counts by status.
type WatchStatsResult struct {
	Total     int `json:"total_watches"`
	Completed int `json:"watches_completed"`
	Expired   int `json:"watches_expired"`
	Cancelled int `json:"watches_cancelled"`
}

// WatchStats returns watch counts by status for a date range (based on started_at).
func (d *DB) WatchStats(dateFrom, dateTo string) (*WatchStatsResult, error) {
	query := `
		SELECT
			COUNT(*),
			COALESCE(SUM(CASE WHEN status = 'COMPLETED' THEN 1 ELSE 0 END), 0),
			COALESCE(SUM(CASE WHEN status = 'EXPIRED' THEN 1 ELSE 0 END), 0),
			COALESCE(SUM(CASE WHEN status = 'CANCELLED' THEN 1 ELSE 0 END), 0)
		FROM watches
		WHERE 1=1
	`
	args := []interface{}{}

	if dateFrom != "" {
		query += " AND date(started_at) >= ?"
		args = append(args, dateFrom)
	}
	if dateTo != "" {
		query += " AND date(started_at) <= ?"
		args = append(args, dateTo)
	}

	var stats WatchStatsResult
	err := d.conn.QueryRow(query, args...).Scan(
		&stats.Total,
		&stats.Completed,
		&stats.Expired,
		&stats.Cancelled,
	)
	if err != nil {
		return nil, fmt.Errorf("watch stats query: %w", err)
	}

	slog.Debug("watch stats queried", "dateFrom", dateFrom, "dateTo", dateTo, "total", stats.Total)
	return &stats, nil
}

// PendingPointsSummary returns the number of accounts with pending points and total pending.
func (d *DB) PendingPointsSummary() (accounts int, total int, err error) {
	err = d.conn.QueryRow(`
		SELECT COUNT(*), COALESCE(SUM(pending), 0) FROM points WHERE pending > 0
	`).Scan(&accounts, &total)
	if err != nil {
		err = fmt.Errorf("pending points summary: %w", err)
	}
	return
}

// ChartByChain returns transaction aggregates grouped by chain.
func (d *DB) ChartByChain(dateFrom, dateTo string) ([]models.ChainBreakdown, error) {
	query := `
		SELECT chain, COALESCE(SUM(usd_value), 0), COUNT(*)
		FROM transactions
		WHERE status = 'CONFIRMED'
	`
	args := []interface{}{}

	if dateFrom != "" {
		query += " AND date(confirmed_at) >= ?"
		args = append(args, dateFrom)
	}
	if dateTo != "" {
		query += " AND date(confirmed_at) <= ?"
		args = append(args, dateTo)
	}

	query += " GROUP BY chain ORDER BY chain"

	rows, err := d.conn.Query(query, args...)
	if err != nil {
		return nil, fmt.Errorf("chart by chain query: %w", err)
	}
	defer rows.Close()

	var result []models.ChainBreakdown
	for rows.Next() {
		var row models.ChainBreakdown
		if err := rows.Scan(&row.Chain, &row.USD, &row.Count); err != nil {
			return nil, fmt.Errorf("chart by chain scan: %w", err)
		}
		result = append(result, row)
	}
	return result, rows.Err()
}

// ChartByToken returns transaction aggregates grouped by token.
func (d *DB) ChartByToken(dateFrom, dateTo string) ([]models.TokenBreakdown, error) {
	query := `
		SELECT token, COALESCE(SUM(usd_value), 0), COUNT(*)
		FROM transactions
		WHERE status = 'CONFIRMED'
	`
	args := []interface{}{}

	if dateFrom != "" {
		query += " AND date(confirmed_at) >= ?"
		args = append(args, dateFrom)
	}
	if dateTo != "" {
		query += " AND date(confirmed_at) <= ?"
		args = append(args, dateTo)
	}

	query += " GROUP BY token ORDER BY token"

	rows, err := d.conn.Query(query, args...)
	if err != nil {
		return nil, fmt.Errorf("chart by token query: %w", err)
	}
	defer rows.Close()

	var result []models.TokenBreakdown
	for rows.Next() {
		var row models.TokenBreakdown
		if err := rows.Scan(&row.Token, &row.USD, &row.Count); err != nil {
			return nil, fmt.Errorf("chart by token scan: %w", err)
		}
		result = append(result, row)
	}
	return result, rows.Err()
}

// ChartByTier returns transaction aggregates grouped by tier.
func (d *DB) ChartByTier(dateFrom, dateTo string) ([]models.TierBreakdown, error) {
	query := `
		SELECT tier, COUNT(*), COALESCE(SUM(points), 0)
		FROM transactions
		WHERE status = 'CONFIRMED'
	`
	args := []interface{}{}

	if dateFrom != "" {
		query += " AND date(confirmed_at) >= ?"
		args = append(args, dateFrom)
	}
	if dateTo != "" {
		query += " AND date(confirmed_at) <= ?"
		args = append(args, dateTo)
	}

	query += " GROUP BY tier ORDER BY tier"

	rows, err := d.conn.Query(query, args...)
	if err != nil {
		return nil, fmt.Errorf("chart by tier query: %w", err)
	}
	defer rows.Close()

	var result []models.TierBreakdown
	for rows.Next() {
		var row models.TierBreakdown
		if err := rows.Scan(&row.Tier, &row.Count, &row.TotalPoints); err != nil {
			return nil, fmt.Errorf("chart by tier scan: %w", err)
		}
		result = append(result, row)
	}
	return result, rows.Err()
}

// ChartWatchesByDay returns daily watch status counts.
func (d *DB) ChartWatchesByDay(dateFrom, dateTo string) ([]models.DailyWatchStat, error) {
	query := `
		SELECT
			date(started_at) as day,
			COALESCE(SUM(CASE WHEN status = 'ACTIVE' THEN 1 ELSE 0 END), 0),
			COALESCE(SUM(CASE WHEN status = 'COMPLETED' THEN 1 ELSE 0 END), 0),
			COALESCE(SUM(CASE WHEN status = 'EXPIRED' THEN 1 ELSE 0 END), 0)
		FROM watches
		WHERE 1=1
	`
	args := []interface{}{}

	if dateFrom != "" {
		query += " AND date(started_at) >= ?"
		args = append(args, dateFrom)
	}
	if dateTo != "" {
		query += " AND date(started_at) <= ?"
		args = append(args, dateTo)
	}

	query += " GROUP BY day ORDER BY day ASC"

	rows, err := d.conn.Query(query, args...)
	if err != nil {
		return nil, fmt.Errorf("chart watches by day query: %w", err)
	}
	defer rows.Close()

	var result []models.DailyWatchStat
	for rows.Next() {
		var row models.DailyWatchStat
		if err := rows.Scan(&row.Date, &row.Active, &row.Completed, &row.Expired); err != nil {
			return nil, fmt.Errorf("chart watches by day scan: %w", err)
		}
		result = append(result, row)
	}
	return result, rows.Err()
}
