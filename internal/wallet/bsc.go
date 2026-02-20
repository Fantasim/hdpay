package wallet

import (
	"fmt"
	"log/slog"

	"github.com/btcsuite/btcd/btcutil/hdkeychain"
	"github.com/ethereum/go-ethereum/crypto"

	"github.com/Fantasim/hdpay/internal/config"
)

// DeriveBSCParentKey pre-derives the BSC parent key to m/44'/60'/0'/0.
// This eliminates redundant derivation of the first 4 levels during batch generation.
// The returned key is safe for concurrent read-only use (Derive creates new child keys).
func DeriveBSCParentKey(masterKey *hdkeychain.ExtendedKey) (*hdkeychain.ExtendedKey, error) {
	// m/44'
	purpose, err := masterKey.Derive(hdkeychain.HardenedKeyStart + uint32(config.BIP44Purpose))
	if err != nil {
		return nil, fmt.Errorf("derive BSC purpose key: %w", err)
	}

	// m/44'/60'
	coin, err := purpose.Derive(hdkeychain.HardenedKeyStart + uint32(config.BSCCoinType))
	if err != nil {
		return nil, fmt.Errorf("derive BSC coin key: %w", err)
	}

	// m/44'/60'/0'
	account, err := coin.Derive(hdkeychain.HardenedKeyStart + 0)
	if err != nil {
		return nil, fmt.Errorf("derive BSC account key: %w", err)
	}

	// m/44'/60'/0'/0
	change, err := account.Derive(0)
	if err != nil {
		return nil, fmt.Errorf("derive BSC change key: %w", err)
	}

	// Force lazy pubkey computation so concurrent Derive() calls don't race.
	if _, err := change.ECPubKey(); err != nil {
		return nil, fmt.Errorf("warm BSC parent pubkey cache: %w", err)
	}

	slog.Debug("pre-derived BSC parent key", "path", "m/44'/60'/0'/0")
	return change, nil
}

// DeriveBSCAddressFromParent derives a BSC address from a pre-derived parent key.
// parentKey must be at m/44'/60'/0'/0 (from DeriveBSCParentKey).
// Only performs 1 derivation (index) instead of 5.
func DeriveBSCAddressFromParent(parentKey *hdkeychain.ExtendedKey, index uint32) (string, error) {
	child, err := parentKey.Derive(index)
	if err != nil {
		return "", fmt.Errorf("derive BSC child key at index %d: %w", index, err)
	}

	privKey, err := child.ECPrivKey()
	if err != nil {
		return "", fmt.Errorf("get BSC private key at index %d: %w", index, err)
	}

	ecdsaPrivKey := privKey.ToECDSA()
	addr := crypto.PubkeyToAddress(ecdsaPrivKey.PublicKey)

	return addr.Hex(), nil
}

// DeriveBSCAddress derives an EVM (BSC) address at the given index.
// Path: m/44'/60'/0'/0/N (same coin type for mainnet and testnet).
// Returns an EIP-55 checksummed address (0x...).
func DeriveBSCAddress(masterKey *hdkeychain.ExtendedKey, index uint32) (string, error) {
	parentKey, err := DeriveBSCParentKey(masterKey)
	if err != nil {
		return "", err
	}

	addr, err := DeriveBSCAddressFromParent(parentKey, index)
	if err != nil {
		return "", err
	}

	slog.Debug("derived BSC address",
		"index", index,
		"address", addr,
	)

	return addr, nil
}
