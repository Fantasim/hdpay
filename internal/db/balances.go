package db

import (
	"fmt"
	"log/slog"
	"strings"
	"time"

	"github.com/Fantasim/hdpay/internal/models"
)

// UpsertBalance inserts or updates a single balance record.
func (d *DB) UpsertBalance(chain models.Chain, addressIndex int, token models.Token, balance string) error {
	now := time.Now().UTC().Format(time.RFC3339)

	_, err := d.conn.Exec(
		`INSERT INTO balances (chain, address_index, token, balance, last_scanned)
		 VALUES (?, ?, ?, ?, ?)
		 ON CONFLICT(chain, address_index, token) DO UPDATE SET balance = excluded.balance, last_scanned = excluded.last_scanned`,
		string(chain), addressIndex, string(token), balance, now,
	)
	if err != nil {
		return fmt.Errorf("upsert balance %s/%d/%s: %w", chain, addressIndex, token, err)
	}

	slog.Debug("balance upserted",
		"chain", chain,
		"index", addressIndex,
		"token", token,
		"balance", balance,
	)

	return nil
}

// UpsertBalanceBatch inserts or updates multiple balance records in a single transaction.
func (d *DB) UpsertBalanceBatch(balances []models.Balance) error {
	if len(balances) == 0 {
		return nil
	}

	now := time.Now().UTC().Format(time.RFC3339)

	tx, err := d.conn.Begin()
	if err != nil {
		return fmt.Errorf("begin balance batch transaction: %w", err)
	}

	stmt, err := tx.Prepare(
		`INSERT INTO balances (chain, address_index, token, balance, last_scanned)
		 VALUES (?, ?, ?, ?, ?)
		 ON CONFLICT(chain, address_index, token) DO UPDATE SET balance = excluded.balance, last_scanned = excluded.last_scanned`,
	)
	if err != nil {
		tx.Rollback()
		return fmt.Errorf("prepare balance upsert: %w", err)
	}
	defer stmt.Close()

	for _, b := range balances {
		if _, err := stmt.Exec(string(b.Chain), b.AddressIndex, string(b.Token), b.Balance, now); err != nil {
			tx.Rollback()
			return fmt.Errorf("exec balance upsert %s/%d/%s: %w", b.Chain, b.AddressIndex, b.Token, err)
		}
	}

	if err := tx.Commit(); err != nil {
		return fmt.Errorf("commit balance batch: %w", err)
	}

	slog.Debug("balance batch upserted",
		"count", len(balances),
		"chain", balances[0].Chain,
	)

	return nil
}

// GetFundedAddresses returns addresses with non-zero balance for a chain and token.
func (d *DB) GetFundedAddresses(chain models.Chain, token models.Token) ([]models.Balance, error) {
	slog.Debug("fetching funded addresses",
		"chain", chain,
		"token", token,
	)

	rows, err := d.conn.Query(
		`SELECT chain, address_index, token, balance, last_scanned FROM balances
		 WHERE chain = ? AND token = ? AND balance != '0'
		 ORDER BY address_index`,
		string(chain), string(token),
	)
	if err != nil {
		return nil, fmt.Errorf("query funded addresses %s/%s: %w", chain, token, err)
	}
	defer rows.Close()

	var results []models.Balance
	for rows.Next() {
		var b models.Balance
		var lastScanned *string
		if err := rows.Scan(&b.Chain, &b.AddressIndex, &b.Token, &b.Balance, &lastScanned); err != nil {
			return nil, fmt.Errorf("scan balance row: %w", err)
		}
		if lastScanned != nil {
			b.LastScanned = *lastScanned
		}
		results = append(results, b)
	}

	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate balance rows: %w", err)
	}

	slog.Debug("funded addresses fetched",
		"chain", chain,
		"token", token,
		"count", len(results),
	)

	return results, nil
}

// BalanceSummary holds aggregated balance info for a chain.
type BalanceSummary struct {
	Chain       models.Chain
	FundedCount int
	Tokens      map[models.Token]TokenSummary
}

// TokenSummary holds aggregate info for a specific token.
type TokenSummary struct {
	TotalBalance string
	FundedCount  int
}

// GetBalanceSummary returns aggregated balance info for a chain.
func (d *DB) GetBalanceSummary(chain models.Chain) (*BalanceSummary, error) {
	slog.Debug("fetching balance summary", "chain", chain)

	rows, err := d.conn.Query(
		`SELECT token, COUNT(*) as funded_count
		 FROM balances
		 WHERE chain = ? AND balance != '0'
		 GROUP BY token`,
		string(chain),
	)
	if err != nil {
		return nil, fmt.Errorf("query balance summary for %s: %w", chain, err)
	}
	defer rows.Close()

	summary := &BalanceSummary{
		Chain:  chain,
		Tokens: make(map[models.Token]TokenSummary),
	}

	for rows.Next() {
		var token string
		var count int
		if err := rows.Scan(&token, &count); err != nil {
			return nil, fmt.Errorf("scan summary row: %w", err)
		}
		summary.Tokens[models.Token(token)] = TokenSummary{FundedCount: count}
		summary.FundedCount += count
	}

	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate summary rows: %w", err)
	}

	slog.Debug("balance summary fetched",
		"chain", chain,
		"tokens", len(summary.Tokens),
		"totalFunded", summary.FundedCount,
	)

	return summary, nil
}

// GetAddressesBatch returns addresses for a chain within an index range.
// Used by the scanner to load batches of addresses for scanning.
func (d *DB) GetAddressesBatch(chain models.Chain, startIndex, count int) ([]models.Address, error) {
	slog.Debug("fetching address batch",
		"chain", chain,
		"startIndex", startIndex,
		"count", count,
	)

	placeholders := make([]string, count)
	args := make([]interface{}, count+1)
	args[0] = string(chain)
	for i := 0; i < count; i++ {
		placeholders[i] = "?"
		args[i+1] = startIndex + i
	}

	query := fmt.Sprintf(
		"SELECT chain, address_index, address, created_at FROM addresses WHERE chain = ? AND address_index IN (%s) ORDER BY address_index",
		strings.Join(placeholders, ","),
	)

	rows, err := d.conn.Query(query, args...)
	if err != nil {
		return nil, fmt.Errorf("query address batch %s [%d:%d]: %w", chain, startIndex, startIndex+count, err)
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
