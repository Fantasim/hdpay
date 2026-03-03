package watcher

import (
	"context"
	"fmt"
	"log/slog"
	"strconv"
	"strings"
	"time"

	"github.com/Fantasim/hdpay/internal/poller/config"
	"github.com/Fantasim/hdpay/internal/poller/models"
	"github.com/Fantasim/hdpay/internal/poller/provider"
)

// runWatch is the main poll loop goroutine for a single watch.
// One goroutine per active watch. It handles the full lifecycle:
// fetch new txs -> process -> recheck pending -> check stop conditions.
func (w *Watcher) runWatch(ctx context.Context, watch *models.Watch, ps *provider.ProviderSet) {
	defer w.wg.Done()
	defer w.removeWatch(watch.ID)
	defer ps.CleanupAddress(watch.Address) // release per-address state (e.g. BSC lastKnownBal)

	// Resolve cutoff timestamp.
	cutoff := w.resolveCutoff(ctx, watch.Address, w.cfg.StartDate)

	// Determine poll interval from chain.
	interval := pollInterval(watch.Chain)

	slog.Info("watch goroutine started",
		"watchID", watch.ID,
		"chain", watch.Chain,
		"address", watch.Address,
		"cutoffUnix", cutoff,
		"pollInterval", interval,
	)

	ticker := time.NewTicker(interval)
	defer ticker.Stop()

	pollCount := watch.PollCount

	// Adaptive polling: back off interval after consecutive empty ticks to reduce API waste.
	consecutiveEmpty := 0
	currentMultiplier := 1

	// Per-watch error tracking.
	consecutiveErrors := 0

	for {
		select {
		case <-ctx.Done():
			// Context cancelled — determine reason.
			status := w.determineStopStatus(ctx, watch)
			completedAt := time.Now().UTC().Format(time.RFC3339)
			if err := w.db.UpdateWatchStatus(watch.ID, status, &completedAt); err != nil {
				slog.Error("failed to update watch status on context done",
					"watchID", watch.ID,
					"status", status,
					"error", err,
				)
			}
			slog.Info("watch goroutine exiting",
				"watchID", watch.ID,
				"status", status,
				"reason", ctx.Err(),
				"pollCount", pollCount,
			)
			return

		case <-ticker.C:
			pollCount++
			slog.Debug("watch tick",
				"watchID", watch.ID,
				"chain", watch.Chain,
				"pollCount", pollCount,
				"intervalMultiplier", currentMultiplier,
			)

			// A. Fetch new transactions.
			newTxs, fetchErr := ps.ExecuteFetch(ctx, watch.Address, cutoff)
			pollResult := "ok"
			if fetchErr != nil {
				// Check if context was cancelled during fetch.
				if ctx.Err() != nil {
					continue // will be caught by select on next iteration
				}
				pollResult = fmt.Sprintf("fetch_error: %v", fetchErr)
				slog.Warn("fetch failed, will retry next tick",
					"watchID", watch.ID,
					"chain", watch.Chain,
					"error", fetchErr,
				)
				// Log to system_errors.
				w.logSystemError(config.ErrorSeverityWarn, config.ErrorCategoryProvider,
					fmt.Sprintf("fetch failed for watch %s on %s", watch.ID, watch.Chain),
					fetchErr.Error(),
				)

				// Track consecutive errors.
				consecutiveErrors++
				if consecutiveErrors >= config.WatchMaxConsecutiveErrors {
					w.logSystemError(config.ErrorSeverityError, config.ErrorCategoryWatcher,
						fmt.Sprintf("watch %s: %d consecutive fetch failures", watch.ID, consecutiveErrors),
						fetchErr.Error(),
					)
				}
			} else {
				// Reset error counter on successful fetch.
				if consecutiveErrors > 0 {
					slog.Debug("watch fetch recovered after errors",
						"watchID", watch.ID,
						"previousErrors", consecutiveErrors,
					)
					consecutiveErrors = 0
				}
			}

			// B. Process each new transaction.
			newDetected := 0
			if fetchErr == nil && len(newTxs) > 0 {
				for _, raw := range newTxs {
					if err := w.processTransaction(ctx, watch, raw); err != nil {
						slog.Error("failed to process transaction",
							"watchID", watch.ID,
							"txHash", raw.TxHash,
							"error", err,
						)
						continue
					}
					newDetected++
					// Update cutoff to the most recent tx blocktime.
					if raw.BlockTime > cutoff {
						cutoff = raw.BlockTime
					}
				}
				if newDetected > 0 {
					pollResult = fmt.Sprintf("detected %d new tx(s)", newDetected)
					slog.Info("new transactions detected",
						"watchID", watch.ID,
						"chain", watch.Chain,
						"count", newDetected,
					)
				}
			}

			// Adaptive polling: adjust interval based on activity.
			if fetchErr == nil && newDetected == 0 {
				consecutiveEmpty++
				newMultiplier := currentMultiplier
				if consecutiveEmpty == config.AdaptiveEmptyThreshold && currentMultiplier < 2 {
					newMultiplier = 2
				} else if consecutiveEmpty == config.AdaptiveEmptyThreshold*3 && currentMultiplier < config.AdaptiveMaxMultiplier {
					newMultiplier = config.AdaptiveMaxMultiplier
				}
				if newMultiplier != currentMultiplier {
					currentMultiplier = newMultiplier
					ticker.Reset(interval * time.Duration(currentMultiplier))
					slog.Info("adaptive polling: backing off",
						"watchID", watch.ID,
						"multiplier", currentMultiplier,
						"effectiveInterval", interval*time.Duration(currentMultiplier),
						"consecutiveEmpty", consecutiveEmpty,
					)
				}
			} else if newDetected > 0 && currentMultiplier > 1 {
				// Activity detected — reset to base interval.
				consecutiveEmpty = 0
				currentMultiplier = 1
				ticker.Reset(interval)
				slog.Info("adaptive polling: reset to base interval",
					"watchID", watch.ID,
					"interval", interval,
				)
			}

			// C. Re-check pending transactions.
			if err := w.recheckPending(ctx, watch, ps); err != nil {
				slog.Warn("pending recheck had errors",
					"watchID", watch.ID,
					"error", err,
				)
			}

			// D. Update watch poll metadata.
			if err := w.db.UpdateWatchPollResult(watch.ID, pollCount, pollResult); err != nil {
				slog.Error("failed to update watch poll result",
					"watchID", watch.ID,
					"error", err,
				)
			}

			// E. Check stop conditions.
			shouldStop, newStatus := w.checkStopConditions(watch)
			if shouldStop {
				completedAt := time.Now().UTC().Format(time.RFC3339)
				if err := w.db.UpdateWatchStatus(watch.ID, newStatus, &completedAt); err != nil {
					slog.Error("failed to update watch status on stop condition",
						"watchID", watch.ID,
						"status", newStatus,
						"error", err,
					)
				}
				slog.Info("watch stopped by condition",
					"watchID", watch.ID,
					"status", newStatus,
					"pollCount", pollCount,
				)
				return
			}
		}
	}
}

