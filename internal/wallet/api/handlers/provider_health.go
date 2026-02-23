package handlers

import (
	"encoding/json"
	"log/slog"
	"net/http"

	"github.com/Fantasim/hdpay/internal/shared/models"
	"github.com/Fantasim/hdpay/internal/shared/scanner"
	"github.com/Fantasim/hdpay/internal/wallet/db"
)

// ProviderHealthResponse is the per-provider health info returned by the API.
type ProviderHealthResponse struct {
	Name             string                  `json:"name"`
	Chain            string                  `json:"chain"`
	Type             string                  `json:"type"`
	Status           string                  `json:"status"`
	CircuitState     string                  `json:"circuitState"`
	ConsecutiveFails int                     `json:"consecutiveFails"`
	LastSuccess      string                  `json:"lastSuccess"`
	LastError        string                  `json:"lastError"`
	LastErrorMsg     string                  `json:"lastErrorMsg"`
	Metrics          scanner.MetricsSnapshot `json:"metrics"`
}

// GetProviderHealth returns a handler for GET /api/health/providers.
// Returns per-chain provider health status from the provider_health table,
// merged with in-memory usage metrics from the scanner.
func GetProviderHealth(database *db.DB, metricsGetter func() []scanner.MetricsSnapshot) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		slog.Debug("provider health requested", "remoteAddr", r.RemoteAddr)

		rows, err := database.GetAllProviderHealth()
		if err != nil {
			slog.Error("failed to get provider health", "error", err)
			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(http.StatusInternalServerError)
			json.NewEncoder(w).Encode(map[string]interface{}{
				"error": map[string]string{
					"code":    "ERROR_DATABASE",
					"message": "failed to fetch provider health",
				},
			})
			return
		}

		// Build a name → MetricsSnapshot index from in-memory scanner metrics.
		metricsIndex := make(map[string]scanner.MetricsSnapshot)
		if metricsGetter != nil {
			for _, snap := range metricsGetter() {
				metricsIndex[snap.Name] = snap
			}
		}

		// Group by chain. Initialize empty arrays for all supported chains.
		result := make(map[string][]ProviderHealthResponse)
		for _, chain := range models.AllChains {
			result[string(chain)] = []ProviderHealthResponse{}
		}

		for _, row := range rows {
			resp := ProviderHealthResponse{
				Name:             row.ProviderName,
				Chain:            row.Chain,
				Type:             row.ProviderType,
				Status:           row.Status,
				CircuitState:     row.CircuitState,
				ConsecutiveFails: row.ConsecutiveFails,
				LastSuccess:      row.LastSuccess,
				LastError:        row.LastError,
				LastErrorMsg:     row.LastErrorMsg,
			}
			if snap, ok := metricsIndex[row.ProviderName]; ok {
				resp.Metrics = snap
			} else {
				// Provide a zero-value snapshot with the provider name set.
				resp.Metrics = scanner.MetricsSnapshot{Name: row.ProviderName}
			}
			result[row.Chain] = append(result[row.Chain], resp)
		}

		slog.Debug("provider health response",
			"providerCount", len(rows),
			"metricsCount", len(metricsIndex),
		)

		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(map[string]interface{}{
			"data": result,
		})
	}
}
