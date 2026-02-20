CREATE TABLE IF NOT EXISTS addresses (
    chain TEXT NOT NULL,
    address_index INTEGER NOT NULL,
    address TEXT NOT NULL,
    created_at TEXT NOT NULL DEFAULT (datetime('now')),
    PRIMARY KEY (chain, address_index)
);
CREATE INDEX IF NOT EXISTS idx_addresses_chain ON addresses(chain);
CREATE INDEX IF NOT EXISTS idx_addresses_address ON addresses(address);

CREATE TABLE IF NOT EXISTS balances (
    chain TEXT NOT NULL,
    address_index INTEGER NOT NULL,
    token TEXT NOT NULL DEFAULT 'NATIVE',
    balance TEXT NOT NULL DEFAULT '0',
    last_scanned TEXT,
    PRIMARY KEY (chain, address_index, token)
);
CREATE INDEX IF NOT EXISTS idx_balances_nonzero ON balances(chain, token) WHERE balance != '0';

CREATE TABLE IF NOT EXISTS scan_state (
    chain TEXT PRIMARY KEY,
    last_scanned_index INTEGER NOT NULL DEFAULT 0,
    max_scan_id INTEGER NOT NULL DEFAULT 0,
    status TEXT NOT NULL DEFAULT 'idle',
    started_at TEXT,
    updated_at TEXT
);

CREATE TABLE IF NOT EXISTS transactions (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    chain TEXT NOT NULL,
    address_index INTEGER NOT NULL,
    tx_hash TEXT NOT NULL,
    direction TEXT NOT NULL,
    token TEXT NOT NULL DEFAULT 'NATIVE',
    amount TEXT NOT NULL,
    from_address TEXT NOT NULL,
    to_address TEXT NOT NULL,
    block_number INTEGER,
    status TEXT NOT NULL DEFAULT 'pending',
    created_at TEXT NOT NULL DEFAULT (datetime('now')),
    confirmed_at TEXT
);
CREATE INDEX IF NOT EXISTS idx_transactions_chain ON transactions(chain);
CREATE INDEX IF NOT EXISTS idx_transactions_hash ON transactions(tx_hash);

CREATE TABLE IF NOT EXISTS settings (
    key TEXT PRIMARY KEY,
    value TEXT NOT NULL,
    updated_at TEXT NOT NULL DEFAULT (datetime('now'))
);
