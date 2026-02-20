package pollerdb

import (
	"database/sql"
	"fmt"
	"log/slog"

	"github.com/Fantasim/hdpay/internal/poller/models"
)

// InsertError records a system error.
func (d *DB) InsertError(severity, category, message, details string) (int64, error) {
	result, err := d.conn.Exec(`
		INSERT INTO system_errors (severity, category, message, details)
		VALUES (?, ?, ?, ?)`,
		severity, category, message, details,
	)
	if err != nil {
		return 0, fmt.Errorf("failed to insert system error: %w", err)
	}

	id, _ := result.LastInsertId()
	slog.Warn("system error recorded",
		"id", id,
		"severity", severity,
		"category", category,
		"message", message,
	)
	return id, nil
}

// ListUnresolved returns all unresolved system errors.
func (d *DB) ListUnresolved() ([]models.SystemError, error) {
	return d.queryErrors(`
		SELECT id, severity, category, message, details, resolved, created_at
		FROM system_errors WHERE resolved = FALSE
		ORDER BY created_at DESC`)
}

// MarkResolved marks a system error as resolved.
func (d *DB) MarkResolved(id int) error {
	result, err := d.conn.Exec(`UPDATE system_errors SET resolved = TRUE WHERE id = ?`, id)
	if err != nil {
		return fmt.Errorf("failed to resolve system error %d: %w", id, err)
	}

	affected, _ := result.RowsAffected()
	if affected == 0 {
		return fmt.Errorf("system error %d not found", id)
	}

	slog.Info("system error resolved", "id", id)
	return nil
}

// ListByCategory returns system errors filtered by category.
func (d *DB) ListByCategory(category string) ([]models.SystemError, error) {
	return d.queryErrors(`
		SELECT id, severity, category, message, details, resolved, created_at
		FROM system_errors WHERE category = ?
		ORDER BY created_at DESC`, category)
}

func (d *DB) queryErrors(query string, args ...interface{}) ([]models.SystemError, error) {
	rows, err := d.conn.Query(query, args...)
	if err != nil {
		return nil, fmt.Errorf("failed to query system errors: %w", err)
	}
	defer rows.Close()

	var errors []models.SystemError
	for rows.Next() {
		var e models.SystemError
		var details sql.NullString
		if err := rows.Scan(&e.ID, &e.Severity, &e.Category, &e.Message, &details, &e.Resolved, &e.CreatedAt); err != nil {
			return nil, fmt.Errorf("failed to scan system error row: %w", err)
		}
		if details.Valid {
			e.Details = details.String
		}
		errors = append(errors, e)
	}
	return errors, rows.Err()
}
