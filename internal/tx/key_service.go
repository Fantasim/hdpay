package tx

import (
	"context"
	"crypto/ecdsa"
	"fmt"
	"log/slog"

	"github.com/btcsuite/btcd/btcec/v2"
	"github.com/btcsuite/btcd/btcutil/hdkeychain"
	"github.com/btcsuite/btcd/chaincfg"
	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/crypto"

	"github.com/Fantasim/hdpay/internal/config"
	"github.com/Fantasim/hdpay/internal/wallet"
)

// KeyService derives private keys on demand from the mnemonic file.
// The mnemonic is read fresh each time to minimize time secrets spend in memory.
type KeyService struct {
	mnemonicFilePath string
	network          string
}

// NewKeyService creates a key derivation service.
// mnemonicFilePath is the path to the file containing the 24-word mnemonic.
func NewKeyService(mnemonicFilePath string, network string) *KeyService {
	slog.Info("key service created",
		"network", network,
		"mnemonicFileConfigured", mnemonicFilePath != "",
	)
	return &KeyService{
		mnemonicFilePath: mnemonicFilePath,
		network:          network,
	}
}

// DeriveBTCPrivateKey derives a BTC private key at the given address index.
// Path: m/84'/0'/0'/0/N (mainnet) or m/84'/1'/0'/0/N (testnet).
// The caller MUST zero the returned private key after use.
func (ks *KeyService) DeriveBTCPrivateKey(ctx context.Context, index uint32) (*btcec.PrivateKey, error) {
	if ks.mnemonicFilePath == "" {
		return nil, config.ErrMnemonicFileNotSet
	}

	slog.Debug("deriving BTC private key", "index", index, "network", ks.network)

	// Check context before potentially slow file I/O.
	if err := ctx.Err(); err != nil {
		return nil, fmt.Errorf("context cancelled before key derivation: %w", err)
	}

	masterKey, err := ks.deriveMasterKey()
	if err != nil {
		return nil, fmt.Errorf("derive master key for BTC key at index %d: %w", index, err)
	}

	net := wallet.NetworkParams(ks.network)
	privKey, err := deriveBTCPrivKeyAtIndex(masterKey, index, net)
	if err != nil {
		return nil, fmt.Errorf("%w: BTC index %d: %s", config.ErrKeyDerivation, index, err)
	}

	slog.Debug("BTC private key derived", "index", index)
	return privKey, nil
}

// DeriveBSCPrivateKey derives a BSC (EVM) ECDSA private key at the given address index.
// Path: m/44'/60'/0'/0/N (same coin type for mainnet and testnet).
// Returns the private key and the corresponding address.
// The caller MUST let the returned private key go out of scope after use (no zeroing needed for stdlib ecdsa).
func (ks *KeyService) DeriveBSCPrivateKey(ctx context.Context, index uint32) (*ecdsa.PrivateKey, common.Address, error) {
	if ks.mnemonicFilePath == "" {
		return nil, common.Address{}, config.ErrMnemonicFileNotSet
	}

	slog.Debug("deriving BSC private key", "index", index, "network", ks.network)

	if err := ctx.Err(); err != nil {
		return nil, common.Address{}, fmt.Errorf("context cancelled before BSC key derivation: %w", err)
	}

	masterKey, err := ks.deriveMasterKey()
	if err != nil {
		return nil, common.Address{}, fmt.Errorf("derive master key for BSC key at index %d: %w", index, err)
	}

	ecdsaKey, addr, err := deriveBSCPrivKeyAtIndex(masterKey, index)
	if err != nil {
		return nil, common.Address{}, fmt.Errorf("%w: BSC index %d: %s", config.ErrKeyDerivation, index, err)
	}

	slog.Debug("BSC private key derived", "index", index, "address", addr.Hex())
	return ecdsaKey, addr, nil
}

