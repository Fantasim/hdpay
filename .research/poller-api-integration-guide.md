# HDPay Poller — Backend Integration Guide

> **Audience**: AI agents and backend developers integrating cryptocurrency payments into a game server (MMORPG).
> **Language**: Go examples throughout. The Poller itself is a Go binary.

---

## What Is the Poller?

The Poller is a **self-hosted payment detection service**. You give it a blockchain address to watch, and it polls free-tier blockchain APIs until it detects an incoming transaction. When it finds one, it converts the crypto amount to USD, applies a tiered multiplier, and awards **points**.

Your game server talks to the Poller over HTTP on localhost. The flow is:

```
Player wants to buy 1000 gold
    → Game server generates/assigns a crypto address for that player
    → Game server tells Poller: "watch this address for 30 minutes"
    → Poller polls the blockchain every 15-60 seconds
    → Player sends crypto to the address
    → Poller detects the transaction, calculates USD value, awards points
    → Game server polls Poller: "any unclaimed points for this address?"
    → Game server claims the points and credits the player's account
```

**Supported chains**: BTC, BSC (BNB + USDC + USDT), SOL (SOL + USDC + USDT)

---

## Architecture Overview

```
┌─────────────────┐         HTTP (localhost)         ┌──────────────────┐
│                 │  ──── POST /api/watch ──────────→ │                  │
│   Game Server   │  ──── GET  /api/points ────────→ │   HDPay Poller   │
│   (your code)   │  ──── POST /api/points/claim ──→ │   (port 8081)    │
│                 │  ←─── JSON responses ─────────── │                  │
└─────────────────┘                                   └────────┬─────────┘
                                                               │
                                                               │ polls free APIs
                                                               ▼
                                                    ┌──────────────────────┐
                                                    │  Blockchain Networks │
                                                    │  BTC / BSC / SOL     │
                                                    └──────────────────────┘
```

Both services run on the same machine. The Poller binds to `localhost:8081` by default. Your game server calls it over `http://127.0.0.1:8081`.

---

## Running the Poller

### Prerequisites

The Poller is a single binary. It needs:

1. A `.env.poller` file (or environment variables)
2. A `tiers.json` file for point multiplier configuration
3. Write access to create a `./data/` directory (SQLite database) and `./logs/` directory

### Environment Variables

Create `.env.poller` in the same directory as the binary:

```env
# Required
POLLER_START_DATE=1709251200          # Unix timestamp — transactions before this date are ignored
POLLER_ADMIN_USERNAME=admin           # Dashboard login
POLLER_ADMIN_PASSWORD=your-secret     # Dashboard login (use a strong password)

# Network — "mainnet" for real money, "testnet" for development
POLLER_NETWORK=mainnet

# Optional
POLLER_PORT=8081                      # Default: 8081
POLLER_DB_PATH=./data/poller.sqlite   # Default: ./data/poller.sqlite
POLLER_LOG_LEVEL=info                 # debug, info, warn, error
POLLER_LOG_DIR=./logs                 # Default: ./logs
POLLER_MAX_ACTIVE_WATCHES=100         # Max concurrent watches (default: 100)
POLLER_DEFAULT_WATCH_TIMEOUT_MIN=30   # Default watch duration (default: 30, max: 120)
POLLER_TIERS_FILE=./tiers.json        # Points tier config file

# Optional API keys (improves reliability with more providers)
POLLER_HELIUS_API_KEY=                # Solana RPC (free tier)
POLLER_ALCHEMY_API_KEY=               # Multi-chain RPC
POLLER_NODEREAL_API_KEY=              # BSC RPC
```

### Tier Configuration

Create `tiers.json` — this controls how many points each USD of crypto translates to:

```json
[
  { "min_usd": 0,    "max_usd": 1,    "multiplier": 0.0 },
  { "min_usd": 1,    "max_usd": 12,   "multiplier": 1.0 },
  { "min_usd": 12,   "max_usd": 30,   "multiplier": 1.1 },
  { "min_usd": 30,   "max_usd": 60,   "multiplier": 1.2 },
  { "min_usd": 60,   "max_usd": 120,  "multiplier": 1.3 },
  { "min_usd": 120,  "max_usd": 240,  "multiplier": 1.4 },
  { "min_usd": 240,  "max_usd": 600,  "multiplier": 1.5 },
  { "min_usd": 600,  "max_usd": 1200, "multiplier": 2.0 },
  { "min_usd": 1200, "max_usd": null,  "multiplier": 3.0 }
]
```

**How points are calculated**: `points = round(floor(usd * 100) * multiplier)`

Example: A $50 SOL payment → tier `$30-$60` → multiplier `1.2` → `round(5000 * 1.2)` = **6000 points**.

