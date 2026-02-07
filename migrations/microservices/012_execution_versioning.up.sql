-- Add flow_version to flow_executions
ALTER TABLE flow_executions
ADD COLUMN flow_version INT;

CREATE INDEX IF NOT EXISTS idx_flow_executions_flow_version ON flow_executions(flow_id, flow_version);
