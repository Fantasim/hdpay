package scanner

import (
	"context"
	"fmt"
	"log/slog"
	"sync"
	"time"

	"github.com/Fantasim/hdpay/internal/config"
	"github.com/Fantasim/hdpay/internal/db"
	"github.com/Fantasim/hdpay/internal/models"
)

// tokenConfigEntry describes a token to scan for a chain.
type tokenConfigEntry struct {
	Token    models.Token
	Contract string
}

// buildTokenConfig returns the token contract map based on network.
func buildTokenConfig(network string) map[models.Chain][]tokenConfigEntry {
	if network == string(models.NetworkTestnet) {
		return map[models.Chain][]tokenConfigEntry{
			models.ChainBTC: {},
			models.ChainBSC: {
				{Token: models.TokenUSDC, Contract: config.BSCTestnetUSDCContract},
				{Token: models.TokenUSDT, Contract: config.BSCTestnetUSDTContract},
			},
			models.ChainSOL: {
				{Token: models.TokenUSDC, Contract: config.SOLDevnetUSDCMint},
				{Token: models.TokenUSDT, Contract: config.SOLDevnetUSDTMint},
			},
		}
	}
	return map[models.Chain][]tokenConfigEntry{
		models.ChainBTC: {},
		models.ChainBSC: {
			{Token: models.TokenUSDC, Contract: config.BSCUSDCContract},
			{Token: models.TokenUSDT, Contract: config.BSCUSDTContract},
		},
		models.ChainSOL: {
			{Token: models.TokenUSDC, Contract: config.SOLUSDCMint},
			{Token: models.TokenUSDT, Contract: config.SOLUSDTMint},
		},
	}
}

// Scanner orchestrates balance scanning across chains.
type Scanner struct {
	db          *db.DB
	pools       map[models.Chain]*Pool
	hub         *SSEHub
	cfg         *config.Config
	cancels     map[models.Chain]context.CancelFunc
	tokenConfig map[models.Chain][]tokenConfigEntry
	mu          sync.Mutex
}

// New creates a new scanner orchestrator.
func New(database *db.DB, cfg *config.Config, hub *SSEHub) *Scanner {
	slog.Info("scanner orchestrator created")
	return &Scanner{
		db:          database,
		pools:       make(map[models.Chain]*Pool),
		hub:         hub,
		cfg:         cfg,
		cancels:     make(map[models.Chain]context.CancelFunc),
		tokenConfig: buildTokenConfig(cfg.Network),
	}
}

// RegisterPool adds a provider pool for a chain.
func (s *Scanner) RegisterPool(chain models.Chain, pool *Pool) {
	s.pools[chain] = pool
	slog.Info("scanner pool registered", "chain", chain)
}

// StartScan begins scanning a chain up to maxID.
func (s *Scanner) StartScan(ctx context.Context, chain models.Chain, maxID int) error {
	s.mu.Lock()
	if _, running := s.cancels[chain]; running {
		s.mu.Unlock()
		slog.Warn("scan already running", "chain", chain)
		return config.ErrScanAlreadyRunning
	}

	pool, ok := s.pools[chain]
	if !ok {
		s.mu.Unlock()
		return fmt.Errorf("no provider pool registered for %s", chain)
	}

	// Use background context with timeout — NOT the HTTP request context, which is
	// cancelled as soon as the handler returns a response.
	// The timeout prevents goroutine leaks if the scan runs indefinitely.
	scanCtx, cancel := context.WithTimeout(context.Background(), config.ScanContextTimeout)
	s.cancels[chain] = cancel
	s.mu.Unlock()

	slog.Info("scan starting",
		"chain", chain,
		"maxID", maxID,
	)

	// Check resume state.
	shouldResume, resumeIndex, err := s.db.ShouldResume(chain)
	if err != nil {
		slog.Warn("failed to check resume state",
			"chain", chain,
			"error", err,
		)
	}

	startIndex := 0
	if shouldResume {
		startIndex = resumeIndex
		slog.Info("resuming scan",
			"chain", chain,
			"fromIndex", startIndex,
		)
	}

	// Set initial scan state.
	now := time.Now().UTC().Format(time.RFC3339)
	if err := s.db.UpsertScanState(models.ScanState{
		Chain:            chain,
		LastScannedIndex: startIndex,
		MaxScanID:        maxID,
		Status:           db.ScanStatusScanning,
		StartedAt:        now,
	}); err != nil {
		s.removeScan(chain)
		return fmt.Errorf("set initial scan state: %w", err)
	}

	// Launch scan in goroutine.
	go s.runScan(scanCtx, chain, pool, startIndex, maxID)

	return nil
}

