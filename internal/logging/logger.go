package logging

import (
	"fmt"
	"io"
	"log/slog"
	"os"
	"strings"
	"time"
)

// Setup initializes the global slog logger with dual output: stdout + daily rotated log file.
func Setup(levelStr, logDir string) error {
	level, err := parseLevel(levelStr)
	if err != nil {
		return fmt.Errorf("failed to parse log level %q: %w", levelStr, err)
	}

	if err := os.MkdirAll(logDir, 0o755); err != nil {
		return fmt.Errorf("failed to create log directory %q: %w", logDir, err)
	}

	filename := fmt.Sprintf("hdpay-%s.log", time.Now().Format("2006-01-02"))
	filepath := fmt.Sprintf("%s/%s", logDir, filename)

	file, err := os.OpenFile(filepath, os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0o644)
	if err != nil {
		return fmt.Errorf("failed to open log file %q: %w", filepath, err)
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

	return nil
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