// resolveCutoff determines the transaction cutoff timestamp for a watch.
// Returns MAX(last recorded tx for address, startDate).
func (w *Watcher) resolveCutoff(ctx context.Context, address string, startDate int64) int64 {
	lastDetected, err := w.db.LastDetectedAt(ctx, address)
	if err != nil {
		slog.Warn("failed to query last detected_at, using startDate",
			"address", address,
			"error", err,
		)
		return startDate
	}

	if lastDetected == "" {
		slog.Debug("no prior transactions for address, using startDate",
			"address", address,
			"startDate", startDate,
		)
		return startDate
	}

	// Parse the RFC3339 timestamp to unix.
	t, err := time.Parse(time.RFC3339, lastDetected)
	if err != nil {
		slog.Warn("failed to parse last detected_at, using startDate",
			"address", address,
			"lastDetected", lastDetected,
			"error", err,
		)
		return startDate
	}

	lastUnix := t.Unix()
	if lastUnix > startDate {
		slog.Debug("using last detected tx as cutoff",
			"address", address,
			"cutoff", lastUnix,
			"lastDetected", lastDetected,
		)
		return lastUnix
	}

	slog.Debug("last detected tx is older than startDate, using startDate",
		"address", address,
		"lastDetected", lastDetected,
		"startDate", startDate,
	)
	return startDate
}

