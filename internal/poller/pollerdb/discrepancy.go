package pollerdb

import (
	"fmt"
	"log/slog"
	"math"

	"github.com/Fantasim/hdpay/internal/poller/config"
	"github.com/Fantasim/hdpay/internal/poller/models"
)

// CheckPointsMismatch finds addresses where SUM(confirmed tx points) != stored total.
func (d *DB) CheckPointsMismatch() ([]models.DiscrepancyRow, error) {
	query := `
		SELECT t.address, t.chain,
			SUM(t.points) as calculated_total,
			p.total as stored_total
		FROM transactions t
		JOIN points p ON t.address = p.address AND t.chain = p.chain
		WHERE t.status = 'CONFIRMED'
		GROUP BY t.address, t.chain
		HAVING calculated_total != stored_total
	`

	rows, err := d.conn.Query(query)
	if err != nil {
		return nil, fmt.Errorf("check points mismatch: %w", err)
	}
	defer rows.Close()

	var result []models.DiscrepancyRow
	for rows.Next() {
		var addr, chain string
		var calculated, stored int
		if err := rows.Scan(&addr, &chain, &calculated, &stored); err != nil {
			return nil, fmt.Errorf("check points mismatch scan: %w", err)
		}
		result = append(result, models.DiscrepancyRow{
			Type:       "POINTS_MISMATCH",
			Address:    addr,
			Chain:      chain,
			Message:    fmt.Sprintf("Points total mismatch: calculated %d, stored %d", calculated, stored),
			Calculated: calculated,
			Stored:     stored,
		})
	}

	slog.Debug("discrepancy check: points mismatch", "found", len(result))
	return result, rows.Err()
}

// CheckUnclaimedExceedsTotal finds addresses where unclaimed > total.
func (d *DB) CheckUnclaimedExceedsTotal() ([]models.DiscrepancyRow, error) {
	query := `
		SELECT address, chain, unclaimed, total
		FROM points
		WHERE unclaimed > total
	`

	rows, err := d.conn.Query(query)
	if err != nil {
		return nil, fmt.Errorf("check unclaimed exceeds total: %w", err)
	}
	defer rows.Close()

	var result []models.DiscrepancyRow
	for rows.Next() {
		var addr, chain string
		var unclaimed, total int
		if err := rows.Scan(&addr, &chain, &unclaimed, &total); err != nil {
			return nil, fmt.Errorf("check unclaimed exceeds total scan: %w", err)
		}
		result = append(result, models.DiscrepancyRow{
			Type:       "UNCLAIMED_EXCEEDS_TOTAL",
			Address:    addr,
			Chain:      chain,
			Message:    fmt.Sprintf("Unclaimed (%d) exceeds total (%d)", unclaimed, total),
			Calculated: unclaimed,
			Stored:     total,
		})
	}

	slog.Debug("discrepancy check: unclaimed exceeds total", "found", len(result))
	return result, rows.Err()
}

// CheckOrphanedTransactions finds transactions referencing non-existent watches.
func (d *DB) CheckOrphanedTransactions() ([]models.DiscrepancyRow, error) {
	query := `
		SELECT t.id, t.tx_hash, t.watch_id
		FROM transactions t
		LEFT JOIN watches w ON t.watch_id = w.id
		WHERE w.id IS NULL
	`

	rows, err := d.conn.Query(query)
	if err != nil {
		return nil, fmt.Errorf("check orphaned transactions: %w", err)
	}
	defer rows.Close()

	var result []models.DiscrepancyRow
	for rows.Next() {
		var id int
		var txHash, watchID string
		if err := rows.Scan(&id, &txHash, &watchID); err != nil {
			return nil, fmt.Errorf("check orphaned transactions scan: %w", err)
		}
		result = append(result, models.DiscrepancyRow{
			Type:    "ORPHANED_TRANSACTION",
			Message: fmt.Sprintf("Transaction %s references non-existent watch %s", txHash, watchID),
		})
	}

	slog.Debug("discrepancy check: orphaned transactions", "found", len(result))
	return result, rows.Err()
}

// CheckStalePending finds PENDING transactions older than the stale threshold (24h).
func (d *DB) CheckStalePending() ([]models.StalePendingRow, error) {
	thresholdHours := config.StalePendingThreshold.Hours()

	query := `
		SELECT tx_hash, chain, address, detected_at,
			(julianday('now') - julianday(detected_at)) * 24 as hours_pending
		FROM transactions
		WHERE status = 'PENDING'
		AND detected_at < datetime('now', '-' || ? || ' hours')
	`

	rows, err := d.conn.Query(query, int(thresholdHours))
	if err != nil {
		return nil, fmt.Errorf("check stale pending: %w", err)
	}
	defer rows.Close()

	var result []models.StalePendingRow
	for rows.Next() {
		var row models.StalePendingRow
		if err := rows.Scan(&row.TxHash, &row.Chain, &row.Address, &row.DetectedAt, &row.HoursPending); err != nil {
			return nil, fmt.Errorf("check stale pending scan: %w", err)
		}
		row.HoursPending = math.Round(row.HoursPending*10) / 10 // 1 decimal
		result = append(result, row)
	}

	slog.Debug("discrepancy check: stale pending", "found", len(result))
	return result, rows.Err()
}
