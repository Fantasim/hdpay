package scanner

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"net/http"
	"strconv"

	"github.com/Fantasim/hdpay/internal/config"
	"github.com/Fantasim/hdpay/internal/models"
)

// mempoolResponse represents the JSON response from Mempool.space /address endpoint.
// Same format as Blockstream Esplora.
type mempoolResponse = blockstreamResponse

// MempoolProvider fetches BTC balances from Mempool.space API.
type MempoolProvider struct {
	client  *http.Client
	rl      *RateLimiter
	baseURL string
}

// NewMempoolProvider creates a provider for the Mempool.space API.
func NewMempoolProvider(client *http.Client, rl *RateLimiter, network string) *MempoolProvider {
	baseURL := config.MempoolMainnetURL
	if network == string(models.NetworkTestnet) {
		baseURL = config.MempoolTestnetURL
	}

	slog.Info("mempool provider created",
		"baseURL", baseURL,
		"network", network,
	)

	return &MempoolProvider{
		client:  client,
		rl:      rl,
		baseURL: baseURL,
	}
}

func (p *MempoolProvider) Name() string              { return "Mempool" }
func (p *MempoolProvider) Chain() models.Chain        { return models.ChainBTC }
func (p *MempoolProvider) MaxBatchSize() int          { return 1 }

// FetchNativeBalances fetches BTC balance for each address (one API call per address).
func (p *MempoolProvider) FetchNativeBalances(ctx context.Context, addresses []models.Address) ([]BalanceResult, error) {
	results := make([]BalanceResult, 0, len(addresses))

	for _, addr := range addresses {
		if err := p.rl.Wait(ctx); err != nil {
			return results, fmt.Errorf("rate limiter wait: %w", err)
		}

		balance, err := p.fetchAddressBalance(ctx, addr.Address)
		if err != nil {
			return results, fmt.Errorf("fetch balance for %s (index %d): %w", addr.Address, addr.AddressIndex, err)
		}

		results = append(results, BalanceResult{
			Address:      addr.Address,
			AddressIndex: addr.AddressIndex,
			Balance:      balance,
		})

		slog.Debug("mempool balance fetched",
			"address", addr.Address,
			"index", addr.AddressIndex,
			"balance", balance,
		)
	}

	return results, nil
}

// FetchTokenBalances is not supported for BTC.
func (p *MempoolProvider) FetchTokenBalances(_ context.Context, _ []models.Address, _ models.Token, _ string) ([]BalanceResult, error) {
	return nil, config.ErrTokensNotSupported
}

// fetchAddressBalance queries a single address and returns the balance in satoshis.
func (p *MempoolProvider) fetchAddressBalance(ctx context.Context, address string) (string, error) {
	url := fmt.Sprintf("%s/address/%s", p.baseURL, address)

	slog.Debug("mempool request",
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
		slog.Warn("mempool rate limited", "address", address)
		return "0", config.ErrProviderRateLimit
	}

	if resp.StatusCode != http.StatusOK {
		slog.Warn("mempool non-200 response",
			"address", address,
			"status", resp.StatusCode,
		)
		return "0", fmt.Errorf("%w: HTTP %d", config.ErrProviderUnavailable, resp.StatusCode)
	}

	var data mempoolResponse
	if err := json.NewDecoder(resp.Body).Decode(&data); err != nil {
		return "0", fmt.Errorf("decode response: %w", err)
	}

	confirmedBalance := data.ChainStats.FundedTxoSum - data.ChainStats.SpentTxoSum
	mempoolBalance := data.MempoolStats.FundedTxoSum - data.MempoolStats.SpentTxoSum
	totalBalance := confirmedBalance + mempoolBalance

	return strconv.FormatInt(totalBalance, 10), nil
}
