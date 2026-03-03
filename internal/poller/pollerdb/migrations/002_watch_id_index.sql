-- Add missing index on transactions(watch_id).
-- This column is queried every poll tick via CountByWatchID, ListPendingByWatchID,
-- and ListByWatchID. Without this index, every query does a full table scan.
CREATE INDEX IF NOT EXISTS idx_transactions_watch_id ON transactions(watch_id);
