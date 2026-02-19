package tx

import (
	"context"
	"fmt"
	"log/slog"
	"net/http"
	"time"

	"github.com/ethereum/go-ethereum"
	"github.com/ethereum/go-ethereum/common"

	"github.com/Fantasim/hdpay/internal/config"
	"github.com/Fantasim/hdpay/internal/db"
)

// TxReconciler checks on-chain status of pending transactions at startup
// and updates both tx_state and transactions tables accordingly.
type TxReconciler struct {
	database        *db.DB
	httpClient      *http.Client
	btcProviderURLs []string
	ethClient       EthClientWrapper
	solClient       SOLRPCClient
}

// NewTxReconciler creates a new reconciler with all chain clients.
func NewTxReconciler(
	database *db.DB,
	httpClient *http.Client,
	btcProviderURLs []string,
	ethClient EthClientWrapper,
	solClient SOLRPCClient,
) *TxReconciler {
	return &TxReconciler{
		database:        database,
		httpClient:      httpClient,
		btcProviderURLs: btcProviderURLs,
		ethClient:       ethClient,
		solClient:       solClient,
	}
}

// ReconcilePending checks all non-terminal tx_state rows and reconciles their status
// against the blockchain. Should be called once at server startup.
func (r *TxReconciler) ReconcilePending(ctx context.Context) {
	slog.Info("tx reconciler: starting pending transaction reconciliation")

	pending, err := r.database.GetAllPendingTxStates()
	if err != nil {
		slog.Error("tx reconciler: failed to fetch pending tx states", "error", err)
		return
	}

	if len(pending) == 0 {
		slog.Info("tx reconciler: no pending transactions to reconcile")
		return
	}

	slog.Info("tx reconciler: found pending transactions", "count", len(pending))

	var reconciled, confirmed, failed, uncertain, repolling int
	for _, txState := range pending {
		if ctx.Err() != nil {
			slog.Warn("tx reconciler: context cancelled, stopping", "reconciled", reconciled)
			return
		}

		// Rows without a txHash were never broadcast — mark as failed.
		if txState.TxHash == "" {
			slog.Warn("tx reconciler: marking unbroadcast tx as failed",
				"id", txState.ID,
				"chain", txState.Chain,
				"status", txState.Status,
			)
			r.updateBothTables(txState, config.TxStateFailed)
			failed++
			reconciled++
			continue
		}

		// Check age for timeout handling.
		createdAt, parseErr := time.Parse("2006-01-02 15:04:05", txState.CreatedAt)
		if parseErr != nil {
			slog.Warn("tx reconciler: could not parse created_at, treating as old",
				"id", txState.ID,
				"createdAt", txState.CreatedAt,
				"error", parseErr,
			)
			r.updateBothTables(txState, config.TxStateUncertain)
			uncertain++
			reconciled++
			continue
		}

		age := time.Since(createdAt)

		// One-shot on-chain status check.
		checkCtx, cancel := context.WithTimeout(ctx, config.ReconcileCheckTimeout)
		status, err := r.checkOnChain(checkCtx, txState)
		cancel()

		switch {
		case err != nil:
			slog.Warn("tx reconciler: on-chain check failed",
				"id", txState.ID,
				"chain", txState.Chain,
				"txHash", txState.TxHash,
				"error", err,
			)
			if age > config.ReconcileMaxAge {
				slog.Warn("tx reconciler: tx too old, marking uncertain",
					"id", txState.ID,
					"age", age,
				)
				r.updateBothTables(txState, config.TxStateUncertain)
				uncertain++
			} else {
				// Still within age limit, launch polling goroutine.
				r.launchPolling(ctx, txState)
				repolling++
			}

		case status == config.TxStateConfirmed:
			slog.Info("tx reconciler: tx confirmed on-chain",
				"id", txState.ID,
				"chain", txState.Chain,
				"txHash", txState.TxHash,
			)
			r.updateBothTables(txState, config.TxStateConfirmed)
			confirmed++

		case status == config.TxStateFailed:
			slog.Info("tx reconciler: tx failed on-chain",
				"id", txState.ID,
				"chain", txState.Chain,
				"txHash", txState.TxHash,
			)
			r.updateBothTables(txState, config.TxStateFailed)
			failed++

		default:
			// Still pending.
			if age > config.ReconcileMaxAge {
				slog.Warn("tx reconciler: tx too old and still pending, marking uncertain",
					"id", txState.ID,
					"age", age,
				)
				r.updateBothTables(txState, config.TxStateUncertain)
				uncertain++
			} else {
				r.launchPolling(ctx, txState)
				repolling++
			}
		}

		reconciled++
	}

	slog.Info("tx reconciler: reconciliation complete",
		"total", len(pending),
		"confirmed", confirmed,
		"failed", failed,
		"uncertain", uncertain,
		"repolling", repolling,
	)
}

// checkOnChain does a single on-chain status check for a transaction.
// Returns the determined status or "" if still pending/unknown.
func (r *TxReconciler) checkOnChain(ctx context.Context, txState db.TxStateRow) (string, error) {
	switch txState.Chain {
	case "BTC":
		return r.checkBTC(ctx, txState.TxHash)
	case "BSC":
		return r.checkBSC(ctx, txState.TxHash)
	case "SOL":
		return r.checkSOL(ctx, txState.TxHash)
	default:
		return "", fmt.Errorf("unknown chain: %s", txState.Chain)
	}
}

