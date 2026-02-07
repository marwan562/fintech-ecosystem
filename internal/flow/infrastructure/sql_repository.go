package infrastructure

import (
	"context"
	"database/sql"
	"encoding/json"

	"github.com/sapliy/fintech-ecosystem/internal/flow/domain"
)

type SQLRepository struct {
	db *sql.DB
}

func NewSQLRepository(db *sql.DB) *SQLRepository {
	return &SQLRepository{db: db}
}

func (r *SQLRepository) CreateFlow(ctx context.Context, flow *domain.Flow) error {
	nodesJSON, _ := json.Marshal(flow.Nodes)
	edgesJSON, _ := json.Marshal(flow.Edges)

	_, err := r.db.ExecContext(ctx,
		"INSERT INTO flows (id, org_id, zone_id, name, description, enabled, nodes, edges) VALUES ($1, $2, $3, $4, $5, $6, $7, $8)",
		flow.ID, flow.OrgID, flow.ZoneID, flow.Name, flow.Description, flow.Enabled, nodesJSON, edgesJSON)
	return err
}

func (r *SQLRepository) GetFlow(ctx context.Context, id string) (*domain.Flow, error) {
	row := r.db.QueryRowContext(ctx, "SELECT id, org_id, zone_id, name, description, enabled, nodes, edges, created_at, updated_at FROM flows WHERE id = $1", id)

	var flow domain.Flow
	var nodesJS, edgesJS []byte
	err := row.Scan(&flow.ID, &flow.OrgID, &flow.ZoneID, &flow.Name, &flow.Description, &flow.Enabled, &nodesJS, &edgesJS, &flow.CreatedAt, &flow.UpdatedAt)
	if err != nil {
		return nil, err
	}

	json.Unmarshal(nodesJS, &flow.Nodes)
	json.Unmarshal(edgesJS, &flow.Edges)
	return &flow, nil
}

func (r *SQLRepository) ListFlows(ctx context.Context, zoneID string) ([]*domain.Flow, error) {
	rows, err := r.db.QueryContext(ctx, "SELECT id, org_id, zone_id, name, description, enabled, nodes, edges, created_at, updated_at FROM flows WHERE zone_id = $1 AND enabled = TRUE", zoneID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var flows []*domain.Flow
	for rows.Next() {
		var f domain.Flow
		var nodesJS, edgesJS []byte
		if err := rows.Scan(&f.ID, &f.OrgID, &f.ZoneID, &f.Name, &f.Description, &f.Enabled, &nodesJS, &edgesJS, &f.CreatedAt, &f.UpdatedAt); err != nil {
			return nil, err
		}
		json.Unmarshal(nodesJS, &f.Nodes)
		json.Unmarshal(edgesJS, &f.Edges)
		flows = append(flows, &f)
	}
	return flows, nil
}

func (r *SQLRepository) UpdateFlow(ctx context.Context, flow *domain.Flow) error {
	nodesJSON, _ := json.Marshal(flow.Nodes)
	edgesJSON, _ := json.Marshal(flow.Edges)

	_, err := r.db.ExecContext(ctx,
		"UPDATE flows SET name = $1, description = $2, enabled = $3, nodes = $4, edges = $5, updated_at = CURRENT_TIMESTAMP WHERE id = $6",
		flow.Name, flow.Description, flow.Enabled, nodesJSON, edgesJSON, flow.ID)
	return err
}

func (r *SQLRepository) CreateExecution(ctx context.Context, exec *domain.FlowExecution) error {
	stepsJSON, _ := json.Marshal(exec.Steps)
	_, err := r.db.ExecContext(ctx,
		"INSERT INTO flow_executions (id, flow_id, status, current_node_id, input, steps, metadata, started_at) VALUES ($1, $2, $3, $4, $5, $6, $7, $8)",
		exec.ID, exec.FlowID, exec.Status, exec.CurrentNodeID, exec.Input, stepsJSON, exec.Metadata, exec.StartedAt)
	return err
}

func (r *SQLRepository) UpdateExecution(ctx context.Context, exec *domain.FlowExecution) error {
	stepsJSON, _ := json.Marshal(exec.Steps)
	_, err := r.db.ExecContext(ctx,
		"UPDATE flow_executions SET status = $1, current_node_id = $2, output = $3, steps = $4, metadata = $5, ended_at = $6 WHERE id = $7",
		exec.Status, exec.CurrentNodeID, exec.Output, stepsJSON, exec.Metadata, exec.EndedAt, exec.ID)
	return err
}

func (r *SQLRepository) GetExecution(ctx context.Context, id string) (*domain.FlowExecution, error) {
	row := r.db.QueryRowContext(ctx, "SELECT id, flow_id, status, current_node_id, input, output, steps, metadata, started_at, ended_at FROM flow_executions WHERE id = $1", id)

	var exec domain.FlowExecution
	var stepsJS []byte
	err := row.Scan(&exec.ID, &exec.FlowID, &exec.Status, &exec.CurrentNodeID, &exec.Input, &exec.Output, &stepsJS, &exec.Metadata, &exec.StartedAt, &exec.EndedAt)
	if err != nil {
		return nil, err
	}
	json.Unmarshal(stepsJS, &exec.Steps)
	return &exec, nil
}

func (r *SQLRepository) BulkUpdateFlowsEnabled(ctx context.Context, ids []string, enabled bool) error {
	for _, id := range ids {
		_, err := r.db.ExecContext(ctx, "UPDATE flows SET enabled = $1, updated_at = CURRENT_TIMESTAMP WHERE id = $2", enabled, id)
		if err != nil {
			return err
		}
	}
	return nil
}

// Event methods

func (r *SQLRepository) CreateEvent(ctx context.Context, event *domain.Event) error {
	metaJSON, _ := json.Marshal(event.Meta)
	_, err := r.db.ExecContext(ctx,
		"INSERT INTO events (id, type, zone_id, org_id, data, meta, idempotency_key, created_at) VALUES ($1, $2, $3, $4, $5, $6, $7, $8)",
		event.ID, event.Type, event.ZoneID, event.OrgID, event.Data, metaJSON, event.IdempotencyKey, event.CreatedAt)
	return err
}

func (r *SQLRepository) GetPastEvents(ctx context.Context, zoneID string, limit, offset int) ([]*domain.Event, error) {
	rows, err := r.db.QueryContext(ctx,
		"SELECT id, type, zone_id, data, created_at FROM events WHERE zone_id = $1 ORDER BY created_at DESC LIMIT $2 OFFSET $3",
		zoneID, limit, offset)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var events []*domain.Event
	for rows.Next() {
		var e domain.Event
		if err := rows.Scan(&e.ID, &e.Type, &e.ZoneID, &e.Data, &e.CreatedAt); err != nil {
			return nil, err
		}
		events = append(events, &e)
	}
	return events, nil
}

func (r *SQLRepository) GetEventByID(ctx context.Context, id string) (*domain.Event, error) {
	row := r.db.QueryRowContext(ctx, "SELECT id, type, zone_id, data, created_at FROM events WHERE id = $1", id)

	var e domain.Event
	if err := row.Scan(&e.ID, &e.Type, &e.ZoneID, &e.Data, &e.CreatedAt); err != nil {
		return nil, err
	}
	return &e, nil
}
