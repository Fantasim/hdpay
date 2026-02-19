-- Migration 007: Add network column to all core tables.
-- Enables mainnet and testnet data to coexist in the same database.
-- Network detection: BTC addresses use prefix (bc1=mainnet, tb1=testnet).
-- BSC/SOL inherit from BTC detection. Default: testnet.

-- ============================================================
-- 1. addresses: recreate with network in PK
-- ============================================================
CREATE TABLE addresses_new (
    chain TEXT NOT NULL,
    network TEXT NOT NULL DEFAULT 'testnet',
    address_index INTEGER NOT NULL,
    address TEXT NOT NULL,
    created_at TEXT NOT NULL DEFAULT (datetime('now')),
    PRIMARY KEY (chain, network, address_index)
);

INSERT INTO addresses_new (chain, network, address_index, address, created_at)
SELECT chain,
       CASE
         WHEN chain = 'BTC' AND address LIKE 'bc1%' THEN 'mainnet'
         WHEN chain = 'BTC' THEN 'testnet'
         ELSE COALESCE(
           (SELECT CASE WHEN a2.address LIKE 'bc1%' THEN 'mainnet' ELSE 'testnet' END
            FROM addresses a2 WHERE a2.chain = 'BTC' LIMIT 1),
           'testnet'
         )
       END,
       address_index, address, created_at
FROM addresses;

DROP TABLE addresses;
ALTER TABLE addresses_new RENAME TO addresses;
CREATE INDEX IF NOT EXISTS idx_addresses_chain ON addresses(chain, network);
CREATE INDEX IF NOT EXISTS idx_addresses_address ON addresses(address);

-- ============================================================
-- 2. balances: recreate with network in PK
--    (addresses already migrated, so we can JOIN for detection)
-- ============================================================
CREATE TABLE balances_new (
    chain TEXT NOT NULL,
    network TEXT NOT NULL DEFAULT 'testnet',
    address_index INTEGER NOT NULL,
    token TEXT NOT NULL DEFAULT 'NATIVE',
    balance TEXT NOT NULL DEFAULT '0',
    last_scanned TEXT,
    PRIMARY KEY (chain, network, address_index, token)
);

INSERT INTO balances_new (chain, network, address_index, token, balance, last_scanned)
SELECT b.chain,
       COALESCE(
         (SELECT a.network FROM addresses a
          WHERE a.chain = b.chain AND a.address_index = b.address_index LIMIT 1),
         'testnet'
       ),
       b.address_index, b.token, b.balance, b.last_scanned
FROM balances b;

DROP TABLE balances;
ALTER TABLE balances_new RENAME TO balances;
CREATE INDEX IF NOT EXISTS idx_balances_nonzero ON balances(chain, network, token) WHERE balance != '0';

-- ============================================================
-- 3. scan_state: recreate with network in PK
-- ============================================================
CREATE TABLE scan_state_new (
    chain TEXT NOT NULL,
    network TEXT NOT NULL DEFAULT 'testnet',
    last_scanned_index INTEGER NOT NULL DEFAULT 0,
    max_scan_id INTEGER NOT NULL DEFAULT 0,
    status TEXT NOT NULL DEFAULT 'idle',
    started_at TEXT,
    updated_at TEXT,
    PRIMARY KEY (chain, network)
);

INSERT INTO scan_state_new (chain, network, last_scanned_index, max_scan_id, status, started_at, updated_at)
SELECT chain,
       COALESCE(
         (SELECT DISTINCT a.network FROM addresses a WHERE a.chain = scan_state.chain LIMIT 1),
         'testnet'
       ),
       last_scanned_index, max_scan_id, status, started_at, updated_at
FROM scan_state;

DROP TABLE scan_state;
ALTER TABLE scan_state_new RENAME TO scan_state;

-- ============================================================
-- 4. transactions: ALTER TABLE ADD COLUMN (PK is autoincrement id)
-- ============================================================
ALTER TABLE transactions ADD COLUMN network TEXT NOT NULL DEFAULT 'testnet';

UPDATE transactions SET network = COALESCE(
  (SELECT DISTINCT a.network FROM addresses a WHERE a.chain = transactions.chain LIMIT 1),
  'testnet'
);

DROP INDEX IF EXISTS idx_transactions_chain;
CREATE INDEX idx_transactions_chain ON transactions(chain, network);

-- ============================================================
-- 5. tx_state: ALTER TABLE ADD COLUMN (PK is TEXT id)
-- ============================================================
ALTER TABLE tx_state ADD COLUMN network TEXT NOT NULL DEFAULT 'testnet';

UPDATE tx_state SET network = COALESCE(
  (SELECT DISTINCT a.network FROM addresses a WHERE a.chain = tx_state.chain LIMIT 1),
  'testnet'
);

DROP INDEX IF EXISTS idx_tx_state_chain;
CREATE INDEX idx_tx_state_chain ON tx_state(chain, network, status);
