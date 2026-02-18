package wallet

import (
	"fmt"
	"log/slog"

	"github.com/btcsuite/btcd/btcutil"
	"github.com/btcsuite/btcd/btcutil/hdkeychain"
	"github.com/btcsuite/btcd/chaincfg"

	"github.com/Fantasim/hdpay/internal/config"
)

// DeriveBTCAddress derives a BTC Native SegWit (bech32) address at the given index.
// Path: m/84'/0'/0'/0/N (mainnet) or m/84'/1'/0'/0/N (testnet) per BIP-84.
func DeriveBTCAddress(masterKey *hdkeychain.ExtendedKey, index uint32, net *chaincfg.Params) (string, error) {
	coinType := uint32(config.BTCCoinType)
	if net == &chaincfg.TestNet3Params {
		coinType = uint32(config.BTCTestCoinType)
	}

	// m/84' (BIP-84 for Native SegWit bech32)
	purpose, err := masterKey.Derive(hdkeychain.HardenedKeyStart + uint32(config.BIP84Purpose))
	if err != nil {
		return "", fmt.Errorf("derive BTC purpose key: %w", err)
	}

	// m/44'/coin'
	coin, err := purpose.Derive(hdkeychain.HardenedKeyStart + coinType)
	if err != nil {
		return "", fmt.Errorf("derive BTC coin key: %w", err)
	}

	// m/44'/coin'/0'
	account, err := coin.Derive(hdkeychain.HardenedKeyStart + 0)
	if err != nil {
		return "", fmt.Errorf("derive BTC account key: %w", err)
	}

	// m/44'/coin'/0'/0
	change, err := account.Derive(0)
	if err != nil {
		return "", fmt.Errorf("derive BTC change key: %w", err)
	}

	// m/44'/coin'/0'/0/N
	child, err := change.Derive(index)
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

	slog.Debug("derived BTC address",
		"index", index,
		"address", addr.EncodeAddress(),
		"network", net.Name,
	)

	return addr.EncodeAddress(), nil
}
