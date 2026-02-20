package pollerdb

import (
	"database/sql"
	"fmt"
	"log/slog"

	"github.com/Fantasim/hdpay/internal/poller/models"
)

// CreateWatch inserts a new watch into the database.
func (d *DB) CreateWatch(w *models.Watch) error {
	_, err := d.conn.Exec(`
		INSERT INTO watches (id, chain, address, status, started_at, expires_at, poll_count, created_at)
		VALUES (?, ?, ?, ?, ?, ?, 0, CURRENT_TIMESTAMP)`,
		w.ID, w.Chain, w.Address, w.Status, w.StartedAt, w.ExpiresAt,
	)
	if err != nil {
		return fmt.Errorf("failed to insert watch %s: %w", w.ID, err)
	}

	slog.Info("watch created",
		"watchID", w.ID,
		"chain", w.Chain,
		"address", w.Address,
		"expiresAt", w.ExpiresAt,
	)
	return nil
}

// GetWatch retrieves a watch by ID.
func (d *DB) GetWatch(id string) (*models.Watch, error) {
	w := &models.Watch{}
	err := d.conn.QueryRow(`
		SELECT id, chain, address, status, started_at, expires_at, completed_at,
		       poll_count, last_poll_at, last_poll_result, created_at
		FROM watches WHERE id = ?`, id,
	).Scan(
		&w.ID, &w.Chain, &w.Address, &w.Status, &w.StartedAt, &w.ExpiresAt,
		&w.CompletedAt, &w.PollCount, &w.LastPollAt, &w.LastPollResult, &w.CreatedAt,
	)
	if err == sql.ErrNoRows {
		return nil, nil
	}
	if err != nil {
		return nil, fmt.Errorf("failed to get watch %s: %w", id, err)
	}
	return w, nil
}

// ListWatches retrieves watches with optional filters.
func (d *DB) ListWatches(filters models.WatchFilters) ([]models.Watch, error) {
	query := `SELECT id, chain, address, status, started_at, expires_at, completed_at,
	                 poll_count, last_poll_at, last_poll_result, created_at
	          FROM watches WHERE 1=1`
	var args []interface{}

	if filters.Status != nil {
		query += " AND status = ?"
		args = append(args, *filters.Status)
	}
	if filters.Chain != nil {
		query += " AND chain = ?"
		args = append(args, *filters.Chain)
	}

	query += " ORDER BY created_at DESC"

	rows, err := d.conn.Query(query, args...)
	if err != nil {
		return nil, fmt.Errorf("failed to list watches: %w", err)
	}
	defer rows.Close()

	var watches []models.Watch
	for rows.Next() {
		var w models.Watch
		if err := rows.Scan(
			&w.ID, &w.Chain, &w.Address, &w.Status, &w.StartedAt, &w.ExpiresAt,
			&w.CompletedAt, &w.PollCount, &w.LastPollAt, &w.LastPollResult, &w.CreatedAt,
		); err != nil {
			return nil, fmt.Errorf("failed to scan watch row: %w", err)
		}
		watches = append(watches, w)
	}
	return watches, rows.Err()
}

// UpdateWatchStatus updates the status and optionally completed_at of a watch.
func (d *DB) UpdateWatchStatus(id string, status models.WatchStatus, completedAt *string) error {
	_, err := d.conn.Exec(`
		UPDATE watches SET status = ?, completed_at = ? WHERE id = ?`,
		status, completedAt, id,
	)
	if err != nil {
		return fmt.Errorf("failed to update watch status %s to %s: %w", id, status, err)
	}

	slog.Info("watch status updated", "watchID", id, "status", status)
	return nil
}

// UpdateWatchPollResult updates poll metadata after a poll iteration.
func (d *DB) UpdateWatchPollResult(id string, pollCount int, lastPollResult string) error {
	_, err := d.conn.Exec(`
		UPDATE watches SET poll_count = ?, last_poll_at = CURRENT_TIMESTAMP, last_poll_result = ?
		WHERE id = ?`,
		pollCount, lastPollResult, id,
	)
	if err != nil {
		return fmt.Errorf("failed to update watch poll result %s: %w", id, err)
	}
	return nil
}

// ExpireAllActiveWatches marks all ACTIVE watches as EXPIRED.
func (d *DB) ExpireAllActiveWatches() (int64, error) {
	result, err := d.conn.Exec(`
		UPDATE watches SET status = 'EXPIRED', completed_at = CURRENT_TIMESTAMP
		WHERE status = 'ACTIVE'`)
	if err != nil {
		return 0, fmt.Errorf("failed to expire active watches: %w", err)
	}

	affected, _ := result.RowsAffected()
	if affected > 0 {
		slog.Info("expired active watches on startup", "count", affected)
	}
	return affected, nil
}

// GetActiveWatchByAddress returns the active watch for a given address, if any.
func (d *DB) GetActiveWatchByAddress(address string) (*models.Watch, error) {
	w := &models.Watch{}
	err := d.conn.QueryRow(`
		SELECT id, chain, address, status, started_at, expires_at, completed_at,
		       poll_count, last_poll_at, last_poll_result, created_at
		FROM watches WHERE address = ? AND status = 'ACTIVE'`, address,
	).Scan(
		&w.ID, &w.Chain, &w.Address, &w.Status, &w.StartedAt, &w.ExpiresAt,
		&w.CompletedAt, &w.PollCount, &w.LastPollAt, &w.LastPollResult, &w.CreatedAt,
	)
	if err == sql.ErrNoRows {
		return nil, nil
	}
	if err != nil {
		return nil, fmt.Errorf("failed to get active watch for address %s: %w", address, err)
	}
	return w, nil
}
