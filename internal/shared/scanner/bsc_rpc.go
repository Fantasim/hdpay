package scanner

import (
	"context"
	"encoding/hex"
	"fmt"
	"log/slog"
	"math/big"

	"github.com/Fantasim/hdpay/internal/shared/config"
	"github.com/Fantasim/hdpay/internal/shared/models"
	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/common/hexutil"
	"github.com/ethereum/go-ethereum/crypto"
	"github.com/ethereum/go-ethereum/ethclient"
	"github.com/ethereum/go-ethereum/rpc"
)

// balanceOfSelector is the first 4 bytes of keccak256("balanceOf(address)").
var balanceOfSelector = crypto.Keccak256([]byte("balanceOf(address)"))[:4]

// ethCallArg is the JSON-RPC argument for eth_call. Serialised to
// {"to":"0x...","data":"0x..."} which matches the Ethereum JSON-RPC spec.
type ethCallArg struct {
	To   common.Address `json:"to"`
	Data hexutil.Bytes  `json:"data"`
}

// BSCRPCProvider fetches BSC balances via JSON-RPC batch calls.
// Uses rpc.Client.BatchCallContext to send up to BSCRPCBatchSize requests
// in a single HTTP POST, reducing round-trips by 20x vs single-call mode.
type BSCRPCProvider struct {
	client    *ethclient.Client
	rpcClient *rpc.Client
	rl        *RateLimiter
	rpcURL    string
	name      string
}

// NewBSCRPCProvider creates a provider that connects to a BSC JSON-RPC endpoint.
// name is used for logging and metrics; rpcURL is the full JSON-RPC endpoint URL.
func NewBSCRPCProvider(rl *RateLimiter, name, rpcURL string) (*BSCRPCProvider, error) {
	slog.Info("bsc rpc provider connecting",
		"name", name,
		"rpcURL", rpcURL,
	)

	client, err := ethclient.Dial(rpcURL)
	if err != nil {
		return nil, fmt.Errorf("dial BSC RPC %s: %w", rpcURL, err)
	}

	slog.Info("bsc rpc provider connected", "name", name, "rpcURL", rpcURL)

	return &BSCRPCProvider{
		client:    client,
		rpcClient: client.Client(),
		rl:        rl,
		rpcURL:    rpcURL,
		name:      name,
	}, nil
}

func (p *BSCRPCProvider) Name() string            { return p.name }
func (p *BSCRPCProvider) Chain() models.Chain      { return models.ChainBSC }
func (p *BSCRPCProvider) MaxBatchSize() int        { return config.BSCRPCBatchSize }
func (p *BSCRPCProvider) RecordSuccess()           { p.rl.RecordSuccess() }
func (p *BSCRPCProvider) RecordFailure(is429 bool) { p.rl.RecordFailure(is429) }
func (p *BSCRPCProvider) Stats() MetricsSnapshot   { return p.rl.Stats() }

// Close closes the underlying ethclient connection.
func (p *BSCRPCProvider) Close() {
	p.client.Close()
	slog.Info("bsc rpc provider closed", "name", p.name, "rpcURL", p.rpcURL)
}

// FetchNativeBalances fetches BNB balances for a batch of addresses in a single
// JSON-RPC batch call using eth_getBalance. One HTTP request = one rate-limit token.
func (p *BSCRPCProvider) FetchNativeBalances(ctx context.Context, addresses []models.Address) ([]BalanceResult, error) {
	if len(addresses) == 0 {
		return nil, nil
	}

	// Single rate-limit wait for the entire batch (1 HTTP request).
	if err := p.rl.Wait(ctx); err != nil {
		return nil, fmt.Errorf("rate limiter wait: %w", err)
	}

	// Build batch elements.
	elems := make([]rpc.BatchElem, len(addresses))
	for i, addr := range addresses {
		elems[i] = rpc.BatchElem{
			Method: "eth_getBalance",
			Args:   []interface{}{common.HexToAddress(addr.Address), "latest"},
			Result: new(hexutil.Big),
		}
	}

	slog.Debug("bsc rpc batch native request",
		"provider", p.name,
		"addressCount", len(addresses),
	)

	err := p.rpcClient.BatchCallContext(ctx, elems)
	if err != nil {
		if ctx.Err() != nil {
			return nil, fmt.Errorf("context cancelled during batch: %w", err)
		}
		slog.Warn("bsc rpc batch call failed",
			"provider", p.name,
			"error", err,
		)
		return nil, fmt.Errorf("batch eth_getBalance: %w", config.NewTransientError(err))
	}

	// Parse results.
	results := make([]BalanceResult, len(addresses))
	var failCount, foundCount int
	for i, addr := range addresses {
		br := BalanceResult{
			Address:      addr.Address,
			AddressIndex: addr.AddressIndex,
			Source:       p.Name(),
		}

		if elems[i].Error != nil {
			slog.Warn("bsc rpc batch element error",
				"provider", p.name,
				"address", addr.Address,
				"index", addr.AddressIndex,
				"error", elems[i].Error,
			)
			br.Balance = "0"
			br.Error = elems[i].Error.Error()
			failCount++
		} else {
			balance := (*big.Int)(elems[i].Result.(*hexutil.Big))
			br.Balance = balance.String()
			if balance.Sign() > 0 {
				foundCount++
			}
		}

		results[i] = br
	}

	slog.Debug("bsc rpc batch native complete",
		"provider", p.name,
		"total", len(addresses),
		"funded", foundCount,
		"failed", failCount,
	)

	if failCount > 0 && failCount == len(addresses) {
		return results, fmt.Errorf("all %d addresses failed: %w", failCount, config.ErrProviderUnavailable)
	}

	return results, nil
}

