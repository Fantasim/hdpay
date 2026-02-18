CREATE TABLE IF NOT EXISTS provider_health (
    provider_name TEXT PRIMARY KEY,
    chain TEXT NOT NULL,
    provider_type TEXT NOT NULL DEFAULT 'scan',
    status TEXT NOT NULL DEFAULT 'healthy',
    consecutive_fails INTEGER NOT NULL DEFAULT 0,
    last_success TEXT,
    last_error TEXT,
    last_error_msg TEXT,
    circuit_state TEXT NOT NULL DEFAULT 'closed',
    updated_at TEXT NOT NULL DEFAULT (datetime('now'))
);