// checkBTC checks BTC transaction confirmation via Esplora API.
func (r *TxReconciler) checkBTC(ctx context.Context, txHash string) (string, error) {
	if len(r.btcProviderURLs) == 0 {
		return "", fmt.Errorf("no BTC provider URLs configured")
	}

	for _, baseURL := range r.btcProviderURLs {
		url := baseURL + fmt.Sprintf(config.BTCTxStatusPath, txHash)
		confirmed, err := pollBTCTxStatus(ctx, r.httpClient, url)
		if err != nil {
			slog.Debug("tx reconciler: BTC check failed on provider",
				"provider", baseURL,
				"txHash", txHash,
				"error", err,
			)
			continue
		}
		if confirmed {
			return config.TxStateConfirmed, nil
		}
		// TX found but not confirmed yet.
		return "", nil
	}

	return "", fmt.Errorf("all BTC providers failed for tx %s", txHash)
}

// checkBSC checks BSC transaction confirmation via eth_getTransactionReceipt.
func (r *TxReconciler) checkBSC(ctx context.Context, txHash string) (string, error) {
	if r.ethClient == nil {
		return "", fmt.Errorf("no BSC client configured")
	}

	receipt, err := r.ethClient.TransactionReceipt(ctx, common.HexToHash(txHash))
	if err != nil {
		if err == ethereum.NotFound {
			// TX not mined yet.
			return "", nil
		}
		return "", fmt.Errorf("BSC receipt check: %w", err)
	}

	if receipt.Status == 0 {
		return config.TxStateFailed, nil
	}
	return config.TxStateConfirmed, nil
}

// checkSOL checks SOL transaction confirmation via getSignatureStatuses.
func (r *TxReconciler) checkSOL(ctx context.Context, signature string) (string, error) {
	if r.solClient == nil {
		return "", fmt.Errorf("no SOL client configured")
	}

	statuses, err := r.solClient.GetSignatureStatuses(ctx, []string{signature})
	if err != nil {
		return "", fmt.Errorf("SOL status check: %w", err)
	}

	if len(statuses) == 0 {
		// No status yet.
		return "", nil
	}

	status := statuses[0]
	if status.Err != nil {
		return config.TxStateFailed, nil
	}
	if status.ConfirmationStatus != nil {
		cs := *status.ConfirmationStatus
		if cs == "confirmed" || cs == "finalized" {
			return config.TxStateConfirmed, nil
		}
	}

	// Still processing.
	return "", nil
}

// updateBothTables updates both tx_state and transactions tables for a given transaction.
func (r *TxReconciler) updateBothTables(txState db.TxStateRow, status string) {
	if err := r.database.UpdateTxStatus(txState.ID, status, txState.TxHash, ""); err != nil {
		slog.Error("tx reconciler: failed to update tx_state",
			"id", txState.ID,
			"status", status,
			"error", err,
		)
	}

	if txState.TxHash != "" {
		if err := r.database.UpdateTransactionStatusByHash(txState.Chain, txState.TxHash, status); err != nil {
			slog.Error("tx reconciler: failed to update transactions table",
				"chain", txState.Chain,
				"txHash", txState.TxHash,
				"status", status,
				"error", err,
			)
		}
	}
}

// launchPolling starts a background confirmation poller for a still-pending transaction.
func (r *TxReconciler) launchPolling(ctx context.Context, txState db.TxStateRow) {
	slog.Info("tx reconciler: launching background poller",
		"id", txState.ID,
		"chain", txState.Chain,
		"txHash", txState.TxHash,
	)

	switch txState.Chain {
	case "BTC":
		go func() {
			bgCtx, cancel := context.WithTimeout(ctx, config.BTCConfirmationTimeout)
			defer cancel()

			if err := WaitForBTCConfirmation(bgCtx, r.httpClient, r.btcProviderURLs, txState.TxHash); err != nil {
				slog.Warn("tx reconciler: BTC confirmation polling failed",
					"txHash", txState.TxHash,
					"error", err,
				)
				r.updateBothTables(txState, config.TxStateUncertain)
			} else {
				slog.Info("tx reconciler: BTC transaction confirmed", "txHash", txState.TxHash)
				r.updateBothTables(txState, config.TxStateConfirmed)
			}
		}()

	case "BSC":
		go func() {
			bgCtx, cancel := context.WithTimeout(ctx, config.BSCReceiptPollTimeout)
			defer cancel()

			receipt, err := WaitForReceipt(bgCtx, r.ethClient, common.HexToHash(txState.TxHash))
			if err != nil {
				slog.Warn("tx reconciler: BSC receipt polling failed",
					"txHash", txState.TxHash,
					"error", err,
				)
				r.updateBothTables(txState, config.TxStateFailed)
			} else if receipt.Status == 0 {
				slog.Warn("tx reconciler: BSC transaction reverted",
					"txHash", txState.TxHash,
				)
				r.updateBothTables(txState, config.TxStateFailed)
			} else {
				slog.Info("tx reconciler: BSC transaction confirmed", "txHash", txState.TxHash)
				r.updateBothTables(txState, config.TxStateConfirmed)
			}
		}()

	case "SOL":
		go func() {
			bgCtx, cancel := context.WithTimeout(ctx, config.SOLConfirmationTimeout)
			defer cancel()

			_, err := WaitForSOLConfirmation(bgCtx, r.solClient, txState.TxHash)
			if err != nil {
				slog.Warn("tx reconciler: SOL confirmation polling failed",
					"signature", txState.TxHash,
					"error", err,
				)
				r.updateBothTables(txState, config.TxStateUncertain)
			} else {
				slog.Info("tx reconciler: SOL transaction confirmed", "signature", txState.TxHash)
				r.updateBothTables(txState, config.TxStateConfirmed)
			}
		}()
	}
}