// StopScan cancels a running scan for a chain.
func (s *Scanner) StopScan(chain models.Chain) {
	s.mu.Lock()
	cancel, ok := s.cancels[chain]
	s.mu.Unlock()

	if !ok {
		slog.Warn("no scan running to stop", "chain", chain)
		return
	}

	slog.Info("stopping scan", "chain", chain)
	cancel()
}

// Status returns the current scan status for a chain from the DB.
func (s *Scanner) Status(chain models.Chain) *models.ScanState {
	state, err := s.db.GetScanState(chain)
	if err != nil {
		slog.Error("failed to get scan status",
			"chain", chain,
			"error", err,
		)
		return nil
	}
	return state
}

// IsRunning returns true if a scan is currently running for a chain.
func (s *Scanner) IsRunning(chain models.Chain) bool {
	s.mu.Lock()
	defer s.mu.Unlock()
	_, ok := s.cancels[chain]
	return ok
}

// removeScan removes the cancel function for a chain.
func (s *Scanner) removeScan(chain models.Chain) {
	s.mu.Lock()
	delete(s.cancels, chain)
	s.mu.Unlock()
}

// runScan performs the actual scanning work.
// V2: atomic DB writes, decoupled native/token, exponential backoff, token error SSE.
func (s *Scanner) runScan(ctx context.Context, chain models.Chain, pool *Pool, startIndex, maxID int) {
	// NOTE: removeScan is called inside finishScan AFTER all DB writes complete,
	// to prevent a race where a new scan starts while final state write is in flight.

	startTime := time.Now()
	batchSize := pool.MaxBatchSize()
	scanned := startIndex
	found := 0
	consecutivePoolFails := 0

	slog.Info("scan goroutine started",
		"chain", chain,
		"startIndex", startIndex,
		"maxID", maxID,
		"batchSize", batchSize,
	)

	for i := startIndex; i < maxID; i += batchSize {
		// Check for cancellation.
		select {
		case <-ctx.Done():
			slog.Info("scan cancelled",
				"chain", chain,
				"scannedUpTo", scanned,
			)
			s.db.UpsertScanState(models.ScanState{
				Chain:            chain,
				LastScannedIndex: scanned,
				MaxScanID:        maxID,
				Status:           db.ScanStatusScanning,
			})
			s.hub.Broadcast(Event{
				Type: "scan_error",
				Data: ScanErrorData{
					Chain:   string(chain),
					Error:   config.ErrorScanInterrupted,
					Message: "scan cancelled by user",
				},
			})
			s.removeScan(chain)
			return
		default:
		}

		// Exponential backoff on consecutive all-provider failures (B11).
		if consecutivePoolFails > 0 {
			backoff := pool.SuggestBackoff(consecutivePoolFails)
			slog.Warn("backing off before retry",
				"chain", chain,
				"consecutiveFailures", consecutivePoolFails,
				"backoff", backoff,
			)
			select {
			case <-time.After(backoff):
			case <-ctx.Done():
				slog.Info("scan cancelled during backoff", "chain", chain)
				s.db.UpsertScanState(models.ScanState{
					Chain:            chain,
					LastScannedIndex: scanned,
					MaxScanID:        maxID,
					Status:           db.ScanStatusScanning,
				})
				s.removeScan(chain)
				return
			}
		}

		// Calculate batch range.
		end := i + batchSize
		if end > maxID {
			end = maxID
		}
		count := end - i

		// Load addresses from DB.
		addresses, err := s.db.GetAddressesBatch(chain, i, count)
		if err != nil {
			slog.Error("failed to load address batch",
				"chain", chain,
				"start", i,
				"count", count,
				"error", err,
			)
			s.finishScan(chain, maxID, scanned, found, db.ScanStatusFailed, startTime, err)
			return
		}

		if len(addresses) == 0 {
			slog.Debug("no addresses in batch, skipping",
				"chain", chain,
				"start", i,
			)
			scanned = end
			continue
		}

		// --- Fetch native balances (B8: failure does not block tokens) ---
		var nativeBalances []models.Balance
		nativeFailed := false

		nativeResults, err := pool.FetchNativeBalances(ctx, addresses)
		if err != nil {
			if ctx.Err() != nil {
				slog.Info("scan cancelled during native fetch", "chain", chain)
				s.db.UpsertScanState(models.ScanState{
					Chain:            chain,
					LastScannedIndex: scanned,
					MaxScanID:        maxID,
					Status:           db.ScanStatusScanning,
				})
				s.removeScan(chain)
				return
			}
			slog.Error("native balance fetch failed, continuing to tokens",
				"chain", chain,
				"start", i,
				"error", err,
			)
			nativeFailed = true
			consecutivePoolFails++
		} else {
			consecutivePoolFails = 0
			nativeBalances = make([]models.Balance, 0, len(nativeResults))
			for _, r := range nativeResults {
				nativeBalances = append(nativeBalances, models.Balance{
					Chain:        chain,
					AddressIndex: r.AddressIndex,
					Token:        models.TokenNative,
					Balance:      r.Balance,
				})
				if r.Balance != "0" {
					found++
				}
			}
		}

		// --- Fetch token balances (B8: decoupled from native) ---
		var allTokenBalances []models.Balance
		for _, tc := range s.tokenConfig[chain] {
			if tc.Contract == "" {
				continue
			}
			tokenResults, err := pool.FetchTokenBalances(ctx, addresses, tc.Token, tc.Contract)
			if err != nil {
				if ctx.Err() != nil {
					s.removeScan(chain)
					return
				}
				// Token fetch failures are non-fatal — broadcast SSE event (B7).
				// Increment failure counter to trigger backoff, preventing
				// repeated hammering of rate-limited providers.
				consecutivePoolFails++
				slog.Warn("token balance fetch failed",
					"chain", chain,
					"token", tc.Token,
					"error", err,
				)
				s.hub.Broadcast(Event{
					Type: "scan_token_error",
					Data: ScanTokenErrorData{
						Chain:   string(chain),
						Token:   string(tc.Token),
						Error:   config.ErrorTokenScanFailed,
						Message: err.Error(),
					},
				})
				continue
			}

			for _, r := range tokenResults {
				allTokenBalances = append(allTokenBalances, models.Balance{
					Chain:        chain,
					AddressIndex: r.AddressIndex,
					Token:        tc.Token,
					Balance:      r.Balance,
				})
			}
		}

		// --- Atomic DB write: balances + scan state in one transaction (B4) ---
		allBalances := append(nativeBalances, allTokenBalances...)
		scanned = end

		if len(allBalances) > 0 || !nativeFailed {
			tx, err := s.db.BeginTx()
			if err != nil {
				slog.Error("failed to begin batch transaction",
					"chain", chain,
					"error", err,
				)
				s.finishScan(chain, maxID, scanned, found, db.ScanStatusFailed, startTime, err)
				return
			}

			if len(allBalances) > 0 {
				if err := s.db.UpsertBalanceBatchTx(tx, allBalances); err != nil {
					tx.Rollback()
					slog.Error("failed to store balances in tx",
						"chain", chain,
						"error", err,
					)
					s.finishScan(chain, maxID, scanned, found, db.ScanStatusFailed, startTime, err)
					return
				}
			}

			if err := s.db.UpsertScanStateTx(tx, models.ScanState{
				Chain:            chain,
				LastScannedIndex: scanned,
				MaxScanID:        maxID,
				Status:           db.ScanStatusScanning,
			}); err != nil {
				tx.Rollback()
				slog.Error("failed to update scan state in tx",
					"chain", chain,
					"error", err,
				)
				s.finishScan(chain, maxID, scanned, found, db.ScanStatusFailed, startTime, err)
				return
			}

			if err := tx.Commit(); err != nil {
				slog.Error("failed to commit batch transaction",
					"chain", chain,
					"error", err,
				)
				s.finishScan(chain, maxID, scanned, found, db.ScanStatusFailed, startTime, err)
				return
			}
		} else {
			// Native failed and no token balances — still update scan state.
			// Retry once on failure to prevent lost progress.
			if err := s.db.UpsertScanState(models.ScanState{
				Chain:            chain,
				LastScannedIndex: scanned,
				MaxScanID:        maxID,
				Status:           db.ScanStatusScanning,
			}); err != nil {
				slog.Warn("scan state write failed, retrying once",
					"chain", chain,
					"error", err,
				)
				// Retry after a brief pause.
				time.Sleep(100 * time.Millisecond)
				if retryErr := s.db.UpsertScanState(models.ScanState{
					Chain:            chain,
					LastScannedIndex: scanned,
					MaxScanID:        maxID,
					Status:           db.ScanStatusScanning,
				}); retryErr != nil {
					slog.Error("scan state write failed on retry, progress may be lost",
						"chain", chain,
						"scanned", scanned,
						"error", retryErr,
					)
				}
			}
		}

		// Abort after too many consecutive all-provider failures.
		if consecutivePoolFails >= config.MaxConsecutivePoolFails {
			slog.Error("too many consecutive provider failures, aborting scan",
				"chain", chain,
				"consecutiveFailures", consecutivePoolFails,
			)
			s.finishScan(chain, maxID, scanned, found, db.ScanStatusFailed, startTime,
				fmt.Errorf("aborted after %d consecutive all-provider failures", consecutivePoolFails))
			return
		}

		// Broadcast progress.
		elapsed := time.Since(startTime).Round(time.Second)
		s.hub.Broadcast(Event{
			Type: "scan_progress",
			Data: ScanProgressData{
				Chain:   string(chain),
				Scanned: scanned,
				Total:   maxID,
				Found:   found,
				Elapsed: elapsed.String(),
			},
		})

		slog.Info("scan batch complete",
			"chain", chain,
			"scanned", scanned,
			"total", maxID,
			"found", found,
			"nativeFailed", nativeFailed,
			"elapsed", elapsed,
		)
	}

	// Scan complete.
	s.finishScan(chain, maxID, scanned, found, db.ScanStatusCompleted, startTime, nil)
}

