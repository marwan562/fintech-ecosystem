-- Add missing fields for full event envelope support
ALTER TABLE events 
ADD COLUMN org_id VARCHAR(64),
ADD COLUMN meta JSONB DEFAULT '{}',
ADD COLUMN idempotency_key VARCHAR(128);

CREATE INDEX IF NOT EXISTS idx_events_org_id ON events(org_id);
CREATE INDEX IF NOT EXISTS idx_events_idempotency_key ON events(idempotency_key);
