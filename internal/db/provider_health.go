package db

import (
	"database/sql"
	"fmt"
	"log/slog"
)

// ProviderHealthRow represents a row in the provider_health table.
type ProviderHealthRow struct {
	ProviderName     string
	Chain            string
	ProviderType     string
	Status           string
	ConsecutiveFails int
	LastSuccess      string
	LastError        string
	LastErrorMsg     string
	CircuitState     string
	UpdatedAt        string
}

// UpsertProviderHealth inserts or updates a provider health record.
func (d *DB) UpsertProviderHealth(ph ProviderHealthRow) error {
	slog.Debug("upserting provider health",
		"provider", ph.ProviderName,
		"chain", ph.Chain,
		"status", ph.Status,
		"circuitState", ph.CircuitState,
	)

	_, err := d.conn.Exec(
		`INSERT INTO provider_health (provider_name, chain, provider_type, status, consecutive_fails, last_success, last_error, last_error_msg, circuit_state)
		 VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?)
		 ON CONFLICT(provider_name) DO UPDATE SET
		   chain = excluded.chain,
		   provider_type = excluded.provider_type,
		   status = excluded.status,
		   consecutive_fails = excluded.consecutive_fails,
		   last_success = excluded.last_success,
		   last_error = excluded.last_error,
		   last_error_msg = excluded.last_error_msg,
		   circuit_state = excluded.circuit_state,
		   updated_at = datetime('now')`,
		ph.ProviderName,
		ph.Chain,
		ph.ProviderType,
		ph.Status,
		ph.ConsecutiveFails,
		ph.LastSuccess,
		ph.LastError,
		ph.LastErrorMsg,
		ph.CircuitState,
	)
	if err != nil {
		return fmt.Errorf("upsert provider health %s: %w", ph.ProviderName, err)
	}

	slog.Info("provider health upserted",
		"provider", ph.ProviderName,
		"status", ph.Status,
		"circuitState", ph.CircuitState,
	)

	return nil
}

// GetProviderHealth returns a single provider's health record.
// Returns nil if not found.
func (d *DB) GetProviderHealth(providerName string) (*ProviderHealthRow, error) {
	slog.Debug("fetching provider health", "provider", providerName)

	row := d.conn.QueryRow(
		`SELECT provider_name, chain, provider_type, status, consecutive_fails,
		        COALESCE(last_success, '') as last_success,
		        COALESCE(last_error, '') as last_error,
		        COALESCE(last_error_msg, '') as last_error_msg,
		        circuit_state, updated_at
		 FROM provider_health WHERE provider_name = ?`,
		providerName,
	)

	var ph ProviderHealthRow
	err := row.Scan(
		&ph.ProviderName, &ph.Chain, &ph.ProviderType, &ph.Status,
		&ph.ConsecutiveFails, &ph.LastSuccess, &ph.LastError,
		&ph.LastErrorMsg, &ph.CircuitState, &ph.UpdatedAt,
	)
	if err == sql.ErrNoRows {
		return nil, nil
	}
	if err != nil {
		return nil, fmt.Errorf("query provider health %s: %w", providerName, err)
	}

	return &ph, nil
}

// GetProviderHealthByChain returns all provider health records for a chain.
func (d *DB) GetProviderHealthByChain(chain string) ([]ProviderHealthRow, error) {
	slog.Debug("fetching provider health by chain", "chain", chain)

	rows, err := d.conn.Query(
		`SELECT provider_name, chain, provider_type, status, consecutive_fails,
		        COALESCE(last_success, '') as last_success,
		        COALESCE(last_error, '') as last_error,
		        COALESCE(last_error_msg, '') as last_error_msg,
		        circuit_state, updated_at
		 FROM provider_health WHERE chain = ?
		 ORDER BY provider_name ASC`,
		chain,
	)
	if err != nil {
		return nil, fmt.Errorf("query provider health for chain %s: %w", chain, err)
	}
	defer rows.Close()

	return scanProviderHealthRows(rows)
}

