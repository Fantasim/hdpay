package db

import (
	"database/sql"
	"embed"
	"fmt"
	"log/slog"
	"os"
	"path/filepath"
	"sort"
	"strings"

	_ "modernc.org/sqlite"
)

//go:embed migrations/*.sql
var migrationsFS embed.FS

// DB wraps the sql.DB connection with application-specific methods.
type DB struct {
	conn *sql.DB
	path string
}

// New opens a SQLite database at the given path with WAL mode and busy timeout.
func New(path string) (*DB, error) {
	dir := filepath.Dir(path)
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return nil, fmt.Errorf("failed to create database directory %q: %w", dir, err)
	}

	dsn := fmt.Sprintf("file:%s?_busy_timeout=5000&_journal_mode=WAL", path)
	conn, err := sql.Open("sqlite", dsn)
	if err != nil {
		return nil, fmt.Errorf("failed to open database %q: %w", path, err)
	}

	if err := conn.Ping(); err != nil {
		conn.Close()
		return nil, fmt.Errorf("failed to ping database: %w", err)
	}

	// Enable WAL mode
	if _, err := conn.Exec("PRAGMA journal_mode=WAL"); err != nil {
		conn.Close()
		return nil, fmt.Errorf("failed to enable WAL mode: %w", err)
	}

	// Verify WAL mode
	var mode string
	if err := conn.QueryRow("PRAGMA journal_mode").Scan(&mode); err != nil {
		conn.Close()
		return nil, fmt.Errorf("failed to verify WAL mode: %w", err)
	}

	slog.Debug("database WAL mode", "mode", mode)

	return &DB{conn: conn, path: path}, nil
}

// Close closes the database connection.
func (d *DB) Close() error {
	slog.Info("closing database", "path", d.path)
	return d.conn.Close()
}

// Conn returns the underlying sql.DB connection.
func (d *DB) Conn() *sql.DB {
	return d.conn
}

// RunMigrations applies all pending SQL migration files from the embedded filesystem.
func (d *DB) RunMigrations() error {
	// Ensure schema_migrations table exists
	if _, err := d.conn.Exec(`
		CREATE TABLE IF NOT EXISTS schema_migrations (
			version INTEGER PRIMARY KEY,
			applied_at TEXT NOT NULL DEFAULT (datetime('now'))
		)
	`); err != nil {
		return fmt.Errorf("failed to create schema_migrations table: %w", err)
	}

	entries, err := migrationsFS.ReadDir("migrations")
	if err != nil {
		return fmt.Errorf("failed to read migrations directory: %w", err)
	}

	// Sort migrations by filename
	sort.Slice(entries, func(i, j int) bool {
		return entries[i].Name() < entries[j].Name()
	})

	for _, entry := range entries {
		if entry.IsDir() || !strings.HasSuffix(entry.Name(), ".sql") {
			continue
		}

		// Extract version number from filename (e.g., "001_initial.sql" → 1)
		var version int
		if _, err := fmt.Sscanf(entry.Name(), "%d", &version); err != nil {
			slog.Warn("skipping migration with unparseable version", "file", entry.Name())
			continue
		}

		// Check if already applied
		var count int
		if err := d.conn.QueryRow("SELECT COUNT(*) FROM schema_migrations WHERE version = ?", version).Scan(&count); err != nil {
			return fmt.Errorf("failed to check migration status for version %d: %w", version, err)
		}

		if count > 0 {
			slog.Debug("migration already applied", "version", version, "file", entry.Name())
			continue
		}

		// Read and execute migration
		content, err := migrationsFS.ReadFile("migrations/" + entry.Name())
		if err != nil {
			return fmt.Errorf("failed to read migration %s: %w", entry.Name(), err)
		}

		slog.Info("applying migration", "version", version, "file", entry.Name())

		tx, err := d.conn.Begin()
		if err != nil {
			return fmt.Errorf("failed to begin transaction for migration %d: %w", version, err)
		}

		if _, err := tx.Exec(string(content)); err != nil {
			tx.Rollback()
			return fmt.Errorf("failed to execute migration %s: %w", entry.Name(), err)
		}

		if _, err := tx.Exec("INSERT INTO schema_migrations (version) VALUES (?)", version); err != nil {
			tx.Rollback()
			return fmt.Errorf("failed to record migration %d: %w", version, err)
		}

		if err := tx.Commit(); err != nil {
			return fmt.Errorf("failed to commit migration %d: %w", version, err)
		}

		slog.Info("migration applied", "version", version, "file", entry.Name())
	}

	return nil
}
