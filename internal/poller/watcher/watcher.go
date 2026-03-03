package watcher

import (
	"context"
	"fmt"
	"log/slog"
	"sync"
	"time"

	"github.com/Fantasim/hdpay/internal/poller/config"
	"github.com/Fantasim/hdpay/internal/poller/models"
	"github.com/Fantasim/hdpay/internal/poller/points"
	"github.com/Fantasim/hdpay/internal/poller/pollerdb"
	"github.com/Fantasim/hdpay/internal/poller/provider"
	"github.com/google/uuid"
)

// Watcher is the central orchestrator for all watch activity.
// It manages one goroutine per active watch and coordinates the full lifecycle:
// creation -> polling -> tx detection -> confirmation -> completion/expiry.
type Watcher struct {
	db         *pollerdb.DB
	providers  map[string]*provider.ProviderSet // keyed by chain: "BTC", "BSC", "SOL"
	pricer     *points.Pricer
	calculator *points.PointsCalculator
	cfg        *config.Config

	mu      sync.Mutex
	watches map[string]context.CancelFunc // watchID -> cancel
	wg      sync.WaitGroup               // tracks active goroutines for graceful shutdown

	// Orphan recovery: periodic background goroutine that rechecks PENDING txs
	// from expired/completed watches. Cancelled by Stop().
	orphanCancel context.CancelFunc

	// Runtime-mutable settings (loaded from config, editable from dashboard).
	settingsMu          sync.RWMutex
	maxActiveWatches    int
	defaultWatchTimeout int // minutes
}

// NewWatcher creates a new Watcher with the given dependencies.
func NewWatcher(
	db *pollerdb.DB,
	providers map[string]*provider.ProviderSet,
	pricer *points.Pricer,
	calculator *points.PointsCalculator,
	cfg *config.Config,
) *Watcher {
	w := &Watcher{
		db:                  db,
		providers:           providers,
		pricer:              pricer,
		calculator:          calculator,
		cfg:                 cfg,
		watches:             make(map[string]context.CancelFunc),
		maxActiveWatches:    cfg.MaxActiveWatches,
		defaultWatchTimeout: cfg.DefaultWatchTimeout,
	}

	slog.Info("watcher initialized",
		"maxActiveWatches", w.maxActiveWatches,
		"defaultWatchTimeout", w.defaultWatchTimeout,
		"providerChains", len(providers),
	)
	return w
}

// ActiveCount returns the number of currently active watch goroutines.
func (w *Watcher) ActiveCount() int {
	w.mu.Lock()
	defer w.mu.Unlock()
	return len(w.watches)
}

// MaxActiveWatches returns the current max active watches setting.
func (w *Watcher) MaxActiveWatches() int {
	w.settingsMu.RLock()
	defer w.settingsMu.RUnlock()
	return w.maxActiveWatches
}

// SetMaxActiveWatches updates the max active watches setting at runtime.
func (w *Watcher) SetMaxActiveWatches(n int) {
	w.settingsMu.Lock()
	defer w.settingsMu.Unlock()
	w.maxActiveWatches = n
	slog.Info("max active watches updated", "maxActiveWatches", n)
}

// DefaultWatchTimeout returns the current default watch timeout in minutes.
func (w *Watcher) DefaultWatchTimeout() int {
	w.settingsMu.RLock()
	defer w.settingsMu.RUnlock()
	return w.defaultWatchTimeout
}

// SetDefaultWatchTimeout updates the default watch timeout setting at runtime.
func (w *Watcher) SetDefaultWatchTimeout(n int) {
	w.settingsMu.Lock()
	defer w.settingsMu.Unlock()
	w.defaultWatchTimeout = n
	slog.Info("default watch timeout updated", "defaultWatchTimeoutMin", n)
}