// GetAllProviderHealth returns all provider health records.
func (d *DB) GetAllProviderHealth() ([]ProviderHealthRow, error) {
	slog.Debug("fetching all provider health")

	rows, err := d.conn.Query(
		`SELECT provider_name, chain, provider_type, status, consecutive_fails,
		        COALESCE(last_success, '') as last_success,
		        COALESCE(last_error, '') as last_error,
		        COALESCE(last_error_msg, '') as last_error_msg,
		        circuit_state, updated_at
		 FROM provider_health
		 ORDER BY chain ASC, provider_name ASC`,
	)
	if err != nil {
		return nil, fmt.Errorf("query all provider health: %w", err)
	}
	defer rows.Close()

	return scanProviderHealthRows(rows)
}

// RecordProviderSuccess resets consecutive failures, updates last_success, and sets status to healthy.
func (d *DB) RecordProviderSuccess(providerName string) error {
	slog.Debug("recording provider success", "provider", providerName)

	result, err := d.conn.Exec(
		`UPDATE provider_health
		 SET consecutive_fails = 0,
		     last_success = datetime('now'),
		     status = 'healthy',
		     circuit_state = 'closed',
		     updated_at = datetime('now')
		 WHERE provider_name = ?`,
		providerName,
	)
	if err != nil {
		return fmt.Errorf("record provider success %s: %w", providerName, err)
	}

	rows, _ := result.RowsAffected()
	if rows == 0 {
		slog.Warn("provider not found for success recording", "provider", providerName)
		return nil
	}

	slog.Info("provider success recorded", "provider", providerName)

	return nil
}

// RecordProviderFailure increments consecutive failures and records the error.
func (d *DB) RecordProviderFailure(providerName, errorMsg string) error {
	slog.Debug("recording provider failure",
		"provider", providerName,
		"error", errorMsg,
	)

	result, err := d.conn.Exec(
		`UPDATE provider_health
		 SET consecutive_fails = consecutive_fails + 1,
		     last_error = datetime('now'),
		     last_error_msg = ?,
		     updated_at = datetime('now')
		 WHERE provider_name = ?`,
		errorMsg,
		providerName,
	)
	if err != nil {
		return fmt.Errorf("record provider failure %s: %w", providerName, err)
	}

	rows, _ := result.RowsAffected()
	if rows == 0 {
		slog.Warn("provider not found for failure recording", "provider", providerName)
		return nil
	}

	slog.Info("provider failure recorded",
		"provider", providerName,
		"error", errorMsg,
	)

	return nil
}

// UpdateProviderCircuitState updates only the circuit_state and status fields for a provider.
// Status is derived from the circuit state: closed=healthy, half_open=degraded, open=down.
func (d *DB) UpdateProviderCircuitState(providerName, circuitState string) error {
	slog.Debug("updating provider circuit state",
		"provider", providerName,
		"circuitState", circuitState,
	)

	// Derive status from circuit state.
	status := "healthy"
	switch circuitState {
	case "open":
		status = "down"
	case "half_open":
		status = "degraded"
	}

	result, err := d.conn.Exec(
		`UPDATE provider_health
		 SET circuit_state = ?,
		     status = ?,
		     updated_at = datetime('now')
		 WHERE provider_name = ?`,
		circuitState,
		status,
		providerName,
	)
	if err != nil {
		return fmt.Errorf("update provider circuit state %s: %w", providerName, err)
	}

	rows, _ := result.RowsAffected()
	if rows == 0 {
		slog.Warn("provider not found for circuit state update", "provider", providerName)
		return nil
	}

	slog.Debug("provider circuit state updated",
		"provider", providerName,
		"circuitState", circuitState,
		"status", status,
	)

	return nil
}

// scanProviderHealthRows scans multiple provider_health rows from a query result.
func scanProviderHealthRows(rows *sql.Rows) ([]ProviderHealthRow, error) {
	var results []ProviderHealthRow
	for rows.Next() {
		var ph ProviderHealthRow
		if err := rows.Scan(
			&ph.ProviderName, &ph.Chain, &ph.ProviderType, &ph.Status,
			&ph.ConsecutiveFails, &ph.LastSuccess, &ph.LastError,
			&ph.LastErrorMsg, &ph.CircuitState, &ph.UpdatedAt,
		); err != nil {
			return nil, fmt.Errorf("scan provider health row: %w", err)
		}
		results = append(results, ph)
	}

	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate provider health rows: %w", err)
	}

	return results, nil
}
