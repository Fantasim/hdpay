package validate

import (
	"fmt"
	"log/slog"
	"regexp"

	"github.com/btcsuite/btcd/btcutil"
	"github.com/btcsuite/btcd/chaincfg"
	"github.com/mr-tron/base58"
)

// bscAddressRegex matches a valid BSC/EVM hex address (0x + 40 hex chars).
var bscAddressRegex = regexp.MustCompile(`^0x[0-9a-fA-F]{40}$`)

// Address validates that addr is a well-formed address for the given chain and network.
// Network must be "mainnet" or "testnet".
func Address(chain, addr, network string) error {
	slog.Debug("validating address",
		"chain", chain,
		"address", addr,
		"network", network,
	)

	switch chain {
	case "BTC":
		return validateBTC(addr, network)
	case "BSC":
		return validateBSC(addr)
	case "SOL":
		return validateSOL(addr)
	default:
		return fmt.Errorf("unsupported chain %q", chain)
	}
}

// validateBTC uses btcutil.DecodeAddress to fully validate a BTC address
// including checksum verification for bech32 addresses, and verifies the
// address belongs to the specified network.
func validateBTC(addr, network string) error {
	var params *chaincfg.Params
	switch network {
	case "mainnet":
		params = &chaincfg.MainNetParams
	case "testnet":
		params = &chaincfg.TestNet3Params
	default:
		return fmt.Errorf("unsupported BTC network %q", network)
	}

	decoded, err := btcutil.DecodeAddress(addr, params)
	if err != nil {
		return fmt.Errorf("invalid BTC address %q: %w", addr, err)
	}

	if !decoded.IsForNet(params) {
		return fmt.Errorf("invalid BTC address %q: address is not for %s network", addr, network)
	}

	return nil
}

// validateBSC checks that addr matches the 0x + 40 hex chars format.
// Same format for mainnet and testnet.
func validateBSC(addr string) error {
	if !bscAddressRegex.MatchString(addr) {
		return fmt.Errorf("invalid BSC address %q: must match 0x + 40 hex characters", addr)
	}
	return nil
}

// validateSOL decodes a base58 address and verifies it is exactly 32 bytes
// (ed25519 public key).
func validateSOL(addr string) error {
	decoded, err := base58.Decode(addr)
	if err != nil {
		return fmt.Errorf("invalid SOL address %q: base58 decode failed: %w", addr, err)
	}
	if len(decoded) != 32 {
		return fmt.Errorf("invalid SOL address %q: decoded to %d bytes, expected 32", addr, len(decoded))
	}
	return nil
}
