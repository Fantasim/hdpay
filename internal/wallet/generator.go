package wallet

import (
	"fmt"
	"log/slog"
	"runtime"
	"sync"
	"sync/atomic"
	"time"

	"github.com/btcsuite/btcd/btcutil/hdkeychain"
	"github.com/btcsuite/btcd/chaincfg"

	"github.com/Fantasim/hdpay/internal/models"
)

// ProgressCallback is called during address generation to report progress.
type ProgressCallback func(chain models.Chain, generated int, total int)

// GenerateBTCAddresses generates BTC Native SegWit addresses from index 0 to count-1.
// Uses runtime.NumCPU() parallel workers with pre-derived parent key.
func GenerateBTCAddresses(masterKey *hdkeychain.ExtendedKey, count int, net *chaincfg.Params, progress ProgressCallback) ([]models.Address, error) {
	numWorkers := runtime.NumCPU()
	slog.Info("generating BTC addresses",
		"count", count,
		"network", net.Name,
		"workers", numWorkers,
	)
	start := time.Now()

	// Pre-derive parent key to m/84'/coin'/0'/0 — done once instead of count times.
	parentKey, err := DeriveBTCParentKey(masterKey, net)
	if err != nil {
		return nil, fmt.Errorf("derive BTC parent key: %w", err)
	}

	addresses := make([]models.Address, count)
	var done atomic.Int64
	var firstErr atomic.Value

	var wg sync.WaitGroup
	chunkSize := (count + numWorkers - 1) / numWorkers

	for w := 0; w < numWorkers; w++ {
		chunkStart := w * chunkSize
		chunkEnd := chunkStart + chunkSize
		if chunkEnd > count {
			chunkEnd = count
		}
		if chunkStart >= count {
			break
		}

		wg.Add(1)
		go func(from, to int) {
			defer wg.Done()
			for i := from; i < to; i++ {
				// Stop early if another worker hit an error.
				if firstErr.Load() != nil {
					return
				}

				addr, err := DeriveBTCAddressFromParent(parentKey, uint32(i), net)
				if err != nil {
					firstErr.CompareAndSwap(nil, fmt.Errorf("generate BTC address at index %d: %w", i, err))
					return
				}

				addresses[i] = models.Address{
					Chain:        models.ChainBTC,
					AddressIndex: i,
					Address:      addr,
				}

				if n := done.Add(1); progress != nil && n%10000 == 0 {
					progress(models.ChainBTC, int(n), count)
				}
			}
		}(chunkStart, chunkEnd)
	}

	wg.Wait()

	if errVal := firstErr.Load(); errVal != nil {
		return nil, errVal.(error)
	}

	slog.Info("BTC address generation complete",
		"count", len(addresses),
		"workers", numWorkers,
		"duration", time.Since(start).Round(time.Millisecond),
	)
	return addresses, nil
}

// GenerateBSCAddresses generates BSC/EVM addresses from index 0 to count-1.
// Uses runtime.NumCPU() parallel workers with pre-derived parent key.
func GenerateBSCAddresses(masterKey *hdkeychain.ExtendedKey, count int, progress ProgressCallback) ([]models.Address, error) {
	numWorkers := runtime.NumCPU()
	slog.Info("generating BSC addresses",
		"count", count,
		"workers", numWorkers,
	)
	start := time.Now()

	// Pre-derive parent key to m/44'/60'/0'/0 — done once instead of count times.
	parentKey, err := DeriveBSCParentKey(masterKey)
	if err != nil {
		return nil, fmt.Errorf("derive BSC parent key: %w", err)
	}

	addresses := make([]models.Address, count)
	var done atomic.Int64
	var firstErr atomic.Value

	var wg sync.WaitGroup
	chunkSize := (count + numWorkers - 1) / numWorkers

	for w := 0; w < numWorkers; w++ {
		chunkStart := w * chunkSize
		chunkEnd := chunkStart + chunkSize
		if chunkEnd > count {
			chunkEnd = count
		}
		if chunkStart >= count {
			break
		}

		wg.Add(1)
		go func(from, to int) {
			defer wg.Done()
			for i := from; i < to; i++ {
				if firstErr.Load() != nil {
					return
				}

				addr, err := DeriveBSCAddressFromParent(parentKey, uint32(i))
				if err != nil {
					firstErr.CompareAndSwap(nil, fmt.Errorf("generate BSC address at index %d: %w", i, err))
					return
				}

				addresses[i] = models.Address{
					Chain:        models.ChainBSC,
					AddressIndex: i,
					Address:      addr,
				}

				if n := done.Add(1); progress != nil && n%10000 == 0 {
					progress(models.ChainBSC, int(n), count)
				}
			}
		}(chunkStart, chunkEnd)
	}

	wg.Wait()

	if errVal := firstErr.Load(); errVal != nil {
		return nil, errVal.(error)
	}

	slog.Info("BSC address generation complete",
		"count", len(addresses),
		"workers", numWorkers,
		"duration", time.Since(start).Round(time.Millisecond),
	)
	return addresses, nil
}

// GenerateSOLAddresses generates SOL addresses from index 0 to count-1.
// Uses runtime.NumCPU() parallel workers with pre-derived parent key.
func GenerateSOLAddresses(seed []byte, count int, progress ProgressCallback) ([]models.Address, error) {
	numWorkers := runtime.NumCPU()
	slog.Info("generating SOL addresses",
		"count", count,
		"workers", numWorkers,
	)
	start := time.Now()

	// Pre-derive parent key to m/44'/501' — done once instead of count times.
	parentKey, err := DeriveSOLParentKey(seed)
	if err != nil {
		return nil, fmt.Errorf("derive SOL parent key: %w", err)
	}

	addresses := make([]models.Address, count)
	var done atomic.Int64
	var firstErr atomic.Value

	var wg sync.WaitGroup
	chunkSize := (count + numWorkers - 1) / numWorkers

	for w := 0; w < numWorkers; w++ {
		chunkStart := w * chunkSize
		chunkEnd := chunkStart + chunkSize
		if chunkEnd > count {
			chunkEnd = count
		}
		if chunkStart >= count {
			break
		}

		wg.Add(1)
		go func(from, to int) {
			defer wg.Done()
			for i := from; i < to; i++ {
				if firstErr.Load() != nil {
					return
				}

				addr, err := DeriveSOLAddressFromParent(parentKey, uint32(i))
				if err != nil {
					firstErr.CompareAndSwap(nil, fmt.Errorf("generate SOL address at index %d: %w", i, err))
					return
				}

				addresses[i] = models.Address{
					Chain:        models.ChainSOL,
					AddressIndex: i,
					Address:      addr,
				}

				if n := done.Add(1); progress != nil && n%10000 == 0 {
					progress(models.ChainSOL, int(n), count)
				}
			}
		}(chunkStart, chunkEnd)
	}

	wg.Wait()

	if errVal := firstErr.Load(); errVal != nil {
		return nil, errVal.(error)
	}

	slog.Info("SOL address generation complete",
		"count", len(addresses),
		"workers", numWorkers,
		"duration", time.Since(start).Round(time.Millisecond),
	)
	return addresses, nil
}
