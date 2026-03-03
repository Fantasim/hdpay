package scanner

import (
	"context"
	"encoding/binary"
	"fmt"
	"log/slog"
	"math/big"

	"github.com/Fantasim/hdpay/internal/shared/config"
	"github.com/Fantasim/hdpay/internal/shared/models"
	"github.com/ethereum/go-ethereum"
	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/crypto"
	"github.com/ethereum/go-ethereum/ethclient"
)

// Multicall3 ABI selectors (first 4 bytes of keccak256).
var (
	// aggregate3((address,bool,bytes)[]) => 0x82ad56cb
	aggregate3Selector = crypto.Keccak256([]byte("aggregate3((address,bool,bytes)[])"))[:4]

	// getEthBalance(address) => 0x4d2301cc
	getEthBalanceSelector = crypto.Keccak256([]byte("getEthBalance(address)"))[:4]
)

// multicallCall3 mirrors the Multicall3.Call3 struct.
type multicallCall3 struct {
	Target       common.Address
	AllowFailure bool
	CallData     []byte
}

// multicallResult mirrors the Multicall3.Result struct.
type multicallResult struct {
	Success    bool
	ReturnData []byte
}

// BSCMulticallProvider fetches BSC balances via Multicall3's aggregate3 function.
// A single eth_call reads up to 200 addresses' balances (native + BEP-20) in one HTTP request.
type BSCMulticallProvider struct {
	client        *ethclient.Client
	rl            *RateLimiter
	rpcURL        string
	name          string
	multicallAddr common.Address
}

// NewBSCMulticallProvider creates a Multicall3-based BSC provider.
func NewBSCMulticallProvider(rl *RateLimiter, name, rpcURL string) (*BSCMulticallProvider, error) {
	slog.Info("bsc multicall provider connecting",
		"name", name,
		"rpcURL", rpcURL,
		"multicallAddr", config.Multicall3Address,
	)

	client, err := ethclient.Dial(rpcURL)
	if err != nil {
		return nil, fmt.Errorf("dial BSC RPC %s: %w", rpcURL, err)
	}

	slog.Info("bsc multicall provider connected", "name", name, "rpcURL", rpcURL)

	return &BSCMulticallProvider{
		client:        client,
		rl:            rl,
		rpcURL:        rpcURL,
		name:          name,
		multicallAddr: common.HexToAddress(config.Multicall3Address),
	}, nil
}

func (p *BSCMulticallProvider) Name() string            { return p.name }
func (p *BSCMulticallProvider) Chain() models.Chain      { return models.ChainBSC }
func (p *BSCMulticallProvider) MaxBatchSize() int        { return config.Multicall3BatchSize }
func (p *BSCMulticallProvider) RecordSuccess()           { p.rl.RecordSuccess() }
func (p *BSCMulticallProvider) RecordFailure(is429 bool) { p.rl.RecordFailure(is429) }
func (p *BSCMulticallProvider) Stats() MetricsSnapshot   { return p.rl.Stats() }

// Close closes the underlying ethclient connection.
func (p *BSCMulticallProvider) Close() {
	p.client.Close()
	slog.Info("bsc multicall provider closed", "name", p.name, "rpcURL", p.rpcURL)
}

