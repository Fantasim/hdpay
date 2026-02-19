package db

import (
	"database/sql"
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
	valueArgs := make([]interface{}, 0, len(addresses)*4)

	for _, addr := range addresses {
		valueStrings = append(valueStrings, "(?, ?, ?, ?)")
		valueArgs = append(valueArgs, string(addr.Chain), d.network, addr.AddressIndex, addr.Address)
	}

	query := "INSERT INTO addresses (chain, network, address_index, address) VALUES " + strings.Join(valueStrings, ", ")

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
	err := d.conn.QueryRow("SELECT COUNT(*) FROM addresses WHERE chain = ? AND network = ?", string(chain), d.network).Scan(&count)
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
		"SELECT chain, address_index, address, created_at FROM addresses WHERE chain = ? AND network = ? ORDER BY address_index LIMIT ? OFFSET ?",
		string(chain), d.network, limit, offset,
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

// AddressFilter holds filter parameters for paginated address queries.
type AddressFilter struct {
	Chain      models.Chain
	Page       int
	PageSize   int
	HasBalance bool
	Token      string // "", "NATIVE", "USDC", "USDT"
}

// GetAddressesWithBalances returns paginated addresses with their balance data.
func (d *DB) GetAddressesWithBalances(f AddressFilter) ([]models.AddressWithBalance, int64, error) {
	offset := (f.Page - 1) * f.PageSize

	slog.Debug("fetching addresses with balances",
		"chain", f.Chain,
		"page", f.Page,
		"pageSize", f.PageSize,
		"hasBalance", f.HasBalance,
		"token", f.Token,
		"offset", offset,
	)

	// Build WHERE clause
	where := "a.chain = ? AND a.network = ?"
	args := []interface{}{string(f.Chain), d.network}

	if f.HasBalance {
		where += " AND EXISTS (SELECT 1 FROM balances b WHERE b.chain = a.chain AND b.network = a.network AND b.address_index = a.address_index AND b.balance != '0')"
	}

	if f.Token != "" {
		if f.Token == "NATIVE" {
			where += " AND EXISTS (SELECT 1 FROM balances b WHERE b.chain = a.chain AND b.network = a.network AND b.address_index = a.address_index AND b.token = 'NATIVE' AND b.balance != '0')"
		} else {
			where += " AND EXISTS (SELECT 1 FROM balances b WHERE b.chain = a.chain AND b.network = a.network AND b.address_index = a.address_index AND b.token = ? AND b.balance != '0')"
			args = append(args, f.Token)
		}
	}

	// Count total
	var total int64
	countQuery := "SELECT COUNT(*) FROM addresses a WHERE " + where
	if err := d.conn.QueryRow(countQuery, args...).Scan(&total); err != nil {
		return nil, 0, fmt.Errorf("count filtered addresses for %s: %w", f.Chain, err)
	}

	slog.Debug("filtered address count", "chain", f.Chain, "total", total)

	// Fetch page
	query := "SELECT a.chain, a.address_index, a.address FROM addresses a WHERE " + where + " ORDER BY a.address_index LIMIT ? OFFSET ?"
	queryArgs := append(args, f.PageSize, offset)

	rows, err := d.conn.Query(query, queryArgs...)
	if err != nil {
		return nil, 0, fmt.Errorf("query filtered addresses for %s: %w", f.Chain, err)
	}
	defer rows.Close()

	var results []models.AddressWithBalance
	for rows.Next() {
		var item models.AddressWithBalance
		if err := rows.Scan(&item.Chain, &item.AddressIndex, &item.Address); err != nil {
			return nil, 0, fmt.Errorf("scan filtered address row: %w", err)
		}
		results = append(results, item)
	}
	if err := rows.Err(); err != nil {
		return nil, 0, fmt.Errorf("iterate filtered address rows: %w", err)
	}

	// Hydrate balances for the page
	if err := d.hydrateBalances(results); err != nil {
		return nil, 0, fmt.Errorf("hydrate balances: %w", err)
	}

	return results, total, nil
}

// hydrateBalances loads balance data for a slice of addresses.
func (d *DB) hydrateBalances(addresses []models.AddressWithBalance) error {
	if len(addresses) == 0 {
		return nil
	}

	// Build (chain, address_index) IN clause, scoped to current network
	placeholders := make([]string, len(addresses))
	args := make([]interface{}, 0, 1+len(addresses)*2)
	args = append(args, d.network)
	for i, addr := range addresses {
		placeholders[i] = "(?, ?)"
		args = append(args, string(addr.Chain), addr.AddressIndex)
	}

	query := "SELECT chain, address_index, token, balance, last_scanned FROM balances WHERE network = ? AND (chain, address_index) IN (" + strings.Join(placeholders, ", ") + ")"

	rows, err := d.conn.Query(query, args...)
	if err != nil {
		return fmt.Errorf("query balances for hydration: %w", err)
	}
	defer rows.Close()

	// Map balances by (chain, index)
	type addrKey struct {
		chain string
		index int
	}
	balanceMap := make(map[addrKey][]models.Balance)

	for rows.Next() {
		var b models.Balance
		var lastScanned sql.NullString
		if err := rows.Scan(&b.Chain, &b.AddressIndex, &b.Token, &b.Balance, &lastScanned); err != nil {
			return fmt.Errorf("scan balance row: %w", err)
		}
		if lastScanned.Valid {
			b.LastScanned = lastScanned.String
		}
		key := addrKey{string(b.Chain), b.AddressIndex}
		balanceMap[key] = append(balanceMap[key], b)
	}
	if err := rows.Err(); err != nil {
		return fmt.Errorf("iterate balance rows: %w", err)
	}

	// Apply balances to addresses
	for i := range addresses {
		key := addrKey{string(addresses[i].Chain), addresses[i].AddressIndex}
		balances := balanceMap[key]

		addresses[i].NativeBalance = "0"
		addresses[i].TokenBalances = []models.TokenBalanceItem{}

		var lastScanned *string
		for _, b := range balances {
			if b.Token == models.TokenNative {
				addresses[i].NativeBalance = b.Balance
			} else {
				addresses[i].TokenBalances = append(addresses[i].TokenBalances, models.TokenBalanceItem{
					Symbol:  b.Token,
					Balance: b.Balance,
				})
			}
			if b.LastScanned != "" {
				if lastScanned == nil || b.LastScanned > *lastScanned {
					ls := b.LastScanned
					lastScanned = &ls
				}
			}
		}
		addresses[i].LastScanned = lastScanned
	}

	return nil
}

// GetAddressByIndex returns a single address by chain and index.
func (d *DB) GetAddressByIndex(chain models.Chain, index int) (*models.Address, error) {
	var addr models.Address
	err := d.conn.QueryRow(
		"SELECT chain, address_index, address, created_at FROM addresses WHERE chain = ? AND network = ? AND address_index = ?",
		string(chain), d.network, index,
	).Scan(&addr.Chain, &addr.AddressIndex, &addr.Address, &addr.CreatedAt)
	if err != nil {
		return nil, fmt.Errorf("get address %s/%d: %w", chain, index, err)
	}

	return &addr, nil
}

// DeleteAddresses deletes all addresses for a chain.
func (d *DB) DeleteAddresses(chain models.Chain) error {
	result, err := d.conn.Exec("DELETE FROM addresses WHERE chain = ? AND network = ?", string(chain), d.network)
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
		"SELECT chain, address_index, address, created_at FROM addresses WHERE chain = ? AND network = ? ORDER BY address_index",
		string(chain), d.network,
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
