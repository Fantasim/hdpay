-- Poller initial schema: watches, points, transactions, ip_allowlist, system_errors

CREATE TABLE IF NOT EXISTS schema_version (
    version INTEGER PRIMARY KEY
);

-- Active and historical watches
CREATE TABLE watches (
    id            TEXT PRIMARY KEY,
    chain         TEXT NOT NULL,
    address       TEXT NOT NULL,
    status        TEXT NOT NULL,
    started_at    DATETIME NOT NULL,
    expires_at    DATETIME NOT NULL,
    completed_at  DATETIME,
    poll_count    INTEGER NOT NULL DEFAULT 0,
    last_poll_at  DATETIME,
    last_poll_result TEXT,
    created_at    DATETIME DEFAULT CURRENT_TIMESTAMP
);

-- Points ledger per address (one row per unique address+chain)
CREATE TABLE points (
    address       TEXT NOT NULL,
    chain         TEXT NOT NULL,
    unclaimed     INTEGER NOT NULL DEFAULT 0,
    pending       INTEGER NOT NULL DEFAULT 0,
    total         INTEGER NOT NULL DEFAULT 0,
    updated_at    DATETIME DEFAULT CURRENT_TIMESTAMP,
    PRIMARY KEY (address, chain)
);

-- Individual transactions (dedup by tx_hash + audit trail)
CREATE TABLE transactions (
    id            INTEGER PRIMARY KEY AUTOINCREMENT,
    watch_id      TEXT NOT NULL REFERENCES watches(id),
    tx_hash       TEXT NOT NULL UNIQUE,
    chain         TEXT NOT NULL,
    address       TEXT NOT NULL,
    token         TEXT NOT NULL,
    amount_raw    TEXT NOT NULL,
    amount_human  TEXT NOT NULL,
    decimals      INTEGER NOT NULL,
    usd_value     REAL NOT NULL,
    usd_price     REAL NOT NULL,
    tier          INTEGER NOT NULL,
    multiplier    REAL NOT NULL,
    points        INTEGER NOT NULL,
    status        TEXT NOT NULL,
    confirmations INTEGER NOT NULL DEFAULT 0,
    block_number  INTEGER,
    detected_at   DATETIME NOT NULL,
    confirmed_at  DATETIME,
    created_at    DATETIME DEFAULT CURRENT_TIMESTAMP
);

-- IP allowlist (managed from dashboard, read by middleware)
CREATE TABLE ip_allowlist (
    id            INTEGER PRIMARY KEY AUTOINCREMENT,
    ip            TEXT NOT NULL UNIQUE,
    description   TEXT,
    added_at      DATETIME DEFAULT CURRENT_TIMESTAMP
);

-- System errors / discrepancies (for error dashboard page)
CREATE TABLE system_errors (
    id            INTEGER PRIMARY KEY AUTOINCREMENT,
    severity      TEXT NOT NULL,
    category      TEXT NOT NULL,
    message       TEXT NOT NULL,
    details       TEXT,
    resolved      BOOLEAN NOT NULL DEFAULT FALSE,
    created_at    DATETIME DEFAULT CURRENT_TIMESTAMP
);

-- Indexes
CREATE INDEX idx_transactions_created_at ON transactions(created_at);
CREATE INDEX idx_transactions_chain ON transactions(chain);
CREATE INDEX idx_transactions_address ON transactions(address);
CREATE INDEX idx_transactions_status ON transactions(status);
CREATE INDEX idx_transactions_detected_at ON transactions(detected_at);
CREATE INDEX idx_watches_status ON watches(status);
CREATE INDEX idx_watches_address ON watches(address);
CREATE INDEX idx_watches_expires_at ON watches(expires_at);
CREATE INDEX idx_points_unclaimed ON points(unclaimed) WHERE unclaimed > 0;
CREATE INDEX idx_points_pending ON points(pending) WHERE pending > 0;
CREATE INDEX idx_system_errors_resolved ON system_errors(resolved) WHERE resolved = FALSE;
CREATE INDEX idx_system_errors_category ON system_errors(category);
