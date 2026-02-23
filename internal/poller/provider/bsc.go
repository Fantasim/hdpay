package provider

import (
	"context"
	"fmt"
	"log/slog"
	"math/big"
	"strings"
	"sync"
	"time"

	hdconfig "github.com/Fantasim/hdpay/internal/shared/config"
	pollerconfig "github.com/Fantasim/hdpay/internal/poller/config"
	"github.com/ethereum/go-ethereum"
	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/crypto"
	"github.com/ethereum/go-ethereum/ethclient"
)

// transferEventTopic is keccak256("Transfer(address,address,uint256)").
var transferEventTopic = crypto.Keccak256Hash([]byte("Transfer(address,address,uint256)"))

// BSCRPCPollerProvider detects incoming BSC transactions using public JSON-RPC.
//
// For token transfers (USDC/USDT): eth_getLogs with Transfer event filtered by
// recipient address and known contract addresses.
//
// For native BNB: eth_getBalance polled every tick. An increase in balance
// compared to the last known value is recorded as an incoming receipt. A
// synthetic tx hash ("bnb-{address}-block-{block}") is used for deduplication
// since native BNB transfers do not emit events.
type BSCRPCPollerProvider struct {
	client  *ethclient.Client
	network string

	// lastKnownBal maps address (lower-case) → last-seen confirmed balance.
	// Used to detect BNB balance increases between polls.
	lastKnownBal sync.Map
}

// NewBSCRPCPollerProvider creates a provider that connects to a public BSC RPC node.
// api.bscscan.com was shut down Dec 18, 2025; this implementation uses only
// go-ethereum ethclient and requires no external API key.
// Rate limiting is handled externally by ProviderSet.
func NewBSCRPCPollerProvider(network string) (*BSCRPCPollerProvider, error) {
	rpcURL := hdconfig.BscRPCMainnetURL
	if network == "testnet" {
		rpcURL = hdconfig.BscRPCTestnetURL
	}

	slog.Info("bsc rpc poller provider connecting",
		"rpcURL", rpcURL,
		"network", network,
	)

	client, err := ethclient.Dial(rpcURL)
	if err != nil {
		// Try the Ankr fallback immediately if the primary fails.
		slog.Warn("primary BSC RPC failed, trying Ankr fallback",
			"primary", rpcURL,
			"fallback", hdconfig.BscRPCMainnetURL2,
			"error", err,
		)
		if network != "testnet" {
			client, err = ethclient.Dial(hdconfig.BscRPCMainnetURL2)
			if err != nil {
				return nil, fmt.Errorf("dial BSC RPC (primary + fallback): %w", err)
			}
			rpcURL = hdconfig.BscRPCMainnetURL2
		} else {
			return nil, fmt.Errorf("dial BSC RPC %s: %w", rpcURL, err)
		}
	}

	slog.Info("bsc rpc poller provider connected", "rpcURL", rpcURL)

	return &BSCRPCPollerProvider{
		client:  client,
		network: network,
	}, nil
}

func (p *BSCRPCPollerProvider) Name() string  { return "bscrpc-poller" }
func (p *BSCRPCPollerProvider) Chain() string { return "BSC" }

// FetchTransactions returns incoming BSC transactions since cutoffUnix.
// Token transfers are detected via eth_getLogs; native BNB via balance delta.
// Rate limiting is handled externally by ProviderSet.
func (p *BSCRPCPollerProvider) FetchTransactions(ctx context.Context, address string, cutoffUnix int64) ([]RawTransaction, error) {
	latestBlock, err := p.client.BlockNumber(ctx)
	if err != nil {
		return nil, fmt.Errorf("bsc rpc get block number: %w", err)
	}

	// Estimate fromBlock from cutoff. BSC produces a block ~every 3 seconds.
	elapsed := time.Now().Unix() - cutoffUnix
	blocksBack := elapsed / 3
	if blocksBack < 0 {
		blocksBack = 0
	}
	// Cap at 50 000 blocks (~41h) to avoid oversized log queries on public RPC.
	if blocksBack > 50_000 {
		blocksBack = 50_000
	}
	fromBlock := int64(latestBlock) - blocksBack
	if fromBlock < 0 {
		fromBlock = 0
	}

	slog.Debug("bsc rpc poller fetching",
		"address", address,
		"fromBlock", fromBlock,
		"latestBlock", latestBlock,
		"cutoffUnix", cutoffUnix,
	)

	var result []RawTransaction

	// 1. Token transfers (USDC + USDT) via eth_getLogs.
	tokenTxs, err := p.fetchTokenLogs(ctx, address, fromBlock, int64(latestBlock))
	if err != nil {
		slog.Warn("bsc rpc token log fetch failed",
			"address", address,
			"error", err,
		)
		// Non-fatal: continue to check BNB balance.
	} else {
		result = append(result, tokenTxs...)
	}

	// 2. Native BNB via balance delta.
	bnbTxs, err := p.fetchBNBDelta(ctx, address, latestBlock)
	if err != nil {
		slog.Warn("bsc rpc bnb balance check failed",
			"address", address,
			"error", err,
		)
	} else {
		result = append(result, bnbTxs...)
	}

	slog.Info("BSC RPC transactions fetched",
		"provider", p.Name(),
		"address", address,
		"tokenCount", len(tokenTxs),
		"bnbCount", len(bnbTxs),
		"totalCount", len(result),
	)

	return result, nil
}

