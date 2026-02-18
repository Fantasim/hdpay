package wallet

import (
	"fmt"
	"log/slog"
	"os"
	"strings"

	"github.com/btcsuite/btcd/btcutil/hdkeychain"
	"github.com/btcsuite/btcd/chaincfg"
	"github.com/tyler-smith/go-bip39"
)

// ValidateMnemonic validates a BIP-39 mnemonic phrase (must be 24 words).
func ValidateMnemonic(mnemonic string) error {
	if !bip39.IsMnemonicValid(mnemonic) {
		return fmt.Errorf("validate mnemonic: %w", ErrInvalidMnemonic)
	}

	words := strings.Fields(mnemonic)
	if len(words) != 24 {
		return fmt.Errorf("expected 24-word mnemonic, got %d words: %w", len(words), ErrInvalidMnemonic)
	}

	slog.Debug("mnemonic validated", "wordCount", len(words))
	return nil
}

// MnemonicToSeed converts a BIP-39 mnemonic to a 64-byte seed (empty passphrase).
func MnemonicToSeed(mnemonic string) ([]byte, error) {
	seed, err := bip39.NewSeedWithErrorChecking(mnemonic, "")
	if err != nil {
		return nil, fmt.Errorf("mnemonic to seed: %w", err)
	}

	slog.Debug("seed derived from mnemonic", "seedLen", len(seed))
	return seed, nil
}

// ReadMnemonicFromFile reads a mnemonic from a file, trims whitespace, and validates it.
func ReadMnemonicFromFile(path string) (string, error) {
	slog.Info("reading mnemonic from file", "path", path)

	data, err := os.ReadFile(path)
	if err != nil {
		return "", fmt.Errorf("read mnemonic file %q: %w", path, err)
	}

	mnemonic := strings.TrimSpace(string(data))
	if mnemonic == "" {
		return "", fmt.Errorf("mnemonic file %q is empty: %w", path, ErrInvalidMnemonic)
	}

	if err := ValidateMnemonic(mnemonic); err != nil {
		return "", fmt.Errorf("mnemonic file %q: %w", path, err)
	}

	slog.Info("mnemonic read and validated from file")
	return mnemonic, nil
}

// DeriveMasterKey derives a BIP-32 master extended key from a seed.
func DeriveMasterKey(seed []byte, net *chaincfg.Params) (*hdkeychain.ExtendedKey, error) {
	masterKey, err := hdkeychain.NewMaster(seed, net)
	if err != nil {
		return nil, fmt.Errorf("derive master key: %w", err)
	}

	slog.Debug("master key derived", "network", net.Name)
	return masterKey, nil
}

// NetworkParams returns the chaincfg.Params for the given network mode.
func NetworkParams(network string) *chaincfg.Params {
	switch network {
	case "testnet":
		return &chaincfg.TestNet3Params
	default:
		return &chaincfg.MainNetParams
	}
}
