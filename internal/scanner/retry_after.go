package scanner

import (
	"log/slog"
	"net/http"
	"strconv"
	"time"
)

// parseRetryAfter extracts a duration from the Retry-After HTTP response header.
// Supports seconds format (e.g., "30") and HTTP-date format (e.g., "Thu, 01 Dec 1994 16:00:00 GMT").
// Returns 0 if the header is missing, unparseable, or in the past.
func parseRetryAfter(header http.Header) time.Duration {
	val := header.Get("Retry-After")
	if val == "" {
		return 0
	}

	// Try seconds format first (most common for APIs).
	if seconds, err := strconv.Atoi(val); err == nil && seconds > 0 {
		slog.Debug("parsed Retry-After as seconds", "seconds", seconds)
		return time.Duration(seconds) * time.Second
	}

	// Try HTTP-date format.
	if t, err := http.ParseTime(val); err == nil {
		d := time.Until(t)
		if d > 0 {
			slog.Debug("parsed Retry-After as HTTP-date", "duration", d)
			return d
		}
	}

	slog.Debug("unparseable Retry-After header", "value", val)
	return 0
}
