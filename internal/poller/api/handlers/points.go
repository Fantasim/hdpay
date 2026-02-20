package handlers

import (
	"encoding/json"
	"log/slog"
	"net/http"

	"github.com/Fantasim/hdpay/internal/poller/httputil"
	pollerconfig "github.com/Fantasim/hdpay/internal/poller/config"
	"github.com/Fantasim/hdpay/internal/poller/models"
	"github.com/Fantasim/hdpay/internal/poller/pollerdb"
)

// pointsAccountResponse is the response format for GET /api/points.
type pointsAccountResponse struct {
	Address      string              `json:"address"`
	Chain        string              `json:"chain"`
	Unclaimed    int                 `json:"unclaimed"`
	Total        int                 `json:"total"`
	Transactions []models.Transaction `json:"transactions"`
}

// pendingAccountResponse is the response format for GET /api/points/pending.
type pendingAccountResponse struct {
	Address       string                  `json:"address"`
	Chain         string                  `json:"chain"`
	PendingPoints int                     `json:"pending_points"`
	Transactions  []pendingTxResponse     `json:"transactions"`
}

type pendingTxResponse struct {
	TxHash                string  `json:"tx_hash"`
	Token                 string  `json:"token"`
	AmountRaw             string  `json:"amount_raw"`
	AmountHuman           string  `json:"amount_human"`
	USDValue              float64 `json:"usd_value"`
	Tier                  int     `json:"tier"`
	Points                int     `json:"points"`
	Status                string  `json:"status"`
	Confirmations         int     `json:"confirmations"`
	ConfirmationsRequired int     `json:"confirmations_required"`
	DetectedAt            string  `json:"detected_at"`
}

type claimRequest struct {
	Addresses []string `json:"addresses"`
}

type claimEntry struct {
	Address       string `json:"address"`
	Chain         string `json:"chain"`
	PointsClaimed int    `json:"points_claimed"`
}

type claimResponse struct {
	Claimed      []claimEntry `json:"claimed"`
	Skipped      []string     `json:"skipped"`
	TotalClaimed int          `json:"total_claimed"`
}

// GetPointsHandler returns a handler for GET /api/points.
// Returns accounts with unclaimed > 0, with their confirmed transactions.
func GetPointsHandler(db *pollerdb.DB) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		accounts, err := db.ListWithUnclaimed()
		if err != nil {
			slog.Error("get points: failed to list unclaimed", "error", err)
			httputil.Error(w, http.StatusInternalServerError, pollerconfig.ErrorDatabase, "Failed to query points")
			return
		}

		var result []pointsAccountResponse
		for _, acct := range accounts {
			txs, err := db.ListByAddress(acct.Address)
			if err != nil {
				slog.Error("get points: failed to list transactions", "address", acct.Address, "error", err)
				continue
			}

			// Filter to confirmed transactions only.
			var confirmed []models.Transaction
			for _, tx := range txs {
				if tx.Status == models.TxStatusConfirmed && tx.Chain == acct.Chain {
					confirmed = append(confirmed, tx)
				}
			}

			result = append(result, pointsAccountResponse{
				Address:      acct.Address,
				Chain:        acct.Chain,
				Unclaimed:    acct.Unclaimed,
				Total:        acct.Total,
				Transactions: confirmed,
			})
		}

		if result == nil {
			result = []pointsAccountResponse{}
		}

		slog.Debug("points listed", "accounts", len(result))
		httputil.JSON(w, http.StatusOK, result)
	}
}

// GetPendingPointsHandler returns a handler for GET /api/points/pending.
// Returns accounts with pending > 0, with their pending transactions.
func GetPendingPointsHandler(db *pollerdb.DB) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		accounts, err := db.ListWithPending()
		if err != nil {
			slog.Error("get pending points: failed to list pending", "error", err)
			httputil.Error(w, http.StatusInternalServerError, pollerconfig.ErrorDatabase, "Failed to query pending points")
			return
		}

		var result []pendingAccountResponse
		for _, acct := range accounts {
			txs, err := db.ListByAddress(acct.Address)
			if err != nil {
				slog.Error("get pending points: failed to list transactions", "address", acct.Address, "error", err)
				continue
			}

			var pending []pendingTxResponse
			for _, tx := range txs {
				if tx.Status == models.TxStatusPending && tx.Chain == acct.Chain {
					pending = append(pending, pendingTxResponse{
						TxHash:                tx.TxHash,
						Token:                 tx.Token,
						AmountRaw:             tx.AmountRaw,
						AmountHuman:           tx.AmountHuman,
						USDValue:              tx.USDValue,
						Tier:                  tx.Tier,
						Points:                tx.Points,
						Status:                string(tx.Status),
						Confirmations:         tx.Confirmations,
						ConfirmationsRequired: confirmationsRequired(tx.Chain),
						DetectedAt:            tx.DetectedAt,
					})
				}
			}

			result = append(result, pendingAccountResponse{
				Address:       acct.Address,
				Chain:         acct.Chain,
				PendingPoints: acct.Pending,
				Transactions:  pending,
			})
		}

		if result == nil {
			result = []pendingAccountResponse{}
		}

		slog.Debug("pending points listed", "accounts", len(result))
		httputil.JSON(w, http.StatusOK, result)
	}
}

// ClaimPointsHandler returns a handler for POST /api/points/claim.
// Resets unclaimed points for the specified addresses. Skips unknown/zero addresses silently.
func ClaimPointsHandler(db *pollerdb.DB) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		var req claimRequest
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			httputil.Error(w, http.StatusBadRequest, pollerconfig.ErrorInvalidRequest, "Invalid request body")
			return
		}

		if len(req.Addresses) == 0 {
			httputil.Error(w, http.StatusBadRequest, pollerconfig.ErrorInvalidRequest, "Addresses list is required")
			return
		}

		resp := claimResponse{
			Claimed: []claimEntry{},
			Skipped: []string{},
		}

		for _, addr := range req.Addresses {
			claimed := false
			// An address can exist on multiple chains — try all.
			for _, chain := range []string{"BTC", "BSC", "SOL"} {
				points, err := db.ClaimPoints(addr, chain)
				if err != nil {
					slog.Error("claim points failed", "address", addr, "chain", chain, "error", err)
					continue
				}
				if points > 0 {
					resp.Claimed = append(resp.Claimed, claimEntry{
						Address:       addr,
						Chain:         chain,
						PointsClaimed: points,
					})
					resp.TotalClaimed += points
					claimed = true

					slog.Info("points claimed",
						"address", addr,
						"chain", chain,
						"pointsClaimed", points,
						"remoteAddr", r.RemoteAddr,
					)
				}
			}

			if !claimed {
				resp.Skipped = append(resp.Skipped, addr)
			}
		}

		slog.Info("claim completed",
			"totalClaimed", resp.TotalClaimed,
			"claimedCount", len(resp.Claimed),
			"skippedCount", len(resp.Skipped),
		)
		httputil.JSON(w, http.StatusOK, resp)
	}
}

// confirmationsRequired returns the confirmation threshold for a chain.
func confirmationsRequired(chain string) int {
	switch chain {
	case "BTC":
		return pollerconfig.ConfirmationsBTC
	case "BSC":
		return pollerconfig.ConfirmationsBSC
	case "SOL":
		return 1 // "finalized" is binary — 0 or 1
	default:
		return 1
	}
}
