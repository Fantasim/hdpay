package provider

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"log/slog"
	"math/big"
	"net/http"
	"strconv"
	"strings"

	hdconfig "github.com/Fantasim/hdpay/internal/shared/config"
)

// BTC API response page size (Blockstream/Mempool return 25 txs per page).
const btcPageSize = 25

// blockstreamTx represents a transaction from Blockstream/Mempool Esplora API.
type blockstreamTx struct {
	TxID   string             `json:"txid"`
	Status blockstreamStatus  `json:"status"`
	Vout   []blockstreamVout  `json:"vout"`
}

type blockstreamStatus struct {
	Confirmed bool  `json:"confirmed"`
	BlockTime int64 `json:"block_time"`
}

type blockstreamVout struct {
	ScriptPubKeyAddr string `json:"scriptpubkey_address"`
	Value            int64  `json:"value"` // satoshis
}

// BlockstreamProvider detects BTC transactions via Blockstream Esplora API.
type BlockstreamProvider struct {
	client  *http.Client
	baseURL string
}

// NewBlockstreamProvider creates a Blockstream provider for the given network.
func NewBlockstreamProvider(client *http.Client, network string) *BlockstreamProvider {
	baseURL := hdconfig.BlockstreamMainnetURL
	if network == "testnet" {
		baseURL = hdconfig.BlockstreamTestnetURL
	}

	slog.Info("blockstream provider created",
		"network", network,
		"baseURL", baseURL,
	)

	return &BlockstreamProvider{
		client:  client,
		baseURL: baseURL,
	}
}

func (p *BlockstreamProvider) Name() string  { return "blockstream" }
func (p *BlockstreamProvider) Chain() string  { return "BTC" }

// FetchTransactions returns incoming BTC transactions for an address since cutoffUnix.
// Paginates through results (25 per page) until reaching transactions older than cutoff.
func (p *BlockstreamProvider) FetchTransactions(ctx context.Context, address string, cutoffUnix int64) ([]RawTransaction, error) {
	var result []RawTransaction
	afterTxID := ""

	for {
		txs, err := p.fetchPage(ctx, address, afterTxID)
		if err != nil {
			return nil, fmt.Errorf("blockstream fetch page: %w", err)
		}

		if len(txs) == 0 {
			break
		}

		reachedCutoff := false
		for _, tx := range txs {
			// Skip unconfirmed transactions with no block_time
			// (they have block_time == 0 from the API)
			if tx.Status.BlockTime > 0 && tx.Status.BlockTime < cutoffUnix {
				reachedCutoff = true
				break
			}

			// Sum outputs that pay to our address
			totalSats := int64(0)
			for _, vout := range tx.Vout {
				if strings.EqualFold(vout.ScriptPubKeyAddr, address) {
					totalSats += vout.Value
				}
			}

			// Skip if no outputs to our address (outgoing or unrelated tx)
			if totalSats == 0 {
				continue
			}

			amountRaw := strconv.FormatInt(totalSats, 10)
			amountHuman := satoshisToHuman(totalSats)

			slog.Debug("BTC incoming tx detected",
				"txid", tx.TxID,
				"address", address,
				"satoshis", totalSats,
				"amountBTC", amountHuman,
				"confirmed", tx.Status.Confirmed,
			)

			confirmations := 0
			if tx.Status.Confirmed {
				confirmations = 1
			}

			result = append(result, RawTransaction{
				TxHash:        tx.TxID,
				Token:         "BTC",
				AmountRaw:     amountRaw,
				AmountHuman:   amountHuman,
				Decimals:      hdconfig.BTCDecimals,
				BlockTime:     tx.Status.BlockTime,
				Confirmed:     tx.Status.Confirmed,
				Confirmations: confirmations,
				BlockNumber:   0, // BTC doesn't use block number for confirmation counting
			})
		}

		if reachedCutoff {
			break
		}

		// If we got a full page, fetch the next page using the last txid
		if len(txs) < btcPageSize {
			break
		}
		afterTxID = txs[len(txs)-1].TxID
	}

	slog.Info("BTC transactions fetched",
		"provider", p.Name(),
		"address", address,
		"count", len(result),
	)
	return result, nil
}

// CheckConfirmation checks whether a BTC transaction is confirmed.
func (p *BlockstreamProvider) CheckConfirmation(ctx context.Context, txHash string, _ int64) (bool, int, error) {
	url := fmt.Sprintf("%s/tx/%s", p.baseURL, txHash)

	slog.Debug("checking BTC tx confirmation",
		"provider", p.Name(),
		"txHash", txHash,
		"url", url,
	)

	body, err := p.doGet(ctx, url)
	if err != nil {
		return false, 0, fmt.Errorf("blockstream check confirmation: %w", err)
	}

	var tx blockstreamTx
	if err := json.Unmarshal(body, &tx); err != nil {
		return false, 0, fmt.Errorf("blockstream parse tx %s: %w", txHash, err)
	}

	confirmations := 0
	if tx.Status.Confirmed {
		confirmations = 1
	}

	slog.Debug("BTC tx confirmation status",
		"txHash", txHash,
		"confirmed", tx.Status.Confirmed,
	)

	return tx.Status.Confirmed, confirmations, nil
}

