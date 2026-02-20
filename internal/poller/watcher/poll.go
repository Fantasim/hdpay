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

	// Resolve cutoff timestamp.
	cutoff := w.resolveCutoff(watch.Address, w.cfg.StartDate)

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
func (w *Watcher) resolveCutoff(address string, startDate int64) int64 {
	lastDetected, err := w.db.LastDetectedAt(address)
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
			slog.Warn("price fetch failed for confirmed tx, inserting as PENDING",
				"txHash", raw.TxHash,
				"token", raw.Token,
				"error", priceErr,
			)
			// Fall back to PENDING — will be confirmed with price on next recheck.
			tx.Status = models.TxStatusPending
			tx.Confirmations = raw.Confirmations
			if _, err := w.db.InsertTransaction(tx); err != nil {
				return fmt.Errorf("failed to insert pending tx: %w", err)
			}
			// Estimate pending points with 0 (no price available).
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

		if _, err := w.db.InsertTransaction(tx); err != nil {
			return fmt.Errorf("failed to insert confirmed tx: %w", err)
		}

		// Add to unclaimed points ledger.
		if calcResult.Points > 0 {
			if err := w.db.AddUnclaimed(watch.Address, watch.Chain, calcResult.Points); err != nil {
				slog.Error("failed to add unclaimed points",
					"address", watch.Address,
					"chain", watch.Chain,
					"points", calcResult.Points,
					"error", err,
				)
			}
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

		if err := w.db.UpdateToConfirmed(
			tx.TxHash, confirmations, tx.BlockNumber, confirmedAt,
			usdValue, usdPrice, calcResult.TierIndex, calcResult.Multiplier, calcResult.Points,
		); err != nil {
			slog.Error("failed to update tx to confirmed",
				"txHash", tx.TxHash,
				"error", err,
			)
			lastErr = err
			continue
		}

		// Move points from pending to unclaimed.
		// Old pending points were estimated — remove them and add confirmed points.
		if err := w.db.MovePendingToUnclaimed(
			tx.Address, watch.Chain, tx.Points, calcResult.Points,
		); err != nil {
			slog.Error("failed to move pending to unclaimed",
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
