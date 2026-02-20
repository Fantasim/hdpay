package pollerdb

import (
	"database/sql"
	"fmt"
	"log/slog"

	"github.com/Fantasim/hdpay/internal/poller/models"
)

// ListAllowedIPs returns all IPs in the allowlist.
func (d *DB) ListAllowedIPs() ([]models.IPAllowEntry, error) {
	rows, err := d.conn.Query(`
		SELECT id, ip, description, added_at FROM ip_allowlist ORDER BY added_at DESC`)
	if err != nil {
		return nil, fmt.Errorf("failed to list allowed IPs: %w", err)
	}
	defer rows.Close()

	var entries []models.IPAllowEntry
	for rows.Next() {
		var e models.IPAllowEntry
		var desc sql.NullString
		if err := rows.Scan(&e.ID, &e.IP, &desc, &e.AddedAt); err != nil {
			return nil, fmt.Errorf("failed to scan IP allowlist row: %w", err)
		}
		if desc.Valid {
			e.Description = desc.String
		}
		entries = append(entries, e)
	}
	return entries, rows.Err()
}

// AddIP adds an IP to the allowlist.
func (d *DB) AddIP(ip, description string) (int64, error) {
	result, err := d.conn.Exec(`
		INSERT INTO ip_allowlist (ip, description) VALUES (?, ?)`,
		ip, description,
	)
	if err != nil {
		return 0, fmt.Errorf("failed to add IP %s to allowlist: %w", ip, err)
	}

	id, _ := result.LastInsertId()
	slog.Info("IP added to allowlist", "ip", ip, "description", description, "id", id)
	return id, nil
}

// RemoveIP removes an IP from the allowlist by ID.
func (d *DB) RemoveIP(id int) error {
	result, err := d.conn.Exec(`DELETE FROM ip_allowlist WHERE id = ?`, id)
	if err != nil {
		return fmt.Errorf("failed to remove IP allowlist entry %d: %w", id, err)
	}

	affected, _ := result.RowsAffected()
	if affected == 0 {
		return fmt.Errorf("IP allowlist entry %d not found", id)
	}

	slog.Info("IP removed from allowlist", "id", id)
	return nil
}

// IsIPAllowed checks if an IP exists in the allowlist.
func (d *DB) IsIPAllowed(ip string) (bool, error) {
	var count int
	err := d.conn.QueryRow(`SELECT COUNT(*) FROM ip_allowlist WHERE ip = ?`, ip).Scan(&count)
	if err != nil {
		return false, fmt.Errorf("failed to check IP %s in allowlist: %w", ip, err)
	}
	return count > 0, nil
}

// LoadAllIPsIntoMap returns all allowed IPs as a map for in-memory caching.
func (d *DB) LoadAllIPsIntoMap() (map[string]bool, error) {
	rows, err := d.conn.Query(`SELECT ip FROM ip_allowlist`)
	if err != nil {
		return nil, fmt.Errorf("failed to load IP allowlist: %w", err)
	}
	defer rows.Close()

	ips := make(map[string]bool)
	for rows.Next() {
		var ip string
		if err := rows.Scan(&ip); err != nil {
			return nil, fmt.Errorf("failed to scan IP: %w", err)
		}
		ips[ip] = true
	}
	return ips, rows.Err()
}