// processTransaction handles a single raw transaction: dedup, insert, and points.
func (w *Watcher) processTransaction(ctx context.Context, watch *models.Watch, raw provider.RawTransaction) error {
	// Dedup check.
	existing, err := w.db.GetByTxHash(raw.TxHash)
	if err != nil {
		return fmt.Errorf("dedup check failed for %s: %w", raw.TxHash, err)
	}
	if existing != nil {
		slog.Debug("transaction already recorded, skipping",
			"txHash", raw.TxHash,
			"watchID", watch.ID,
		)
		return nil
	}

	// Ensure points account exists.
	if _, err := w.db.GetOrCreatePoints(watch.Address, watch.Chain); err != nil {
		return fmt.Errorf("failed to ensure points account: %w", err)
	}

	// Build transaction record.
	now := time.Now().UTC().Format(time.RFC3339)
	tx := &models.Transaction{
		WatchID:     watch.ID,
		TxHash:      raw.TxHash,
		Chain:       watch.Chain,
		Address:     watch.Address,
		Token:       raw.Token,
		AmountRaw:   raw.AmountRaw,
		AmountHuman: raw.AmountHuman,
		Decimals:    raw.Decimals,
		DetectedAt:  now,
		BlockNumber: nilIfZero(raw.BlockNumber),
	}

	if raw.Confirmed {
		// Fetch price and calculate points.
		usdPrice, priceErr := w.pricer.GetTokenPrice(ctx, raw.Token)
		if priceErr != nil {
			// Fallback: try cached price before degrading to PENDING.
			cachedPrice, cacheErr := w.pricer.GetCachedPrice(raw.Token)
			if cacheErr == nil {
				slog.Warn("price fetch failed for confirmed tx, using cached price as fallback",
					"txHash", raw.TxHash,
					"token", raw.Token,
					"cachedPrice", cachedPrice,
					"error", priceErr,
				)
				usdPrice = cachedPrice
				priceErr = nil
			}
		}
		if priceErr != nil {
			slog.Warn("price fetch failed for confirmed tx, inserting as PENDING for price retry",
				"txHash", raw.TxHash,
				"token", raw.Token,
				"confirmations", raw.Confirmations,
				"error", priceErr,
			)
			// Insert as PENDING so recheckPending will retry with price later.
			// We preserve the confirmation count so it won't need re-verification.
			tx.Status = models.TxStatusPending
			tx.Confirmations = raw.Confirmations
			if _, err := w.db.InsertTransaction(tx); err != nil {
				return fmt.Errorf("failed to insert pending tx: %w", err)
			}
			return nil
		}

		// Calculate USD value and points.
		amountFloat := parseAmountHuman(raw.AmountHuman)
		usdValue := amountFloat * usdPrice
		calcResult := w.calculator.Calculate(usdValue)

		tx.Status = models.TxStatusConfirmed
		tx.Confirmations = raw.Confirmations
		tx.USDValue = usdValue
		tx.USDPrice = usdPrice
		tx.Tier = calcResult.TierIndex
		tx.Multiplier = calcResult.Multiplier
		tx.Points = calcResult.Points
		tx.ConfirmedAt = &now

		// Atomically insert confirmed tx and add points in a single DB transaction.
		// This prevents points loss if the process crashes between the two operations.
		if _, err := w.db.InsertConfirmedTxWithPoints(tx, watch.Address, watch.Chain, calcResult.Points); err != nil {
			return fmt.Errorf("failed to insert confirmed tx with points: %w", err)
		}

		slog.Info("confirmed transaction processed",
			"txHash", raw.TxHash,
			"watchID", watch.ID,
			"token", raw.Token,
			"amountHuman", raw.AmountHuman,
			"usdValue", usdValue,
			"points", calcResult.Points,
			"tier", calcResult.TierIndex,
		)
	} else {
		// Pending transaction — insert and estimate points.
		tx.Status = models.TxStatusPending
		tx.Confirmations = raw.Confirmations

		if _, err := w.db.InsertTransaction(tx); err != nil {
			return fmt.Errorf("failed to insert pending tx: %w", err)
		}

		// Estimate pending points using current price.
		estimatedPoints := w.estimatePendingPoints(ctx, raw)
		if estimatedPoints > 0 {
			if err := w.db.AddPending(watch.Address, watch.Chain, estimatedPoints); err != nil {
				slog.Error("failed to add pending points estimate",
					"address", watch.Address,
					"chain", watch.Chain,
					"points", estimatedPoints,
					"error", err,
				)
			}
		}

		slog.Info("pending transaction detected",
			"txHash", raw.TxHash,
			"watchID", watch.ID,
			"token", raw.Token,
			"amountHuman", raw.AmountHuman,
			"estimatedPoints", estimatedPoints,
		)
	}

	return nil
}