// FetchNativeBalances reads native BNB balances for all addresses in a single eth_call
// to Multicall3's aggregate3 function using getEthBalance(address) subcalls.
func (p *BSCMulticallProvider) FetchNativeBalances(ctx context.Context, addresses []models.Address) ([]BalanceResult, error) {
	if len(addresses) == 0 {
		return nil, nil
	}

	// Single rate-limit wait for the entire batch (1 HTTP request).
	if err := p.rl.Wait(ctx); err != nil {
		return nil, fmt.Errorf("rate limiter wait: %w", err)
	}

	// Build Call3[] — one getEthBalance(addr) per address.
	calls := make([]multicallCall3, len(addresses))
	for i, addr := range addresses {
		calls[i] = multicallCall3{
			Target:       p.multicallAddr,
			AllowFailure: true,
			CallData:     encodeGetEthBalance(common.HexToAddress(addr.Address)),
		}
	}

	slog.Debug("multicall3 native batch",
		"provider", p.name,
		"addressCount", len(addresses),
	)

	results, err := p.callAggregate3(ctx, calls)
	if err != nil {
		return nil, err
	}

	if len(results) != len(addresses) {
		slog.Warn("multicall3 result count mismatch",
			"provider", p.name,
			"expected", len(addresses),
			"got", len(results),
		)
		return nil, fmt.Errorf("multicall3 result count mismatch: expected %d, got %d: %w",
			len(addresses), len(results), config.ErrMulticall3Failed)
	}

	// Parse results.
	balances := make([]BalanceResult, len(addresses))
	var foundCount int
	for i, addr := range addresses {
		br := BalanceResult{
			Address:      addr.Address,
			AddressIndex: addr.AddressIndex,
			Source:       p.Name(),
		}

		if !results[i].Success {
			br.Balance = "0"
			br.Error = "multicall3 subcall failed"
			slog.Debug("multicall3 native subcall failed",
				"address", addr.Address,
				"index", addr.AddressIndex,
			)
		} else {
			balance := parseUint256(results[i].ReturnData)
			br.Balance = balance.String()
			if balance.Sign() > 0 {
				foundCount++
			}
		}

		balances[i] = br
	}

	slog.Debug("multicall3 native batch complete",
		"provider", p.name,
		"total", len(addresses),
		"funded", foundCount,
	)

	return balances, nil
}

// FetchTokenBalances reads BEP-20 token balances for all addresses in a single eth_call
// to Multicall3's aggregate3 function using balanceOf(address) subcalls.
func (p *BSCMulticallProvider) FetchTokenBalances(ctx context.Context, addresses []models.Address, token models.Token, contractAddress string) ([]BalanceResult, error) {
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

	tokenContract := common.HexToAddress(contractAddress)

	// Build Call3[] — one balanceOf(addr) per address, targeting the token contract.
	calls := make([]multicallCall3, len(addresses))
	for i, addr := range addresses {
		calls[i] = multicallCall3{
			Target:       tokenContract,
			AllowFailure: true,
			CallData:     encodeBalanceOfCall(common.HexToAddress(addr.Address)),
		}
	}

	slog.Debug("multicall3 token batch",
		"provider", p.name,
		"token", token,
		"contract", contractAddress,
		"addressCount", len(addresses),
	)

	results, err := p.callAggregate3(ctx, calls)
	if err != nil {
		return nil, err
	}

	if len(results) != len(addresses) {
		slog.Warn("multicall3 token result count mismatch",
			"provider", p.name,
			"token", token,
			"expected", len(addresses),
			"got", len(results),
		)
		return nil, fmt.Errorf("multicall3 token result count mismatch: expected %d, got %d: %w",
			len(addresses), len(results), config.ErrMulticall3Failed)
	}

	// Parse results.
	balances := make([]BalanceResult, len(addresses))
	var foundCount int
	for i, addr := range addresses {
		br := BalanceResult{
			Address:      addr.Address,
			AddressIndex: addr.AddressIndex,
			Source:       p.Name(),
		}

		if !results[i].Success {
			br.Balance = "0"
			br.Error = "multicall3 token subcall failed"
			slog.Debug("multicall3 token subcall failed",
				"address", addr.Address,
				"index", addr.AddressIndex,
				"token", token,
			)
		} else {
			balance := parseUint256(results[i].ReturnData)
			br.Balance = balance.String()
			if balance.Sign() > 0 {
				foundCount++
			}
		}

		balances[i] = br
	}

	slog.Debug("multicall3 token batch complete",
		"provider", p.name,
		"token", token,
		"total", len(addresses),
		"funded", foundCount,
	)

	return balances, nil
}

// callAggregate3 encodes and sends a single eth_call to Multicall3's aggregate3 function.
func (p *BSCMulticallProvider) callAggregate3(ctx context.Context, calls []multicallCall3) ([]multicallResult, error) {
	calldata, err := encodeAggregate3(calls)
	if err != nil {
		return nil, fmt.Errorf("encode aggregate3: %w", err)
	}

	to := p.multicallAddr
	msg := ethereum.CallMsg{
		To:   &to,
		Data: calldata,
	}

	output, err := p.client.CallContract(ctx, msg, nil)
	if err != nil {
		slog.Warn("multicall3 eth_call failed",
			"provider", p.name,
			"callCount", len(calls),
			"error", err,
		)
		return nil, fmt.Errorf("multicall3 eth_call: %w", config.NewTransientError(err))
	}

	results, err := decodeAggregate3Results(output, len(calls))
	if err != nil {
		return nil, fmt.Errorf("decode aggregate3 results: %w", err)
	}

	slog.Debug("multicall3 eth_call success",
		"provider", p.name,
		"callCount", len(calls),
		"resultCount", len(results),
		"outputBytes", len(output),
	)

	return results, nil
}

