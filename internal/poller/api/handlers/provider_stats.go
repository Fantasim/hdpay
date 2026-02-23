package handlers

import (
	"log/slog"
	"net/http"

	"github.com/Fantasim/hdpay/internal/poller/httputil"
	"github.com/Fantasim/hdpay/internal/poller/provider"
	"github.com/Fantasim/hdpay/internal/shared/scanner"
)

// ProviderStatsResponse is the API response for GET /api/admin/provider-stats.
type ProviderStatsResponse struct {
	Chains map[string][]scanner.MetricsSnapshot `json:"chains"`
}

// GetProviderStatsHandler returns usage metrics for all blockchain providers grouped by chain.
// Requires admin session.
func GetProviderStatsHandler(providerSets map[string]*provider.ProviderSet) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		slog.Debug("provider stats requested")

		chains := make(map[string][]scanner.MetricsSnapshot, len(providerSets))
		for chain, ps := range providerSets {
			if ps == nil {
				chains[chain] = []scanner.MetricsSnapshot{}
				continue
			}
			chains[chain] = ps.Stats()
		}

		slog.Info("provider stats served",
			"chains", len(chains),
		)

		httputil.JSON(w, http.StatusOK, ProviderStatsResponse{Chains: chains})
	}
}
