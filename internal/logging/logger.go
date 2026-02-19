package logging

import (
	"fmt"
	"io"
	"log/slog"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/Fantasim/hdpay/internal/config"
)

// Setup initializes the global slog logger with dual output: stdout + daily rotated log file.
// Returns an io.Closer that the caller should close on shutdown (closes the log file handle).
func Setup(levelStr, logDir string) (io.Closer, error) {
	level, err := parseLevel(levelStr)
	if err != nil {
		return nil, fmt.Errorf("failed to parse log level %q: %w", levelStr, err)
	}

	if err := os.MkdirAll(logDir, 0o755); err != nil {
		return nil, fmt.Errorf("failed to create log directory %q: %w", logDir, err)
	}

	filename := fmt.Sprintf("hdpay-%s.log", time.Now().Format("2006-01-02"))
	logFilePath := fmt.Sprintf("%s/%s", logDir, filename)

	file, err := os.OpenFile(logFilePath, os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0o644)
	if err != nil {
		return nil, fmt.Errorf("failed to open log file %q: %w", logFilePath, err)
	}

	writer := io.MultiWriter(os.Stdout, file)

	handler := slog.NewJSONHandler(writer, &slog.HandlerOptions{
		Level: level,
	})

	slog.SetDefault(slog.New(handler))

	slog.Info("logging initialized",
		"level", levelStr,
		"logDir", logDir,
		"logFile", filename,
	)

	// Clean up old log files on startup.
	removed := CleanOldLogs(logDir, config.LogMaxAgeDays)
	if removed > 0 {
		slog.Info("cleaned old log files", "removed", removed, "maxAgeDays", config.LogMaxAgeDays)
	}

	return file, nil
}

// CleanOldLogs deletes log files in logDir that are older than maxAgeDays.
// Returns the number of files removed.
func CleanOldLogs(logDir string, maxAgeDays int) int {
	cutoff := time.Now().AddDate(0, 0, -maxAgeDays)
	removed := 0

	entries, err := os.ReadDir(logDir)
	if err != nil {
		slog.Warn("failed to read log directory for cleanup", "logDir", logDir, "error", err)
		return 0
	}

	for _, entry := range entries {
		if entry.IsDir() {
			continue
		}

		name := entry.Name()
		if !strings.HasPrefix(name, "hdpay-") || !strings.HasSuffix(name, ".log") {
			continue
		}

		info, err := entry.Info()
		if err != nil {
			continue
		}

		if info.ModTime().Before(cutoff) {
			fullPath := filepath.Join(logDir, name)
			if err := os.Remove(fullPath); err != nil {
				slog.Warn("failed to remove old log file", "file", fullPath, "error", err)
			} else {
				removed++
			}
		}
	}

	return removed
}

func parseLevel(s string) (slog.Level, error) {
	switch strings.ToLower(s) {
	case "debug":
		return slog.LevelDebug, nil
	case "info":
		return slog.LevelInfo, nil
	case "warn", "warning":
		return slog.LevelWarn, nil
	case "error":
		return slog.LevelError, nil
	default:
		return slog.LevelInfo, fmt.Errorf("unknown log level: %s", s)
	}
}
