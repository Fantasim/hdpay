package handlers

import (
	"log/slog"
	"net/http"
	"time"

	"github.com/Fantasim/hdpay/internal/poller/config"
	"github.com/Fantasim/hdpay/internal/poller/httputil"
	"github.com/Fantasim/hdpay/internal/poller/pollerdb"
	"github.com/Fantasim/hdpay/internal/poller/provider"
	"github.com/Fantasim/hdpay/internal/shared/scanner"
)

// ProviderStatsResponse is the API response for GET /api/admin/provider-stats.
type ProviderStatsResponse struct {
	Chains map[string][]scanner.MetricsSnapshot `json:"chains"`
}

// GetProviderStatsHandler returns usage metrics for all blockchain providers grouped by chain.
// Reads persisted daily/monthly counters from the database and merges with in-memory
// monthly limits from ProviderSets.
func GetProviderStatsHandler(db *pollerdb.DB, providerSets map[string]*provider.ProviderSet) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		slog.Debug("provider stats requested")

		// Get daily usage from DB.
		today := time.Now().UTC().Format(config.ProviderUsageDateFormat)
		dailyRows, err := db.GetDailyUsage(today)
		if err != nil {
			slog.Error("failed to query daily provider usage", "error", err)
		}

		// Get monthly usage from DB.
		monthlyRows, err := db.GetAllMonthlyUsage()
		if err != nil {
			slog.Error("failed to query monthly provider usage", "error", err)
		}

		// Index daily and monthly by chain/provider for fast lookup.
		type key struct{ chain, provider string }
		dailyMap := make(map[key]*pollerdb.ProviderUsageRow, len(dailyRows))
		for i := range dailyRows {
			dailyMap[key{dailyRows[i].Chain, dailyRows[i].Provider}] = &dailyRows[i]
		}
		monthlyMap := make(map[key]*pollerdb.ProviderUsageRow, len(monthlyRows))
		for i := range monthlyRows {
			monthlyMap[key{monthlyRows[i].Chain, monthlyRows[i].Provider}] = &monthlyRows[i]
		}

		// Build response using in-memory provider list (ensures all providers appear
		// even if they have zero DB rows) and merge DB counters.
		chains := make(map[string][]scanner.MetricsSnapshot, len(providerSets))
		for chain, ps := range providerSets {
			if ps == nil {
				chains[chain] = []scanner.MetricsSnapshot{}
				continue
			}

			// Get in-memory stats (for provider names and monthly limits).
			inMemStats := ps.Stats()
			snapshots := make([]scanner.MetricsSnapshot, len(inMemStats))

			for i, mem := range inMemStats {
				k := key{chain, mem.Name}

				snap := scanner.MetricsSnapshot{
					Name:              mem.Name,
					KnownMonthlyLimit: mem.KnownMonthlyLimit,
				}

				// Fill daily from DB.
				if d, ok := dailyMap[k]; ok {
					snap.Daily = scanner.PeriodSnapshot{
						Requests:  d.Requests,
						Successes: d.Successes,
						Failures:  d.Failures,
						Hits429:   d.Hits429,
					}
				}

				// Fill monthly from DB.
				if m, ok := monthlyMap[k]; ok {
					snap.Monthly = scanner.PeriodSnapshot{
						Requests:  m.Requests,
						Successes: m.Successes,
						Failures:  m.Failures,
						Hits429:   m.Hits429,
					}
				}

				// Total = in-memory lifetime (since last restart).
				snap.Total = mem.Total

				// Weekly = in-memory (not persisted, acceptable).
				snap.Weekly = mem.Weekly

				snapshots[i] = snap
			}

			chains[chain] = snapshots
		}

		slog.Info("provider stats served",
			"chains", len(chains),
		)

		httputil.JSON(w, http.StatusOK, ProviderStatsResponse{Chains: chains})
	}
}
