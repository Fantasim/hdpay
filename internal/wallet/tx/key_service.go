package tx

import (
	"context"
	"crypto/ecdsa"
	"crypto/ed25519"
	"fmt"
	"log/slog"
	"os"

	"github.com/btcsuite/btcd/btcec/v2"
	"github.com/btcsuite/btcd/btcutil/hdkeychain"
	"github.com/btcsuite/btcd/chaincfg"
	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/crypto"

	"github.com/Fantasim/hdpay/internal/shared/config"
	"github.com/Fantasim/hdpay/internal/wallet/hd"
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

	net := hd.NetworkParams(ks.network)
	privKey, err := deriveBTCPrivKeyAtIndex(masterKey, index, net)
	if err != nil {
		return nil, fmt.Errorf("%w: BTC index %d: %s", config.ErrKeyDerivation, index, err)
	}

	slog.Debug("BTC private key derived", "index", index)
	return privKey, nil
}

// ZeroECDSAKey overwrites the ECDSA private key scalar with zeros.
// Not perfect (GC may have copied the big.Int), but reduces the exposure window.
// The caller should defer this immediately after obtaining the key.
func ZeroECDSAKey(key *ecdsa.PrivateKey) {
	if key == nil || key.D == nil {
		return
	}
	key.D.SetInt64(0)
}

// ZeroEd25519Key overwrites an ed25519 private key slice with zeros.
// The caller should defer this immediately after obtaining the key.
func ZeroEd25519Key(key ed25519.PrivateKey) {
	for i := range key {
		key[i] = 0
	}
}

// CheckMnemonicAvailable checks whether the mnemonic file is accessible (exists and is readable).
// This is a fast pre-flight check (os.Stat only, no read) for the external-disk workflow
// where the mnemonic lives on removable media that may not be plugged in.
func (ks *KeyService) CheckMnemonicAvailable() error {
	if ks.mnemonicFilePath == "" {
		return config.ErrMnemonicFileNotSet
	}

	if _, err := os.Stat(ks.mnemonicFilePath); err != nil {
		slog.Warn("mnemonic file not accessible",
			"path", ks.mnemonicFilePath,
			"error", err,
		)
		return fmt.Errorf("%w: %s", config.ErrMnemonicFileUnavailable, err)
	}

	slog.Debug("mnemonic file accessible", "path", ks.mnemonicFilePath)
	return nil
}

// DeriveBSCPrivateKey derives a BSC (EVM) ECDSA private key at the given address index.
// Path: m/44'/60'/0'/0/N (same coin type for mainnet and testnet).
// Returns the private key and the corresponding address.
// The caller MUST call ZeroECDSAKey(privKey) via defer after use.
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

	// Use go-ethereum's crypto.ToECDSA to ensure the curve matches geth's secp256k1 implementation.
	// btcec.PrivateKey.ToECDSA() sets curve to btcec.S256(), but geth's crypto.Sign() requires crypto.S256().
	ecdsaKey, err := crypto.ToECDSA(privKey.Serialize())
	if err != nil {
		return nil, common.Address{}, fmt.Errorf("convert to ECDSA key at index %d: %w", index, err)
	}
	addr := crypto.PubkeyToAddress(ecdsaKey.PublicKey)

	return ecdsaKey, addr, nil
}

// deriveMasterKey reads the mnemonic file, converts to seed, and derives the BIP-32 master key.
// Sensitive data (mnemonic bytes, seed) is mlocked to prevent swapping and zeroed after use.
func (ks *KeyService) deriveMasterKey() (*hdkeychain.ExtendedKey, error) {
	mnemonicBytes, err := hd.ReadMnemonicBytesFromFile(ks.mnemonicFilePath)
	if err != nil {
		return nil, fmt.Errorf("read mnemonic: %w", err)
	}
	hd.MlockBytes(mnemonicBytes)
	defer func() {
		hd.MunlockBytes(mnemonicBytes)
		hd.ZeroBytes(mnemonicBytes)
	}()

	seed, err := hd.MnemonicBytesToSeed(mnemonicBytes)
	if err != nil {
		return nil, fmt.Errorf("mnemonic to seed: %w", err)
	}
	hd.MlockBytes(seed)
	defer func() {
		hd.MunlockBytes(seed)
		hd.ZeroBytes(seed)
	}()

	net := hd.NetworkParams(ks.network)
	masterKey, err := hd.DeriveMasterKey(seed, net)
	if err != nil {
		return nil, fmt.Errorf("derive master key: %w", err)
	}

	return masterKey, nil
}

// DeriveSOLPrivateKey derives a SOL ed25519 private key at the given address index.
// Path: m/44'/501'/N'/0' (SLIP-10 hardened, all levels).
// Unlike BTC/BSC, SOL uses SLIP-10 from the raw BIP-39 seed, not BIP-32 extended keys.
// The caller MUST discard the returned private key after use.
func (ks *KeyService) DeriveSOLPrivateKey(ctx context.Context, index uint32) (ed25519.PrivateKey, error) {
	if ks.mnemonicFilePath == "" {
		return nil, config.ErrMnemonicFileNotSet
	}

	slog.Debug("deriving SOL private key", "index", index, "network", ks.network)

	if err := ctx.Err(); err != nil {
		return nil, fmt.Errorf("context cancelled before SOL key derivation: %w", err)
	}

	mnemonicBytes, err := hd.ReadMnemonicBytesFromFile(ks.mnemonicFilePath)
	if err != nil {
		return nil, fmt.Errorf("read mnemonic for SOL key at index %d: %w", index, err)
	}
	hd.MlockBytes(mnemonicBytes)
	defer func() {
		hd.MunlockBytes(mnemonicBytes)
		hd.ZeroBytes(mnemonicBytes)
	}()

	seed, err := hd.MnemonicBytesToSeed(mnemonicBytes)
	if err != nil {
		return nil, fmt.Errorf("mnemonic to seed for SOL key at index %d: %w", index, err)
	}
	hd.MlockBytes(seed)
	defer func() {
		hd.MunlockBytes(seed)
		hd.ZeroBytes(seed)
	}()

	privKey, err := hd.DeriveSOLPrivateKey(seed, index)
	if err != nil {
		return nil, fmt.Errorf("%w: SOL index %d: %s", config.ErrKeyDerivation, index, err)
	}

	slog.Debug("SOL private key derived", "index", index)
	return privKey, nil
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
