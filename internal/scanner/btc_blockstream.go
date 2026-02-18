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

// blockstreamResponse represents the JSON response from Blockstream Esplora /address endpoint.
type blockstreamResponse struct {
	Address    string `json:"address"`
	ChainStats struct {
		FundedTxoSum int64 `json:"funded_txo_sum"`
		SpentTxoSum  int64 `json:"spent_txo_sum"`
	} `json:"chain_stats"`
	MempoolStats struct {
		FundedTxoSum int64 `json:"funded_txo_sum"`
		SpentTxoSum  int64 `json:"spent_txo_sum"`
	} `json:"mempool_stats"`
}

// BlockstreamProvider fetches BTC balances from Blockstream Esplora API.
type BlockstreamProvider struct {
	client  *http.Client
	rl      *RateLimiter
	baseURL string
}

// NewBlockstreamProvider creates a provider for the Blockstream Esplora API.
func NewBlockstreamProvider(client *http.Client, rl *RateLimiter, network string) *BlockstreamProvider {
	baseURL := config.BlockstreamMainnetURL
	if network == string(models.NetworkTestnet) {
		baseURL = config.BlockstreamTestnetURL
	}

	slog.Info("blockstream provider created",
		"baseURL", baseURL,
		"network", network,
	)

	return &BlockstreamProvider{
		client:  client,
		rl:      rl,
		baseURL: baseURL,
	}
}

func (p *BlockstreamProvider) Name() string              { return "Blockstream" }
func (p *BlockstreamProvider) Chain() models.Chain        { return models.ChainBTC }
func (p *BlockstreamProvider) MaxBatchSize() int          { return 1 }

// FetchNativeBalances fetches BTC balance for each address (one API call per address).
func (p *BlockstreamProvider) FetchNativeBalances(ctx context.Context, addresses []models.Address) ([]BalanceResult, error) {
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

		slog.Debug("blockstream balance fetched",
			"address", addr.Address,
			"index", addr.AddressIndex,
			"balance", balance,
		)
	}

	return results, nil
}

// FetchTokenBalances is not supported for BTC.
func (p *BlockstreamProvider) FetchTokenBalances(_ context.Context, _ []models.Address, _ models.Token, _ string) ([]BalanceResult, error) {
	return nil, config.ErrTokensNotSupported
}

// fetchAddressBalance queries a single address and returns the balance in satoshis.
func (p *BlockstreamProvider) fetchAddressBalance(ctx context.Context, address string) (string, error) {
	url := fmt.Sprintf("%s/address/%s", p.baseURL, address)

	slog.Debug("blockstream request",
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
		slog.Warn("blockstream rate limited", "address", address)
		return "0", config.ErrProviderRateLimit
	}

	if resp.StatusCode != http.StatusOK {
		slog.Warn("blockstream non-200 response",
			"address", address,
			"status", resp.StatusCode,
		)
		return "0", fmt.Errorf("%w: HTTP %d", config.ErrProviderUnavailable, resp.StatusCode)
	}

	var data blockstreamResponse
	if err := json.NewDecoder(resp.Body).Decode(&data); err != nil {
		return "0", fmt.Errorf("decode response: %w", err)
	}

	// Balance = confirmed funded - confirmed spent (satoshis).
	// We include mempool (unconfirmed) for a more complete picture.
	confirmedBalance := data.ChainStats.FundedTxoSum - data.ChainStats.SpentTxoSum
	mempoolBalance := data.MempoolStats.FundedTxoSum - data.MempoolStats.SpentTxoSum
	totalBalance := confirmedBalance + mempoolBalance

	return strconv.FormatInt(totalBalance, 10), nil
}
