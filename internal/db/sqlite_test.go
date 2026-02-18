package db

import (
	"os"
	"path/filepath"
	"testing"
)

func TestNewDB(t *testing.T) {
	tmpDir := t.TempDir()
	dbPath := filepath.Join(tmpDir, "test.sqlite")

	d, err := New(dbPath)
	if err != nil {
		t.Fatalf("New() error = %v", err)
	}
	defer d.Close()

	if _, err := os.Stat(dbPath); os.IsNotExist(err) {
		t.Error("expected database file to be created")
	}

	// Verify WAL mode
	var mode string
	if err := d.Conn().QueryRow("PRAGMA journal_mode").Scan(&mode); err != nil {
		t.Fatalf("failed to query journal_mode: %v", err)
	}
	if mode != "wal" {
		t.Errorf("expected journal_mode=wal, got %q", mode)
	}
}

func TestRunMigrations(t *testing.T) {
	tmpDir := t.TempDir()
	dbPath := filepath.Join(tmpDir, "test.sqlite")

	d, err := New(dbPath)
	if err != nil {
		t.Fatalf("New() error = %v", err)
	}
	defer d.Close()

	if err := d.RunMigrations(); err != nil {
		t.Fatalf("RunMigrations() error = %v", err)
	}

	// Verify tables exist
	tables := []string{"addresses", "balances", "scan_state", "transactions", "settings", "schema_migrations"}
	for _, table := range tables {
		var name string
		err := d.Conn().QueryRow("SELECT name FROM sqlite_master WHERE type='table' AND name=?", table).Scan(&name)
		if err != nil {
			t.Errorf("table %q not found: %v", table, err)
		}
	}
}

func TestRunMigrationsIdempotent(t *testing.T) {
	tmpDir := t.TempDir()
	dbPath := filepath.Join(tmpDir, "test.sqlite")

	d, err := New(dbPath)
	if err != nil {
		t.Fatalf("New() error = %v", err)
	}
	defer d.Close()

	if err := d.RunMigrations(); err != nil {
		t.Fatalf("first RunMigrations() error = %v", err)
	}

	if err := d.RunMigrations(); err != nil {
		t.Fatalf("second RunMigrations() error = %v", err)
	}

	// Verify each migration recorded exactly once (no duplicates from second run)
	var count int
	if err := d.Conn().QueryRow("SELECT COUNT(*) FROM schema_migrations").Scan(&count); err != nil {
		t.Fatalf("failed to count migrations: %v", err)
	}
	// Count all embedded migration files to verify idempotency
	entries, _ := migrationsFS.ReadDir("migrations")
	expectedCount := 0
	for _, e := range entries {
		if !e.IsDir() {
			expectedCount++
		}
	}
	if count != expectedCount {
		t.Errorf("expected %d migration records, got %d", expectedCount, count)
	}
}
