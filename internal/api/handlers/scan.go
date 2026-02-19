package handlers

import (
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
	"net/http"
	"strings"
	"time"

	"github.com/Fantasim/hdpay/internal/config"
	"github.com/Fantasim/hdpay/internal/db"
	"github.com/Fantasim/hdpay/internal/models"
	"github.com/Fantasim/hdpay/internal/scanner"
)

// startScanRequest is the JSON body for POST /api/scan/start.
type startScanRequest struct {
	Chain string `json:"chain"`
	MaxID int    `json:"maxId"`
}

// stopScanRequest is the JSON body for POST /api/scan/stop.
type stopScanRequest struct {
	Chain string `json:"chain"`
}

// scanStatusResponse augments ScanState with live running status.
type scanStatusResponse struct {
	Chain            models.Chain `json:"chain"`
	LastScannedIndex int          `json:"lastScannedIndex"`
	MaxScanID        int          `json:"maxScanId"`
	Status           string       `json:"status"`
	StartedAt        string       `json:"startedAt,omitempty"`
	UpdatedAt        string       `json:"updatedAt,omitempty"`
	IsRunning        bool         `json:"isRunning"`
	FundedCount      int          `json:"fundedCount"`
}

// StartScan handles POST /api/scan/start.
func StartScan(sc *scanner.Scanner) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		start := time.Now()

		var req startScanRequest
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			slog.Warn("invalid start scan request body",
				"error", err,
				"remoteAddr", r.RemoteAddr,
			)
			writeError(w, http.StatusBadRequest, config.ErrorScanFailed, "invalid request body")
			return
		}

		slog.Info("start scan requested",
			"chain", req.Chain,
			"maxId", req.MaxID,
			"remoteAddr", r.RemoteAddr,
		)

		// Validate chain.
		chain := models.Chain(strings.ToUpper(req.Chain))
		if !isValidChain(chain) {
			slog.Warn("invalid chain for scan start", "chain", req.Chain)
			writeError(w, http.StatusBadRequest, config.ErrorInvalidChain, "invalid chain: "+req.Chain+", must be BTC, BSC, or SOL")
			return
		}

		// Validate maxId.
		if req.MaxID <= 0 {
			slog.Warn("invalid maxId for scan start", "maxId", req.MaxID)
			writeError(w, http.StatusBadRequest, config.ErrorScanFailed, "maxId must be greater than 0")
			return
		}
		if req.MaxID > config.MaxAddressesPerChain {
			slog.Warn("maxId exceeds maximum",
				"maxId", req.MaxID,
				"max", config.MaxAddressesPerChain,
			)
			writeError(w, http.StatusBadRequest, config.ErrorScanFailed, fmt.Sprintf("maxId must not exceed %d", config.MaxAddressesPerChain))
			return
		}

		// Start the scan.
		if err := sc.StartScan(r.Context(), chain, req.MaxID); err != nil {
			if errors.Is(err, config.ErrScanAlreadyRunning) {
				slog.Warn("scan already running",
					"chain", chain,
				)
				writeError(w, http.StatusConflict, config.ErrorScanFailed, "scan already running for "+string(chain))
				return
			}
			slog.Error("failed to start scan",
				"chain", chain,
				"maxId", req.MaxID,
				"error", err,
			)
			writeError(w, http.StatusInternalServerError, config.ErrorScanFailed, "failed to start scan: "+err.Error())
			return
		}

		elapsed := time.Since(start).Milliseconds()

		slog.Info("scan started successfully",
			"chain", chain,
			"maxId", req.MaxID,
			"elapsed_ms", elapsed,
		)

		writeJSON(w, http.StatusOK, models.APIResponse{
			Data: map[string]interface{}{
				"message": "scan started",
				"chain":   chain,
				"maxId":   req.MaxID,
			},
		})
	}
}

// StopScan handles POST /api/scan/stop.
func StopScan(sc *scanner.Scanner) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		var req stopScanRequest
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			slog.Warn("invalid stop scan request body",
				"error", err,
				"remoteAddr", r.RemoteAddr,
			)
			writeError(w, http.StatusBadRequest, config.ErrorScanFailed, "invalid request body")
			return
		}

		slog.Info("stop scan requested",
			"chain", req.Chain,
			"remoteAddr", r.RemoteAddr,
		)

		chain := models.Chain(strings.ToUpper(req.Chain))
		if !isValidChain(chain) {
			slog.Warn("invalid chain for scan stop", "chain", req.Chain)
			writeError(w, http.StatusBadRequest, config.ErrorInvalidChain, "invalid chain: "+req.Chain+", must be BTC, BSC, or SOL")
			return
		}

		sc.StopScan(chain)

		slog.Info("scan stop signal sent", "chain", chain)

		writeJSON(w, http.StatusOK, models.APIResponse{
			Data: map[string]interface{}{
				"message": "scan stop requested",
				"chain":   chain,
			},
		})
	}
}

