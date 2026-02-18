package wallet

import (
	"fmt"
	"log/slog"
	"time"

	"github.com/btcsuite/btcd/btcutil/hdkeychain"
	"github.com/btcsuite/btcd/chaincfg"

	"github.com/Fantasim/hdpay/internal/models"
)

// ProgressCallback is called during address generation to report progress.
type ProgressCallback func(chain models.Chain, generated int, total int)

// GenerateBTCAddresses generates BTC Native SegWit addresses from index 0 to count-1.
func GenerateBTCAddresses(masterKey *hdkeychain.ExtendedKey, count int, net *chaincfg.Params, progress ProgressCallback) ([]models.Address, error) {
	slog.Info("generating BTC addresses", "count", count, "network", net.Name)
	start := time.Now()

	addresses := make([]models.Address, 0, count)
	for i := 0; i < count; i++ {
		addr, err := DeriveBTCAddress(masterKey, uint32(i), net)
		if err != nil {
			return nil, fmt.Errorf("generate BTC address at index %d: %w", i, err)
		}

		addresses = append(addresses, models.Address{
			Chain:        models.ChainBTC,
			AddressIndex: i,
			Address:      addr,
		})

		if progress != nil && (i+1)%10000 == 0 {
			progress(models.ChainBTC, i+1, count)
		}
	}

	slog.Info("BTC address generation complete",
		"count", len(addresses),
		"duration", time.Since(start).Round(time.Millisecond),
	)
	return addresses, nil
}

// GenerateBSCAddresses generates BSC/EVM addresses from index 0 to count-1.
func GenerateBSCAddresses(masterKey *hdkeychain.ExtendedKey, count int, progress ProgressCallback) ([]models.Address, error) {
	slog.Info("generating BSC addresses", "count", count)
	start := time.Now()

	addresses := make([]models.Address, 0, count)
	for i := 0; i < count; i++ {
		addr, err := DeriveBSCAddress(masterKey, uint32(i))
		if err != nil {
			return nil, fmt.Errorf("generate BSC address at index %d: %w", i, err)
		}

		addresses = append(addresses, models.Address{
			Chain:        models.ChainBSC,
			AddressIndex: i,
			Address:      addr,
		})

		if progress != nil && (i+1)%10000 == 0 {
			progress(models.ChainBSC, i+1, count)
		}
	}

	slog.Info("BSC address generation complete",
		"count", len(addresses),
		"duration", time.Since(start).Round(time.Millisecond),
	)
	return addresses, nil
}

// GenerateSOLAddresses generates SOL addresses from index 0 to count-1.
func GenerateSOLAddresses(seed []byte, count int, progress ProgressCallback) ([]models.Address, error) {
	slog.Info("generating SOL addresses", "count", count)
	start := time.Now()

	addresses := make([]models.Address, 0, count)
	for i := 0; i < count; i++ {
		addr, err := DeriveSOLAddress(seed, uint32(i))
		if err != nil {
			return nil, fmt.Errorf("generate SOL address at index %d: %w", i, err)
		}

		addresses = append(addresses, models.Address{
			Chain:        models.ChainSOL,
			AddressIndex: i,
			Address:      addr,
		})

		if progress != nil && (i+1)%10000 == 0 {
			progress(models.ChainSOL, i+1, count)
		}
	}

	slog.Info("SOL address generation complete",
		"count", len(addresses),
		"duration", time.Since(start).Round(time.Millisecond),
	)
	return addresses, nil
}