// recheckPending re-checks all PENDING transactions for a watch and confirms them
// when the blockchain reports sufficient confirmations.
func (w *Watcher) recheckPending(ctx context.Context, watch *models.Watch, ps *provider.ProviderSet) error {
	pending, err := w.db.ListPendingByWatchID(watch.ID)
	if err != nil {
		return fmt.Errorf("failed to list pending txs: %w", err)
	}

	if len(pending) == 0 {
		return nil
	}

	slog.Debug("rechecking pending transactions",
		"watchID", watch.ID,
		"count", len(pending),
	)

	var lastErr error
	for _, tx := range pending {
		if ctx.Err() != nil {
			return ctx.Err()
		}

		// Smart confirmation scheduling: skip recheck if not enough time has passed
		// for the chain to produce enough blocks for confirmation.
		// Saves ~80% of BTC confirmation API calls (~10 min block time vs 60s poll).
		detectedAt, parseErr := time.Parse(time.RFC3339, tx.DetectedAt)
		if parseErr == nil {
			minWait := confirmationMinWait(watch.Chain)
			elapsed := time.Since(detectedAt)
			if elapsed < minWait {
				slog.Debug("skipping confirmation check, too early for chain block time",
					"txHash", tx.TxHash,
					"chain", watch.Chain,
					"elapsed", elapsed,
					"minWait", minWait,
				)
				continue
			}
		}

		// For SOL composite tx_hash, extract the base signature for confirmation check.
		checkHash := tx.TxHash
		if watch.Chain == "SOL" {
			checkHash = extractBaseSignature(tx.TxHash)
		}

		var blockNum int64
		if tx.BlockNumber != nil {
			blockNum = *tx.BlockNumber
		}

		confirmed, confirmations, checkErr := ps.ExecuteConfirmation(ctx, checkHash, blockNum)
		if checkErr != nil {
			if ctx.Err() != nil {
				return ctx.Err()
			}
			slog.Warn("confirmation check failed",
				"txHash", tx.TxHash,
				"watchID", watch.ID,
				"error", checkErr,
			)
			lastErr = checkErr
			continue
		}

		if !confirmed {
			slog.Debug("transaction still pending",
				"txHash", tx.TxHash,
				"confirmations", confirmations,
			)
			continue
		}

		// Transaction confirmed — fetch price and calculate points.
		usdPrice, priceErr := w.pricer.GetTokenPrice(ctx, tx.Token)
		if priceErr != nil {
			// Fallback: try cached price before skipping this tick.
			cachedPrice, cacheErr := w.pricer.GetCachedPrice(tx.Token)
			if cacheErr == nil {
				slog.Warn("price fetch failed during confirmation, using cached price",
					"txHash", tx.TxHash,
					"token", tx.Token,
					"cachedPrice", cachedPrice,
					"error", priceErr,
				)
				usdPrice = cachedPrice
				priceErr = nil
			}
		}
		if priceErr != nil {
			slog.Warn("price fetch failed during confirmation, will retry next tick",
				"txHash", tx.TxHash,
				"token", tx.Token,
				"error", priceErr,
			)
			lastErr = priceErr
			continue
		}

		amountFloat := parseAmountHuman(tx.AmountHuman)
		usdValue := amountFloat * usdPrice
		calcResult := w.calculator.Calculate(usdValue)
		confirmedAt := time.Now().UTC().Format(time.RFC3339)

		// Atomically confirm tx and move points in a single DB transaction.
		if err := w.db.ConfirmTxAndMovePoints(
			tx.TxHash, confirmations, tx.BlockNumber, confirmedAt,
			usdValue, usdPrice, calcResult.TierIndex, calcResult.Multiplier, calcResult.Points,
			tx.Address, watch.Chain, tx.Points,
		); err != nil {
			slog.Error("failed to confirm tx and move points",
				"txHash", tx.TxHash,
				"address", tx.Address,
				"error", err,
			)
			lastErr = err
			continue
		}

		slog.Info("pending transaction confirmed",
			"txHash", tx.TxHash,
			"watchID", watch.ID,
			"token", tx.Token,
			"usdValue", usdValue,
			"points", calcResult.Points,
			"confirmations", confirmations,
		)
	}

	return lastErr
}

