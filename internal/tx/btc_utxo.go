package tx

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"net/http"
	"sync/atomic"

	"github.com/Fantasim/hdpay/internal/config"
	"github.com/Fantasim/hdpay/internal/models"
	"github.com/Fantasim/hdpay/internal/scanner"
)

// esploraUTXO is the JSON response from Blockstream/Mempool UTXO endpoint.
type esploraUTXO struct {
	TxID   string `json:"txid"`
	Vout   uint32 `json:"vout"`
	Status struct {
		Confirmed   bool  `json:"confirmed"`
		BlockHeight int64 `json:"block_height"`
	} `json:"status"`
	Value int64 `json:"value"` // satoshis
}

// BTCUTXOFetcher fetches UTXOs for BTC addresses using Esplora-compatible APIs.
type BTCUTXOFetcher struct {
	client       *http.Client
	providerURLs []string
	rateLimiters []*scanner.RateLimiter
	nextProvider atomic.Uint64
}

// NewBTCUTXOFetcher creates a UTXO fetcher with round-robin provider rotation.
// providerURLs and rateLimiters must have the same length and correspond by index.
func NewBTCUTXOFetcher(client *http.Client, providerURLs []string, rateLimiters []*scanner.RateLimiter) *BTCUTXOFetcher {
	slog.Info("BTC UTXO fetcher created",
		"providerCount", len(providerURLs),
		"providers", providerURLs,
	)
	return &BTCUTXOFetcher{
		client:       client,
		providerURLs: providerURLs,
		rateLimiters: rateLimiters,
	}
}

// FetchUTXOs fetches confirmed UTXOs for a single address.
func (f *BTCUTXOFetcher) FetchUTXOs(ctx context.Context, address string, addressIndex int) ([]models.UTXO, error) {
	idx := int(f.nextProvider.Add(1)-1) % len(f.providerURLs)
	baseURL := f.providerURLs[idx]
	rl := f.rateLimiters[idx]

	if err := rl.Wait(ctx); err != nil {
		return nil, fmt.Errorf("rate limiter wait for UTXO fetch: %w", err)
	}

	url := fmt.Sprintf("%s/address/%s/utxo", baseURL, address)

	slog.Debug("fetching UTXOs",
		"address", address,
		"index", addressIndex,
		"provider", rl.Name(),
		"url", url,
	)

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return nil, fmt.Errorf("create UTXO request: %w", err)
	}

	resp, err := f.client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("%w: %s", config.ErrUTXOFetchFailed, err)
	}
	defer resp.Body.Close()

	if resp.StatusCode == http.StatusTooManyRequests {
		slog.Warn("UTXO fetch rate limited",
			"address", address,
			"provider", rl.Name(),
		)
		return nil, config.ErrProviderRateLimit
	}

	if resp.StatusCode != http.StatusOK {
		slog.Warn("UTXO fetch non-200 response",
			"address", address,
			"provider", rl.Name(),
			"status", resp.StatusCode,
		)
		return nil, fmt.Errorf("%w: HTTP %d from %s", config.ErrUTXOFetchFailed, resp.StatusCode, rl.Name())
	}

	var raw []esploraUTXO
	if err := json.NewDecoder(resp.Body).Decode(&raw); err != nil {
		return nil, fmt.Errorf("decode UTXO response: %w", err)
	}

	// Filter to confirmed UTXOs only.
	var utxos []models.UTXO
	for _, u := range raw {
		if !u.Status.Confirmed {
			slog.Debug("skipping unconfirmed UTXO",
				"txid", u.TxID,
				"vout", u.Vout,
				"value", u.Value,
				"address", address,
			)
			continue
		}
		utxos = append(utxos, models.UTXO{
			TxID:         u.TxID,
			Vout:         u.Vout,
			Value:        u.Value,
			Confirmed:    true,
			BlockHeight:  u.Status.BlockHeight,
			Address:      address,
			AddressIndex: addressIndex,
		})
	}

	slog.Debug("UTXOs fetched",
		"address", address,
		"index", addressIndex,
		"total", len(raw),
		"confirmed", len(utxos),
		"provider", rl.Name(),
	)

	return utxos, nil
}

// FetchAllUTXOs fetches confirmed UTXOs for multiple addresses with round-robin provider rotation.
// Returns all UTXOs concatenated. Addresses with no UTXOs are silently skipped.
func (f *BTCUTXOFetcher) FetchAllUTXOs(ctx context.Context, addresses []models.Address) ([]models.UTXO, error) {
	slog.Info("fetching UTXOs for addresses",
		"addressCount", len(addresses),
	)

	var allUTXOs []models.UTXO
	var totalValue int64

	for _, addr := range addresses {
		if err := ctx.Err(); err != nil {
			return allUTXOs, fmt.Errorf("context cancelled during UTXO fetch: %w", err)
		}

		utxos, err := f.FetchUTXOs(ctx, addr.Address, addr.AddressIndex)
		if err != nil {
			return allUTXOs, fmt.Errorf("fetch UTXOs for address %s (index %d): %w", addr.Address, addr.AddressIndex, err)
		}

		for _, u := range utxos {
			totalValue += u.Value
		}
		allUTXOs = append(allUTXOs, utxos...)
	}

	slog.Info("UTXO fetch complete",
		"addressCount", len(addresses),
		"utxoCount", len(allUTXOs),
		"totalValueSats", totalValue,
	)

	return allUTXOs, nil
}
