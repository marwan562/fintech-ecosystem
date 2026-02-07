package domain

import (
	"context"
	"encoding/json"
	"errors"
	"time"
)

type NodeType string

const (
	NodeTrigger       NodeType = "eventTrigger"
	NodeCondition     NodeType = "condition"
	NodeWebhook       NodeType = "webhook"
	NodeApproval      NodeType = "approval"
	NodeAuditLog      NodeType = "auditLog"
	NodeTransform     NodeType = "transform"
	NodeDelay         NodeType = "delay"
	NodeLoop          NodeType = "loop"
	NodeSubflow       NodeType = "subflow"
	NodeInternalEvent NodeType = "internalEvent"
)

type Flow struct {
	ID          string    `json:"id"`
	OrgID       string    `json:"org_id"`
	ZoneID      string    `json:"zone_id"`
	Name        string    `json:"name"`
	Description string    `json:"description"`
	Enabled     bool      `json:"enabled"`
	Nodes       []Node    `json:"nodes"`
	Edges       []Edge    `json:"edges"`
	CreatedAt   time.Time `json:"created_at"`
	UpdatedAt   time.Time `json:"updated_at"`
}

type Node struct {
	ID       string          `json:"id"`
	Type     NodeType        `json:"type"`     // React Flow type (e.g. "eventTrigger", "condition")
	Position json.RawMessage `json:"position"` // React Flow position
	Data     json.RawMessage `json:"data"`     // React Flow data (node configuration)
}

type Edge struct {
	ID           string `json:"id"`
	Source       string `json:"source"`
	Target       string `json:"target"`
	SourceHandle string `json:"source_handle,omitempty"`
}

type ExecutionStatus string

const (
	ExecutionPending   ExecutionStatus = "pending"
	ExecutionRunning   ExecutionStatus = "running"
	ExecutionPaused    ExecutionStatus = "paused"
	ExecutionCompleted ExecutionStatus = "completed"
	ExecutionFailed    ExecutionStatus = "failed"
)

type FlowExecution struct {
	ID            string          `json:"id"`
	FlowID        string          `json:"flow_id"`
	TriggerID     string          `json:"trigger_id"` // Reference to the event that started it
	Status        ExecutionStatus `json:"status"`
	CurrentNodeID string          `json:"current_node_id,omitempty"` // For resuming
	Input         json.RawMessage `json:"input"`
	Output        json.RawMessage `json:"output"`
	Steps         []ExecutionStep `json:"steps"`
	Metadata      json.RawMessage `json:"metadata,omitempty"` // Execution context
	StartedAt     time.Time       `json:"started_at"`
	EndedAt       time.Time       `json:"ended_at,omitempty"`
}

type ExecutionStep struct {
	NodeID string          `json:"node_id"`
	Status ExecutionStatus `json:"status"`
	Input  json.RawMessage `json:"input"`
	Output json.RawMessage `json:"output"`
	Error  string          `json:"error,omitempty"`
}

// Event represents a business event that can trigger flows
type Event struct {
	ID             string            `json:"id"`
	Type           string            `json:"type"`
	ZoneID         string            `json:"zone_id"`
	OrgID          string            `json:"org_id"`
	Data           json.RawMessage   `json:"data"` // Payload
	Meta           map[string]string `json:"meta"`
	IdempotencyKey string            `json:"idempotency_key"`
	CreatedAt      time.Time         `json:"created_at"`
}

type Repository interface {
	CreateFlow(ctx context.Context, flow *Flow) error
	GetFlow(ctx context.Context, id string) (*Flow, error)
	ListFlows(ctx context.Context, zoneID string) ([]*Flow, error)
	UpdateFlow(ctx context.Context, flow *Flow) error

	CreateExecution(ctx context.Context, exec *FlowExecution) error
	UpdateExecution(ctx context.Context, exec *FlowExecution) error
	GetExecution(ctx context.Context, id string) (*FlowExecution, error)

	// Event methods for replay
	CreateEvent(ctx context.Context, event *Event) error
	GetPastEvents(ctx context.Context, zoneID string, limit, offset int) ([]*Event, error)
	GetEventByID(ctx context.Context, id string) (*Event, error)

	BulkUpdateFlowsEnabled(ctx context.Context, ids []string, enabled bool) error
}

// Errors
var (
	ErrFlowNotFound      = errors.New("flow not found")
	ErrExecutionNotFound = errors.New("execution not found")
)