// GetCurrentBlock is not used for BTC confirmation (BTC uses 1-conf from API).
// Returns 0 as a no-op.
func (p *BlockstreamProvider) GetCurrentBlock(_ context.Context) (uint64, error) {
	return 0, nil
}

// fetchPage fetches a page of transactions from the Esplora API.
func (p *BlockstreamProvider) fetchPage(ctx context.Context, address, afterTxID string) ([]blockstreamTx, error) {
	url := fmt.Sprintf("%s/address/%s/txs", p.baseURL, address)
	if afterTxID != "" {
		url += "/chain/" + afterTxID
	}

	slog.Debug("fetching BTC tx page",
		"provider", p.Name(),
		"address", address,
		"afterTxID", afterTxID,
		"url", url,
	)

	body, err := p.doGet(ctx, url)
	if err != nil {
		return nil, err
	}

	var txs []blockstreamTx
	if err := json.Unmarshal(body, &txs); err != nil {
		return nil, fmt.Errorf("blockstream parse response: %w", err)
	}

	return txs, nil
}

// doGet performs a GET request and returns the response body.
func (p *BlockstreamProvider) doGet(ctx context.Context, url string) ([]byte, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return nil, fmt.Errorf("create request: %w", err)
	}

	resp, err := p.client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("http request failed: %w", err)
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("read response body: %w", err)
	}

	if resp.StatusCode == http.StatusTooManyRequests {
		return nil, fmt.Errorf("rate limited (HTTP 429) from %s", url)
	}
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("unexpected status %d from %s: %s", resp.StatusCode, url, string(body))
	}

	return body, nil
}

// MempoolProvider detects BTC transactions via Mempool.space Esplora API.
// Same API format as Blockstream — shares all parsing logic.
type MempoolProvider struct {
	BlockstreamProvider // embed to reuse all methods
}

// NewMempoolProvider creates a Mempool.space provider for the given network.
func NewMempoolProvider(client *http.Client, network string) *MempoolProvider {
	baseURL := hdconfig.MempoolMainnetURL
	if network == "testnet" {
		baseURL = hdconfig.MempoolTestnetURL
	}

	slog.Info("mempool provider created",
		"network", network,
		"baseURL", baseURL,
	)

	return &MempoolProvider{
		BlockstreamProvider: BlockstreamProvider{
			client:  client,
			baseURL: baseURL,
		},
	}
}

func (p *MempoolProvider) Name() string { return "mempool" }

// bitapsProviderTx represents a transaction from the Bitaps address transactions API.
type bitapsProviderTx struct {
	TxID          string `json:"txId"`
	BlockTime     int64  `json:"blockTime"`
	Confirmations int    `json:"confirmations"`
	// Amount is the net satoshi change to the address:
	// positive = received, negative = sent.
	Amount int64 `json:"amount"`
}

// bitapsProviderResponse is the Bitaps /address/transactions/{address} response.
type bitapsProviderResponse struct {
	Data struct {
		Page  int                `json:"page"`
		Pages int                `json:"pages"`
		Limit int                `json:"limit"`
		List  []bitapsProviderTx `json:"list"`
	} `json:"data"`
	ErrorList []interface{} `json:"error_list"`
}

// bitapsTxResponse is the Bitaps /transaction/{txid} response.
type bitapsTxResponse struct {
	Data struct {
		TxID          string `json:"txId"`
		Confirmations int    `json:"confirmations"`
		BlockTime     int64  `json:"blockTime"`
	} `json:"data"`
	ErrorList []interface{} `json:"error_list"`
}

// BitapsProvider detects BTC transactions via the Bitaps blockchain API.
// Unlike Blockstream/Mempool (Esplora), Bitaps pre-calculates net amounts per address,
// so FetchTransactions only needs to check amount > 0 for incoming detection.
// API docs: https://developer.bitaps.com/blockchain
// Free tier — no API key required.
type BitapsProvider struct {
	client  *http.Client
	baseURL string
}

// NewBitapsProvider creates a Bitaps provider for the given network.
func NewBitapsProvider(client *http.Client, network string) *BitapsProvider {
	baseURL := hdconfig.BitapsMainnetURL
	if network == "testnet" {
		baseURL = hdconfig.BitapsTestnetURL
	}

	slog.Info("bitaps provider created",
		"network", network,
		"baseURL", baseURL,
	)

	return &BitapsProvider{
		client:  client,
		baseURL: baseURL,
	}
}

func (p *BitapsProvider) Name() string  { return "bitaps" }
func (p *BitapsProvider) Chain() string  { return "BTC" }