The first tier (`$0-$1`, multiplier `0.0`) filters out dust transactions. The last tier must have `max_usd: null` (unbounded).

### Starting the Poller

```bash
./poller
# Poller starts on http://127.0.0.1:8081
# Dashboard at http://127.0.0.1:8081/ (browser)
# API at http://127.0.0.1:8081/api/
```

---

## API Reference

### Base URL

```
http://127.0.0.1:8081/api
```

### Authentication

The API uses **IP-based authentication**. Requests from `127.0.0.1`, `::1`, and private network ranges (`10.0.0.0/8`, `172.16.0.0/12`, `192.168.0.0/16`) are always allowed.

For your game server running on the same machine — **no authentication headers are needed**. Just call the endpoints directly.

If your game server runs on a different machine, you must add its IP to the allowlist via the admin dashboard or the admin API.

### Response Format

**Success**:
```json
{
  "watch_id": "...",
  "chain": "SOL",
  "status": "ACTIVE"
}
```

**Error**:
```json
{
  "error": {
    "code": "ERROR_ALREADY_WATCHING",
    "message": "Address is already being watched"
  }
}
```

All timestamps are RFC 3339 format: `"2026-03-03T14:30:00Z"`.

---

### Core Endpoints (What Your Game Server Uses)

#### 1. Health Check

```
GET /api/health
```

No authentication. Use this to verify the Poller is running before your game server starts accepting payments.

**Response** `200`:
```json
{
  "status": "ok",
  "network": "mainnet",
  "uptime": "2d 5h 30m",
  "version": "1.0.0",
  "active_watches": 12
}
```

**Go example**:

```go
func checkPollerHealth(pollerURL string) error {
	resp, err := http.Get(pollerURL + "/api/health")
	if err != nil {
		return fmt.Errorf("poller unreachable: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("poller unhealthy: status %d", resp.StatusCode)
	}

	var health struct {
		Status        string `json:"status"`
		ActiveWatches int    `json:"active_watches"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&health); err != nil {
		return fmt.Errorf("invalid health response: %w", err)
	}

	if health.Status != "ok" {
		return fmt.Errorf("poller status: %s", health.Status)
	}
	return nil
}
```

---

#### 2. Create a Watch

```
POST /api/watch
```

Tell the Poller to start monitoring a blockchain address for incoming transactions.

**Request body**:
```json
{
  "chain": "SOL",
  "address": "3Cy3YNTFywCmxoxt8n7UH6hg6dLo5uACowX3CFceaSnx",
  "timeout_minutes": 30
}
```

| Field | Type | Required | Description |
|-------|------|----------|-------------|
| `chain` | string | yes | `"BTC"`, `"BSC"`, or `"SOL"` |
| `address` | string | yes | Valid blockchain address for the specified chain |
| `timeout_minutes` | int | no | How long to watch (1-120 min). Default: 30 |

**Response** `201`:
```json
{
  "watch_id": "a1b2c3d4-e5f6-7890-abcd-ef1234567890",
  "chain": "SOL",
  "address": "3Cy3YNTFywCmxoxt8n7UH6hg6dLo5uACowX3CFceaSnx",
  "status": "ACTIVE",
  "started_at": "2026-03-03T14:00:00Z",
  "expires_at": "2026-03-03T14:30:00Z",
  "poll_interval_seconds": 15
}
```

**Error codes**:
| HTTP | Code | When |
|------|------|------|
| 400 | `ERROR_INVALID_CHAIN` | Chain is not BTC/BSC/SOL |
| 400 | `ERROR_ADDRESS_INVALID` | Address format doesn't match chain |
| 400 | `ERROR_INVALID_TIMEOUT` | Timeout outside 1-120 range |
| 409 | `ERROR_ALREADY_WATCHING` | This address already has an active watch |
| 429 | `ERROR_MAX_WATCHES` | Too many concurrent watches (default limit: 100) |
| 500 | `ERROR_PROVIDER_UNAVAILABLE` | No blockchain providers available |

**Polling intervals by chain**:
| Chain | Interval | Block time | Confirmation threshold |
|-------|----------|------------|----------------------|
| BTC | 60 seconds | ~10 minutes | 1 confirmation |
| BSC | 15 seconds | ~3 seconds | 12 confirmations |
| SOL | 15 seconds | ~0.4 seconds | Finalized commitment |

**Go example**:

```go
type CreateWatchRequest struct {
	Chain          string `json:"chain"`
	Address        string `json:"address"`
	TimeoutMinutes int    `json:"timeout_minutes,omitempty"`
}

