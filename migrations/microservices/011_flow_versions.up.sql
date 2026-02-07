-- Create flow_versions table for historical tracking
CREATE TABLE IF NOT EXISTS flow_versions (
    id SERIAL PRIMARY KEY,
    flow_id TEXT NOT NULL REFERENCES flows(id),
    version INT NOT NULL,
    nodes JSONB NOT NULL,
    edges JSONB NOT NULL,
    created_at TIMESTAMP WITH TIME ZONE DEFAULT CURRENT_TIMESTAMP,
    UNIQUE(flow_id, version)
);

CREATE INDEX IF NOT EXISTS idx_flow_versions_flow_id ON flow_versions(flow_id);
