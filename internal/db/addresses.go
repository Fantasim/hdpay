package db

import (
	"fmt"
	"log/slog"
	"strings"
	"time"

	"github.com/Fantasim/hdpay/internal/models"
)

const insertBatchSize = 10_000

// InsertAddressBatch inserts addresses in batches of 10K per transaction.
func (d *DB) InsertAddressBatch(chain models.Chain, addresses []models.Address) error {
	total := len(addresses)
	slog.Info("inserting addresses", "chain", chain, "total", total, "batchSize", insertBatchSize)
	start := time.Now()

	for i := 0; i < total; i += insertBatchSize {
		end := i + insertBatchSize
		if end > total {
			end = total
		}
		batch := addresses[i:end]

		if err := d.insertBatch(batch); err != nil {
			return fmt.Errorf("insert address batch [%d:%d] for %s: %w", i, end, chain, err)
		}

		slog.Info("address batch inserted",
			"chain", chain,
			"inserted", end,
			"total", total,
			"progress", fmt.Sprintf("%.1f%%", float64(end)/float64(total)*100),
		)
	}

	slog.Info("address insertion complete",
		"chain", chain,
		"total", total,
		"duration", time.Since(start).Round(time.Millisecond),
	)
	return nil
}

// insertBatch inserts a single batch of addresses in one transaction.
func (d *DB) insertBatch(addresses []models.Address) error {
	tx, err := d.conn.Begin()
	if err != nil {
		return fmt.Errorf("begin transaction: %w", err)
	}

	// Build multi-value INSERT for performance.
	valueStrings := make([]string, 0, len(addresses))
	valueArgs := make([]interface{}, 0, len(addresses)*3)

	for _, addr := range addresses {
		valueStrings = append(valueStrings, "(?, ?, ?)")
		valueArgs = append(valueArgs, string(addr.Chain), addr.AddressIndex, addr.Address)
	}

	query := "INSERT INTO addresses (chain, address_index, address) VALUES " + strings.Join(valueStrings, ", ")

	if _, err := tx.Exec(query, valueArgs...); err != nil {
		tx.Rollback()
		return fmt.Errorf("exec batch insert: %w", err)
	}

	if err := tx.Commit(); err != nil {
		return fmt.Errorf("commit batch: %w", err)
	}

	return nil
}

// CountAddresses returns the number of addresses stored for a chain.
func (d *DB) CountAddresses(chain models.Chain) (int, error) {
	var count int
	err := d.conn.QueryRow("SELECT COUNT(*) FROM addresses WHERE chain = ?", string(chain)).Scan(&count)
	if err != nil {
		return 0, fmt.Errorf("count addresses for %s: %w", chain, err)
	}

	slog.Debug("counted addresses", "chain", chain, "count", count)
	return count, nil
}

// GetAddresses returns a paginated list of addresses for a chain.
func (d *DB) GetAddresses(chain models.Chain, offset, limit int) ([]models.Address, error) {
	slog.Debug("fetching addresses", "chain", chain, "offset", offset, "limit", limit)

	rows, err := d.conn.Query(
		"SELECT chain, address_index, address, created_at FROM addresses WHERE chain = ? ORDER BY address_index LIMIT ? OFFSET ?",
		string(chain), limit, offset,
	)
	if err != nil {
		return nil, fmt.Errorf("query addresses for %s: %w", chain, err)
	}
	defer rows.Close()

	var addresses []models.Address
	for rows.Next() {
		var addr models.Address
		if err := rows.Scan(&addr.Chain, &addr.AddressIndex, &addr.Address, &addr.CreatedAt); err != nil {
			return nil, fmt.Errorf("scan address row: %w", err)
		}
		addresses = append(addresses, addr)
	}

	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate address rows: %w", err)
	}

	return addresses, nil
}

// GetAddressByIndex returns a single address by chain and index.
func (d *DB) GetAddressByIndex(chain models.Chain, index int) (*models.Address, error) {
	var addr models.Address
	err := d.conn.QueryRow(
		"SELECT chain, address_index, address, created_at FROM addresses WHERE chain = ? AND address_index = ?",
		string(chain), index,
	).Scan(&addr.Chain, &addr.AddressIndex, &addr.Address, &addr.CreatedAt)
	if err != nil {
		return nil, fmt.Errorf("get address %s/%d: %w", chain, index, err)
	}

	return &addr, nil
}

// DeleteAddresses deletes all addresses for a chain.
func (d *DB) DeleteAddresses(chain models.Chain) error {
	result, err := d.conn.Exec("DELETE FROM addresses WHERE chain = ?", string(chain))
	if err != nil {
		return fmt.Errorf("delete addresses for %s: %w", chain, err)
	}

	affected, _ := result.RowsAffected()
	slog.Info("deleted addresses", "chain", chain, "count", affected)
	return nil
}

// StreamAddresses streams all addresses for a chain via a callback, avoiding loading all into memory.
func (d *DB) StreamAddresses(chain models.Chain, fn func(addr models.Address) error) error {
	rows, err := d.conn.Query(
		"SELECT chain, address_index, address, created_at FROM addresses WHERE chain = ? ORDER BY address_index",
		string(chain),
	)
	if err != nil {
		return fmt.Errorf("query addresses for streaming %s: %w", chain, err)
	}
	defer rows.Close()

	for rows.Next() {
		var addr models.Address
		if err := rows.Scan(&addr.Chain, &addr.AddressIndex, &addr.Address, &addr.CreatedAt); err != nil {
			return fmt.Errorf("scan address row during streaming: %w", err)
		}
		if err := fn(addr); err != nil {
			return fmt.Errorf("stream callback error: %w", err)
		}
	}

	return rows.Err()
}
