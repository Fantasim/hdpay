package pollerdb

import (
	"database/sql"
	"fmt"
	"log/slog"

	"github.com/Fantasim/hdpay/internal/poller/models"
)

// GetOrCreatePoints retrieves the points account for an address+chain,
// creating it if it doesn't exist.
func (d *DB) GetOrCreatePoints(address, chain string) (*models.PointsAccount, error) {
	_, err := d.conn.Exec(`
		INSERT OR IGNORE INTO points (address, chain, unclaimed, pending, total, updated_at)
		VALUES (?, ?, 0, 0, 0, CURRENT_TIMESTAMP)`,
		address, chain,
	)
	if err != nil {
		return nil, fmt.Errorf("failed to ensure points row for %s/%s: %w", address, chain, err)
	}

	p := &models.PointsAccount{}
	err = d.conn.QueryRow(`
		SELECT address, chain, unclaimed, pending, total, updated_at
		FROM points WHERE address = ? AND chain = ?`, address, chain,
	).Scan(&p.Address, &p.Chain, &p.Unclaimed, &p.Pending, &p.Total, &p.UpdatedAt)
	if err != nil {
		return nil, fmt.Errorf("failed to get points for %s/%s: %w", address, chain, err)
	}
	return p, nil
}

// AddUnclaimed adds confirmed points to unclaimed and total.
func (d *DB) AddUnclaimed(address, chain string, points int) error {
	_, err := d.conn.Exec(`
		UPDATE points SET unclaimed = unclaimed + ?, total = total + ?, updated_at = CURRENT_TIMESTAMP
		WHERE address = ? AND chain = ?`,
		points, points, address, chain,
	)
	if err != nil {
		return fmt.Errorf("failed to add unclaimed points for %s/%s: %w", address, chain, err)
	}

	slog.Info("unclaimed points added",
		"address", address,
		"chain", chain,
		"points", points,
	)
	return nil
}

// AddPending adds pending (unconfirmed) points.
func (d *DB) AddPending(address, chain string, points int) error {
	_, err := d.conn.Exec(`
		UPDATE points SET pending = pending + ?, updated_at = CURRENT_TIMESTAMP
		WHERE address = ? AND chain = ?`,
		points, address, chain,
	)
	if err != nil {
		return fmt.Errorf("failed to add pending points for %s/%s: %w", address, chain, err)
	}
	return nil
}

// MovePendingToUnclaimed moves points from pending to unclaimed+total when a tx confirms.
func (d *DB) MovePendingToUnclaimed(address, chain string, pendingPoints, confirmedPoints int) error {
	_, err := d.conn.Exec(`
		UPDATE points
		SET pending = pending - ?,
		    unclaimed = unclaimed + ?,
		    total = total + ?,
		    updated_at = CURRENT_TIMESTAMP
		WHERE address = ? AND chain = ?`,
		pendingPoints, confirmedPoints, confirmedPoints, address, chain,
	)
	if err != nil {
		return fmt.Errorf("failed to move pending to unclaimed for %s/%s: %w", address, chain, err)
	}

	slog.Info("pending points moved to unclaimed",
		"address", address,
		"chain", chain,
		"pendingRemoved", pendingPoints,
		"unclaimedAdded", confirmedPoints,
	)
	return nil
}

// ClaimPoints resets unclaimed to 0 for the given address+chain. Returns the amount claimed.
func (d *DB) ClaimPoints(address, chain string) (int, error) {
	var unclaimed int
	err := d.conn.QueryRow(`
		SELECT unclaimed FROM points WHERE address = ? AND chain = ?`,
		address, chain,
	).Scan(&unclaimed)
	if err == sql.ErrNoRows {
		return 0, nil // Skip silently per spec
	}
	if err != nil {
		return 0, fmt.Errorf("failed to get unclaimed for %s/%s: %w", address, chain, err)
	}

	if unclaimed == 0 {
		return 0, nil
	}

	_, err = d.conn.Exec(`
		UPDATE points SET unclaimed = 0, updated_at = CURRENT_TIMESTAMP
		WHERE address = ? AND chain = ?`,
		address, chain,
	)
	if err != nil {
		return 0, fmt.Errorf("failed to claim points for %s/%s: %w", address, chain, err)
	}

	slog.Info("points claimed",
		"address", address,
		"chain", chain,
		"pointsClaimed", unclaimed,
	)
	return unclaimed, nil
}

// ListWithUnclaimed returns all points accounts with unclaimed > 0.
func (d *DB) ListWithUnclaimed() ([]models.PointsAccount, error) {
	rows, err := d.conn.Query(`
		SELECT address, chain, unclaimed, pending, total, updated_at
		FROM points WHERE unclaimed > 0
		ORDER BY unclaimed DESC`)
	if err != nil {
		return nil, fmt.Errorf("failed to list unclaimed points: %w", err)
	}
	defer rows.Close()

	var accounts []models.PointsAccount
	for rows.Next() {
		var p models.PointsAccount
		if err := rows.Scan(&p.Address, &p.Chain, &p.Unclaimed, &p.Pending, &p.Total, &p.UpdatedAt); err != nil {
			return nil, fmt.Errorf("failed to scan points row: %w", err)
		}
		accounts = append(accounts, p)
	}
	return accounts, rows.Err()
}

// ListWithPending returns all points accounts with pending > 0.
func (d *DB) ListWithPending() ([]models.PointsAccount, error) {
	rows, err := d.conn.Query(`
		SELECT address, chain, unclaimed, pending, total, updated_at
		FROM points WHERE pending > 0
		ORDER BY pending DESC`)
	if err != nil {
		return nil, fmt.Errorf("failed to list pending points: %w", err)
	}
	defer rows.Close()

	var accounts []models.PointsAccount
	for rows.Next() {
		var p models.PointsAccount
		if err := rows.Scan(&p.Address, &p.Chain, &p.Unclaimed, &p.Pending, &p.Total, &p.UpdatedAt); err != nil {
			return nil, fmt.Errorf("failed to scan points row: %w", err)
		}
		accounts = append(accounts, p)
	}
	return accounts, rows.Err()
}
