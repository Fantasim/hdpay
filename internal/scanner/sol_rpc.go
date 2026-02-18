package scanner

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"net/http"
	"strconv"

	"github.com/Fantasim/hdpay/internal/config"
	"github.com/Fantasim/hdpay/internal/models"
)

// solanaRPCRequest is a JSON-RPC 2.0 request.
type solanaRPCRequest struct {
	JSONRPC string        `json:"jsonrpc"`
	ID      int           `json:"id"`
	Method  string        `json:"method"`
	Params  []interface{} `json:"params"`
}

// solanaRPCResponse is a JSON-RPC 2.0 response for getMultipleAccounts.
type solanaRPCResponse struct {
	JSONRPC string `json:"jsonrpc"`
	ID      int    `json:"id"`
	Error   *struct {
		Code    int    `json:"code"`
		Message string `json:"message"`
	} `json:"error,omitempty"`
	Result *struct {
		Context struct {
			Slot uint64 `json:"slot"`
		} `json:"context"`
		Value []json.RawMessage `json:"value"`
	} `json:"result,omitempty"`
}

// solanaAccountBase is the account info for native balance queries (base64 encoding).
type solanaAccountBase struct {
	Lamports   uint64 `json:"lamports"`
	Owner      string `json:"owner"`
	Executable bool   `json:"executable"`
}

// solanaAccountParsed is the account info for token balance queries (jsonParsed encoding).
type solanaAccountParsed struct {
	Lamports uint64 `json:"lamports"`
	Owner    string `json:"owner"`
	Data     struct {
		Program string `json:"program"`
		Parsed  struct {
			Type string `json:"type"`
			Info struct {
				Mint        string `json:"mint"`
				Owner       string `json:"owner"`
				TokenAmount struct {
					Amount   string `json:"amount"`
					Decimals int    `json:"decimals"`
				} `json:"tokenAmount"`
			} `json:"info"`
		} `json:"parsed"`
	} `json:"data"`
}

// SolanaRPCProvider fetches SOL balances via Solana JSON-RPC.
// Reusable for both public RPC and Helius (same interface, different URL).
type SolanaRPCProvider struct {
	client  *http.Client
	rl      *RateLimiter
	rpcURL  string
	name    string
}

// NewSolanaRPCProvider creates a Solana RPC provider.
func NewSolanaRPCProvider(client *http.Client, rl *RateLimiter, rpcURL, name string) *SolanaRPCProvider {
	slog.Info("solana rpc provider created",
		"name", name,
		"rpcURL", rpcURL,
	)
	return &SolanaRPCProvider{
		client: client,
		rl:     rl,
		rpcURL: rpcURL,
		name:   name,
	}
}

func (p *SolanaRPCProvider) Name() string              { return p.name }
func (p *SolanaRPCProvider) Chain() models.Chain        { return models.ChainSOL }
func (p *SolanaRPCProvider) MaxBatchSize() int          { return config.ScanBatchSizeSolanaRPC }

// FetchNativeBalances fetches SOL balances using getMultipleAccounts.
func (p *SolanaRPCProvider) FetchNativeBalances(ctx context.Context, addresses []models.Address) ([]BalanceResult, error) {
	if len(addresses) == 0 {
		return nil, nil
	}

	if err := p.rl.Wait(ctx); err != nil {
		return nil, fmt.Errorf("rate limiter wait: %w", err)
	}

	// Build pubkey list.
	pubkeys := make([]string, len(addresses))
	for i, a := range addresses {
		pubkeys[i] = a.Address
	}

	slog.Debug("solana rpc getMultipleAccounts (native)",
		"provider", p.name,
		"count", len(pubkeys),
	)

	rpcReq := solanaRPCRequest{
		JSONRPC: "2.0",
		ID:      1,
		Method:  "getMultipleAccounts",
		Params: []interface{}{
			pubkeys,
			map[string]string{"encoding": "base64"},
		},
	}

	respBody, err := p.doRPCCall(ctx, rpcReq)
	if err != nil {
		return nil, err
	}

	if respBody.Error != nil {
		slog.Warn("solana rpc error",
			"provider", p.name,
			"code", respBody.Error.Code,
			"message", respBody.Error.Message,
		)
		return nil, fmt.Errorf("%w: %s", config.ErrProviderUnavailable, respBody.Error.Message)
	}

	if respBody.Result == nil {
		return nil, fmt.Errorf("%w: nil result", config.ErrProviderUnavailable)
	}

	results := make([]BalanceResult, 0, len(addresses))
	for i, raw := range respBody.Result.Value {
		if i >= len(addresses) {
			break
		}

		balance := "0"

		// null means account doesn't exist (zero balance).
		if string(raw) != "null" {
			var account solanaAccountBase
			if err := json.Unmarshal(raw, &account); err != nil {
				slog.Warn("solana rpc unmarshal account error",
					"provider", p.name,
					"index", addresses[i].AddressIndex,
					"error", err,
				)
			} else {
				balance = strconv.FormatUint(account.Lamports, 10)
			}
		}

		results = append(results, BalanceResult{
			Address:      addresses[i].Address,
			AddressIndex: addresses[i].AddressIndex,
			Balance:      balance,
			Source:       p.Name(),
		})

		slog.Debug("solana native balance",
			"provider", p.name,
			"address", addresses[i].Address,
			"index", addresses[i].AddressIndex,
			"balance", balance,
		)
	}

	return results, nil
}