// deriveBSCPrivKeyAtIndex walks the BIP-44 path m/44'/60'/0'/0/N and returns the ECDSA private key + address.
func deriveBSCPrivKeyAtIndex(masterKey *hdkeychain.ExtendedKey, index uint32) (*ecdsa.PrivateKey, common.Address, error) {
	// m/44'
	purpose, err := masterKey.Derive(hdkeychain.HardenedKeyStart + uint32(config.BIP44Purpose))
	if err != nil {
		return nil, common.Address{}, fmt.Errorf("derive purpose key: %w", err)
	}

	// m/44'/60'
	coin, err := purpose.Derive(hdkeychain.HardenedKeyStart + uint32(config.BSCCoinType))
	if err != nil {
		return nil, common.Address{}, fmt.Errorf("derive coin key: %w", err)
	}

	// m/44'/60'/0'
	account, err := coin.Derive(hdkeychain.HardenedKeyStart + 0)
	if err != nil {
		return nil, common.Address{}, fmt.Errorf("derive account key: %w", err)
	}

	// m/44'/60'/0'/0
	change, err := account.Derive(0)
	if err != nil {
		return nil, common.Address{}, fmt.Errorf("derive change key: %w", err)
	}

	// m/44'/60'/0'/0/N
	child, err := change.Derive(index)
	if err != nil {
		return nil, common.Address{}, fmt.Errorf("derive child key at index %d: %w", index, err)
	}

	privKey, err := child.ECPrivKey()
	if err != nil {
		return nil, common.Address{}, fmt.Errorf("extract private key at index %d: %w", index, err)
	}

	ecdsaKey := privKey.ToECDSA()
	addr := crypto.PubkeyToAddress(ecdsaKey.PublicKey)

	return ecdsaKey, addr, nil
}

// deriveMasterKey reads the mnemonic file, converts to seed, and derives the BIP-32 master key.
func (ks *KeyService) deriveMasterKey() (*hdkeychain.ExtendedKey, error) {
	mnemonic, err := wallet.ReadMnemonicFromFile(ks.mnemonicFilePath)
	if err != nil {
		return nil, fmt.Errorf("read mnemonic: %w", err)
	}

	seed, err := wallet.MnemonicToSeed(mnemonic)
	if err != nil {
		return nil, fmt.Errorf("mnemonic to seed: %w", err)
	}

	net := wallet.NetworkParams(ks.network)
	masterKey, err := wallet.DeriveMasterKey(seed, net)
	if err != nil {
		return nil, fmt.Errorf("derive master key: %w", err)
	}

	return masterKey, nil
}

// deriveBTCPrivKeyAtIndex walks the BIP-84 path m/84'/coin'/0'/0/N and returns the private key.
func deriveBTCPrivKeyAtIndex(masterKey *hdkeychain.ExtendedKey, index uint32, net *chaincfg.Params) (*btcec.PrivateKey, error) {
	coinType := uint32(config.BTCCoinType)
	if net == &chaincfg.TestNet3Params {
		coinType = uint32(config.BTCTestCoinType)
	}

	// m/84'
	purpose, err := masterKey.Derive(hdkeychain.HardenedKeyStart + uint32(config.BIP84Purpose))
	if err != nil {
		return nil, fmt.Errorf("derive purpose key: %w", err)
	}

	// m/84'/coin'
	coin, err := purpose.Derive(hdkeychain.HardenedKeyStart + coinType)
	if err != nil {
		return nil, fmt.Errorf("derive coin key: %w", err)
	}

	// m/84'/coin'/0'
	account, err := coin.Derive(hdkeychain.HardenedKeyStart + 0)
	if err != nil {
		return nil, fmt.Errorf("derive account key: %w", err)
	}

	// m/84'/coin'/0'/0
	change, err := account.Derive(0)
	if err != nil {
		return nil, fmt.Errorf("derive change key: %w", err)
	}

	// m/84'/coin'/0'/0/N
	child, err := change.Derive(index)
	if err != nil {
		return nil, fmt.Errorf("derive child key at index %d: %w", index, err)
	}

	privKey, err := child.ECPrivKey()
	if err != nil {
		return nil, fmt.Errorf("extract private key at index %d: %w", index, err)
	}

	return privKey, nil
}