// FetchTransactions returns incoming BTC transactions for an address since cutoffUnix.
// Paginates through results (25 per page) until reaching transactions older than cutoff.
func (p *BitapsProvider) FetchTransactions(ctx context.Context, address string, cutoffUnix int64) ([]RawTransaction, error) {
	var result []RawTransaction
	page := 1

	for {
		txs, pages, err := p.fetchPage(ctx, address, page)
		if err != nil {
			return nil, fmt.Errorf("bitaps fetch page %d: %w", page, err)
		}

		if len(txs) == 0 {
			break
		}

		reachedCutoff := false
		for _, tx := range txs {
			// Skip unconfirmed (blockTime == 0) before cutoff check.
			if tx.BlockTime > 0 && tx.BlockTime < cutoffUnix {
				reachedCutoff = true
				break
			}

			// Only record incoming transfers (net positive amount).
			if tx.Amount <= 0 {
				continue
			}

			amountRaw := strconv.FormatInt(tx.Amount, 10)
			amountHuman := satoshisToHuman(tx.Amount)
			confirmed := tx.Confirmations > 0

			slog.Debug("BTC incoming tx detected (bitaps)",
				"txid", tx.TxID,
				"address", address,
				"satoshis", tx.Amount,
				"amountBTC", amountHuman,
				"confirmations", tx.Confirmations,
			)

			result = append(result, RawTransaction{
				TxHash:        tx.TxID,
				Token:         "BTC",
				AmountRaw:     amountRaw,
				AmountHuman:   amountHuman,
				Decimals:      hdconfig.BTCDecimals,
				BlockTime:     tx.BlockTime,
				Confirmed:     confirmed,
				Confirmations: tx.Confirmations,
				BlockNumber:   0,
			})
		}

		if reachedCutoff || page >= pages {
			break
		}
		page++
	}

	slog.Info("BTC transactions fetched",
		"provider", p.Name(),
		"address", address,
		"count", len(result),
	)
	return result, nil
}

// CheckConfirmation checks whether a BTC transaction has at least 1 confirmation.
func (p *BitapsProvider) CheckConfirmation(ctx context.Context, txHash string, _ int64) (bool, int, error) {
	url := fmt.Sprintf("%s/transaction/%s", p.baseURL, txHash)

	slog.Debug("checking BTC tx confirmation (bitaps)",
		"provider", p.Name(),
		"txHash", txHash,
	)

	body, err := p.doGet(ctx, url)
	if err != nil {
		return false, 0, fmt.Errorf("bitaps check confirmation: %w", err)
	}

	var txResp bitapsTxResponse
	if err := json.Unmarshal(body, &txResp); err != nil {
		return false, 0, fmt.Errorf("bitaps parse tx %s: %w", txHash, err)
	}

	confirmed := txResp.Data.Confirmations > 0

	slog.Debug("BTC tx confirmation status (bitaps)",
		"txHash", txHash,
		"confirmations", txResp.Data.Confirmations,
		"confirmed", confirmed,
	)

	return confirmed, txResp.Data.Confirmations, nil
}

// GetCurrentBlock is not used for BTC confirmation.
func (p *BitapsProvider) GetCurrentBlock(_ context.Context) (uint64, error) {
	return 0, nil
}

// fetchPage fetches one page of transactions from the Bitaps API.
// Returns the transaction list, total page count, and any error.
func (p *BitapsProvider) fetchPage(ctx context.Context, address string, page int) ([]bitapsProviderTx, int, error) {
	url := fmt.Sprintf("%s/address/transactions/%s?limit=%d&page=%d", p.baseURL, address, btcPageSize, page)

	slog.Debug("fetching BTC tx page (bitaps)",
		"address", address,
		"page", page,
		"url", url,
	)

	body, err := p.doGet(ctx, url)
	if err != nil {
		return nil, 0, err
	}

	var resp bitapsProviderResponse
	if err := json.Unmarshal(body, &resp); err != nil {
		return nil, 0, fmt.Errorf("bitaps parse response: %w", err)
	}

	return resp.Data.List, resp.Data.Pages, nil
}

// doGet performs a GET request and returns the response body.
func (p *BitapsProvider) doGet(ctx context.Context, url string) ([]byte, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return nil, fmt.Errorf("create request: %w", err)
	}

	resp, err := p.client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("http request failed: %w", err)
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("read response body: %w", err)
	}

	if resp.StatusCode == http.StatusTooManyRequests {
		return nil, fmt.Errorf("rate limited (HTTP 429) from %s", url)
	}
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("unexpected status %d from %s: %s", resp.StatusCode, url, string(body))
	}

	return body, nil
}

// satoshisToHuman converts satoshis to a human-readable BTC string.
func satoshisToHuman(sats int64) string {
	// Use big.Float for precision
	satsBig := new(big.Float).SetInt64(sats)
	divisor := new(big.Float).SetInt64(100_000_000)
	result := new(big.Float).Quo(satsBig, divisor)
	return result.Text('f', hdconfig.BTCDecimals)
}
