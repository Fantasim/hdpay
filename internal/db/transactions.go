package db

import (
	"database/sql"
	"fmt"
	"log/slog"

	"github.com/Fantasim/hdpay/internal/models"
)

// InsertTransaction inserts a new transaction record and returns the auto-generated ID.
func (d *DB) InsertTransaction(tx models.Transaction) (int64, error) {
	slog.Debug("inserting transaction",
		"chain", tx.Chain,
		"addressIndex", tx.AddressIndex,
		"txHash", tx.TxHash,
		"direction", tx.Direction,
		"token", tx.Token,
		"amount", tx.Amount,
		"status", tx.Status,
	)

	result, err := d.conn.Exec(
		`INSERT INTO transactions (chain, address_index, tx_hash, direction, token, amount, from_address, to_address, status)
		 VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?)`,
		string(tx.Chain),
		tx.AddressIndex,
		tx.TxHash,
		tx.Direction,
		string(tx.Token),
		tx.Amount,
		tx.FromAddress,
		tx.ToAddress,
		tx.Status,
	)
	if err != nil {
		return 0, fmt.Errorf("insert transaction: %w", err)
	}

	id, err := result.LastInsertId()
	if err != nil {
		return 0, fmt.Errorf("get last insert id: %w", err)
	}

	slog.Info("transaction recorded",
		"id", id,
		"chain", tx.Chain,
		"txHash", tx.TxHash,
		"status", tx.Status,
	)

	return id, nil
}

// UpdateTransactionStatus updates the status of a transaction by ID.
// If confirmedAt is non-nil, the confirmed_at timestamp is also updated.
func (d *DB) UpdateTransactionStatus(id int64, status string, confirmedAt *string) error {
	slog.Debug("updating transaction status",
		"id", id,
		"status", status,
		"confirmedAt", confirmedAt,
	)

	var err error
	if confirmedAt != nil {
		_, err = d.conn.Exec(
			"UPDATE transactions SET status = ?, confirmed_at = ? WHERE id = ?",
			status, *confirmedAt, id,
		)
	} else {
		_, err = d.conn.Exec(
			"UPDATE transactions SET status = ? WHERE id = ?",
			status, id,
		)
	}
	if err != nil {
		return fmt.Errorf("update transaction %d status: %w", id, err)
	}

	slog.Info("transaction status updated",
		"id", id,
		"status", status,
	)

	return nil
}

// GetTransaction retrieves a transaction by its ID.
func (d *DB) GetTransaction(id int64) (*models.Transaction, error) {
	slog.Debug("getting transaction", "id", id)

	var tx models.Transaction
	var blockNumber sql.NullInt64
	var confirmedAt sql.NullString

	err := d.conn.QueryRow(
		`SELECT id, chain, address_index, tx_hash, direction, token, amount,
		        from_address, to_address, block_number, status, created_at, confirmed_at
		 FROM transactions WHERE id = ?`,
		id,
	).Scan(
		&tx.ID, &tx.Chain, &tx.AddressIndex, &tx.TxHash, &tx.Direction,
		&tx.Token, &tx.Amount, &tx.FromAddress, &tx.ToAddress,
		&blockNumber, &tx.Status, &tx.CreatedAt, &confirmedAt,
	)
	if err != nil {
		return nil, fmt.Errorf("get transaction %d: %w", id, err)
	}

	if blockNumber.Valid {
		bn := int(blockNumber.Int64)
		tx.BlockNumber = &bn
	}
	if confirmedAt.Valid {
		tx.ConfirmedAt = confirmedAt.String
	}

	return &tx, nil
}

// GetTransactionByHash retrieves a transaction by chain and transaction hash.
func (d *DB) GetTransactionByHash(chain models.Chain, txHash string) (*models.Transaction, error) {
	slog.Debug("getting transaction by hash",
		"chain", chain,
		"txHash", txHash,
	)

	var tx models.Transaction
	var blockNumber sql.NullInt64
	var confirmedAt sql.NullString

	err := d.conn.QueryRow(
		`SELECT id, chain, address_index, tx_hash, direction, token, amount,
		        from_address, to_address, block_number, status, created_at, confirmed_at
		 FROM transactions WHERE chain = ? AND tx_hash = ? LIMIT 1`,
		string(chain), txHash,
	).Scan(
		&tx.ID, &tx.Chain, &tx.AddressIndex, &tx.TxHash, &tx.Direction,
		&tx.Token, &tx.Amount, &tx.FromAddress, &tx.ToAddress,
		&blockNumber, &tx.Status, &tx.CreatedAt, &confirmedAt,
	)
	if err != nil {
		return nil, fmt.Errorf("get transaction by hash %s/%s: %w", chain, txHash, err)
	}

	if blockNumber.Valid {
		bn := int(blockNumber.Int64)
		tx.BlockNumber = &bn
	}
	if confirmedAt.Valid {
		tx.ConfirmedAt = confirmedAt.String
	}

	return &tx, nil
}

// ListTransactions returns paginated transactions, optionally filtered by chain.
func (d *DB) ListTransactions(chain *models.Chain, page, pageSize int) ([]models.Transaction, int64, error) {
	offset := (page - 1) * pageSize

	slog.Debug("listing transactions",
		"chain", chain,
		"page", page,
		"pageSize", pageSize,
		"offset", offset,
	)

	// Build WHERE clause.
	where := "1=1"
	var args []interface{}
	if chain != nil {
		where = "chain = ?"
		args = append(args, string(*chain))
	}

	// Count total.
	var total int64
	countArgs := make([]interface{}, len(args))
	copy(countArgs, args)
	if err := d.conn.QueryRow("SELECT COUNT(*) FROM transactions WHERE "+where, countArgs...).Scan(&total); err != nil {
		return nil, 0, fmt.Errorf("count transactions: %w", err)
	}

	// Fetch page.
	queryArgs := append(args, pageSize, offset)
	rows, err := d.conn.Query(
		`SELECT id, chain, address_index, tx_hash, direction, token, amount,
		        from_address, to_address, block_number, status, created_at, confirmed_at
		 FROM transactions WHERE `+where+` ORDER BY id DESC LIMIT ? OFFSET ?`,
		queryArgs...,
	)
	if err != nil {
		return nil, 0, fmt.Errorf("query transactions: %w", err)
	}
	defer rows.Close()

	var txs []models.Transaction
	for rows.Next() {
		var tx models.Transaction
		var blockNumber sql.NullInt64
		var confirmedAt sql.NullString

		if err := rows.Scan(
			&tx.ID, &tx.Chain, &tx.AddressIndex, &tx.TxHash, &tx.Direction,
			&tx.Token, &tx.Amount, &tx.FromAddress, &tx.ToAddress,
			&blockNumber, &tx.Status, &tx.CreatedAt, &confirmedAt,
		); err != nil {
			return nil, 0, fmt.Errorf("scan transaction row: %w", err)
		}

		if blockNumber.Valid {
			bn := int(blockNumber.Int64)
			tx.BlockNumber = &bn
		}
		if confirmedAt.Valid {
			tx.ConfirmedAt = confirmedAt.String
		}

		txs = append(txs, tx)
	}
	if err := rows.Err(); err != nil {
		return nil, 0, fmt.Errorf("iterate transaction rows: %w", err)
	}

	slog.Debug("transactions listed",
		"total", total,
		"returned", len(txs),
	)

	return txs, total, nil
}
