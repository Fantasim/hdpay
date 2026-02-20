package tx

import (
	"crypto/rand"
	"encoding/hex"
	"fmt"
	"log/slog"
	"time"
)

// GenerateSweepID generates a unique sweep identifier for grouping transactions.
func GenerateSweepID() string {
	b := make([]byte, 16)
	if _, err := rand.Read(b); err != nil {
		slog.Warn("crypto/rand failed for sweep ID, falling back to timestamp", "error", err)
		return fmt.Sprintf("sweep-%d", time.Now().UnixNano())
	}

	id := hex.EncodeToString(b)
	slog.Debug("generated sweep ID", "sweepID", id)

	return id
}

// GenerateTxStateID generates a unique ID for an individual transaction state row.
func GenerateTxStateID() string {
	b := make([]byte, 8)
	if _, err := rand.Read(b); err != nil {
		slog.Warn("crypto/rand failed for tx state ID, falling back to timestamp", "error", err)
		return fmt.Sprintf("tx-%d", time.Now().UnixNano())
	}

	id := hex.EncodeToString(b)
	slog.Debug("generated tx state ID", "txStateID", id)

	return id
}
