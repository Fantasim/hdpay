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
func (w *Watcher) CreateWatch(chain, address string, timeoutMinutes int) (*models.Watch, error) {
	// Validate chain.
	if chain != "BTC" && chain != "BSC" && chain != "SOL" {
		return nil, fmt.Errorf("%s: %s", config.ErrorInvalidChain, chain)
	}

	// Check provider set exists for chain.
	ps, ok := w.providers[chain]
	if !ok {
		return nil, fmt.Errorf("%s: no providers configured for %s", config.ErrorProviderUnavailable, chain)
	}

	// Check active watch limit.
	maxWatches := w.MaxActiveWatches()
	if w.ActiveCount() >= maxWatches {
		return nil, fmt.Errorf("%s: limit is %d", config.ErrorMaxWatches, maxWatches)
	}

	// Check for duplicate active watch on this address.
	existing, err := w.db.GetActiveWatchByAddress(address)
	if err != nil {
		return nil, fmt.Errorf("failed to check existing watch: %w", err)
	}
	if existing != nil {
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
		return nil, fmt.Errorf("failed to create watch: %w", err)
	}

	// Create context from Background with timeout.
	// Add a grace period so the goroutine can clean up before the context dies.
	ctxTimeout := time.Duration(timeoutMinutes)*time.Minute + config.WatchContextGracePeriod
	watchCtx, cancel := context.WithTimeout(context.Background(), ctxTimeout)

	// Store cancel func and spawn goroutine.
	w.mu.Lock()
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
	}

	// Expire any remaining active watches in DB (belt-and-suspenders).
	expired, err := w.db.ExpireAllActiveWatches()
	if err != nil {
		slog.Error("failed to expire remaining active watches", "error", err)
	} else if expired > 0 {
		slog.Warn("expired remaining active watches during shutdown", "count", expired)
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
