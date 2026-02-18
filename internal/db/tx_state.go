package db

import (
	"database/sql"
	"fmt"
	"log/slog"
)

// TxStateRow represents a row in the tx_state table.
type TxStateRow struct {
	ID           string
	SweepID      string
	Chain        string
	Token        string
	AddressIndex int
	FromAddress  string
	ToAddress    string
	Amount       string
	TxHash       string
	Nonce        int64
	Status       string
	CreatedAt    string
	UpdatedAt    string
	Error        string
}

// CreateTxState inserts a new pending transaction state.
func (d *DB) CreateTxState(tx TxStateRow) error {
	slog.Debug("creating tx state",
		"id", tx.ID,
		"sweepID", tx.SweepID,
		"chain", tx.Chain,
		"token", tx.Token,
		"addressIndex", tx.AddressIndex,
		"fromAddress", tx.FromAddress,
		"toAddress", tx.ToAddress,
		"amount", tx.Amount,
		"status", tx.Status,
	)

	_, err := d.conn.Exec(
		`INSERT INTO tx_state (id, sweep_id, chain, token, address_index, from_address, to_address, amount, tx_hash, nonce, status, error)
		 VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`,
		tx.ID,
		tx.SweepID,
		tx.Chain,
		tx.Token,
		tx.AddressIndex,
		tx.FromAddress,
		tx.ToAddress,
		tx.Amount,
		tx.TxHash,
		tx.Nonce,
		tx.Status,
		tx.Error,
	)
	if err != nil {
		return fmt.Errorf("insert tx state %s: %w", tx.ID, err)
	}

	slog.Info("tx state created",
		"id", tx.ID,
		"sweepID", tx.SweepID,
		"chain", tx.Chain,
		"status", tx.Status,
	)

	return nil
}

// UpdateTxStatus updates the status, optional tx hash, and optional error for a transaction state.
func (d *DB) UpdateTxStatus(id, status, txHash, txError string) error {
	slog.Debug("updating tx status",
		"id", id,
		"status", status,
		"txHash", txHash,
		"error", txError,
	)

	result, err := d.conn.Exec(
		`UPDATE tx_state SET status = ?, tx_hash = COALESCE(NULLIF(?, ''), tx_hash), error = ?, updated_at = datetime('now')
		 WHERE id = ?`,
		status,
		txHash,
		txError,
		id,
	)
	if err != nil {
		return fmt.Errorf("update tx status %s: %w", id, err)
	}

	rows, _ := result.RowsAffected()
	slog.Info("tx status updated",
		"id", id,
		"status", status,
		"rowsAffected", rows,
	)

	return nil
}

// GetPendingTxStates returns all non-terminal transaction states for a chain.
// Includes: pending, broadcasting, confirming, uncertain.
func (d *DB) GetPendingTxStates(chain string) ([]TxStateRow, error) {
	slog.Debug("fetching pending tx states", "chain", chain)

	rows, err := d.conn.Query(
		`SELECT id, sweep_id, chain, token, address_index, from_address, to_address, amount,
		        COALESCE(tx_hash, '') as tx_hash, COALESCE(nonce, 0) as nonce, status, created_at, updated_at, COALESCE(error, '') as error
		 FROM tx_state
		 WHERE chain = ? AND status IN ('pending', 'broadcasting', 'confirming', 'uncertain')
		 ORDER BY created_at ASC`,
		chain,
	)
	if err != nil {
		return nil, fmt.Errorf("query pending tx states for %s: %w", chain, err)
	}
	defer rows.Close()

	return scanTxStateRows(rows)
}

// GetTxStatesBySweepID returns all transaction states for a given sweep ID.
func (d *DB) GetTxStatesBySweepID(sweepID string) ([]TxStateRow, error) {
	slog.Debug("fetching tx states by sweep", "sweepID", sweepID)

	rows, err := d.conn.Query(
		`SELECT id, sweep_id, chain, token, address_index, from_address, to_address, amount,
		        COALESCE(tx_hash, '') as tx_hash, COALESCE(nonce, 0) as nonce, status, created_at, updated_at, COALESCE(error, '') as error
		 FROM tx_state
		 WHERE sweep_id = ?
		 ORDER BY address_index ASC`,
		sweepID,
	)
	if err != nil {
		return nil, fmt.Errorf("query tx states for sweep %s: %w", sweepID, err)
	}
	defer rows.Close()

	return scanTxStateRows(rows)
}

// GetTxStateByNonce returns a transaction state matching the chain, from address, and nonce.
// Returns nil if not found.
func (d *DB) GetTxStateByNonce(chain, fromAddress string, nonce int64) (*TxStateRow, error) {
	slog.Debug("fetching tx state by nonce",
		"chain", chain,
		"fromAddress", fromAddress,
		"nonce", nonce,
	)

	row := d.conn.QueryRow(
		`SELECT id, sweep_id, chain, token, address_index, from_address, to_address, amount,
		        COALESCE(tx_hash, '') as tx_hash, COALESCE(nonce, 0) as nonce, status, created_at, updated_at, COALESCE(error, '') as error
		 FROM tx_state
		 WHERE chain = ? AND from_address = ? AND nonce = ?
		 ORDER BY created_at DESC LIMIT 1`,
		chain,
		fromAddress,
		nonce,
	)

	var tx TxStateRow
	err := row.Scan(
		&tx.ID, &tx.SweepID, &tx.Chain, &tx.Token, &tx.AddressIndex,
		&tx.FromAddress, &tx.ToAddress, &tx.Amount, &tx.TxHash, &tx.Nonce,
		&tx.Status, &tx.CreatedAt, &tx.UpdatedAt, &tx.Error,
	)
	if err == sql.ErrNoRows {
		return nil, nil
	}
	if err != nil {
		return nil, fmt.Errorf("query tx state by nonce: %w", err)
	}

	return &tx, nil
}

// CountTxStatesByStatus returns a count of transactions per status for a sweep.
func (d *DB) CountTxStatesByStatus(sweepID string) (map[string]int, error) {
	slog.Debug("counting tx states by status", "sweepID", sweepID)

	rows, err := d.conn.Query(
		`SELECT status, COUNT(*) FROM tx_state WHERE sweep_id = ? GROUP BY status`,
		sweepID,
	)
	if err != nil {
		return nil, fmt.Errorf("count tx states for sweep %s: %w", sweepID, err)
	}
	defer rows.Close()

	counts := make(map[string]int)
	for rows.Next() {
		var status string
		var count int
		if err := rows.Scan(&status, &count); err != nil {
			return nil, fmt.Errorf("scan tx state count: %w", err)
		}
		counts[status] = count
	}

	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate tx state counts: %w", err)
	}

	slog.Debug("tx state counts",
		"sweepID", sweepID,
		"counts", fmt.Sprintf("%v", counts),
	)

	return counts, nil
}

// scanTxStateRows scans multiple tx_state rows from a query result.
func scanTxStateRows(rows *sql.Rows) ([]TxStateRow, error) {
	var results []TxStateRow
	for rows.Next() {
		var tx TxStateRow
		if err := rows.Scan(
			&tx.ID, &tx.SweepID, &tx.Chain, &tx.Token, &tx.AddressIndex,
			&tx.FromAddress, &tx.ToAddress, &tx.Amount, &tx.TxHash, &tx.Nonce,
			&tx.Status, &tx.CreatedAt, &tx.UpdatedAt, &tx.Error,
		); err != nil {
			return nil, fmt.Errorf("scan tx state row: %w", err)
		}
		results = append(results, tx)
	}

	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate tx state rows: %w", err)
	}

	return results, nil
}
