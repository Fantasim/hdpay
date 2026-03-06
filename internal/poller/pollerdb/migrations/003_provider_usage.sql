CREATE TABLE IF NOT EXISTS provider_usage (
    chain       TEXT NOT NULL,
    provider    TEXT NOT NULL,
    date        TEXT NOT NULL,
    requests    INTEGER NOT NULL DEFAULT 0,
    successes   INTEGER NOT NULL DEFAULT 0,
    failures    INTEGER NOT NULL DEFAULT 0,
    hits_429    INTEGER NOT NULL DEFAULT 0,
    PRIMARY KEY (chain, provider, date)
);
