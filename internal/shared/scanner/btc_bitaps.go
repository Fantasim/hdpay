package scanner

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"net/http"
	"strconv"

	"github.com/Fantasim/hdpay/internal/shared/config"
	"github.com/Fantasim/hdpay/internal/shared/models"
)

// bitapsAddressResponse is the Bitaps /address/state/{address} response.
type bitapsAddressResponse struct {
	Data struct {
		Balance        int64 `json:"balance"`
		PendingReceived int64 `json:"pending_received"`
		PendingSpent   int64 `json:"pending_spent"`
	} `json:"data"`
	ErrorList []interface{} `json:"error_list"`
}

// BitapsProvider fetches BTC balances from the Bitaps REST API.
// API docs: https://developer.bitaps.com/blockchain
// Free tier — no API key required.
type BitapsProvider struct {
	client  *http.Client
	rl      *RateLimiter
	baseURL string
}

// NewBitapsProvider creates a provider for the Bitaps blockchain API.
func NewBitapsProvider(client *http.Client, rl *RateLimiter, network string) *BitapsProvider {
	baseURL := config.BitapsMainnetURL
	if network == string(models.NetworkTestnet) {
		baseURL = config.BitapsTestnetURL
	}

	slog.Info("bitaps provider created",
		"baseURL", baseURL,
		"network", network,
	)

	return &BitapsProvider{
		client:  client,
		rl:      rl,
		baseURL: baseURL,
	}
}

func (p *BitapsProvider) Name() string              { return "Bitaps" }
func (p *BitapsProvider) Chain() models.Chain        { return models.ChainBTC }
func (p *BitapsProvider) MaxBatchSize() int          { return 1 }
func (p *BitapsProvider) RecordSuccess()             { p.rl.RecordSuccess() }
func (p *BitapsProvider) RecordFailure(is429 bool)   { p.rl.RecordFailure(is429) }
func (p *BitapsProvider) Stats() MetricsSnapshot     { return p.rl.Stats() }

// FetchNativeBalances fetches BTC balance for each address (one API call per address).
// Continues on per-address errors instead of early-returning, annotating failed results.
func (p *BitapsProvider) FetchNativeBalances(ctx context.Context, addresses []models.Address) ([]BalanceResult, error) {
	results := make([]BalanceResult, 0, len(addresses))
	var failCount int

	for _, addr := range addresses {
		if err := p.rl.Wait(ctx); err != nil {
			return results, fmt.Errorf("rate limiter wait: %w", err)
		}

		balance, err := p.fetchAddressBalance(ctx, addr.Address)
		if err != nil {
			if ctx.Err() != nil {
				return results, fmt.Errorf("context cancelled during fetch: %w", err)
			}

			slog.Warn("bitaps address balance fetch failed",
				"provider", p.Name(),
				"address", addr.Address,
				"index", addr.AddressIndex,
				"error", err,
			)
			failCount++
			results = append(results, BalanceResult{
				Address:      addr.Address,
				AddressIndex: addr.AddressIndex,
				Balance:      "0",
				Error:        err.Error(),
				Source:       p.Name(),
			})
			continue
		}

		results = append(results, BalanceResult{
			Address:      addr.Address,
			AddressIndex: addr.AddressIndex,
			Balance:      balance,
			Source:       p.Name(),
		})

		slog.Debug("bitaps balance fetched",
			"address", addr.Address,
			"index", addr.AddressIndex,
			"balance", balance,
		)
	}

	if failCount > 0 && failCount == len(addresses) {
		return results, fmt.Errorf("all %d addresses failed: %w", failCount, config.ErrProviderUnavailable)
	}

	return results, nil
}

// FetchTokenBalances is not supported for BTC.
func (p *BitapsProvider) FetchTokenBalances(_ context.Context, _ []models.Address, _ models.Token, _ string) ([]BalanceResult, error) {
	return nil, config.ErrTokensNotSupported
}

// fetchAddressBalance queries a single address and returns the balance in satoshis.
// Bitaps includes both confirmed balance and pending (mempool) balance.
func (p *BitapsProvider) fetchAddressBalance(ctx context.Context, address string) (string, error) {
	url := fmt.Sprintf("%s/address/state/%s", p.baseURL, address)

	slog.Debug("bitaps request",
		"url", url,
	)

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return "0", fmt.Errorf("create request: %w", err)
	}

	resp, err := p.client.Do(req)
	if err != nil {
		return "0", fmt.Errorf("execute request: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode == http.StatusTooManyRequests {
		retryAfter := parseRetryAfter(resp.Header)
		slog.Warn("bitaps rate limited",
			"address", address,
			"retryAfter", retryAfter,
		)
		return "0", config.NewTransientErrorWithRetry(config.ErrProviderRateLimit, retryAfter)
	}

	if resp.StatusCode != http.StatusOK {
		slog.Warn("bitaps non-200 response",
			"address", address,
			"status", resp.StatusCode,
		)
		return "0", config.NewTransientError(fmt.Errorf("%w: HTTP %d", config.ErrProviderUnavailable, resp.StatusCode))
	}

	var data bitapsAddressResponse
	if err := json.NewDecoder(resp.Body).Decode(&data); err != nil {
		return "0", fmt.Errorf("decode response: %w", err)
	}

	// Confirmed balance plus pending incoming minus pending outgoing.
	totalBalance := data.Data.Balance + data.Data.PendingReceived - data.Data.PendingSpent
	return strconv.FormatInt(totalBalance, 10), nil
}
