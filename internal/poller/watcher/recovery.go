package watcher

import (
	"context"
	"fmt"
	"log/slog"
	"time"

	"github.com/Fantasim/hdpay/internal/poller/config"
)

// RunRecovery handles startup recovery. It must be called once before accepting
// new watches. It runs synchronously and blocks until complete.
//
// Steps:
// 1. Expire all ACTIVE watches from a previous run (crash recovery)
// 2. Re-check all PENDING transactions on-chain with retries
func (w *Watcher) RunRecovery(ctx context.Context) error {
	slog.Info("starting watcher recovery")

	// Step 1: Expire all active watches from previous run.
	expired, err := w.db.ExpireAllActiveWatches()
	if err != nil {
		return fmt.Errorf("recovery: failed to expire active watches: %w", err)
	}
	slog.Info("recovery: expired stale active watches", "count", expired)

	// Step 2: Re-check all pending transactions.
	pending, err := w.db.ListPending()
	if err != nil {
		return fmt.Errorf("recovery: failed to list pending transactions: %w", err)
	}

	if len(pending) == 0 {
		slog.Info("recovery: no pending transactions to re-check")
		return nil
	}

	slog.Info("recovery: re-checking pending transactions", "count", len(pending))

	// Group pending txs by chain for provider lookup.
	unresolved := 0
	resolved := 0

	for _, tx := range pending {
		if ctx.Err() != nil {
			return fmt.Errorf("recovery: cancelled: %w", ctx.Err())
		}

		ps, ok := w.providers[tx.Chain]
		if !ok {
			slog.Warn("recovery: no provider for chain, skipping tx",
				"chain", tx.Chain,
				"txHash", tx.TxHash,
			)
			unresolved++
			continue
		}

		// Retry confirmation check.
		confirmedThisRound := false
		for attempt := 1; attempt <= config.RecoveryPendingRetries; attempt++ {
			if ctx.Err() != nil {
				return fmt.Errorf("recovery: cancelled during retry: %w", ctx.Err())
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
				slog.Warn("recovery: confirmation check failed",
					"txHash", tx.TxHash,
					"attempt", attempt,
					"maxAttempts", config.RecoveryPendingRetries,
					"error", checkErr,
				)

				if attempt < config.RecoveryPendingRetries {
					// Wait before next retry.
					select {
					case <-ctx.Done():
						return fmt.Errorf("recovery: cancelled during wait: %w", ctx.Err())
					case <-time.After(config.RecoveryPendingInterval):
					}
				}
				continue
			}

			if !confirmed {
				slog.Debug("recovery: tx still pending",
					"txHash", tx.TxHash,
					"confirmations", confirmations,
					"attempt", attempt,
				)

				if attempt < config.RecoveryPendingRetries {
					select {
					case <-ctx.Done():
						return fmt.Errorf("recovery: cancelled during wait: %w", ctx.Err())
					case <-time.After(config.RecoveryPendingInterval):
					}
				}
				continue
			}

			// Confirmed! Fetch price and update.
			usdPrice, priceErr := w.pricer.GetTokenPrice(ctx, tx.Token)
			if priceErr != nil {
				slog.Warn("recovery: price fetch failed for confirmed tx",
					"txHash", tx.TxHash,
					"token", tx.Token,
					"error", priceErr,
				)
				// Can't complete without price — leave pending.
				break
			}

			amountFloat := parseAmountHuman(tx.AmountHuman)
			usdValue := amountFloat * usdPrice
			calcResult := w.calculator.Calculate(usdValue)
			confirmedAt := time.Now().UTC().Format(time.RFC3339)

			if err := w.db.UpdateToConfirmed(
				tx.TxHash, confirmations, tx.BlockNumber, confirmedAt,
				usdValue, usdPrice, calcResult.TierIndex, calcResult.Multiplier, calcResult.Points,
			); err != nil {
				slog.Error("recovery: failed to update tx to confirmed",
					"txHash", tx.TxHash,
					"error", err,
				)
				break
			}

			// Move points from pending to unclaimed.
			if err := w.db.MovePendingToUnclaimed(
				tx.Address, tx.Chain, tx.Points, calcResult.Points,
			); err != nil {
				slog.Error("recovery: failed to move pending to unclaimed",
					"txHash", tx.TxHash,
					"error", err,
				)
			}

			slog.Info("recovery: pending tx confirmed",
				"txHash", tx.TxHash,
				"chain", tx.Chain,
				"usdValue", usdValue,
				"points", calcResult.Points,
			)

			confirmedThisRound = true
			resolved++
			break
		}

		if !confirmedThisRound {
			unresolved++
			slog.Warn("recovery: tx still pending after all retries",
				"txHash", tx.TxHash,
				"chain", tx.Chain,
				"retries", config.RecoveryPendingRetries,
			)
			// Log system error.
			w.logSystemError(
				config.ErrorSeverityWarn,
				config.ErrorCategoryWatcher,
				fmt.Sprintf("pending tx %s still unresolved after %d recovery retries", tx.TxHash, config.RecoveryPendingRetries),
				fmt.Sprintf("chain=%s address=%s token=%s amount=%s", tx.Chain, tx.Address, tx.Token, tx.AmountHuman),
			)
		}
	}

	slog.Info("recovery complete",
		"totalPending", len(pending),
		"resolved", resolved,
		"unresolved", unresolved,
	)

	return nil
}
