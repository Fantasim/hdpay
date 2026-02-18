package db

import (
	"database/sql"
	"fmt"
	"log/slog"
	"time"

	"github.com/Fantasim/hdpay/internal/config"
	"github.com/Fantasim/hdpay/internal/models"
)

// Scan status constants.
const (
	ScanStatusIdle      = "idle"
	ScanStatusScanning  = "scanning"
	ScanStatusCompleted = "completed"
	ScanStatusFailed    = "failed"
)

// GetScanState returns the current scan state for a chain.
// Returns nil if no scan state exists.
func (d *DB) GetScanState(chain models.Chain) (*models.ScanState, error) {
	slog.Debug("fetching scan state", "chain", chain)

	var state models.ScanState
	var startedAt, updatedAt sql.NullString

	err := d.conn.QueryRow(
		`SELECT chain, last_scanned_index, max_scan_id, status, started_at, updated_at
		 FROM scan_state WHERE chain = ?`,
		string(chain),
	).Scan(&state.Chain, &state.LastScannedIndex, &state.MaxScanID, &state.Status, &startedAt, &updatedAt)

	if err == sql.ErrNoRows {
		slog.Debug("no scan state found", "chain", chain)
		return nil, nil
	}
	if err != nil {
		return nil, fmt.Errorf("query scan state for %s: %w", chain, err)
	}

	if startedAt.Valid {
		state.StartedAt = startedAt.String
	}
	if updatedAt.Valid {
		state.UpdatedAt = updatedAt.String
	}

	slog.Debug("scan state fetched",
		"chain", chain,
		"lastIndex", state.LastScannedIndex,
		"maxID", state.MaxScanID,
		"status", state.Status,
	)

	return &state, nil
}

// UpsertScanState creates or updates the scan state for a chain.
func (d *DB) UpsertScanState(state models.ScanState) error {
	now := time.Now().UTC().Format(time.RFC3339)

	_, err := d.conn.Exec(
		`INSERT INTO scan_state (chain, last_scanned_index, max_scan_id, status, started_at, updated_at)
		 VALUES (?, ?, ?, ?, ?, ?)
		 ON CONFLICT(chain) DO UPDATE SET
		   last_scanned_index = excluded.last_scanned_index,
		   max_scan_id = excluded.max_scan_id,
		   status = excluded.status,
		   started_at = COALESCE(NULLIF(excluded.started_at, ''), scan_state.started_at),
		   updated_at = excluded.updated_at`,
		string(state.Chain), state.LastScannedIndex, state.MaxScanID, state.Status,
		state.StartedAt, now,
	)
	if err != nil {
		return fmt.Errorf("upsert scan state for %s: %w", state.Chain, err)
	}

	slog.Debug("scan state upserted",
		"chain", state.Chain,
		"lastIndex", state.LastScannedIndex,
		"maxID", state.MaxScanID,
		"status", state.Status,
	)

	return nil
}

// ShouldResume checks if a scan can be resumed for the given chain.
// Returns true if a scan exists, is recent enough (within ScanResumeThreshold),
// and was in "scanning" or "completed" status.
// Also returns the last scanned index to resume from.
func (d *DB) ShouldResume(chain models.Chain) (bool, int, error) {
	state, err := d.GetScanState(chain)
	if err != nil {
		return false, 0, err
	}
	if state == nil {
		slog.Debug("no scan state to resume from", "chain", chain)
		return false, 0, nil
	}

	// Only resume from scanning or completed states.
	if state.Status != ScanStatusScanning && state.Status != ScanStatusCompleted {
		slog.Debug("scan state not resumable",
			"chain", chain,
			"status", state.Status,
		)
		return false, 0, nil
	}

	// Check if the scan is recent enough.
	if state.UpdatedAt == "" {
		return false, 0, nil
	}

	updatedAt, err := time.Parse(time.RFC3339, state.UpdatedAt)
	if err != nil {
		slog.Warn("unparseable scan updated_at",
			"chain", chain,
			"updatedAt", state.UpdatedAt,
			"error", err,
		)
		return false, 0, nil
	}

	age := time.Since(updatedAt)
	if age > config.ScanResumeThreshold {
		slog.Info("scan state too old, starting fresh",
			"chain", chain,
			"age", age.Round(time.Minute),
			"threshold", config.ScanResumeThreshold,
		)
		return false, 0, nil
	}

	// If completed, no need to resume.
	if state.Status == ScanStatusCompleted {
		slog.Info("previous scan completed recently, starting fresh",
			"chain", chain,
			"age", age.Round(time.Minute),
		)
		return false, 0, nil
	}

	slog.Info("scan can be resumed",
		"chain", chain,
		"lastIndex", state.LastScannedIndex,
		"age", age.Round(time.Minute),
	)

	return true, state.LastScannedIndex, nil
}