type CreateWatchResponse struct {
	WatchID             string `json:"watch_id"`
	Chain               string `json:"chain"`
	Address             string `json:"address"`
	Status              string `json:"status"`
	StartedAt           string `json:"started_at"`
	ExpiresAt           string `json:"expires_at"`
	PollIntervalSeconds int    `json:"poll_interval_seconds"`
}

func createWatch(pollerURL, chain, address string, timeoutMin int) (*CreateWatchResponse, error) {
	body, _ := json.Marshal(CreateWatchRequest{
		Chain:          chain,
		Address:        address,
		TimeoutMinutes: timeoutMin,
	})

	resp, err := http.Post(pollerURL+"/api/watch", "application/json", bytes.NewReader(body))
	if err != nil {
		return nil, fmt.Errorf("request failed: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusCreated {
		var errResp struct {
			Error struct {
				Code    string `json:"code"`
				Message string `json:"message"`
			} `json:"error"`
		}
		json.NewDecoder(resp.Body).Decode(&errResp)
		return nil, fmt.Errorf("watch creation failed [%s]: %s", errResp.Error.Code, errResp.Error.Message)
	}

	var watch CreateWatchResponse
	if err := json.NewDecoder(resp.Body).Decode(&watch); err != nil {
		return nil, fmt.Errorf("invalid response: %w", err)
	}
	return &watch, nil
}
```

---

#### 3. Cancel a Watch

```
DELETE /api/watch/{id}
```

Stop watching an address early (e.g., player cancelled the purchase).

**Response** `200`:
```json
{
  "watch_id": "a1b2c3d4-e5f6-7890-abcd-ef1234567890",
  "status": "CANCELLED"
}
```

**Error codes**:
| HTTP | Code | When |
|------|------|------|
| 404 | `ERROR_WATCH_NOT_FOUND` | Watch ID doesn't exist |
| 409 | `ERROR_WATCH_EXPIRED` | Watch already expired |

**Go example**:

```go
func cancelWatch(pollerURL, watchID string) error {
	req, _ := http.NewRequest(http.MethodDelete, pollerURL+"/api/watch/"+watchID, nil)
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return fmt.Errorf("request failed: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("cancel failed: status %d", resp.StatusCode)
	}
	return nil
}
```

---

#### 4. List Watches

```
GET /api/watches?status=ACTIVE&chain=SOL
```

Get all watches, optionally filtered.

**Query parameters** (all optional):
| Param | Values | Description |
|-------|--------|-------------|
| `status` | `ACTIVE`, `COMPLETED`, `EXPIRED`, `CANCELLED` | Filter by watch status |
| `chain` | `BTC`, `BSC`, `SOL` | Filter by chain |

**Response** `200`:
```json
[
  {
    "id": "a1b2c3d4-e5f6-7890-abcd-ef1234567890",
    "chain": "SOL",
    "address": "3Cy3YNT...",
    "status": "ACTIVE",
    "started_at": "2026-03-03T14:00:00Z",
    "expires_at": "2026-03-03T14:30:00Z",
    "completed_at": null,
    "poll_count": 42,
    "last_poll_at": "2026-03-03T14:10:30Z",
    "last_poll_result": "no new transactions",
    "created_at": "2026-03-03T14:00:00Z"
  }
]
```

Returns an empty array `[]` if no watches match.

---

#### 5. Get Unclaimed Points

```
GET /api/points
```

Returns all addresses that have **unclaimed points** (points earned from confirmed transactions, not yet claimed by your game server). This is the endpoint you poll periodically to detect completed payments.

**Response** `200`:
```json
[
  {
    "address": "3Cy3YNTFywCmxoxt8n7UH6hg6dLo5uACowX3CFceaSnx",
    "chain": "SOL",
    "unclaimed": 6000,
    "total": 6000,
    "transactions": [
      {
        "id": 1,
        "watch_id": "a1b2c3d4-...",
        "tx_hash": "5xR9kP...",
        "chain": "SOL",
        "address": "3Cy3YNT...",
        "token": "SOL",
        "amount_raw": "1000000000",
        "amount_human": "1.000000000",
        "decimals": 9,
        "usd_value": 50.25,
        "usd_price": 50.25,
        "tier": 4,
        "multiplier": 1.2,
        "points": 6030,
        "status": "CONFIRMED",
        "confirmations": 1,
        "block_number": 287654321,
        "detected_at": "2026-03-03T14:05:00Z",
        "confirmed_at": "2026-03-03T14:05:15Z",
        "created_at": "2026-03-03T14:05:00Z"
      }
    ]
  }
]
```

Returns an empty array `[]` if no addresses have unclaimed points.

**Key fields**:
- `unclaimed`: Points ready to be claimed (from confirmed transactions only)
- `total`: Lifetime points earned (including already-claimed)
- `transactions`: The confirmed transactions that generated these points

---

#### 6. Get Pending Points

```
GET /api/points/pending
```

Returns addresses with **pending points** — transactions detected but not yet confirmed on the blockchain. Useful for showing "payment detected, confirming..." in your game UI.

**Response** `200`:
```json
[
  {
    "address": "3Cy3YNTFywCmxoxt8n7UH6hg6dLo5uACowX3CFceaSnx",
    "chain": "SOL",
    "pending_points": 6000,
    "transactions": [
      {
        "tx_hash": "5xR9kP...",
        "token": "SOL",
        "amount_raw": "1000000000",
        "amount_human": "1.000000000",
        "usd_value": 50.25,
        "tier": 4,
        "points": 6030,
        "status": "PENDING",
        "confirmations": 0,
        "confirmations_required": 1,
        "detected_at": "2026-03-03T14:05:00Z"
      }
    ]
  }
]
```

---

#### 7. Claim Points

```
POST /api/points/claim
```

**This is how your game server "consumes" the points.** Claiming resets the `unclaimed` counter to 0 for the specified addresses. Your game server should credit the player's in-game account, then call this endpoint to acknowledge receipt.

**Request body**:
```json
{
  "addresses": [
    "3Cy3YNTFywCmxoxt8n7UH6hg6dLo5uACowX3CFceaSnx",
    "bc1qxy2kgdygjrsqtzq2n0yrf2493p83kkfjhx0wlh"
  ]
}
```

| Field | Type | Required | Description |
|-------|------|----------|-------------|
| `addresses` | string[] | yes | Up to 500 addresses to claim |

**Response** `200`:
```json
{
  "claimed": [
    {
      "address": "3Cy3YNTFywCmxoxt8n7UH6hg6dLo5uACowX3CFceaSnx",
      "chain": "SOL",
      "points_claimed": 6000
    }
  ],
  "skipped": [
    "bc1qxy2kgdygjrsqtzq2n0yrf2493p83kkfjhx0wlh"
  ],
  "total_claimed": 6000
}
```

- `claimed`: Addresses where points were successfully claimed (with breakdown by chain)
- `skipped`: Addresses with zero unclaimed points (no-op, not an error)
- `total_claimed`: Sum of all claimed points in this request

**Important**: An address can receive payments on multiple chains. The claim operation checks all chains (BTC, BSC, SOL) for each address and returns separate entries per chain.

**Go example**:

```go
type ClaimRequest struct {
	Addresses []string `json:"addresses"`
}

type ClaimEntry struct {
	Address       string `json:"address"`
	Chain         string `json:"chain"`
	PointsClaimed int    `json:"points_claimed"`
}

type ClaimResponse struct {
	Claimed      []ClaimEntry `json:"claimed"`
	Skipped      []string     `json:"skipped"`
	TotalClaimed int          `json:"total_claimed"`
}

func claimPoints(pollerURL string, addresses []string) (*ClaimResponse, error) {
	body, _ := json.Marshal(ClaimRequest{Addresses: addresses})
	resp, err := http.Post(pollerURL+"/api/points/claim", "application/json", bytes.NewReader(body))
	if err != nil {
		return nil, fmt.Errorf("request failed: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("claim failed: status %d", resp.StatusCode)
	}

	var result ClaimResponse
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return nil, fmt.Errorf("invalid response: %w", err)
	}
	return &result, nil
}
```

---

## Full Integration Example

Here's a complete game server integration pattern:

```go
package payment

import (
	"bytes"
	"encoding/json"
	"fmt"
	"log/slog"
	"net/http"
	"sync"
	"time"
)

const pollerBaseURL = "http://127.0.0.1:8081"

// PaymentSession tracks a player's pending payment.
type PaymentSession struct {
	PlayerID       string
	Chain          string
	Address        string
	WatchID        string
	ItemID         string    // what the player is buying
	PointsExpected int       // minimum points needed
	CreatedAt      time.Time
}

// PaymentService handles the payment lifecycle.
type PaymentService struct {
	mu       sync.RWMutex
	sessions map[string]*PaymentSession // address → session

	// pollerPollInterval controls how often we check for unclaimed points.
	pollerPollInterval time.Duration
}

func NewPaymentService() *PaymentService {
	ps := &PaymentService{
		sessions:           make(map[string]*PaymentSession),
		pollerPollInterval: 10 * time.Second,
	}
	go ps.pollForPayments()
	return ps
}

// StartPayment initiates a payment session for a player.
// The caller must provide a crypto address assigned to this player.
func (ps *PaymentService) StartPayment(playerID, chain, address, itemID string, pointsNeeded int) (*PaymentSession, error) {
	// Tell the Poller to watch this address.
	watchReq, _ := json.Marshal(map[string]interface{}{
		"chain":           chain,
		"address":         address,
		"timeout_minutes": 30,
	})

	resp, err := http.Post(pollerBaseURL+"/api/watch", "application/json", bytes.NewReader(watchReq))
	if err != nil {
		return nil, fmt.Errorf("failed to contact poller: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusCreated {
		var errBody struct {
			Error struct {
				Code    string `json:"code"`
				Message string `json:"message"`
			} `json:"error"`
		}
		json.NewDecoder(resp.Body).Decode(&errBody)
		return nil, fmt.Errorf("watch creation failed [%s]: %s", errBody.Error.Code, errBody.Error.Message)
	}

	var watchResp struct {
		WatchID   string `json:"watch_id"`
		ExpiresAt string `json:"expires_at"`
	}
	json.NewDecoder(resp.Body).Decode(&watchResp)

	session := &PaymentSession{
		PlayerID:       playerID,
		Chain:          chain,
		Address:        address,
		WatchID:        watchResp.WatchID,
		ItemID:         itemID,
		PointsExpected: pointsNeeded,
		CreatedAt:      time.Now(),
	}

	ps.mu.Lock()
	ps.sessions[address] = session
	ps.mu.Unlock()

	slog.Info("payment session started",
		"playerID", playerID,
		"chain", chain,
		"address", address,
		"watchID", watchResp.WatchID,
		"item", itemID,
	)

	return session, nil
}

// pollForPayments runs in the background, checking for completed payments.
func (ps *PaymentService) pollForPayments() {
	ticker := time.NewTicker(ps.pollerPollInterval)
	defer ticker.Stop()

	for range ticker.C {
		ps.checkUnclaimed()
	}
}

func (ps *PaymentService) checkUnclaimed() {
	resp, err := http.Get(pollerBaseURL + "/api/points")
	if err != nil {
		slog.Error("failed to query poller points", "error", err)
		return
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return
	}

	var accounts []struct {
		Address   string `json:"address"`
		Chain     string `json:"chain"`
		Unclaimed int    `json:"unclaimed"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&accounts); err != nil {
		slog.Error("failed to decode points response", "error", err)
		return
	}

	// Find accounts that match our active sessions.
	var toClaim []string
	for _, acct := range accounts {
		ps.mu.RLock()
		session, exists := ps.sessions[acct.Address]
		ps.mu.RUnlock()

		if !exists || acct.Unclaimed <= 0 {
			continue
		}

		slog.Info("payment detected",
			"playerID", session.PlayerID,
			"address", acct.Address,
			"chain", acct.Chain,
			"unclaimed", acct.Unclaimed,
			"expected", session.PointsExpected,
		)

		// Credit the player. This is your game logic.
		ps.creditPlayer(session, acct.Unclaimed)
		toClaim = append(toClaim, acct.Address)
	}

	// Claim all points we just processed (resets unclaimed to 0).
	if len(toClaim) > 0 {
		ps.claimPoints(toClaim)
	}
}

func (ps *PaymentService) creditPlayer(session *PaymentSession, points int) {
	// YOUR GAME LOGIC HERE:
	// - Add gold/gems/items to the player's account
	// - Record the transaction in your game database
	// - Send an in-game notification to the player
	slog.Info("crediting player",
		"playerID", session.PlayerID,
		"points", points,
		"item", session.ItemID,
	)

	// Clean up the session.
	ps.mu.Lock()
	delete(ps.sessions, session.Address)
	ps.mu.Unlock()
}

func (ps *PaymentService) claimPoints(addresses []string) {
	body, _ := json.Marshal(map[string][]string{"addresses": addresses})
	resp, err := http.Post(pollerBaseURL+"/api/points/claim", "application/json", bytes.NewReader(body))
	if err != nil {
		slog.Error("failed to claim points", "error", err, "addresses", addresses)
		return
	}
	defer resp.Body.Close()

	var result struct {
		TotalClaimed int `json:"total_claimed"`
	}
	json.NewDecoder(resp.Body).Decode(&result)
	slog.Info("points claimed", "total", result.TotalClaimed, "addresses", len(addresses))
}

// CancelPayment allows the player to cancel a pending payment.
func (ps *PaymentService) CancelPayment(address string) error {
	ps.mu.RLock()
	session, exists := ps.sessions[address]
	ps.mu.RUnlock()

	if !exists {
		return fmt.Errorf("no active payment session for address %s", address)
	}

	// Cancel the watch in the Poller.
	req, _ := http.NewRequest(http.MethodDelete, pollerBaseURL+"/api/watch/"+session.WatchID, nil)
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return fmt.Errorf("failed to cancel watch: %w", err)
	}
	resp.Body.Close()

	ps.mu.Lock()
	delete(ps.sessions, address)
	ps.mu.Unlock()

	slog.Info("payment cancelled", "playerID", session.PlayerID, "address", address)
	return nil
}

// CheckPending returns pending (unconfirmed) payment info for a player's address.
// Use this to show "Payment detected, waiting for confirmation..." in your game UI.
func (ps *PaymentService) CheckPending(address string) (int, error) {
	resp, err := http.Get(pollerBaseURL + "/api/points/pending")
	if err != nil {
		return 0, fmt.Errorf("failed to query pending points: %w", err)
	}
	defer resp.Body.Close()

	var accounts []struct {
		Address       string `json:"address"`
		PendingPoints int    `json:"pending_points"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&accounts); err != nil {
		return 0, fmt.Errorf("failed to decode response: %w", err)
	}

	for _, acct := range accounts {
		if acct.Address == address {
			return acct.PendingPoints, nil
		}
	}
	return 0, nil
}
```

---

## Tracking Payment Status for a Specific Address

The Poller doesn't have a single "get everything for address X" endpoint, but you can build a complete payment status view by combining three existing endpoints. Here's the pattern:

### Which endpoints give you address-level data

| Endpoint | What it tells you about an address |
|----------|-----------------------------------|
| `GET /api/watches?status=ACTIVE` | Whether the address is currently being watched (find by matching the `address` field in the response array) |
| `GET /api/points/pending` | Whether a payment was detected but not yet confirmed (find by `address` in the response array — includes transaction details, confirmation progress) |
| `GET /api/points` | Whether confirmed points are ready to claim (find by `address` — includes the confirmed transactions with amounts, USD value, points) |

### Building a player payment status screen

Combine these three calls to determine the current state for any address:

```go
// PaymentStatus represents the player-facing payment state.
type PaymentStatus int

const (
	StatusWaiting    PaymentStatus = iota // Watch active, no transaction yet
	StatusDetected                        // Transaction seen, awaiting confirmation
	StatusConfirmed                       // Confirmed, points ready to claim
	StatusClaimed                         // Points claimed, payment complete
	StatusExpired                         // Watch expired with no payment
	StatusNotFound                        // No watch found for this address
)

type PaymentInfo struct {
	Status            PaymentStatus
	PendingPoints     int      // Points awaiting confirmation
	UnclaimedPoints   int      // Confirmed points ready to claim
	TxHash            string   // Transaction hash (if detected)
	Token             string   // What token was sent (BTC/BNB/SOL/USDC/USDT)
	AmountHuman       string   // Human-readable amount (e.g. "1.500000000")
	USDValue          float64  // USD value at time of detection
	Confirmations     int      // Current confirmation count
	ConfirmationsReq  int      // Confirmations required for this chain
}

func (ps *PaymentService) GetPaymentStatus(address string) (*PaymentInfo, error) {
	info := &PaymentInfo{Status: StatusNotFound}

	// 1. Check for confirmed/unclaimed points (payment complete, ready to claim).
	pointsResp, err := http.Get(pollerBaseURL + "/api/points")
	if err != nil {
		return nil, fmt.Errorf("failed to query points: %w", err)
	}
	defer pointsResp.Body.Close()

	var unclaimedAccounts []struct {
		Address      string `json:"address"`
		Unclaimed    int    `json:"unclaimed"`
		Transactions []struct {
			TxHash      string  `json:"tx_hash"`
			Token       string  `json:"token"`
			AmountHuman string  `json:"amount_human"`
			USDValue    float64 `json:"usd_value"`
		} `json:"transactions"`
	}
	json.NewDecoder(pointsResp.Body).Decode(&unclaimedAccounts)

	for _, acct := range unclaimedAccounts {
		if acct.Address == address && acct.Unclaimed > 0 {
			info.Status = StatusConfirmed
			info.UnclaimedPoints = acct.Unclaimed
			if len(acct.Transactions) > 0 {
				tx := acct.Transactions[0]
				info.TxHash = tx.TxHash
				info.Token = tx.Token
				info.AmountHuman = tx.AmountHuman
				info.USDValue = tx.USDValue
			}
			return info, nil
		}
	}

	// 2. Check for pending (detected, not yet confirmed).
	pendingResp, err := http.Get(pollerBaseURL + "/api/points/pending")
	if err != nil {
		return nil, fmt.Errorf("failed to query pending: %w", err)
	}
	defer pendingResp.Body.Close()

	var pendingAccounts []struct {
		Address       string `json:"address"`
		PendingPoints int    `json:"pending_points"`
		Transactions  []struct {
			TxHash            string  `json:"tx_hash"`
			Token             string  `json:"token"`
			AmountHuman       string  `json:"amount_human"`
			USDValue          float64 `json:"usd_value"`
			Confirmations     int     `json:"confirmations"`
			ConfirmationsReq  int     `json:"confirmations_required"`
		} `json:"transactions"`
	}
	json.NewDecoder(pendingResp.Body).Decode(&pendingAccounts)

	for _, acct := range pendingAccounts {
		if acct.Address == address && acct.PendingPoints > 0 {
			info.Status = StatusDetected
			info.PendingPoints = acct.PendingPoints
			if len(acct.Transactions) > 0 {
				tx := acct.Transactions[0]
				info.TxHash = tx.TxHash
				info.Token = tx.Token
				info.AmountHuman = tx.AmountHuman
				info.USDValue = tx.USDValue
				info.Confirmations = tx.Confirmations
				info.ConfirmationsReq = tx.ConfirmationsReq
			}
			return info, nil
		}
	}

	// 3. Check if there's an active watch (still waiting for payment).
	watchResp, err := http.Get(pollerBaseURL + "/api/watches?status=ACTIVE")
	if err != nil {
		return nil, fmt.Errorf("failed to query watches: %w", err)
	}
	defer watchResp.Body.Close()

	var watches []struct {
		Address string `json:"address"`
		Status  string `json:"status"`
	}
	json.NewDecoder(watchResp.Body).Decode(&watches)

	for _, w := range watches {
		if w.Address == address {
			info.Status = StatusWaiting
			return info, nil
		}
	}

	// 4. Check expired watches.
	expiredResp, err := http.Get(pollerBaseURL + "/api/watches?status=EXPIRED")
	if err != nil {
		return nil, fmt.Errorf("failed to query expired watches: %w", err)
	}
	defer expiredResp.Body.Close()

	var expired []struct {
		Address string `json:"address"`
	}
	json.NewDecoder(expiredResp.Body).Decode(&expired)

	for _, w := range expired {
		if w.Address == address {
			info.Status = StatusExpired
			return info, nil
		}
	}

	return info, nil // StatusNotFound
}
```

### What to show the player at each status

| Status | Player-facing message | Data to display |
|--------|----------------------|-----------------|
| `StatusWaiting` | "Waiting for payment..." | Show the address + QR code, countdown timer from watch `expires_at` |
| `StatusDetected` | "Payment detected! Confirming..." | Show `tx_hash`, `amount_human` + `token`, confirmation progress (`confirmations`/`confirmations_required`) |
| `StatusConfirmed` | "Payment confirmed!" | Show `unclaimed_points`, `usd_value`, `token`, `amount_human` |
| `StatusExpired` | "Payment window expired" | Offer to start a new payment session |
| `StatusNotFound` | "No active payment" | Offer to start a new payment |

---

## Transaction Lifecycle

Understanding the states a transaction goes through:

```
Player sends crypto
    │
    ▼
┌─────────┐   Poller detects tx in the next poll cycle (15-60s)
│ PENDING  │   Points added to "pending" ledger
│          │   Poller schedules confirmation check
└────┬─────┘
     │
     │  Blockchain confirms the transaction:
     │    BTC: 1 confirmation (~10 min)
     │    BSC: 12 confirmations (~36 sec)
     │    SOL: finalized (~13 sec)
     │
     ▼
┌───────────┐  Points moved from "pending" to "unclaimed" ledger
│ CONFIRMED │  Appears in GET /api/points response
│           │  Ready for your game server to claim
└────┬──────┘
     │
     │  Your game server calls POST /api/points/claim
     │
     ▼
┌─────────┐  "unclaimed" reset to 0
│ CLAIMED │  Points remain in "total" (lifetime counter)
│         │  Player has been credited in your game
└─────────┘
```

**Timing expectations**:
| Chain | Detection | Confirmation | Total (typical) |
|-------|-----------|-------------|-----------------|
| BTC | 0-60 sec | ~10 min | ~11 min |
| BSC | 0-15 sec | ~36 sec | ~1 min |
| SOL | 0-15 sec | ~13 sec | ~30 sec |

---

## Watch Lifecycle

```
POST /api/watch
    │
    ▼
┌────────┐  Poller is actively polling the blockchain
│ ACTIVE │  poll_count increments every tick
│        │  Adaptive: slows down after 5 empty ticks
└───┬────┘
    │
    ├──→ Timeout reached → EXPIRED (no transactions or transactions still processing)
    ├──→ DELETE /api/watch/{id} → CANCELLED
    └──→ (implicit) → COMPLETED
```

A watch expiring does **not** lose detected transactions. If a transaction was detected and is still PENDING when the watch expires, a background recovery process will continue checking confirmations.

---

## Accepted Tokens by Chain

| Chain | Native Token | Stablecoins |
|-------|-------------|-------------|
| BTC | BTC | — |
| BSC | BNB | USDC, USDT (BEP-20) |
| SOL | SOL | USDC, USDT (SPL) |

The `token` field in transaction responses tells you what was received:
- `"BTC"`, `"BNB"`, `"SOL"` — native tokens (price fetched from CoinGecko)
- `"USDC"`, `"USDT"` — stablecoins (assumed $1.00, no price lookup needed)

---

## Address Validation Rules

The Poller validates address format before accepting a watch:

| Chain | Network | Format |
|-------|---------|--------|
| BTC | mainnet | Starts with `bc1` (bech32/Native SegWit) |
| BTC | testnet | Starts with `tb1` |
| BSC | both | `0x` + 40 hex chars (EIP-55 checksum validated) |
| SOL | both | Base58, 32-44 chars |

---

## Error Handling Best Practices

```go
// Always check these error codes and handle them:

switch errorCode {
case "ERROR_ALREADY_WATCHING":
	// Address is already being watched — safe to ignore,
	// or show "payment already in progress" to the player.

case "ERROR_MAX_WATCHES":
	// Too many concurrent watches. Either wait for some to expire,
	// or increase POLLER_MAX_ACTIVE_WATCHES.
	// Show "payment system busy, try again later" to the player.

case "ERROR_ADDRESS_INVALID":
	// The address format is wrong for the specified chain.
	// This is a bug in your address generation — fix it.

case "ERROR_PROVIDER_UNAVAILABLE":
	// All blockchain API providers for this chain are down.
	// Retry after a delay, or disable that chain temporarily.

case "ERROR_INVALID_CHAIN":
	// Typo in chain name. Must be exactly "BTC", "BSC", or "SOL".
}
```

---

## Recommended Polling Strategy

Your game server needs to poll `GET /api/points` to detect completed payments. Here's a sensible strategy:

```go
// Poll every 10 seconds. This is independent of the Poller's own blockchain polling.
// The Poller handles blockchain polling internally (15-60s per chain).
// Your poll to /api/points just checks the Poller's local database — it's fast.
const gameServerPollInterval = 10 * time.Second
```

There are **no SSE or WebSocket endpoints** — the Poller uses a traditional request/response model. Polling `GET /api/points` every 10 seconds is cheap (it reads from local SQLite) and the recommended approach.

---

## Operational Notes

### Concurrent Watch Limit

Default: 100 concurrent active watches. This means 100 addresses can be monitored simultaneously. For an MMORPG with heavy traffic, you may want to increase this:

```env
POLLER_MAX_ACTIVE_WATCHES=500
```

Each watch spawns one goroutine and makes 1 API call per poll interval per provider, so keep provider rate limits in mind.

### Watch Timeout Strategy

Set the timeout based on your game's UX:
- **30 minutes** (default): Good for most purchases. Player has time to open their wallet app, confirm the transaction, and wait for blockchain confirmation.
- **60 minutes**: For larger purchases where the player might need to transfer funds first.
- **120 minutes** (maximum): For high-value purchases or when dealing with BTC (10+ min confirmation).

If the watch expires before the player pays, they need to initiate a new purchase flow.

### What Happens on Restart

- **Active watches are lost** — the Poller does not persist watch goroutines. After a restart, no addresses are being actively polled.
- **Pending transactions are recovered** — on startup, the Poller scans the database for PENDING transactions and rechecks their confirmation status.
- **Admin sessions are cleared** — sessions are in-memory only.
- **Points ledger is preserved** — all unclaimed/total points survive restarts.

### Logs

Logs are written to `./logs/` with daily rotation:
```
logs/
  poller-2026-03-03-info.log
  poller-2026-03-03-warn.log
  poller-2026-03-03-error.log
  poller-2026-03-03-debug.log    # only if POLLER_LOG_LEVEL=debug
```

### Database

SQLite database at `./data/poller.sqlite`. The Poller manages its own schema migrations automatically on startup. No manual database setup is needed.

---

## Quick Start Checklist

1. Create `.env.poller` with required variables (`POLLER_START_DATE`, `POLLER_ADMIN_USERNAME`, `POLLER_ADMIN_PASSWORD`)
2. Create `tiers.json` with your points multiplier configuration
3. Start the binary: `./poller`
4. Verify it's running: `curl http://127.0.0.1:8081/api/health`
5. From your game server, create watches with `POST /api/watch`
6. Poll `GET /api/points` every ~10 seconds to detect completed payments
7. Claim points with `POST /api/points/claim` after crediting the player
