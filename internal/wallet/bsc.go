package wallet

import (
	"fmt"
	"log/slog"

	"github.com/btcsuite/btcd/btcutil/hdkeychain"
	"github.com/ethereum/go-ethereum/crypto"

	"github.com/Fantasim/hdpay/internal/config"
)

// DeriveBSCAddress derives an EVM (BSC) address at the given index.
// Path: m/44'/60'/0'/0/N (same coin type for mainnet and testnet).
// Returns an EIP-55 checksummed address (0x...).
func DeriveBSCAddress(masterKey *hdkeychain.ExtendedKey, index uint32) (string, error) {
	// m/44'
	purpose, err := masterKey.Derive(hdkeychain.HardenedKeyStart + uint32(config.BIP44Purpose))
	if err != nil {
		return "", fmt.Errorf("derive BSC purpose key: %w", err)
	}

	// m/44'/60'
	coin, err := purpose.Derive(hdkeychain.HardenedKeyStart + uint32(config.BSCCoinType))
	if err != nil {
		return "", fmt.Errorf("derive BSC coin key: %w", err)
	}

	// m/44'/60'/0'
	account, err := coin.Derive(hdkeychain.HardenedKeyStart + 0)
	if err != nil {
		return "", fmt.Errorf("derive BSC account key: %w", err)
	}

	// m/44'/60'/0'/0
	change, err := account.Derive(0)
	if err != nil {
		return "", fmt.Errorf("derive BSC change key: %w", err)
	}

	// m/44'/60'/0'/0/N
	child, err := change.Derive(index)
	if err != nil {
		return "", fmt.Errorf("derive BSC child key at index %d: %w", index, err)
	}

	privKey, err := child.ECPrivKey()
	if err != nil {
		return "", fmt.Errorf("get BSC private key at index %d: %w", index, err)
	}

	ecdsaPrivKey := privKey.ToECDSA()
	addr := crypto.PubkeyToAddress(ecdsaPrivKey.PublicKey)

	slog.Debug("derived BSC address",
		"index", index,
		"address", addr.Hex(),
	)

	return addr.Hex(), nil
}
