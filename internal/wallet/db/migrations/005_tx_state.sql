CREATE TABLE IF NOT EXISTS tx_state (
    id TEXT PRIMARY KEY,
    sweep_id TEXT NOT NULL,
    chain TEXT NOT NULL,
    token TEXT NOT NULL,
    address_index INTEGER NOT NULL,
    from_address TEXT NOT NULL,
    to_address TEXT NOT NULL,
    amount TEXT NOT NULL,
    tx_hash TEXT,
    nonce INTEGER,
    status TEXT NOT NULL DEFAULT 'pending',
    created_at TEXT NOT NULL DEFAULT (datetime('now')),
    updated_at TEXT NOT NULL DEFAULT (datetime('now')),
    error TEXT
);
CREATE INDEX IF NOT EXISTS idx_tx_state_status ON tx_state(status);
CREATE INDEX IF NOT EXISTS idx_tx_state_sweep ON tx_state(sweep_id, status);
CREATE INDEX IF NOT EXISTS idx_tx_state_chain ON tx_state(chain, status);
CREATE INDEX IF NOT EXISTS idx_tx_state_nonce ON tx_state(chain, from_address, nonce);