// CreateWatch creates a new watch for the given address on the specified chain.
// It validates inputs, checks for duplicates and limits, creates the DB record,
// and spawns a poll goroutine.
//
// The limit check, duplicate check, and map insertion are all performed under
// w.mu to prevent TOCTOU races from concurrent CreateWatch calls.
func (w *Watcher) CreateWatch(chain, address string, timeoutMinutes int) (*models.Watch, error) {
	// Validate chain (no lock needed — immutable).
	if chain != "BTC" && chain != "BSC" && chain != "SOL" {
		return nil, fmt.Errorf("%s: %s", config.ErrorInvalidChain, chain)
	}

	// Check provider set exists for chain (no lock needed — immutable after init).
	ps, ok := w.providers[chain]
	if !ok {
		return nil, fmt.Errorf("%s: no providers configured for %s", config.ErrorProviderUnavailable, chain)
	}

	// Read max watches before acquiring w.mu to avoid nested lock with settingsMu.
	maxWatches := w.MaxActiveWatches()

	// Hold w.mu for the entire check-and-insert block to prevent TOCTOU races:
	// two concurrent CreateWatch calls could both pass the limit/duplicate checks
	// without this serialization.
	w.mu.Lock()

	// Check active watch limit under lock.
	if len(w.watches) >= maxWatches {
		w.mu.Unlock()
		return nil, fmt.Errorf("%s: limit is %d", config.ErrorMaxWatches, maxWatches)
	}

	// Check for duplicate active watch on this address under lock.
	existing, err := w.db.GetActiveWatchByAddress(address)
	if err != nil {
		w.mu.Unlock()
		return nil, fmt.Errorf("failed to check existing watch: %w", err)
	}
	if existing != nil {
		w.mu.Unlock()
		return nil, fmt.Errorf("%s: address %s already watched by %s", config.ErrorAlreadyWatching, address, existing.ID)
	}

	// Generate watch ID.
	watchID := uuid.New().String()

	// Calculate timestamps.
	now := time.Now().UTC()
	startedAt := now.Format(time.RFC3339)
	expiresAt := now.Add(time.Duration(timeoutMinutes) * time.Minute).Format(time.RFC3339)

	watch := &models.Watch{
		ID:        watchID,
		Chain:     chain,
		Address:   address,
		Status:    models.WatchStatusActive,
		StartedAt: startedAt,
		ExpiresAt: expiresAt,
	}

	// Insert into DB.
	if err := w.db.CreateWatch(watch); err != nil {
		w.mu.Unlock()
		return nil, fmt.Errorf("failed to create watch: %w", err)
	}

	// Create context from Background with timeout.
	// Add a grace period so the goroutine can clean up before the context dies.
	ctxTimeout := time.Duration(timeoutMinutes)*time.Minute + config.WatchContextGracePeriod
	watchCtx, cancel := context.WithTimeout(context.Background(), ctxTimeout)

	// Store cancel func and spawn goroutine (still under lock).
	w.watches[watchID] = cancel
	w.mu.Unlock()

	w.wg.Add(1)
	go w.runWatch(watchCtx, watch, ps)

	slog.Info("watch created and goroutine started",
		"watchID", watchID,
		"chain", chain,
		"address", address,
		"timeoutMinutes", timeoutMinutes,
		"expiresAt", expiresAt,
	)

	return watch, nil
}

// StartOrphanRecovery launches a background goroutine that periodically rechecks
// PENDING transactions from expired/completed/cancelled watches. Without this,
// pending txs are only rechecked on poller restart (via RunRecovery).
// Call after RunRecovery and before accepting new watches.
func (w *Watcher) StartOrphanRecovery() {
	ctx, cancel := context.WithCancel(context.Background())
	w.orphanCancel = cancel
	go w.runOrphanRecovery(ctx)
	slog.Info("orphan recovery goroutine started", "interval", config.OrphanRecoveryInterval)
}

// runOrphanRecovery is the background loop that periodically rechecks orphaned
// pending transactions. It queries for PENDING txs whose watch is no longer
// ACTIVE, then attempts to confirm each one using the chain provider.
func (w *Watcher) runOrphanRecovery(ctx context.Context) {
	ticker := time.NewTicker(config.OrphanRecoveryInterval)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			slog.Info("orphan recovery goroutine stopped")
			return
		case <-ticker.C:
			w.checkOrphanedPending(ctx)
		}
	}
}

