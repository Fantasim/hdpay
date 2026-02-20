package logging

import (
	"context"
	"fmt"
	"io"
	"log/slog"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/Fantasim/hdpay/internal/config"
)

// standardLevels lists slog levels in ascending order.
var standardLevels = []slog.Level{
	slog.LevelDebug,
	slog.LevelInfo,
	slog.LevelWarn,
	slog.LevelError,
}

// levelName returns the lowercase name for a standard slog level.
func levelName(l slog.Level) string {
	switch l {
	case slog.LevelDebug:
		return "debug"
	case slog.LevelInfo:
		return "info"
	case slog.LevelWarn:
		return "warn"
	case slog.LevelError:
		return "error"
	default:
		return "unknown"
	}
}

// multiHandler routes log records to stdout (all levels) and per-level file handlers.
type multiHandler struct {
	level       slog.Level
	stdout      slog.Handler
	fileByLevel map[slog.Level]slog.Handler
}

func (h *multiHandler) Enabled(_ context.Context, l slog.Level) bool {
	return l >= h.level
}

func (h *multiHandler) Handle(ctx context.Context, r slog.Record) error {
	if err := h.stdout.Handle(ctx, r); err != nil {
		return err
	}
	if fh, ok := h.fileByLevel[r.Level]; ok {
		return fh.Handle(ctx, r)
	}
	return nil
}

func (h *multiHandler) WithAttrs(attrs []slog.Attr) slog.Handler {
	newFileByLevel := make(map[slog.Level]slog.Handler, len(h.fileByLevel))
	for lvl, fh := range h.fileByLevel {
		newFileByLevel[lvl] = fh.WithAttrs(attrs)
	}
	return &multiHandler{
		level:       h.level,
		stdout:      h.stdout.WithAttrs(attrs),
		fileByLevel: newFileByLevel,
	}
}

func (h *multiHandler) WithGroup(name string) slog.Handler {
	newFileByLevel := make(map[slog.Level]slog.Handler, len(h.fileByLevel))
	for lvl, fh := range h.fileByLevel {
		newFileByLevel[lvl] = fh.WithGroup(name)
	}
	return &multiHandler{
		level:       h.level,
		stdout:      h.stdout.WithGroup(name),
		fileByLevel: newFileByLevel,
	}
}

// multiCloser closes multiple io.Closers, returning the first error encountered.
type multiCloser struct {
	closers []io.Closer
}

func (mc *multiCloser) Close() error {
	var firstErr error
	for _, c := range mc.closers {
		if err := c.Close(); err != nil && firstErr == nil {
			firstErr = err
		}
	}
	return firstErr
}

// Setup initializes the global slog logger with dual output: stdout + per-level daily rotated log files.
// Uses the default HDPay log file pattern and prefix.
// Returns an io.Closer that the caller should close on shutdown (closes all log file handles).
func Setup(levelStr, logDir string) (io.Closer, error) {
	return SetupWithPrefix(levelStr, logDir, config.LogFilePattern, "hdpay-")
}

// SetupWithPrefix initializes the global slog logger with a custom log file pattern and prefix.
// filePattern is a fmt format string with two %s placeholders: date and level (e.g. "poller-%s-%s.log").
// cleanPrefix is the filename prefix used by CleanOldLogs to identify which files to clean (e.g. "poller-").
func SetupWithPrefix(levelStr, logDir, filePattern, cleanPrefix string) (io.Closer, error) {
	level, err := parseLevel(levelStr)
	if err != nil {
		return nil, fmt.Errorf("failed to parse log level %q: %w", levelStr, err)
	}

	if err := os.MkdirAll(logDir, 0o755); err != nil {
		return nil, fmt.Errorf("failed to create log directory %q: %w", logDir, err)
	}

	dateStr := time.Now().Format("2006-01-02")

	// Create stdout handler (receives all levels >= configured minimum).
	stdoutHandler := slog.NewJSONHandler(os.Stdout, &slog.HandlerOptions{
		Level: level,
	})

	// Create per-level file handlers.
	fileByLevel := make(map[slog.Level]slog.Handler)
	var closers []io.Closer
	var fileNames []string

	for _, lvl := range standardLevels {
		if lvl < level {
			continue
		}

		filename := fmt.Sprintf(filePattern, dateStr, levelName(lvl))
		logFilePath := filepath.Join(logDir, filename)

		file, err := os.OpenFile(logFilePath, os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0o644)
		if err != nil {
			// Close any files already opened before returning error.
			for _, c := range closers {
				c.Close()
			}
			return nil, fmt.Errorf("failed to open log file %q: %w", logFilePath, err)
		}

		closers = append(closers, file)
		fileNames = append(fileNames, filename)

		// Each file handler accepts all levels (filtering is done by multiHandler routing).
		fileByLevel[lvl] = slog.NewJSONHandler(file, &slog.HandlerOptions{
			Level: slog.LevelDebug, // accept everything; routing is by exact level match
		})
	}

	handler := &multiHandler{
		level:       level,
		stdout:      stdoutHandler,
		fileByLevel: fileByLevel,
	}

	slog.SetDefault(slog.New(handler))

	slog.Info("logging initialized",
		"level", levelStr,
		"logDir", logDir,
		"logFiles", fileNames,
	)

	// Clean up old log files on startup.
	removed := CleanOldLogs(logDir, config.LogMaxAgeDays, cleanPrefix)
	if removed > 0 {
		slog.Info("cleaned old log files", "removed", removed, "maxAgeDays", config.LogMaxAgeDays)
	}

	return &multiCloser{closers: closers}, nil
}

// CleanOldLogs deletes log files in logDir that are older than maxAgeDays.
// prefix filters which files to clean (e.g. "hdpay-" or "poller-").
// Returns the number of files removed.
func CleanOldLogs(logDir string, maxAgeDays int, prefix string) int {
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
		if !strings.HasPrefix(name, prefix) || !strings.HasSuffix(name, ".log") {
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