// checkStopConditions evaluates whether the watch should stop.
// Returns (shouldStop, newStatus).
func (w *Watcher) checkStopConditions(watch *models.Watch) (bool, models.WatchStatus) {
	// Check expiry.
	expiresAt, err := time.Parse(time.RFC3339, watch.ExpiresAt)
	if err != nil {
		slog.Error("failed to parse watch expiresAt",
			"watchID", watch.ID,
			"expiresAt", watch.ExpiresAt,
			"error", err,
		)
		return false, ""
	}

	if time.Now().UTC().After(expiresAt) {
		slog.Info("watch expired",
			"watchID", watch.ID,
			"expiresAt", watch.ExpiresAt,
		)
		return true, models.WatchStatusExpired
	}

	// Check completion: at least 1 tx, all confirmed.
	total, pending, err := w.db.CountByWatchID(watch.ID)
	if err != nil {
		slog.Error("failed to count txs for stop condition check",
			"watchID", watch.ID,
			"error", err,
		)
		return false, ""
	}

	if total > 0 && pending == 0 {
		slog.Info("watch completed: all transactions confirmed",
			"watchID", watch.ID,
			"totalTxs", total,
		)
		return true, models.WatchStatusCompleted
	}

	return false, ""
}

// determineStopStatus determines the watch status when the context is done.
func (w *Watcher) determineStopStatus(ctx context.Context, watch *models.Watch) models.WatchStatus {
	// Check if the watch was manually cancelled (cancel func called).
	// context.DeadlineExceeded means the timeout expired.
	if ctx.Err() == context.DeadlineExceeded {
		return models.WatchStatusExpired
	}
	return models.WatchStatusCancelled
}

// estimatePendingPoints calculates an estimated points value for a pending tx
// using the current price. This is a best-effort estimate — the final value
// is calculated when the tx is confirmed.
func (w *Watcher) estimatePendingPoints(ctx context.Context, raw provider.RawTransaction) int {
	usdPrice, err := w.pricer.GetTokenPrice(ctx, raw.Token)
	if err != nil {
		slog.Debug("could not estimate pending points, price unavailable",
			"token", raw.Token,
			"error", err,
		)
		return 0
	}

	amountFloat := parseAmountHuman(raw.AmountHuman)
	usdValue := amountFloat * usdPrice
	result := w.calculator.Calculate(usdValue)
	return result.Points
}

// logSystemError records a system error in the database. Non-blocking — errors
// in logging are themselves logged but don't propagate.
func (w *Watcher) logSystemError(severity, category, message, details string) {
	if _, err := w.db.InsertError(severity, category, message, details); err != nil {
		slog.Error("failed to log system error to DB",
			"severity", severity,
			"category", category,
			"message", message,
			"error", err,
		)
	}
}

// confirmationMinWait returns the minimum elapsed time before checking confirmation
// for a pending transaction on the given chain. Based on block time × required
// confirmations, slightly reduced to avoid missing fast blocks.
func confirmationMinWait(chain string) time.Duration {
	switch chain {
	case "BTC":
		return config.ConfirmationMinWaitBTC
	case "BSC":
		return config.ConfirmationMinWaitBSC
	case "SOL":
		return config.ConfirmationMinWaitSOL
	default:
		return 0
	}
}

// pollInterval returns the poll interval for a chain.
func pollInterval(chain string) time.Duration {
	switch chain {
	case "BTC":
		return config.PollIntervalBTC
	case "BSC":
		return config.PollIntervalBSC
	case "SOL":
		return config.PollIntervalSOL
	default:
		return config.PollIntervalBTC // conservative fallback
	}
}

// parseAmountHuman parses a human-readable amount string to float64.
// Returns 0 if parsing fails.
func parseAmountHuman(amount string) float64 {
	f, err := strconv.ParseFloat(amount, 64)
	if err != nil {
		slog.Warn("failed to parse amount_human", "amount", amount, "error", err)
		return 0
	}
	return f
}

// extractBaseSignature extracts the base signature from a SOL composite tx_hash.
// Composite format: "signature:TOKEN" (e.g. "abc123:SOL", "abc123:USDC").
// If the hash doesn't contain ":", it is returned as-is.
func extractBaseSignature(txHash string) string {
	idx := strings.LastIndex(txHash, ":")
	if idx == -1 {
		return txHash
	}
	return txHash[:idx]
}

// nilIfZero returns a pointer to v if v != 0, or nil if v == 0.
func nilIfZero(v int64) *int64 {
	if v == 0 {
		return nil
	}
	return &v
}
