-- Revert changes
DROP INDEX IF EXISTS idx_events_idempotency_key;
DROP INDEX IF EXISTS idx_events_org_id;

ALTER TABLE events
DROP COLUMN IF EXISTS idempotency_key,
DROP COLUMN IF EXISTS meta,
DROP COLUMN IF EXISTS org_id;