// ── ABI Encoding Helpers ─────────────────────────────────────────────────────

// encodeGetEthBalance encodes getEthBalance(address) calldata.
// Result: 4-byte selector + 32-byte left-padded address = 36 bytes.
func encodeGetEthBalance(addr common.Address) []byte {
	data := make([]byte, 36)
	copy(data[:4], getEthBalanceSelector)
	copy(data[4+12:], addr.Bytes()) // 20-byte address, left-padded to 32 bytes
	return data
}

// encodeBalanceOfCall encodes balanceOf(address) calldata.
// Uses the same selector as bsc_rpc.go's balanceOfSelector.
func encodeBalanceOfCall(addr common.Address) []byte {
	data := make([]byte, 36)
	copy(data[:4], balanceOfSelector) // from bsc_rpc.go
	copy(data[4+12:], addr.Bytes())
	return data
}

// encodeAggregate3 encodes the full aggregate3((address,bool,bytes)[]) calldata.
//
// ABI layout:
//
//	[0:4]   function selector (0x82ad56cb)
//	[4:36]  offset to array data = 0x20
//	[36:68] array length
//	[68..]  per-element offsets (32 bytes each), then element data
//
// Each Call3 element: (address target, bool allowFailure, bytes callData)
func encodeAggregate3(calls []multicallCall3) ([]byte, error) {
	if len(calls) == 0 {
		return nil, fmt.Errorf("no calls to encode")
	}

	// Pre-calculate total size.
	// Header: 4 (selector) + 32 (array offset) + 32 (array length) = 68
	// Offsets: 32 * len(calls)
	// Elements: each has target(32) + allowFailure(32) + callData offset(32) + callData length(32) + callData padded
	headerSize := 68
	offsetsSize := 32 * len(calls)

	var elementsSize int
	for _, c := range calls {
		// Each element: target(32) + allowFailure(32) + callData_offset(32) + callData_len(32) + callData_padded
		paddedDataLen := ((len(c.CallData) + 31) / 32) * 32
		elementsSize += 32 + 32 + 32 + 32 + paddedDataLen
	}

	totalSize := headerSize + offsetsSize + elementsSize
	buf := make([]byte, totalSize)

	// Function selector.
	copy(buf[:4], aggregate3Selector)

	// Offset to array data = 0x20 (32).
	writeUint256(buf[4:36], 32)

	// Array length.
	writeUint256(buf[36:68], uint64(len(calls)))

	// Per-element offsets (relative to start of array data, after the length slot).
	// The offsets section starts at buf[68], and each offset points to where the element
	// data begins, relative to the start of the offsets section.
	offsetsStart := 68
	elementDataStart := offsetsStart + offsetsSize
	currentElementOffset := uint64(offsetsSize) // first element starts right after all offsets
	elementWritePos := elementDataStart

	for i, c := range calls {
		// Write offset for this element.
		writeUint256(buf[offsetsStart+32*i:offsetsStart+32*(i+1)], currentElementOffset)

		// Write element data.
		// target (address, left-padded to 32 bytes)
		copy(buf[elementWritePos+12:elementWritePos+32], c.Target.Bytes())
		elementWritePos += 32

		// allowFailure (bool, uint256)
		if c.AllowFailure {
			buf[elementWritePos+31] = 1
		}
		elementWritePos += 32

		// offset to callData within this tuple = 96 (3 × 32 = offset past target + allowFailure + this offset slot)
		writeUint256(buf[elementWritePos:elementWritePos+32], 96)
		elementWritePos += 32

		// callData length
		writeUint256(buf[elementWritePos:elementWritePos+32], uint64(len(c.CallData)))
		elementWritePos += 32

		// callData bytes (padded to 32-byte boundary)
		copy(buf[elementWritePos:], c.CallData)
		paddedDataLen := ((len(c.CallData) + 31) / 32) * 32
		elementWritePos += paddedDataLen

		// Update offset for next element.
		elemSize := 32 + 32 + 32 + 32 + paddedDataLen
		currentElementOffset += uint64(elemSize)
	}

	return buf, nil
}

