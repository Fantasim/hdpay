package logging

import (
	"log/slog"
	"os"
	"path/filepath"
	"testing"
	"time"
)

func TestSetup(t *testing.T) {
	tmpDir := t.TempDir()

	closer, err := Setup("info", tmpDir)
	if err != nil {
		t.Fatalf("Setup() error = %v", err)
	}
	defer closer.Close()

	expectedFile := filepath.Join(tmpDir, "hdpay-"+time.Now().Format("2006-01-02")+".log")
	if _, err := os.Stat(expectedFile); os.IsNotExist(err) {
		t.Errorf("expected log file %q to exist", expectedFile)
	}
}

func TestSetupDebugLevel(t *testing.T) {
	tmpDir := t.TempDir()

	closer, err := Setup("debug", tmpDir)
	if err != nil {
		t.Fatalf("Setup() error = %v", err)
	}
	defer closer.Close()

	slog.Debug("test debug message")
}

func TestSetupInvalidLevel(t *testing.T) {
	tmpDir := t.TempDir()

	closer, err := Setup("invalid", tmpDir)
	if closer != nil {
		defer closer.Close()
	}
	if err == nil {
		t.Fatal("expected error for invalid log level")
	}
}

func TestCleanOldLogs_RemovesOldFiles(t *testing.T) {
	tmpDir := t.TempDir()

	// Create a "recent" log file (today).
	recentFile := filepath.Join(tmpDir, "hdpay-"+time.Now().Format("2006-01-02")+".log")
	if err := os.WriteFile(recentFile, []byte("recent log"), 0o644); err != nil {
		t.Fatal(err)
	}

	// Create an "old" log file and backdate its modification time.
	oldFile := filepath.Join(tmpDir, "hdpay-2020-01-01.log")
	if err := os.WriteFile(oldFile, []byte("old log"), 0o644); err != nil {
		t.Fatal(err)
	}
	oldTime := time.Now().AddDate(0, 0, -60)
	os.Chtimes(oldFile, oldTime, oldTime)

	removed := CleanOldLogs(tmpDir, 30)

	if removed != 1 {
		t.Errorf("CleanOldLogs() removed = %d, want 1", removed)
	}

	// Recent file should still exist.
	if _, err := os.Stat(recentFile); os.IsNotExist(err) {
		t.Error("recent log file should still exist")
	}

	// Old file should be deleted.
	if _, err := os.Stat(oldFile); !os.IsNotExist(err) {
		t.Error("old log file should have been deleted")
	}
}

func TestCleanOldLogs_IgnoresNonMatchingFiles(t *testing.T) {
	tmpDir := t.TempDir()

	// Create files that don't match the hdpay-*.log pattern.
	otherFile := filepath.Join(tmpDir, "other.txt")
	if err := os.WriteFile(otherFile, []byte("not a log"), 0o644); err != nil {
		t.Fatal(err)
	}
	oldTime := time.Now().AddDate(0, 0, -60)
	os.Chtimes(otherFile, oldTime, oldTime)

	// Create a file with wrong prefix.
	wrongPrefix := filepath.Join(tmpDir, "app-2020-01-01.log")
	if err := os.WriteFile(wrongPrefix, []byte("wrong prefix"), 0o644); err != nil {
		t.Fatal(err)
	}
	os.Chtimes(wrongPrefix, oldTime, oldTime)

	removed := CleanOldLogs(tmpDir, 30)

	if removed != 0 {
		t.Errorf("CleanOldLogs() removed = %d, want 0 (non-matching files)", removed)
	}

	// Both files should still exist.
	if _, err := os.Stat(otherFile); os.IsNotExist(err) {
		t.Error("other.txt should still exist")
	}
	if _, err := os.Stat(wrongPrefix); os.IsNotExist(err) {
		t.Error("app-2020-01-01.log should still exist")
	}
}

func TestCleanOldLogs_RetainsRecentFiles(t *testing.T) {
	tmpDir := t.TempDir()

	// Create files within the retention period.
	for i := 0; i < 5; i++ {
		d := time.Now().AddDate(0, 0, -i)
		name := filepath.Join(tmpDir, "hdpay-"+d.Format("2006-01-02")+".log")
		if err := os.WriteFile(name, []byte("log"), 0o644); err != nil {
			t.Fatal(err)
		}
	}

	removed := CleanOldLogs(tmpDir, 30)

	if removed != 0 {
		t.Errorf("CleanOldLogs() removed = %d, want 0 (all recent)", removed)
	}
}

func TestCleanOldLogs_MissingDirectory(t *testing.T) {
	// Should not panic on non-existent directory.
	removed := CleanOldLogs("/tmp/nonexistent-dir-hdpay-test", 30)
	if removed != 0 {
		t.Errorf("CleanOldLogs() removed = %d, want 0", removed)
	}
}

func TestParseLevel(t *testing.T) {
	tests := []struct {
		input   string
		want    slog.Level
		wantErr bool
	}{
		{"debug", slog.LevelDebug, false},
		{"info", slog.LevelInfo, false},
		{"warn", slog.LevelWarn, false},
		{"warning", slog.LevelWarn, false},
		{"error", slog.LevelError, false},
		{"DEBUG", slog.LevelDebug, false},
		{"INFO", slog.LevelInfo, false},
		{"invalid", slog.LevelInfo, true},
	}

	for _, tt := range tests {
		t.Run(tt.input, func(t *testing.T) {
			got, err := parseLevel(tt.input)
			if (err != nil) != tt.wantErr {
				t.Errorf("parseLevel(%q) error = %v, wantErr %v", tt.input, err, tt.wantErr)
				return
			}
			if !tt.wantErr && got != tt.want {
				t.Errorf("parseLevel(%q) = %v, want %v", tt.input, got, tt.want)
			}
		})
	}
}
