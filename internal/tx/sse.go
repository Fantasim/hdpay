package tx

import (
	"context"
	"log/slog"
	"sync"

	"github.com/Fantasim/hdpay/internal/config"
)

// TxEvent represents an SSE event for transaction status updates.
type TxEvent struct {
	Type string      `json:"type"` // "tx_status", "tx_complete", "tx_error"
	Data interface{} `json:"data"` // JSON-serializable payload
}

// TxStatusData is the payload for tx_status events (per-TX progress).
type TxStatusData struct {
	Chain        string `json:"chain"`
	Token        string `json:"token"`
	AddressIndex int    `json:"addressIndex"`
	FromAddress  string `json:"fromAddress"`
	TxHash       string `json:"txHash"`
	Status       string `json:"status"` // "pending", "broadcasting", "success", "confirmed", "failed"
	Amount       string `json:"amount"`
	Error        string `json:"error,omitempty"`
	Current      int    `json:"current"` // 1-based index of current TX
	Total        int    `json:"total"`   // total TXs in sweep
}

// TxCompleteData is the payload for tx_complete events (sweep finished).
type TxCompleteData struct {
	Chain        string         `json:"chain"`
	Token        string         `json:"token"`
	SuccessCount int            `json:"successCount"`
	FailCount    int            `json:"failCount"`
	TotalSwept   string         `json:"totalSwept"`
	TxResults    []TxStatusData `json:"txResults"` // full per-TX results for completion view
}

// TxErrorData is the payload for tx_error events.
type TxErrorData struct {
	Chain   string `json:"chain"`
	Error   string `json:"error"`
	Message string `json:"message"`
}

// TxSSEHub manages fan-out broadcasting of transaction events to connected SSE clients.
type TxSSEHub struct {
	clients map[chan TxEvent]struct{}
	mu      sync.RWMutex
}

// NewTxSSEHub creates a new TX SSE event hub.
func NewTxSSEHub() *TxSSEHub {
	slog.Info("TX SSE hub created")
	return &TxSSEHub{
		clients: make(map[chan TxEvent]struct{}),
	}
}

// Run starts the hub's background processing. Blocks until ctx is cancelled.
func (h *TxSSEHub) Run(ctx context.Context) {
	slog.Info("TX SSE hub running")
	<-ctx.Done()

	h.mu.Lock()
	defer h.mu.Unlock()

	for ch := range h.clients {
		close(ch)
		delete(h.clients, ch)
	}

	slog.Info("TX SSE hub stopped", "reason", ctx.Err())
}

// Subscribe registers a new client and returns a channel to receive events.
func (h *TxSSEHub) Subscribe() chan TxEvent {
	ch := make(chan TxEvent, config.TxSSEHubBuffer)

	h.mu.Lock()
	h.clients[ch] = struct{}{}
	clientCount := len(h.clients)
	h.mu.Unlock()

	slog.Info("TX SSE client subscribed", "totalClients", clientCount)

	return ch
}

// Unsubscribe removes a client and closes its channel.
func (h *TxSSEHub) Unsubscribe(ch chan TxEvent) {
	h.mu.Lock()
	if _, ok := h.clients[ch]; ok {
		delete(h.clients, ch)
		close(ch)
	}
	clientCount := len(h.clients)
	h.mu.Unlock()

	slog.Info("TX SSE client unsubscribed", "totalClients", clientCount)
}

// Broadcast sends an event to all connected clients.
// Non-blocking: if a client's channel is full, the event is dropped for that client.
func (h *TxSSEHub) Broadcast(event TxEvent) {
	h.mu.RLock()
	defer h.mu.RUnlock()

	for ch := range h.clients {
		select {
		case ch <- event:
		default:
			slog.Warn("TX SSE event dropped for slow client",
				"eventType", event.Type,
			)
		}
	}

	slog.Debug("TX SSE event broadcast",
		"type", event.Type,
		"clients", len(h.clients),
	)
}

// ClientCount returns the number of connected TX SSE clients.
func (h *TxSSEHub) ClientCount() int {
	h.mu.RLock()
	defer h.mu.RUnlock()
	return len(h.clients)
}
