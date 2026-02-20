package pollerdb

import (
	"database/sql"
	"embed"
	"fmt"
	"log/slog"
	"os"
	"path/filepath"
	"sort"
	"strings"

	pollerconfig "github.com/Fantasim/hdpay/internal/poller/config"
	_ "modernc.org/sqlite"
)

//go:embed migrations/*.sql
var migrationsFS embed.FS

// DB wraps the sql.DB connection for Poller's database.
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

	dsn := fmt.Sprintf("file:%s?_busy_timeout=%d&_journal_mode=WAL", path, pollerconfig.PollerDBBusyTimeout)
	conn, err := sql.Open("sqlite", dsn)
	if err != nil {
		return nil, fmt.Errorf("failed to open database %q: %w", path, err)
	}

	if err := conn.Ping(); err != nil {
		conn.Close()
		return nil, fmt.Errorf("failed to ping database: %w", err)
	}

	// Enable WAL mode.
	if _, err := conn.Exec("PRAGMA journal_mode=WAL"); err != nil {
		conn.Close()
		return nil, fmt.Errorf("failed to enable WAL mode: %w", err)
	}

	// Set busy timeout.
	if _, err := conn.Exec(fmt.Sprintf("PRAGMA busy_timeout=%d", pollerconfig.PollerDBBusyTimeout)); err != nil {
		conn.Close()
		return nil, fmt.Errorf("failed to set busy_timeout: %w", err)
	}

	// SQLite single writer.
	conn.SetMaxOpenConns(1)
	conn.SetMaxIdleConns(1)

	slog.Info("poller database opened", "path", path)

	return &DB{conn: conn, path: path}, nil
}

// Close closes the database connection.
func (d *DB) Close() error {
	slog.Info("closing poller database", "path", d.path)
	return d.conn.Close()
}

// Conn returns the underlying sql.DB connection.
func (d *DB) Conn() *sql.DB {
	return d.conn
}

// RunMigrations applies all pending SQL migration files from the embedded filesystem.
func (d *DB) RunMigrations() error {
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

	sort.Slice(entries, func(i, j int) bool {
		return entries[i].Name() < entries[j].Name()
	})

	for _, entry := range entries {
		if entry.IsDir() || !strings.HasSuffix(entry.Name(), ".sql") {
			continue
		}

		var version int
		if _, err := fmt.Sscanf(entry.Name(), "%d", &version); err != nil {
			slog.Warn("skipping migration with unparseable version", "file", entry.Name())
			continue
		}

		var count int
		if err := d.conn.QueryRow("SELECT COUNT(*) FROM schema_migrations WHERE version = ?", version).Scan(&count); err != nil {
			return fmt.Errorf("failed to check migration status for version %d: %w", version, err)
		}

		if count > 0 {
			slog.Debug("migration already applied", "version", version, "file", entry.Name())
			continue
		}

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
