package pollerdb

import (
	"database/sql"
	"fmt"
	"log/slog"

	"github.com/Fantasim/hdpay/internal/poller/models"
)

// InsertTransaction inserts a new transaction into the database.
func (d *DB) InsertTransaction(tx *models.Transaction) (int64, error) {
	result, err := d.conn.Exec(`
		INSERT INTO transactions (
			watch_id, tx_hash, chain, address, token,
			amount_raw, amount_human, decimals,
			usd_value, usd_price, tier, multiplier, points,
			status, confirmations, block_number,
			detected_at, confirmed_at
		) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`,
		tx.WatchID, tx.TxHash, tx.Chain, tx.Address, tx.Token,
		tx.AmountRaw, tx.AmountHuman, tx.Decimals,
		tx.USDValue, tx.USDPrice, tx.Tier, tx.Multiplier, tx.Points,
		tx.Status, tx.Confirmations, tx.BlockNumber,
		tx.DetectedAt, tx.ConfirmedAt,
	)
	if err != nil {
		return 0, fmt.Errorf("failed to insert transaction %s: %w", tx.TxHash, err)
	}

	id, _ := result.LastInsertId()

	slog.Info("transaction recorded",
		"id", id,
		"txHash", tx.TxHash,
		"chain", tx.Chain,
		"address", tx.Address,
		"token", tx.Token,
		"amountHuman", tx.AmountHuman,
		"status", tx.Status,
		"points", tx.Points,
	)
	return id, nil
}

// GetByTxHash retrieves a transaction by its hash.
func (d *DB) GetByTxHash(txHash string) (*models.Transaction, error) {
	tx := &models.Transaction{}
	err := d.conn.QueryRow(`
		SELECT id, watch_id, tx_hash, chain, address, token,
		       amount_raw, amount_human, decimals,
		       usd_value, usd_price, tier, multiplier, points,
		       status, confirmations, block_number,
		       detected_at, confirmed_at, created_at
		FROM transactions WHERE tx_hash = ?`, txHash,
	).Scan(
		&tx.ID, &tx.WatchID, &tx.TxHash, &tx.Chain, &tx.Address, &tx.Token,
		&tx.AmountRaw, &tx.AmountHuman, &tx.Decimals,
		&tx.USDValue, &tx.USDPrice, &tx.Tier, &tx.Multiplier, &tx.Points,
		&tx.Status, &tx.Confirmations, &tx.BlockNumber,
		&tx.DetectedAt, &tx.ConfirmedAt, &tx.CreatedAt,
	)
	if err == sql.ErrNoRows {
		return nil, nil
	}
	if err != nil {
		return nil, fmt.Errorf("failed to get transaction by hash %s: %w", txHash, err)
	}
	return tx, nil
}

// UpdateToConfirmed updates a pending transaction to confirmed status.
func (d *DB) UpdateToConfirmed(txHash string, confirmations int, blockNumber *int64, confirmedAt string, usdValue, usdPrice float64, tier int, multiplier float64, points int) error {
	_, err := d.conn.Exec(`
		UPDATE transactions
		SET status = 'CONFIRMED', confirmations = ?, block_number = ?,
		    confirmed_at = ?, usd_value = ?, usd_price = ?,
		    tier = ?, multiplier = ?, points = ?
		WHERE tx_hash = ?`,
		confirmations, blockNumber, confirmedAt, usdValue, usdPrice, tier, multiplier, points, txHash,
	)
	if err != nil {
		return fmt.Errorf("failed to confirm transaction %s: %w", txHash, err)
	}

	slog.Info("transaction confirmed",
		"txHash", txHash,
		"confirmations", confirmations,
		"usdValue", usdValue,
		"points", points,
	)
	return nil
}

// ListPending returns all pending transactions.
func (d *DB) ListPending() ([]models.Transaction, error) {
	return d.queryTransactions(`
		SELECT id, watch_id, tx_hash, chain, address, token,
		       amount_raw, amount_human, decimals,
		       usd_value, usd_price, tier, multiplier, points,
		       status, confirmations, block_number,
		       detected_at, confirmed_at, created_at
		FROM transactions WHERE status = 'PENDING'
		ORDER BY detected_at ASC`)
}

// ListByAddress returns all transactions for a given address.
func (d *DB) ListByAddress(address string) ([]models.Transaction, error) {
	return d.queryTransactions(`
		SELECT id, watch_id, tx_hash, chain, address, token,
		       amount_raw, amount_human, decimals,
		       usd_value, usd_price, tier, multiplier, points,
		       status, confirmations, block_number,
		       detected_at, confirmed_at, created_at
		FROM transactions WHERE address = ?
		ORDER BY detected_at DESC`, address)
}