// decodeAggregate3Results decodes the aggregate3 return data: Result[] where Result = (bool success, bytes returnData).
//
// ABI layout:
//
//	[0:32]    offset to array = 0x20
//	[32:64]   array length
//	[64..]    per-element offsets, then element data
func decodeAggregate3Results(output []byte, expectedCount int) ([]multicallResult, error) {
	if len(output) < 64 {
		return nil, fmt.Errorf("output too short: %d bytes, need at least 64", len(output))
	}

	// Read array offset — should be 32 (0x20).
	arrayOffset := readUint256AsUint64(output[0:32])
	if arrayOffset != 32 || int(arrayOffset) > len(output) {
		return nil, fmt.Errorf("unexpected array offset: %d", arrayOffset)
	}

	// Read array length.
	arrayLen := readUint256AsUint64(output[32:64])
	if int(arrayLen) != expectedCount {
		return nil, fmt.Errorf("array length %d != expected %d", arrayLen, expectedCount)
	}

	results := make([]multicallResult, arrayLen)

	// Read per-element offsets (relative to output[64], i.e. start of offsets section).
	offsetsStart := 64
	for i := 0; i < int(arrayLen); i++ {
		elemOffsetPos := offsetsStart + 32*i
		if elemOffsetPos+32 > len(output) {
			return nil, fmt.Errorf("truncated at element offset %d", i)
		}
		elemOffset := readUint256AsUint64(output[elemOffsetPos : elemOffsetPos+32])

		// Element data starts at offsetsStart + elemOffset.
		elemStart := offsetsStart + int(elemOffset)
		if elemStart+64 > len(output) {
			return nil, fmt.Errorf("truncated at element data %d (start=%d, outputLen=%d)", i, elemStart, len(output))
		}

		// success (bool at first 32 bytes)
		results[i].Success = output[elemStart+31] != 0

		// offset to returnData within this tuple
		returnDataOffset := readUint256AsUint64(output[elemStart+32 : elemStart+64])
		returnDataLenPos := elemStart + int(returnDataOffset)
		if returnDataLenPos+32 > len(output) {
			return nil, fmt.Errorf("truncated at returnData length %d", i)
		}

		returnDataLen := readUint256AsUint64(output[returnDataLenPos : returnDataLenPos+32])
		returnDataStart := returnDataLenPos + 32
		if returnDataStart+int(returnDataLen) > len(output) {
			return nil, fmt.Errorf("truncated at returnData bytes %d (need %d, have %d)",
				i, returnDataStart+int(returnDataLen), len(output))
		}

		results[i].ReturnData = make([]byte, returnDataLen)
		copy(results[i].ReturnData, output[returnDataStart:returnDataStart+int(returnDataLen)])
	}

	return results, nil
}

// ── Utility Helpers ──────────────────────────────────────────────────────────

// writeUint256 writes a uint64 value as a big-endian 32-byte ABI uint256.
func writeUint256(dst []byte, val uint64) {
	// Zero the full 32 bytes first (for safety).
	for i := range 32 {
		dst[i] = 0
	}
	binary.BigEndian.PutUint64(dst[24:32], val)
}

// readUint256AsUint64 reads a 32-byte big-endian uint256 as a uint64 (lower 8 bytes).
// Sufficient for array lengths, offsets, and small values.
func readUint256AsUint64(src []byte) uint64 {
	if len(src) < 32 {
		return 0
	}
	return binary.BigEndian.Uint64(src[24:32])
}

// parseUint256 parses a 32-byte ABI-encoded uint256 as a *big.Int.
// Returns 0 for empty or short data.
func parseUint256(data []byte) *big.Int {
	if len(data) == 0 {
		return new(big.Int)
	}
	if len(data) < 32 {
		return new(big.Int).SetBytes(data)
	}
	return new(big.Int).SetBytes(data[:32])
}
