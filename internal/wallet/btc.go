package wallet

import (
	"fmt"
	"log/slog"

	"github.com/btcsuite/btcd/btcutil"
	"github.com/btcsuite/btcd/btcutil/hdkeychain"
	"github.com/btcsuite/btcd/chaincfg"

	"github.com/Fantasim/hdpay/internal/config"
)

// DeriveBTCParentKey pre-derives the BTC parent key to m/84'/coin'/0'/0.
// This eliminates redundant derivation of the first 4 levels during batch generation.
// The returned key is safe for concurrent read-only use (Derive creates new child keys).
func DeriveBTCParentKey(masterKey *hdkeychain.ExtendedKey, net *chaincfg.Params) (*hdkeychain.ExtendedKey, error) {
	coinType := uint32(config.BTCCoinType)
	if net == &chaincfg.TestNet3Params {
		coinType = uint32(config.BTCTestCoinType)
	}

	// m/84'
	purpose, err := masterKey.Derive(hdkeychain.HardenedKeyStart + uint32(config.BIP84Purpose))
	if err != nil {
		return nil, fmt.Errorf("derive BTC purpose key: %w", err)
	}

	// m/84'/coin'
	coin, err := purpose.Derive(hdkeychain.HardenedKeyStart + coinType)
	if err != nil {
		return nil, fmt.Errorf("derive BTC coin key: %w", err)
	}

	// m/84'/coin'/0'
	account, err := coin.Derive(hdkeychain.HardenedKeyStart + 0)
	if err != nil {
		return nil, fmt.Errorf("derive BTC account key: %w", err)
	}

	// m/84'/coin'/0'/0
	change, err := account.Derive(0)
	if err != nil {
		return nil, fmt.Errorf("derive BTC change key: %w", err)
	}

	// Force lazy pubkey computation so concurrent Derive() calls don't race.
	if _, err := change.ECPubKey(); err != nil {
		return nil, fmt.Errorf("warm BTC parent pubkey cache: %w", err)
	}

	slog.Debug("pre-derived BTC parent key", "path", fmt.Sprintf("m/84'/%d'/0'/0", coinType))
	return change, nil
}

// DeriveBTCAddressFromParent derives a BTC address from a pre-derived parent key.
// parentKey must be at m/84'/coin'/0'/0 (from DeriveBTCParentKey).
// Only performs 1 derivation (index) instead of 5.
func DeriveBTCAddressFromParent(parentKey *hdkeychain.ExtendedKey, index uint32, net *chaincfg.Params) (string, error) {
	child, err := parentKey.Derive(index)
	if err != nil {
		return "", fmt.Errorf("derive BTC child key at index %d: %w", index, err)
	}

	pubKey, err := child.ECPubKey()
	if err != nil {
		return "", fmt.Errorf("get BTC public key at index %d: %w", index, err)
	}

	witnessProg := btcutil.Hash160(pubKey.SerializeCompressed())
	addr, err := btcutil.NewAddressWitnessPubKeyHash(witnessProg, net)
	if err != nil {
		return "", fmt.Errorf("create BTC bech32 address at index %d: %w", index, err)
	}

	return addr.EncodeAddress(), nil
}

// DeriveBTCAddress derives a BTC Native SegWit (bech32) address at the given index.
// Path: m/84'/0'/0'/0/N (mainnet) or m/84'/1'/0'/0/N (testnet) per BIP-84.
func DeriveBTCAddress(masterKey *hdkeychain.ExtendedKey, index uint32, net *chaincfg.Params) (string, error) {
	parentKey, err := DeriveBTCParentKey(masterKey, net)
	if err != nil {
		return "", err
	}

	addr, err := DeriveBTCAddressFromParent(parentKey, index, net)
	if err != nil {
		return "", err
	}

	slog.Debug("derived BTC address",
		"index", index,
		"address", addr,
		"network", net.Name,
	)

	return addr, nil
}