// checkOrphanedPending runs a single pass over orphaned pending transactions.
func (w *Watcher) checkOrphanedPending(ctx context.Context) {
	orphaned, err := w.db.ListOrphanedPending()
	if err != nil {
		slog.Error("orphan recovery: failed to list orphaned pending txs", "error", err)
		return
	}

	if len(orphaned) == 0 {
		return
	}

	slog.Info("orphan recovery: checking orphaned pending txs", "count", len(orphaned))

	resolved := 0
	for _, tx := range orphaned {
		if ctx.Err() != nil {
			return
		}

		ps, ok := w.providers[tx.Chain]
		if !ok {
			continue
		}

		// Smart confirmation scheduling: skip if not enough time has elapsed.
		detectedAt, parseErr := time.Parse(time.RFC3339, tx.DetectedAt)
		if parseErr == nil {
			minWait := confirmationMinWait(tx.Chain)
			elapsed := time.Since(detectedAt)
			if elapsed < minWait {
				slog.Debug("orphan recovery: skipping confirmation check, too early",
					"txHash", tx.TxHash,
					"chain", tx.Chain,
					"elapsed", elapsed,
					"minWait", minWait,
				)
				continue
			}
		}

		// Extract base signature for SOL composite tx_hash.
		checkHash := tx.TxHash
		if tx.Chain == "SOL" {
			checkHash = extractBaseSignature(tx.TxHash)
		}

		var blockNum int64
		if tx.BlockNumber != nil {
			blockNum = *tx.BlockNumber
		}

		confirmed, confirmations, checkErr := ps.ExecuteConfirmation(ctx, checkHash, blockNum)
		if checkErr != nil {
			slog.Debug("orphan recovery: confirmation check failed",
				"txHash", tx.TxHash,
				"chain", tx.Chain,
				"error", checkErr,
			)
			continue
		}

		if !confirmed {
			continue
		}

		// Confirmed — fetch price and finalize.
		usdPrice, priceErr := w.pricer.GetTokenPrice(ctx, tx.Token)
		if priceErr != nil {
			// Try cached price as fallback.
			cachedPrice, cacheErr := w.pricer.GetCachedPrice(tx.Token)
			if cacheErr != nil {
				slog.Debug("orphan recovery: price unavailable, will retry next interval",
					"txHash", tx.TxHash,
					"token", tx.Token,
				)
				continue
			}
			usdPrice = cachedPrice
		}

		amountFloat := parseAmountHuman(tx.AmountHuman)
		usdValue := amountFloat * usdPrice
		calcResult := w.calculator.Calculate(usdValue)
		confirmedAt := time.Now().UTC().Format(time.RFC3339)

		if err := w.db.ConfirmTxAndMovePoints(
			tx.TxHash, confirmations, tx.BlockNumber, confirmedAt,
			usdValue, usdPrice, calcResult.TierIndex, calcResult.Multiplier, calcResult.Points,
			tx.Address, tx.Chain, tx.Points,
		); err != nil {
			slog.Error("orphan recovery: failed to confirm tx",
				"txHash", tx.TxHash,
				"error", err,
			)
			continue
		}

		resolved++
		slog.Info("orphan recovery: pending tx confirmed",
			"txHash", tx.TxHash,
			"chain", tx.Chain,
			"usdValue", usdValue,
			"points", calcResult.Points,
		)
	}

	if resolved > 0 {
		slog.Info("orphan recovery: pass complete",
			"checked", len(orphaned),
			"resolved", resolved,
		)
	}
}

// CancelWatch cancels an active watch by its ID.
// The goroutine's cleanup path handles the DB status update to CANCELLED.
func (w *Watcher) CancelWatch(watchID string) error {
	w.mu.Lock()
	cancel, running := w.watches[watchID]
	w.mu.Unlock()

	if running {
		slog.Info("cancelling watch", "watchID", watchID)
		cancel()
		return nil
	}

	// Not in active map — check DB to give a helpful error.
	watch, err := w.db.GetWatch(watchID)
	if err != nil {
		return fmt.Errorf("failed to look up watch: %w", err)
	}
	if watch == nil {
		return fmt.Errorf("%s: %s", config.ErrorWatchNotFound, watchID)
	}

	return fmt.Errorf("%s: watch %s has status %s", config.ErrorWatchExpired, watchID, watch.Status)
}

// Stop gracefully shuts down the watcher. It cancels all active watch contexts,
// waits for goroutines to finish (with timeout), and expires any remaining active
// watches in the DB.
func (w *Watcher) Stop() {
	slog.Info("watcher stopping, cancelling all active watches", "activeCount", w.ActiveCount())

	// Stop orphan recovery goroutine first.
	if w.orphanCancel != nil {
		w.orphanCancel()
	}

	// Cancel all active watch contexts.
	w.mu.Lock()
	for watchID, cancel := range w.watches {
		slog.Debug("cancelling watch context for shutdown", "watchID", watchID)
		cancel()
	}
	w.mu.Unlock()

	// Wait for all goroutines to finish with timeout.
	done := make(chan struct{})
	go func() {
		w.wg.Wait()
		close(done)
	}()

	select {
	case <-done:
		slog.Info("all watch goroutines stopped cleanly")
	case <-time.After(config.ShutdownTimeout):
		slog.Warn("watcher shutdown timed out, some goroutines may still be running",
			"timeout", config.ShutdownTimeout,
		)
		// Only expire remaining active watches if goroutines didn't finish in time.
		// This prevents overwriting a goroutine's final COMPLETED status with EXPIRED.
		expired, err := w.db.ExpireAllActiveWatches()
		if err != nil {
			slog.Error("failed to expire remaining active watches", "error", err)
		} else if expired > 0 {
			slog.Warn("expired remaining active watches during shutdown", "count", expired)
		}
	}

	slog.Info("watcher stopped")
}

// removeWatch removes a watch from the active map. Called by the goroutine
// during its cleanup. Must be called AFTER all DB writes are complete.
func (w *Watcher) removeWatch(watchID string) {
	w.mu.Lock()
	delete(w.watches, watchID)
	w.mu.Unlock()

	slog.Debug("watch removed from active map", "watchID", watchID)
}