// FetchTokenBalances fetches BEP-20 token balances for a batch of addresses in a
// single JSON-RPC batch call using eth_call with balanceOf(address).
func (p *BSCRPCProvider) FetchTokenBalances(ctx context.Context, addresses []models.Address, token models.Token, contractAddress string) ([]BalanceResult, error) {
	if len(addresses) == 0 {
		return nil, nil
	}
	if contractAddress == "" {
		return nil, fmt.Errorf("contract address required for BSC token balance")
	}

	// Single rate-limit wait for the entire batch (1 HTTP request).
	if err := p.rl.Wait(ctx); err != nil {
		return nil, fmt.Errorf("rate limiter wait: %w", err)
	}

	contract := common.HexToAddress(contractAddress)

	// Build batch elements — one eth_call per address with balanceOf(address).
	elems := make([]rpc.BatchElem, len(addresses))
	for i, addr := range addresses {
		calldata := make([]byte, 36)
		copy(calldata[:4], balanceOfSelector)
		copy(calldata[4+12:], common.HexToAddress(addr.Address).Bytes())

		elems[i] = rpc.BatchElem{
			Method: "eth_call",
			Args: []interface{}{
				ethCallArg{To: contract, Data: calldata},
				"latest",
			},
			Result: new(hexutil.Bytes),
		}
	}

	slog.Debug("bsc rpc batch token request",
		"provider", p.name,
		"token", token,
		"contract", contractAddress,
		"addressCount", len(addresses),
	)

	err := p.rpcClient.BatchCallContext(ctx, elems)
	if err != nil {
		if ctx.Err() != nil {
			return nil, fmt.Errorf("context cancelled during token batch: %w", err)
		}
		slog.Warn("bsc rpc batch token call failed",
			"provider", p.name,
			"token", token,
			"error", err,
		)
		return nil, fmt.Errorf("batch eth_call balanceOf: %w", config.NewTransientError(err))
	}

	// Parse results.
	results := make([]BalanceResult, len(addresses))
	var failCount, foundCount int
	for i, addr := range addresses {
		br := BalanceResult{
			Address:      addr.Address,
			AddressIndex: addr.AddressIndex,
			Source:       p.Name(),
		}

		if elems[i].Error != nil {
			slog.Warn("bsc rpc batch token element error",
				"provider", p.name,
				"address", addr.Address,
				"index", addr.AddressIndex,
				"token", token,
				"error", elems[i].Error,
			)
			br.Balance = "0"
			br.Error = elems[i].Error.Error()
			failCount++
		} else {
			output := *elems[i].Result.(*hexutil.Bytes)
			if len(output) < 32 {
				slog.Warn("bsc rpc batch token malformed response",
					"provider", p.name,
					"address", addr.Address,
					"outputLen", len(output),
					"outputHex", hex.EncodeToString(output),
				)
				br.Balance = "0"
				br.Error = fmt.Sprintf("malformed balanceOf response: %d bytes", len(output))
				failCount++
			} else {
				balance := new(big.Int).SetBytes(output[:32])
				br.Balance = balance.String()
				if balance.Sign() > 0 {
					foundCount++
				}
			}
		}

		results[i] = br
	}

	slog.Debug("bsc rpc batch token complete",
		"provider", p.name,
		"token", token,
		"total", len(addresses),
		"funded", foundCount,
		"failed", failCount,
	)

	if failCount > 0 && failCount == len(addresses) {
		return results, fmt.Errorf("all %d token balance fetches failed: %w", failCount, config.ErrProviderUnavailable)
	}

	return results, nil
}
