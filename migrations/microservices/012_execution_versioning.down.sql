-- Drop flow_version from flow_executions
DROP INDEX IF EXISTS idx_flow_executions_flow_version;

ALTER TABLE flow_executions
DROP COLUMN IF EXISTS flow_version;
