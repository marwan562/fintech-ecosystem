package testutil

import (
	"context"
	"fmt"

	"github.com/sapliy/fintech-ecosystem/internal/flow/domain"
)

// Exported MockFlowRepository for testing
type MockFlowRepository struct {
	flows      map[string]*domain.Flow
	executions map[string]*domain.FlowExecution
	events     map[string]*domain.Event
}

func NewMockFlowRepository() *MockFlowRepository {
	return &MockFlowRepository{
		flows:      make(map[string]*domain.Flow),
		executions: make(map[string]*domain.FlowExecution),
		events:     make(map[string]*domain.Event),
	}
}

func (m *MockFlowRepository) CreateFlow(ctx context.Context, flow *domain.Flow) error {
	m.flows[flow.ID] = flow
	return nil
}

func (m *MockFlowRepository) GetFlow(ctx context.Context, id string) (*domain.Flow, error) {
	if flow, exists := m.flows[id]; exists {
		return flow, nil
	}
	return nil, domain.ErrFlowNotFound
}

func (m *MockFlowRepository) ListFlows(ctx context.Context, zoneID string) ([]*domain.Flow, error) {
	var flows []*domain.Flow
	for _, flow := range m.flows {
		if flow.ZoneID == zoneID {
			flows = append(flows, flow)
		}
	}
	return flows, nil
}

func (m *MockFlowRepository) UpdateFlow(ctx context.Context, flow *domain.Flow) error {
	m.flows[flow.ID] = flow
	return nil
}

func (m *MockFlowRepository) CreateExecution(ctx context.Context, exec *domain.FlowExecution) error {
	m.executions[exec.ID] = exec
	return nil
}

func (m *MockFlowRepository) UpdateExecution(ctx context.Context, exec *domain.FlowExecution) error {
	m.executions[exec.ID] = exec
	return nil
}

func (m *MockFlowRepository) GetExecution(ctx context.Context, id string) (*domain.FlowExecution, error) {
	if exec, exists := m.executions[id]; exists {
		return exec, nil
	}
	return nil, domain.ErrExecutionNotFound
}

func (m *MockFlowRepository) BulkUpdateFlowsEnabled(ctx context.Context, ids []string, enabled bool) error {
	for _, id := range ids {
		if flow, exists := m.flows[id]; exists {
			flow.Enabled = enabled
		}
	}
	return nil
}

func (m *MockFlowRepository) CreateEvent(ctx context.Context, event *domain.Event) error {
	m.events[event.ID] = event
	return nil
}

func (m *MockFlowRepository) GetPastEvents(ctx context.Context, zoneID string, limit, offset int) ([]*domain.Event, error) {
	var events []*domain.Event
	for _, event := range m.events {
		if event.ZoneID == zoneID {
			events = append(events, event)
		}
	}
	return events, nil
}

func (m *MockFlowRepository) GetEventByID(ctx context.Context, id string) (*domain.Event, error) {
	if event, exists := m.events[id]; exists {
		return event, nil
	}
	return nil, fmt.Errorf("event not found")
}

func (m *MockFlowRepository) CreateFlowVersion(ctx context.Context, version *domain.FlowVersion) error {
	return nil
}

func (m *MockFlowRepository) GetFlowVersions(ctx context.Context, flowID string) ([]*domain.FlowVersion, error) {
	return nil, nil
}

func (m *MockFlowRepository) GetFlowVersion(ctx context.Context, flowID string, version int) (*domain.FlowVersion, error) {
	return nil, nil
}