// finishScan updates state and broadcasts completion/error events.
func (s *Scanner) finishScan(chain models.Chain, maxID, scanned, found int, status string, startTime time.Time, scanErr error) {
	duration := time.Since(startTime).Round(time.Second)

	if err := s.db.UpsertScanState(models.ScanState{
		Chain:            chain,
		LastScannedIndex: scanned,
		MaxScanID:        maxID,
		Status:           status,
	}); err != nil {
		slog.Error("failed to update final scan state",
			"chain", chain,
			"error", err,
		)
	}

	if status == db.ScanStatusCompleted {
		slog.Info("scan completed",
			"chain", chain,
			"scanned", scanned,
			"found", found,
			"duration", duration,
		)
		s.hub.Broadcast(Event{
			Type: "scan_complete",
			Data: ScanCompleteData{
				Chain:    string(chain),
				Scanned: scanned,
				Found:   found,
				Duration: duration.String(),
			},
		})
	} else {
		errMsg := "unknown error"
		if scanErr != nil {
			errMsg = scanErr.Error()
		}
		slog.Error("scan failed",
			"chain", chain,
			"scanned", scanned,
			"found", found,
			"error", errMsg,
			"duration", duration,
		)
		s.hub.Broadcast(Event{
			Type: "scan_error",
			Data: ScanErrorData{
				Chain:   string(chain),
				Error:   config.ErrorScanFailed,
				Message: errMsg,
			},
		})
	}

	// Remove scan from active set AFTER all DB writes and broadcasts are complete.
	// This prevents a race where StartScan on the same chain proceeds while
	// finishScan is still writing final state.
	s.removeScan(chain)
}
