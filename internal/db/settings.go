package db

import (
	"fmt"
	"log/slog"
)

// Default settings values.
var defaultSettings = map[string]string{
	"max_scan_id":            "5000",
	"auto_resume_scans":      "true",
	"resume_threshold_hours": "24",
	"btc_fee_rate":           "10",
	"bsc_gas_preseed_bnb":    "0.005",
	"log_level":              "info",
	"network":                "testnet",
}

// GetSetting retrieves a single setting value by key, returning the default if not set.
func (d *DB) GetSetting(key string) (string, error) {
	slog.Debug("getting setting", "key", key)

	var value string
	err := d.conn.QueryRow("SELECT value FROM settings WHERE key = ?", key).Scan(&value)
	if err != nil {
		// Return default if not found.
		if defVal, ok := defaultSettings[key]; ok {
			slog.Debug("setting not found, returning default", "key", key, "default", defVal)
			return defVal, nil
		}
		return "", fmt.Errorf("get setting %q: %w", key, err)
	}

	return value, nil
}

// SetSetting upserts a setting key-value pair.
func (d *DB) SetSetting(key, value string) error {
	slog.Debug("setting value", "key", key, "value", value)

	_, err := d.conn.Exec(
		`INSERT INTO settings (key, value, updated_at) VALUES (?, ?, datetime('now'))
		 ON CONFLICT(key) DO UPDATE SET value = excluded.value, updated_at = excluded.updated_at`,
		key, value,
	)
	if err != nil {
		return fmt.Errorf("set setting %q: %w", key, err)
	}

	slog.Info("setting updated", "key", key, "value", value)
	return nil
}

// GetAllSettings retrieves all settings, filling in defaults for missing keys.
func (d *DB) GetAllSettings() (map[string]string, error) {
	slog.Debug("getting all settings")

	// Start with defaults.
	result := make(map[string]string)
	for k, v := range defaultSettings {
		result[k] = v
	}

	// Override with DB values.
	rows, err := d.conn.Query("SELECT key, value FROM settings")
	if err != nil {
		return nil, fmt.Errorf("query settings: %w", err)
	}
	defer rows.Close()

	for rows.Next() {
		var key, value string
		if err := rows.Scan(&key, &value); err != nil {
			return nil, fmt.Errorf("scan setting row: %w", err)
		}
		result[key] = value
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate setting rows: %w", err)
	}

	slog.Debug("settings loaded", "count", len(result))
	return result, nil
}

// ResetBalances deletes all balances, scan state, and transactions.
// Addresses are preserved.
func (d *DB) ResetBalances() error {
	slog.Warn("resetting balances, scan state, and transactions")

	tx, err := d.conn.Begin()
	if err != nil {
		return fmt.Errorf("begin transaction: %w", err)
	}
	defer tx.Rollback()

	if _, err := tx.Exec("DELETE FROM balances"); err != nil {
		return fmt.Errorf("delete balances: %w", err)
	}
	if _, err := tx.Exec("DELETE FROM scan_state"); err != nil {
		return fmt.Errorf("delete scan_state: %w", err)
	}
	if _, err := tx.Exec("DELETE FROM transactions"); err != nil {
		return fmt.Errorf("delete transactions: %w", err)
	}

	if err := tx.Commit(); err != nil {
		return fmt.Errorf("commit reset: %w", err)
	}

	slog.Info("balances reset complete")
	return nil
}

// ResetAll deletes all data: addresses, balances, scan state, and transactions.
func (d *DB) ResetAll() error {
	slog.Warn("resetting ALL data — addresses, balances, scan state, transactions")

	tx, err := d.conn.Begin()
	if err != nil {
		return fmt.Errorf("begin transaction: %w", err)
	}
	defer tx.Rollback()

	if _, err := tx.Exec("DELETE FROM balances"); err != nil {
		return fmt.Errorf("delete balances: %w", err)
	}
	if _, err := tx.Exec("DELETE FROM scan_state"); err != nil {
		return fmt.Errorf("delete scan_state: %w", err)
	}
	if _, err := tx.Exec("DELETE FROM transactions"); err != nil {
		return fmt.Errorf("delete transactions: %w", err)
	}
	if _, err := tx.Exec("DELETE FROM addresses"); err != nil {
		return fmt.Errorf("delete addresses: %w", err)
	}

	if err := tx.Commit(); err != nil {
		return fmt.Errorf("commit reset: %w", err)
	}

	slog.Info("full reset complete")
	return nil
}
