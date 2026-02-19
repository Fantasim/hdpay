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
