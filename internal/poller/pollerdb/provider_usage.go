package pollerdb

import (
	"fmt"
	"log/slog"
	"time"

	"github.com/Fantasim/hdpay/internal/poller/config"
)

// ProviderUsageRow represents a single day's usage counters for one provider.
type ProviderUsageRow struct {
	Chain    string
	Provider string
	Date     string
	Requests int64
	Successes int64
	Failures int64
	Hits429  int64
}

// IncrementUsage atomically increments today's usage counters for a provider.
func (d *DB) IncrementUsage(chain, provider string, success bool, is429 bool) error {
	date := time.Now().UTC().Format(config.ProviderUsageDateFormat)

	var query string
	if success {
		query = `
			INSERT INTO provider_usage (chain, provider, date, requests, successes, failures, hits_429)
			VALUES (?, ?, ?, 1, 1, 0, 0)
			ON CONFLICT (chain, provider, date) DO UPDATE SET
				requests = requests + 1,
				successes = successes + 1`
	} else if is429 {
		query = `
			INSERT INTO provider_usage (chain, provider, date, requests, successes, failures, hits_429)
			VALUES (?, ?, ?, 1, 0, 1, 1)
			ON CONFLICT (chain, provider, date) DO UPDATE SET
				requests = requests + 1,
				failures = failures + 1,
				hits_429 = hits_429 + 1`
	} else {
		query = `
			INSERT INTO provider_usage (chain, provider, date, requests, successes, failures, hits_429)
			VALUES (?, ?, ?, 1, 0, 1, 0)
			ON CONFLICT (chain, provider, date) DO UPDATE SET
				requests = requests + 1,
				failures = failures + 1`
	}

	_, err := d.conn.Exec(query, chain, provider, date)
	if err != nil {
		return fmt.Errorf("failed to increment provider usage for %s/%s: %w", chain, provider, err)
	}

	slog.Debug("provider usage incremented",
		"chain", chain,
		"provider", provider,
		"date", date,
		"success", success,
		"is429", is429,
	)
	return nil
}

// GetDailyUsage returns all provider usage rows for a specific date.
func (d *DB) GetDailyUsage(date string) ([]ProviderUsageRow, error) {
	rows, err := d.conn.Query(`
		SELECT chain, provider, date, requests, successes, failures, hits_429
		FROM provider_usage
		WHERE date = ?
		ORDER BY chain, provider`, date)
	if err != nil {
		return nil, fmt.Errorf("failed to query daily usage for %s: %w", date, err)
	}
	defer rows.Close()

	var result []ProviderUsageRow
	for rows.Next() {
		var r ProviderUsageRow
		if err := rows.Scan(&r.Chain, &r.Provider, &r.Date, &r.Requests, &r.Successes, &r.Failures, &r.Hits429); err != nil {
			return nil, fmt.Errorf("failed to scan provider usage row: %w", err)
		}
		result = append(result, r)
	}
	return result, rows.Err()
}

// GetMonthlyUsage returns aggregated usage for a chain/provider over the last N days.
func (d *DB) GetMonthlyUsage(chain, provider string, days int) (*ProviderUsageRow, error) {
	fromDate := time.Now().UTC().AddDate(0, 0, -days).Format(config.ProviderUsageDateFormat)

	r := &ProviderUsageRow{Chain: chain, Provider: provider}
	err := d.conn.QueryRow(`
		SELECT COALESCE(SUM(requests), 0), COALESCE(SUM(successes), 0),
		       COALESCE(SUM(failures), 0), COALESCE(SUM(hits_429), 0)
		FROM provider_usage
		WHERE chain = ? AND provider = ? AND date >= ?`,
		chain, provider, fromDate,
	).Scan(&r.Requests, &r.Successes, &r.Failures, &r.Hits429)
	if err != nil {
		return nil, fmt.Errorf("failed to query monthly usage for %s/%s: %w", chain, provider, err)
	}
	return r, nil
}

// GetAllDailyUsage returns today's usage grouped by chain and provider.
func (d *DB) GetAllDailyUsage() ([]ProviderUsageRow, error) {
	today := time.Now().UTC().Format(config.ProviderUsageDateFormat)
	return d.GetDailyUsage(today)
}

// GetAllMonthlyUsage returns aggregated usage over the last 30 days for all providers.
func (d *DB) GetAllMonthlyUsage() ([]ProviderUsageRow, error) {
	fromDate := time.Now().UTC().AddDate(0, 0, -config.ProviderUsageMonthDays).Format(config.ProviderUsageDateFormat)

	rows, err := d.conn.Query(`
		SELECT chain, provider, '' AS date,
		       COALESCE(SUM(requests), 0), COALESCE(SUM(successes), 0),
		       COALESCE(SUM(failures), 0), COALESCE(SUM(hits_429), 0)
		FROM provider_usage
		WHERE date >= ?
		GROUP BY chain, provider
		ORDER BY chain, provider`, fromDate)
	if err != nil {
		return nil, fmt.Errorf("failed to query all monthly usage: %w", err)
	}
	defer rows.Close()

	var result []ProviderUsageRow
	for rows.Next() {
		var r ProviderUsageRow
		if err := rows.Scan(&r.Chain, &r.Provider, &r.Date, &r.Requests, &r.Successes, &r.Failures, &r.Hits429); err != nil {
			return nil, fmt.Errorf("failed to scan monthly usage row: %w", err)
		}
		result = append(result, r)
	}
	return result, rows.Err()
}
