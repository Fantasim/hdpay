package logging

import (
	"encoding/json"
	"log/slog"
	"os"
	"path/filepath"
	"strings"
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

	dateStr := time.Now().Format("2006-01-02")

	// Should create info, warn, error files (not debug since level is info).
	for _, lvl := range []string{"info", "warn", "error"} {
		expected := filepath.Join(tmpDir, "hdpay-"+dateStr+"-"+lvl+".log")
		if _, err := os.Stat(expected); os.IsNotExist(err) {
			t.Errorf("expected log file %q to exist", expected)
		}
	}

	// Debug file should NOT exist.
	debugFile := filepath.Join(tmpDir, "hdpay-"+dateStr+"-debug.log")
	if _, err := os.Stat(debugFile); !os.IsNotExist(err) {
		t.Errorf("debug log file should not exist when level is info")
	}
}

func TestSetupDebugLevel(t *testing.T) {
	tmpDir := t.TempDir()

	closer, err := Setup("debug", tmpDir)
	if err != nil {
		t.Fatalf("Setup() error = %v", err)
	}
	defer closer.Close()

	dateStr := time.Now().Format("2006-01-02")

	// Should create all 4 level files.
	for _, lvl := range []string{"debug", "info", "warn", "error"} {
		expected := filepath.Join(tmpDir, "hdpay-"+dateStr+"-"+lvl+".log")
		if _, err := os.Stat(expected); os.IsNotExist(err) {
			t.Errorf("expected log file %q to exist", expected)
		}
	}

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

func TestLogRoutesToCorrectFile(t *testing.T) {
	tmpDir := t.TempDir()

	closer, err := Setup("debug", tmpDir)
	if err != nil {
		t.Fatalf("Setup() error = %v", err)
	}

	// Write one message at each level with a unique marker.
	slog.Debug("marker-debug-only")
	slog.Info("marker-info-only")
	slog.Warn("marker-warn-only")
	slog.Error("marker-error-only")

	// Close to flush all files.
	closer.Close()

	dateStr := time.Now().Format("2006-01-02")
	levels := []string{"debug", "info", "warn", "error"}
	markers := map[string]string{
		"debug": "marker-debug-only",
		"info":  "marker-info-only",
		"warn":  "marker-warn-only",
		"error": "marker-error-only",
	}

	for _, lvl := range levels {
		filePath := filepath.Join(tmpDir, "hdpay-"+dateStr+"-"+lvl+".log")
		data, err := os.ReadFile(filePath)
		if err != nil {
			t.Fatalf("failed to read %s: %v", filePath, err)
		}
		content := string(data)

		// This level's marker MUST be present.
		expectedMarker := markers[lvl]
		if !strings.Contains(content, expectedMarker) {
			t.Errorf("%s.log should contain %q but doesn't", lvl, expectedMarker)
		}

		// Other levels' markers must NOT be present.
		for otherLvl, otherMarker := range markers {
			if otherLvl == lvl {
				continue
			}
			if strings.Contains(content, otherMarker) {
				t.Errorf("%s.log should NOT contain %q (from %s level)", lvl, otherMarker, otherLvl)
			}
		}
	}
}

func TestLogFileContainsValidJSON(t *testing.T) {
	tmpDir := t.TempDir()

	closer, err := Setup("info", tmpDir)
	if err != nil {
		t.Fatalf("Setup() error = %v", err)
	}

	slog.Warn("json-test-message", "key", "value")
	closer.Close()

	dateStr := time.Now().Format("2006-01-02")
	filePath := filepath.Join(tmpDir, "hdpay-"+dateStr+"-warn.log")
	data, err := os.ReadFile(filePath)
	if err != nil {
		t.Fatalf("failed to read warn log: %v", err)
	}

	// Each non-empty line should be valid JSON.
	for _, line := range strings.Split(strings.TrimSpace(string(data)), "\n") {
		if line == "" {
			continue
		}
		var obj map[string]interface{}
		if err := json.Unmarshal([]byte(line), &obj); err != nil {
			t.Errorf("line is not valid JSON: %s", line)
		}
	}
}

func TestCleanOldLogs_RemovesOldFiles(t *testing.T) {
	tmpDir := t.TempDir()

	// Create a "recent" log file (today).
	recentFile := filepath.Join(tmpDir, "hdpay-"+time.Now().Format("2006-01-02")+"-info.log")
	if err := os.WriteFile(recentFile, []byte("recent log"), 0o644); err != nil {
		t.Fatal(err)
	}

	// Create an "old" log file and backdate its modification time.
	oldFile := filepath.Join(tmpDir, "hdpay-2020-01-01-error.log")
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
		name := filepath.Join(tmpDir, "hdpay-"+d.Format("2006-01-02")+"-info.log")
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