// FetchTokenBalances fetches SPL token balances by deriving ATAs and querying them.
func (p *SolanaRPCProvider) FetchTokenBalances(ctx context.Context, addresses []models.Address, token models.Token, mintAddress string) ([]BalanceResult, error) {
	if len(addresses) == 0 {
		return nil, nil
	}

	if err := p.rl.Wait(ctx); err != nil {
		return nil, fmt.Errorf("rate limiter wait: %w", err)
	}

	// Derive ATA addresses for each wallet + mint combination.
	ataAddresses := make([]string, len(addresses))
	for i, addr := range addresses {
		ata, err := DeriveATA(addr.Address, mintAddress)
		if err != nil {
			slog.Warn("ata derivation failed",
				"provider", p.name,
				"wallet", addr.Address,
				"mint", mintAddress,
				"error", err,
			)
			ataAddresses[i] = "" // will produce null in response
			continue
		}
		ataAddresses[i] = ata
	}

	// Filter out empty ATA addresses.
	validATAs := make([]string, 0, len(ataAddresses))
	ataToIndex := make(map[string]int) // ATA address → position in addresses slice
	for i, ata := range ataAddresses {
		if ata != "" {
			validATAs = append(validATAs, ata)
			ataToIndex[ata] = i
		}
	}

	if len(validATAs) == 0 {
		return nil, nil
	}

	slog.Debug("solana rpc getMultipleAccounts (token)",
		"provider", p.name,
		"token", token,
		"mint", mintAddress,
		"ataCount", len(validATAs),
	)

	rpcReq := solanaRPCRequest{
		JSONRPC: "2.0",
		ID:      1,
		Method:  "getMultipleAccounts",
		Params: []interface{}{
			validATAs,
			map[string]string{"encoding": "jsonParsed"},
		},
	}

	respBody, err := p.doRPCCall(ctx, rpcReq)
	if err != nil {
		return nil, err
	}

	if respBody.Error != nil {
		slog.Warn("solana rpc token error",
			"provider", p.name,
			"code", respBody.Error.Code,
			"message", respBody.Error.Message,
		)
		return nil, fmt.Errorf("%w: %s", config.ErrProviderUnavailable, respBody.Error.Message)
	}

	if respBody.Result == nil {
		return nil, fmt.Errorf("%w: nil result", config.ErrProviderUnavailable)
	}

	// Parse results and map back to original wallet addresses.
	results := make([]BalanceResult, 0, len(addresses))
	for i, raw := range respBody.Result.Value {
		if i >= len(validATAs) {
			break
		}

		ata := validATAs[i]
		addrIdx, ok := ataToIndex[ata]
		if !ok {
			continue
		}
		addr := addresses[addrIdx]

		balance := "0"

		if string(raw) != "null" {
			var account solanaAccountParsed
			if err := json.Unmarshal(raw, &account); err != nil {
				slog.Warn("solana rpc unmarshal token account error",
					"provider", p.name,
					"ata", ata,
					"error", err,
				)
			} else if account.Data.Parsed.Info.TokenAmount.Amount != "" {
				balance = account.Data.Parsed.Info.TokenAmount.Amount
			}
		}

		results = append(results, BalanceResult{
			Address:      addr.Address,
			AddressIndex: addr.AddressIndex,
			Balance:      balance,
			Source:       p.Name(),
		})

		slog.Debug("solana token balance",
			"provider", p.name,
			"wallet", addr.Address,
			"index", addr.AddressIndex,
			"token", token,
			"balance", balance,
		)
	}

	return results, nil
}

// doRPCCall sends a JSON-RPC request and returns the parsed response.
func (p *SolanaRPCProvider) doRPCCall(ctx context.Context, rpcReq solanaRPCRequest) (*solanaRPCResponse, error) {
	body, err := json.Marshal(rpcReq)
	if err != nil {
		return nil, fmt.Errorf("marshal rpc request: %w", err)
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, p.rpcURL, bytes.NewReader(body))
	if err != nil {
		return nil, fmt.Errorf("create request: %w", err)
	}
	req.Header.Set("Content-Type", "application/json")

	resp, err := p.client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("execute request: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode == http.StatusTooManyRequests {
		slog.Warn("solana rpc rate limited", "provider", p.name)
		return nil, config.ErrProviderRateLimit
	}

	if resp.StatusCode != http.StatusOK {
		slog.Warn("solana rpc non-200",
			"provider", p.name,
			"status", resp.StatusCode,
		)
		return nil, fmt.Errorf("%w: HTTP %d", config.ErrProviderUnavailable, resp.StatusCode)
	}

	var rpcResp solanaRPCResponse
	if err := json.NewDecoder(resp.Body).Decode(&rpcResp); err != nil {
		return nil, fmt.Errorf("decode rpc response: %w", err)
	}

	return &rpcResp, nil
}
