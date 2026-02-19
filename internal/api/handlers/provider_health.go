package handlers

import (
	"encoding/json"
	"log/slog"
	"net/http"

	"github.com/Fantasim/hdpay/internal/db"
	"github.com/Fantasim/hdpay/internal/models"
)

// ProviderHealthResponse is the per-provider health info returned by the API.
type ProviderHealthResponse struct {
	Name             string `json:"name"`
	Chain            string `json:"chain"`
	Type             string `json:"type"`
	Status           string `json:"status"`
	CircuitState     string `json:"circuitState"`
	ConsecutiveFails int    `json:"consecutiveFails"`
	LastSuccess      string `json:"lastSuccess"`
	LastError        string `json:"lastError"`
	LastErrorMsg     string `json:"lastErrorMsg"`
}

// GetProviderHealth returns a handler for GET /api/health/providers.
// Returns per-chain provider health status from the provider_health table.
func GetProviderHealth(database *db.DB) http.HandlerFunc {
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

		// Group by chain. Initialize empty arrays for all supported chains.
		result := make(map[string][]ProviderHealthResponse)
		for _, chain := range models.AllChains {
			result[string(chain)] = []ProviderHealthResponse{}
		}

		for _, row := range rows {
			result[row.Chain] = append(result[row.Chain], ProviderHealthResponse{
				Name:             row.ProviderName,
				Chain:            row.Chain,
				Type:             row.ProviderType,
				Status:           row.Status,
				CircuitState:     row.CircuitState,
				ConsecutiveFails: row.ConsecutiveFails,
				LastSuccess:      row.LastSuccess,
				LastError:        row.LastError,
				LastErrorMsg:     row.LastErrorMsg,
			})
		}

		slog.Debug("provider health response",
			"providerCount", len(rows),
		)

		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(map[string]interface{}{
			"data": result,
		})
	}
}