// GetScanStatus handles GET /api/scan/status.
func GetScanStatus(sc *scanner.Scanner, database *db.DB) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		start := time.Now()
		chainParam := strings.ToUpper(r.URL.Query().Get("chain"))

		slog.Debug("scan status requested",
			"chain", chainParam,
			"remoteAddr", r.RemoteAddr,
		)

		// Fetch funded counts from DB to include in scan status.
		fundedCounts, err := database.GetFundedCountByChain()
		if err != nil {
			slog.Warn("failed to fetch funded counts for scan status", "error", err)
			fundedCounts = make(map[models.Chain]int)
		}

		if chainParam != "" {
			// Single chain status.
			chain := models.Chain(chainParam)
			if !isValidChain(chain) {
				writeError(w, http.StatusBadRequest, config.ErrorInvalidChain, "invalid chain: "+chainParam)
				return
			}

			status := buildScanStatus(sc, chain, fundedCounts[chain])
			elapsed := time.Since(start).Milliseconds()

			slog.Debug("single chain scan status returned",
				"chain", chain,
				"status", status.Status,
				"isRunning", status.IsRunning,
				"elapsed_ms", elapsed,
			)

			writeJSON(w, http.StatusOK, models.APIResponse{
				Data: status,
				Meta: &models.APIMeta{ExecutionTime: elapsed},
			})
			return
		}

		// All chains status.
		result := make(map[string]*scanStatusResponse, len(models.AllChains))
		for _, chain := range models.AllChains {
			result[string(chain)] = buildScanStatus(sc, chain, fundedCounts[chain])
		}

		elapsed := time.Since(start).Milliseconds()

		slog.Debug("all chains scan status returned",
			"elapsed_ms", elapsed,
		)

		writeJSON(w, http.StatusOK, models.APIResponse{
			Data: result,
			Meta: &models.APIMeta{ExecutionTime: elapsed},
		})
	}
}

// buildScanStatus builds a scanStatusResponse for a chain.
func buildScanStatus(sc *scanner.Scanner, chain models.Chain, fundedCount int) *scanStatusResponse {
	state := sc.Status(chain)
	isRunning := sc.IsRunning(chain)

	if state == nil {
		return &scanStatusResponse{
			Chain:       chain,
			Status:      "idle",
			IsRunning:   isRunning,
			FundedCount: fundedCount,
		}
	}

	return &scanStatusResponse{
		Chain:            state.Chain,
		LastScannedIndex: state.LastScannedIndex,
		MaxScanID:        state.MaxScanID,
		Status:           state.Status,
		StartedAt:        state.StartedAt,
		UpdatedAt:        state.UpdatedAt,
		IsRunning:        isRunning,
		FundedCount:      fundedCount,
	}
}

// ScanSSE handles GET /api/scan/sse — Server-Sent Events stream.
// Sends scan_state snapshots on connect for client resync (B10).
func ScanSSE(hub *scanner.SSEHub, sc *scanner.Scanner, database *db.DB) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		flusher, ok := w.(http.Flusher)
		if !ok {
			slog.Error("SSE not supported: response writer does not implement http.Flusher")
			writeError(w, http.StatusInternalServerError, config.ErrorScanFailed, "streaming not supported")
			return
		}

		slog.Info("SSE client connecting",
			"remoteAddr", r.RemoteAddr,
		)

		// Set SSE headers.
		w.Header().Set("Content-Type", "text/event-stream")
		w.Header().Set("Cache-Control", "no-cache")
		w.Header().Set("Connection", "keep-alive")
		w.Header().Set("X-Accel-Buffering", "no")
		w.WriteHeader(http.StatusOK)
		flusher.Flush()

		// Subscribe to SSE hub.
		ch := hub.Subscribe()
		defer func() {
			hub.Unsubscribe(ch)
			slog.Info("SSE client disconnected",
				"remoteAddr", r.RemoteAddr,
			)
		}()

		slog.Info("SSE client connected",
			"remoteAddr", r.RemoteAddr,
			"totalClients", hub.ClientCount(),
		)

		// Fetch funded counts for SSE snapshots.
		fundedCounts, err := database.GetFundedCountByChain()
		if err != nil {
			slog.Warn("failed to fetch funded counts for SSE snapshot", "error", err)
			fundedCounts = make(map[models.Chain]int)
		}

		// Send scan_state snapshots for all chains on connect (B10 resync).
		for _, chain := range models.AllChains {
			state := sc.Status(chain)
			isRunning := sc.IsRunning(chain)

			snapshot := scanner.ScanStateSnapshotData{
				Chain:       string(chain),
				IsRunning:   isRunning,
				FundedCount: fundedCounts[chain],
			}
			if state != nil {
				snapshot.LastScannedIndex = state.LastScannedIndex
				snapshot.MaxScanID = state.MaxScanID
				snapshot.Status = state.Status
			} else {
				snapshot.Status = "idle"
			}

			data, err := json.Marshal(snapshot)
			if err != nil {
				slog.Error("failed to marshal scan_state snapshot",
					"chain", chain,
					"error", err,
				)
				continue
			}
			fmt.Fprintf(w, "event: scan_state\ndata: %s\n\n", string(data))
		}
		flusher.Flush()

		slog.Debug("SSE resync snapshots sent",
			"remoteAddr", r.RemoteAddr,
			"chains", len(models.AllChains),
		)

		keepAlive := time.NewTicker(config.SSEKeepAliveInterval)
		defer keepAlive.Stop()

		for {
			select {
			case event, ok := <-ch:
				if !ok {
					// Channel closed (hub shutdown).
					slog.Info("SSE channel closed, ending stream",
						"remoteAddr", r.RemoteAddr,
					)
					return
				}

				data, err := json.Marshal(event.Data)
				if err != nil {
					slog.Error("failed to marshal SSE event data",
						"type", event.Type,
						"error", err,
					)
					continue
				}

				// Write SSE formatted event.
				fmt.Fprintf(w, "event: %s\ndata: %s\n\n", event.Type, string(data))
				flusher.Flush()

				slog.Debug("SSE event sent",
					"type", event.Type,
					"remoteAddr", r.RemoteAddr,
				)

			case <-keepAlive.C:
				// Send keepalive comment to prevent connection timeout.
				fmt.Fprint(w, ": keepalive\n\n")
				flusher.Flush()

				slog.Debug("SSE keepalive sent",
					"remoteAddr", r.RemoteAddr,
				)

			case <-r.Context().Done():
				slog.Info("SSE client context done",
					"remoteAddr", r.RemoteAddr,
					"reason", r.Context().Err(),
				)
				return
			}
		}
	}
}