// fetchTokenLogs fetches USDC and USDT Transfer events directed to the watched address.
func (p *BSCRPCPollerProvider) fetchTokenLogs(ctx context.Context, address string, fromBlock, toBlock int64) ([]RawTransaction, error) {
	usdcAddr, usdtAddr, usdcDecimals, usdtDecimals := p.tokenConfig()
	if usdcAddr == "" && usdtAddr == "" {
		return nil, nil // no tokens configured for this network
	}

	// Build contract address list.
	var contracts []common.Address
	if usdcAddr != "" {
		contracts = append(contracts, common.HexToAddress(usdcAddr))
	}
	if usdtAddr != "" {
		contracts = append(contracts, common.HexToAddress(usdtAddr))
	}

	// topics[2] = recipient address (left-padded to 32 bytes).
	recipientTopic := common.BytesToHash(common.HexToAddress(address).Bytes())

	query := ethereum.FilterQuery{
		FromBlock: big.NewInt(fromBlock),
		ToBlock:   big.NewInt(toBlock),
		Addresses: contracts,
		Topics: [][]common.Hash{
			{transferEventTopic},
			{}, // from — any address
			{recipientTopic},
		},
	}

	logs, err := p.client.FilterLogs(ctx, query)
	if err != nil {
		return nil, fmt.Errorf("eth_getLogs: %w", err)
	}

	usdcAddrLower := strings.ToLower(usdcAddr)
	usdtAddrLower := strings.ToLower(usdtAddr)
	now := time.Now().Unix()

	var result []RawTransaction
	for _, log := range logs {
		if log.Removed {
			continue
		}

		contract := strings.ToLower(log.Address.Hex())
		token := ""
		decimals := 0
		switch {
		case contract == usdcAddrLower && usdcAddr != "":
			token = "USDC"
			decimals = usdcDecimals
		case contract == usdtAddrLower && usdtAddr != "":
			token = "USDT"
			decimals = usdtDecimals
		default:
			continue
		}

		// Data field is the uint256 amount (32 bytes, big-endian).
		if len(log.Data) < 32 {
			slog.Warn("bsc rpc: short log data, skipping", "txHash", log.TxHash.Hex())
			continue
		}
		amountInt := new(big.Int).SetBytes(log.Data[:32])
		amountRaw := amountInt.String()
		amountHuman := weiToHuman(amountRaw, decimals)

		slog.Debug("BSC token Transfer event detected",
			"txHash", log.TxHash.Hex(),
			"token", token,
			"amount", amountHuman,
			"blockNumber", log.BlockNumber,
		)

		result = append(result, RawTransaction{
			TxHash:      log.TxHash.Hex(),
			Token:       token,
			AmountRaw:   amountRaw,
			AmountHuman: amountHuman,
			Decimals:    decimals,
			BlockTime:   now,
			Confirmed:   true,
			BlockNumber: int64(log.BlockNumber),
		})
	}

	return result, nil
}