// ListAll returns transactions with optional filters and pagination.
func (d *DB) ListAll(filters models.TransactionFilters, pag models.Pagination) ([]models.Transaction, int64, error) {
	where := "WHERE 1=1"
	var args []interface{}

	if filters.Chain != nil {
		where += " AND chain = ?"
		args = append(args, *filters.Chain)
	}
	if filters.Token != nil {
		where += " AND token = ?"
		args = append(args, *filters.Token)
	}
	if filters.Status != nil {
		where += " AND status = ?"
		args = append(args, *filters.Status)
	}
	if filters.Tier != nil {
		where += " AND tier = ?"
		args = append(args, *filters.Tier)
	}
	if filters.MinUSD != nil {
		where += " AND usd_value >= ?"
		args = append(args, *filters.MinUSD)
	}
	if filters.MaxUSD != nil {
		where += " AND usd_value <= ?"
		args = append(args, *filters.MaxUSD)
	}
	if filters.DateFrom != nil {
		where += " AND detected_at >= ?"
		args = append(args, *filters.DateFrom)
	}
	if filters.DateTo != nil {
		where += " AND detected_at <= ?"
		args = append(args, *filters.DateTo)
	}

	// Count total
	var total int64
	countQuery := "SELECT COUNT(*) FROM transactions " + where
	if err := d.conn.QueryRow(countQuery, args...).Scan(&total); err != nil {
		return nil, 0, fmt.Errorf("failed to count transactions: %w", err)
	}

	// Fetch page
	query := `SELECT id, watch_id, tx_hash, chain, address, token,
	                 amount_raw, amount_human, decimals,
	                 usd_value, usd_price, tier, multiplier, points,
	                 status, confirmations, block_number,
	                 detected_at, confirmed_at, created_at
	          FROM transactions ` + where + ` ORDER BY detected_at DESC LIMIT ? OFFSET ?`
	offset := (pag.Page - 1) * pag.PageSize
	args = append(args, pag.PageSize, offset)

	txs, err := d.queryTransactions(query, args...)
	if err != nil {
		return nil, 0, err
	}
	return txs, total, nil
}

// ListPendingByWatchID returns all pending transactions for a specific watch.
func (d *DB) ListPendingByWatchID(watchID string) ([]models.Transaction, error) {
	return d.queryTransactions(`
		SELECT id, watch_id, tx_hash, chain, address, token,
		       amount_raw, amount_human, decimals,
		       usd_value, usd_price, tier, multiplier, points,
		       status, confirmations, block_number,
		       detected_at, confirmed_at, created_at
		FROM transactions WHERE watch_id = ? AND status = 'PENDING'
		ORDER BY detected_at ASC`, watchID)
}

// CountByWatchID returns the total and pending transaction counts for a watch.
func (d *DB) CountByWatchID(watchID string) (total int, pending int, err error) {
	err = d.conn.QueryRow(`
		SELECT COUNT(*) FROM transactions WHERE watch_id = ?`, watchID,
	).Scan(&total)
	if err != nil {
		return 0, 0, fmt.Errorf("failed to count transactions for watch %s: %w", watchID, err)
	}

	err = d.conn.QueryRow(`
		SELECT COUNT(*) FROM transactions WHERE watch_id = ? AND status = 'PENDING'`, watchID,
	).Scan(&pending)
	if err != nil {
		return 0, 0, fmt.Errorf("failed to count pending transactions for watch %s: %w", watchID, err)
	}

	return total, pending, nil
}

// LastDetectedAt returns the most recent detected_at timestamp for a given address.
// Returns empty string if no transactions exist for this address.
func (d *DB) LastDetectedAt(address string) (string, error) {
	var detectedAt sql.NullString
	err := d.conn.QueryRow(`
		SELECT MAX(detected_at) FROM transactions WHERE address = ?`, address,
	).Scan(&detectedAt)
	if err != nil {
		return "", fmt.Errorf("failed to get last detected_at for %s: %w", address, err)
	}
	if !detectedAt.Valid {
		return "", nil
	}
	return detectedAt.String, nil
}

func (d *DB) queryTransactions(query string, args ...interface{}) ([]models.Transaction, error) {
	rows, err := d.conn.Query(query, args...)
	if err != nil {
		return nil, fmt.Errorf("failed to query transactions: %w", err)
	}
	defer rows.Close()

	var txs []models.Transaction
	for rows.Next() {
		var tx models.Transaction
		if err := rows.Scan(
			&tx.ID, &tx.WatchID, &tx.TxHash, &tx.Chain, &tx.Address, &tx.Token,
			&tx.AmountRaw, &tx.AmountHuman, &tx.Decimals,
			&tx.USDValue, &tx.USDPrice, &tx.Tier, &tx.Multiplier, &tx.Points,
			&tx.Status, &tx.Confirmations, &tx.BlockNumber,
			&tx.DetectedAt, &tx.ConfirmedAt, &tx.CreatedAt,
		); err != nil {
			return nil, fmt.Errorf("failed to scan transaction row: %w", err)
		}
		txs = append(txs, tx)
	}
	return txs, rows.Err()
}
