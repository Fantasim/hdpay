package scanner

import (
	"context"
	"log/slog"
	"sync"

	"github.com/Fantasim/hdpay/internal/config"
)

// Event represents an SSE event to broadcast to connected clients.
type Event struct {
	Type string      `json:"type"` // "scan_progress", "scan_complete", "scan_error"
	Data interface{} `json:"data"` // JSON-serializable payload
}

// ScanProgressData is the payload for scan_progress events.
type ScanProgressData struct {
	Chain   string `json:"chain"`
	Scanned int    `json:"scanned"`
	Total   int    `json:"total"`
	Found   int    `json:"found"`
	Elapsed string `json:"elapsed"`
}

// ScanCompleteData is the payload for scan_complete events.
type ScanCompleteData struct {
	Chain    string `json:"chain"`
	Scanned int    `json:"scanned"`
	Found   int    `json:"found"`
	Duration string `json:"duration"`
}

// ScanErrorData is the payload for scan_error events.
type ScanErrorData struct {
	Chain   string `json:"chain"`
	Error   string `json:"error"`
	Message string `json:"message"`
}

// ScanTokenErrorData is the payload for scan_token_error events (B7).
// Emitted when a token scan fails but native scan continues.
type ScanTokenErrorData struct {
	Chain   string `json:"chain"`
	Token   string `json:"token"`
	Error   string `json:"error"`
	Message string `json:"message"`
}

// ScanStateSnapshotData is the payload for scan_state events (B10).
// Sent to newly connected SSE clients so they can resync.
type ScanStateSnapshotData struct {
	Chain            string `json:"chain"`
	LastScannedIndex int    `json:"lastScannedIndex"`
	MaxScanID        int    `json:"maxScanId"`
	Status           string `json:"status"`
	IsRunning        bool   `json:"isRunning"`
	FundedCount      int    `json:"fundedCount"`
}

// SSEHub manages fan-out broadcasting of events to connected SSE clients.
type SSEHub struct {
	clients map[chan Event]struct{}
	mu      sync.RWMutex
}

// NewSSEHub creates a new SSE event hub.
func NewSSEHub() *SSEHub {
	slog.Info("SSE hub created")
	return &SSEHub{
		clients: make(map[chan Event]struct{}),
	}
}

// Run starts the hub's background processing. Blocks until ctx is cancelled.
func (h *SSEHub) Run(ctx context.Context) {
	slog.Info("SSE hub running")
	<-ctx.Done()

	h.mu.Lock()
	defer h.mu.Unlock()

	// Close all client channels on shutdown.
	for ch := range h.clients {
		close(ch)
		delete(h.clients, ch)
	}

	slog.Info("SSE hub stopped", "reason", ctx.Err())
}

// Subscribe registers a new client and returns a channel to receive events.
func (h *SSEHub) Subscribe() chan Event {
	ch := make(chan Event, config.SSEHubChannelBuffer)

	h.mu.Lock()
	h.clients[ch] = struct{}{}
	clientCount := len(h.clients)
	h.mu.Unlock()

	slog.Info("SSE client subscribed", "totalClients", clientCount)

	return ch
}

// Unsubscribe removes a client and closes its channel.
func (h *SSEHub) Unsubscribe(ch chan Event) {
	h.mu.Lock()
	if _, ok := h.clients[ch]; ok {
		delete(h.clients, ch)
		close(ch)
	}
	clientCount := len(h.clients)
	h.mu.Unlock()

	slog.Info("SSE client unsubscribed", "totalClients", clientCount)
}

// Broadcast sends an event to all connected clients.
// Non-blocking: if a client's channel is full, the event is dropped for that client.
func (h *SSEHub) Broadcast(event Event) {
	h.mu.RLock()
	defer h.mu.RUnlock()

	for ch := range h.clients {
		select {
		case ch <- event:
		default:
			slog.Warn("SSE event dropped for slow client",
				"eventType", event.Type,
			)
		}
	}

	slog.Debug("SSE event broadcast",
		"type", event.Type,
		"clients", len(h.clients),
	)
}

// ClientCount returns the number of connected clients.
func (h *SSEHub) ClientCount() int {
	h.mu.RLock()
	defer h.mu.RUnlock()
	return len(h.clients)
}