// fetchBNBDelta checks whether the native BNB balance increased since last poll.
// If so, a synthetic RawTransaction is returned for the detected delta.
// The synthetic tx hash is stable within a block to ensure deduplication.
func (p *BSCRPCPollerProvider) fetchBNBDelta(ctx context.Context, address string, currentBlock uint64) ([]RawTransaction, error) {
	addr := common.HexToAddress(address)
	currentBalance, err := p.client.BalanceAt(ctx, addr, nil) // nil = latest
	if err != nil {
		return nil, fmt.Errorf("eth_getBalance: %w", err)
	}

	addrKey := strings.ToLower(address)

	var result []RawTransaction
	if prev, loaded := p.lastKnownBal.Load(addrKey); loaded {
		prevBal := prev.(*big.Int)
		if currentBalance.Cmp(prevBal) > 0 {
			delta := new(big.Int).Sub(currentBalance, prevBal)
			amountRaw := delta.String()
			amountHuman := weiToHuman(amountRaw, hdconfig.BNBDecimals)

			// Synthetic hash: stable for this address + block combination.
			syntheticHash := fmt.Sprintf("bnb-%s-block-%d", addrKey, currentBlock)

			slog.Debug("BSC BNB balance increase detected",
				"address", address,
				"prevBalance", prevBal.String(),
				"currentBalance", currentBalance.String(),
				"delta", amountRaw,
				"blockNumber", currentBlock,
			)

			result = append(result, RawTransaction{
				TxHash:      syntheticHash,
				Token:       "BNB",
				AmountRaw:   amountRaw,
				AmountHuman: amountHuman,
				Decimals:    hdconfig.BNBDecimals,
				BlockTime:   time.Now().Unix(),
				Confirmed:   true,
				BlockNumber: int64(currentBlock),
			})
		}
	}

	// Always update last known balance.
	p.lastKnownBal.Store(addrKey, new(big.Int).Set(currentBalance))

	return result, nil
}

// CheckConfirmation checks BSC transaction confirmation.
// For synthetic BNB-balance hashes, returns confirmed immediately since
// they are only emitted after reading confirmed chain state.
func (p *BSCRPCPollerProvider) CheckConfirmation(ctx context.Context, txHash string, blockNumber int64) (bool, int, error) {
	// Synthetic BNB hashes are already based on confirmed balance reads.
	if strings.HasPrefix(txHash, "bnb-") {
		return true, pollerconfig.ConfirmationsBSC, nil
	}

	currentBlock, err := p.GetCurrentBlock(ctx)
	if err != nil {
		return false, 0, fmt.Errorf("bsc rpc get current block: %w", err)
	}

	if blockNumber <= 0 {
		return false, 0, nil
	}

	confirmations := int(currentBlock) - int(blockNumber)
	if confirmations < 0 {
		confirmations = 0
	}
	confirmed := confirmations >= pollerconfig.ConfirmationsBSC

	slog.Debug("BSC RPC confirmation check",
		"txHash", txHash,
		"blockNumber", blockNumber,
		"currentBlock", currentBlock,
		"confirmations", confirmations,
		"confirmed", confirmed,
		"threshold", pollerconfig.ConfirmationsBSC,
	)

	return confirmed, confirmations, nil
}

// GetCurrentBlock returns the latest BSC block number.
// Rate limiting is handled externally by ProviderSet.
func (p *BSCRPCPollerProvider) GetCurrentBlock(ctx context.Context) (uint64, error) {
	block, err := p.client.BlockNumber(ctx)
	if err != nil {
		return 0, fmt.Errorf("eth_blockNumber: %w", err)
	}

	slog.Debug("BSC RPC current block", "block", block)
	return block, nil
}

// tokenConfig returns the USDC and USDT contract addresses and decimals for the current network.
func (p *BSCRPCPollerProvider) tokenConfig() (usdcAddr, usdtAddr string, usdcDecimals, usdtDecimals int) {
	if p.network == "testnet" {
		return hdconfig.BSCTestnetUSDCContract, hdconfig.BSCTestnetUSDTContract,
			hdconfig.BSCUSDCDecimals, hdconfig.BSCUSDTDecimals
	}
	return hdconfig.BSCUSDCContract, hdconfig.BSCUSDTContract,
		hdconfig.BSCUSDCDecimals, hdconfig.BSCUSDTDecimals
}

// weiToHuman converts a raw integer amount string to a human-readable decimal string.
// decimals is the number of decimal places (e.g. 18 for BNB/ETH, 6 for USDC on BSC).
// Returns "0" on parse error.
func weiToHuman(raw string, decimals int) string {
	n, ok := new(big.Int).SetString(raw, 10)
	if !ok {
		return "0"
	}
	if decimals == 0 {
		return n.String()
	}
	divisor := new(big.Int).Exp(big.NewInt(10), big.NewInt(int64(decimals)), nil)
	quotient := new(big.Int).Quo(n, divisor)
	remainder := new(big.Int).Mod(n, divisor)

	// Format remainder with leading zeros to fill the decimals positions.
	fracFmt := fmt.Sprintf("%0*s", decimals, remainder.String())
	return fmt.Sprintf("%s.%s", quotient.String(), fracFmt)
}
