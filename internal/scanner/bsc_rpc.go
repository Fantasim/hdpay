package scanner

import (
	"context"
	"fmt"
	"log/slog"
	"math/big"

	"github.com/Fantasim/hdpay/internal/config"
	"github.com/Fantasim/hdpay/internal/models"
	"github.com/ethereum/go-ethereum"
	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/crypto"
	"github.com/ethereum/go-ethereum/ethclient"
)

// balanceOfSelector is the first 4 bytes of keccak256("balanceOf(address)").
var balanceOfSelector = crypto.Keccak256([]byte("balanceOf(address)"))[:4]

// BSCRPCProvider fetches BSC balances via ethclient JSON-RPC.
type BSCRPCProvider struct {
	client *ethclient.Client
	rl     *RateLimiter
	rpcURL string
}

// NewBSCRPCProvider creates a provider that connects to a BSC JSON-RPC endpoint.
func NewBSCRPCProvider(rl *RateLimiter, network string) (*BSCRPCProvider, error) {
	rpcURL := config.BscRPCMainnetURL
	if network == string(models.NetworkTestnet) {
		rpcURL = config.BscRPCTestnetURL
	}

	slog.Info("bsc rpc provider connecting",
		"rpcURL", rpcURL,
		"network", network,
	)

	client, err := ethclient.Dial(rpcURL)
	if err != nil {
		return nil, fmt.Errorf("dial BSC RPC %s: %w", rpcURL, err)
	}

	slog.Info("bsc rpc provider connected", "rpcURL", rpcURL)

	return &BSCRPCProvider{
		client: client,
		rl:     rl,
		rpcURL: rpcURL,
	}, nil
}

func (p *BSCRPCProvider) Name() string              { return "BSCRPC" }
func (p *BSCRPCProvider) Chain() models.Chain        { return models.ChainBSC }
func (p *BSCRPCProvider) MaxBatchSize() int          { return 1 }

// Close closes the underlying ethclient connection.
func (p *BSCRPCProvider) Close() {
	p.client.Close()
	slog.Info("bsc rpc provider closed", "rpcURL", p.rpcURL)
}

// FetchNativeBalances fetches BNB balance for each address using eth_getBalance.
// Continues on per-address errors instead of early-returning.
func (p *BSCRPCProvider) FetchNativeBalances(ctx context.Context, addresses []models.Address) ([]BalanceResult, error) {
	results := make([]BalanceResult, 0, len(addresses))
	var failCount int

	for _, addr := range addresses {
		if err := p.rl.Wait(ctx); err != nil {
			return results, fmt.Errorf("rate limiter wait: %w", err)
		}

		balance, err := p.client.BalanceAt(ctx, common.HexToAddress(addr.Address), nil)
		if err != nil {
			if ctx.Err() != nil {
				return results, fmt.Errorf("context cancelled during balance fetch: %w", err)
			}

			slog.Warn("bsc rpc balance error",
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

		balanceStr := balance.String()
		results = append(results, BalanceResult{
			Address:      addr.Address,
			AddressIndex: addr.AddressIndex,
			Balance:      balanceStr,
			Source:       p.Name(),
		})

		slog.Debug("bsc rpc native balance fetched",
			"address", addr.Address,
			"index", addr.AddressIndex,
			"balance", balanceStr,
		)
	}

	if failCount > 0 && failCount == len(addresses) {
		return results, fmt.Errorf("all %d addresses failed: %w", failCount, config.ErrProviderUnavailable)
	}

	return results, nil
}

// FetchTokenBalances fetches BEP-20 token balances using eth_call with balanceOf(address).
// Continues on per-address errors instead of early-returning.
func (p *BSCRPCProvider) FetchTokenBalances(ctx context.Context, addresses []models.Address, token models.Token, contractAddress string) ([]BalanceResult, error) {
	if contractAddress == "" {
		return nil, fmt.Errorf("contract address required for BSC token balance")
	}

	contract := common.HexToAddress(contractAddress)
	results := make([]BalanceResult, 0, len(addresses))
	var failCount int

	for _, addr := range addresses {
		if err := p.rl.Wait(ctx); err != nil {
			return results, fmt.Errorf("rate limiter wait: %w", err)
		}

		balance, err := p.callBalanceOf(ctx, contract, common.HexToAddress(addr.Address))
		if err != nil {
			if ctx.Err() != nil {
				return results, fmt.Errorf("context cancelled during token fetch: %w", err)
			}

			slog.Warn("bsc rpc token balance error",
				"provider", p.Name(),
				"address", addr.Address,
				"index", addr.AddressIndex,
				"token", token,
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

		balanceStr := balance.String()
		results = append(results, BalanceResult{
			Address:      addr.Address,
			AddressIndex: addr.AddressIndex,
			Balance:      balanceStr,
			Source:       p.Name(),
		})

		slog.Debug("bsc rpc token balance fetched",
			"address", addr.Address,
			"index", addr.AddressIndex,
			"token", token,
			"balance", balanceStr,
		)
	}

	if failCount > 0 && failCount == len(addresses) {
		return results, fmt.Errorf("all %d token balance fetches failed: %w", failCount, config.ErrProviderUnavailable)
	}

	return results, nil
}

// callBalanceOf executes an eth_call for ERC-20 balanceOf(address).
func (p *BSCRPCProvider) callBalanceOf(ctx context.Context, contract, holder common.Address) (*big.Int, error) {
	// ABI encode: balanceOf(address) = selector + padded address.
	data := make([]byte, 4+32)
	copy(data[:4], balanceOfSelector)
	copy(data[4+12:], holder.Bytes()) // 20-byte address, left-padded to 32 bytes

	msg := ethereum.CallMsg{
		To:   &contract,
		Data: data,
	}

	output, err := p.client.CallContract(ctx, msg, nil)
	if err != nil {
		return nil, fmt.Errorf("call contract: %w", err)
	}

	if len(output) < 32 {
		slog.Warn("bsc rpc malformed contract response",
			"outputLen", len(output),
			"expected", 32,
		)
		return nil, fmt.Errorf("malformed contract response: expected 32 bytes, got %d", len(output))
	}

	balance := new(big.Int).SetBytes(output)
	return balance, nil
}
