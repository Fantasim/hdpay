package tx

import (
	"context"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"strings"

	"github.com/Fantasim/hdpay/internal/config"
)

// Broadcaster broadcasts a raw signed transaction to the network.
type Broadcaster interface {
	Broadcast(ctx context.Context, rawHex string) (txHash string, err error)
}

// BTCBroadcaster broadcasts BTC transactions via Esplora-compatible APIs.
// Tries providers in order, falling back to the next on network/server errors.
type BTCBroadcaster struct {
	client       *http.Client
	providerURLs []string
}

// NewBTCBroadcaster creates a broadcaster with ordered fallback providers.
func NewBTCBroadcaster(client *http.Client, providerURLs []string) *BTCBroadcaster {
	slog.Info("BTC broadcaster created",
		"providerCount", len(providerURLs),
		"providers", providerURLs,
	)
	return &BTCBroadcaster{
		client:       client,
		providerURLs: providerURLs,
	}
}

// Broadcast sends the raw transaction hex to the BTC network.
// Tries each provider in order. Does NOT retry on 400 (bad transaction).
func (b *BTCBroadcaster) Broadcast(ctx context.Context, rawHex string) (string, error) {
	slog.Info("broadcasting BTC transaction", "hexLength", len(rawHex))

	var lastErr error

	for i, baseURL := range b.providerURLs {
		txHash, err := b.broadcastToProvider(ctx, rawHex, baseURL)
		if err == nil {
			slog.Info("BTC broadcast successful",
				"provider", baseURL,
				"txHash", txHash,
			)
			return txHash, nil
		}

		lastErr = err

		// Don't retry on 400 (bad transaction) — the TX itself is invalid.
		if isBadTxError(err) {
			slog.Error("BTC broadcast rejected (bad transaction)",
				"provider", baseURL,
				"error", err,
			)
			return "", fmt.Errorf("%w: %s", config.ErrTransactionFailed, err)
		}

		slog.Warn("BTC broadcast failed, trying next provider",
			"provider", baseURL,
			"providerIndex", i,
			"remaining", len(b.providerURLs)-i-1,
			"error", err,
		)
	}

	return "", fmt.Errorf("%w: all providers failed: %s", config.ErrTransactionFailed, lastErr)
}

// broadcastToProvider sends the raw hex to a single Esplora provider.
func (b *BTCBroadcaster) broadcastToProvider(ctx context.Context, rawHex string, baseURL string) (string, error) {
	url := baseURL + "/tx"

	slog.Debug("broadcast request", "url", url)

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, url, strings.NewReader(rawHex))
	if err != nil {
		return "", fmt.Errorf("create broadcast request: %w", err)
	}
	req.Header.Set("Content-Type", "text/plain")

	resp, err := b.client.Do(req)
	if err != nil {
		return "", fmt.Errorf("broadcast request to %s: %w", baseURL, err)
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return "", fmt.Errorf("read broadcast response: %w", err)
	}

	if resp.StatusCode == http.StatusBadRequest {
		return "", &badTxError{message: strings.TrimSpace(string(body))}
	}

	if resp.StatusCode != http.StatusOK {
		return "", fmt.Errorf("broadcast HTTP %d from %s: %s", resp.StatusCode, baseURL, string(body))
	}

	txHash := strings.TrimSpace(string(body))

	slog.Debug("broadcast response",
		"provider", baseURL,
		"status", resp.StatusCode,
		"txHash", txHash,
	)

	return txHash, nil
}

// badTxError represents a 400 response — the transaction itself is invalid.
type badTxError struct {
	message string
}

func (e *badTxError) Error() string {
	return "bad transaction: " + e.message
}

// isBadTxError checks if an error is a bad transaction error (400 response).
func isBadTxError(err error) bool {
	_, ok := err.(*badTxError)
	return ok
}
