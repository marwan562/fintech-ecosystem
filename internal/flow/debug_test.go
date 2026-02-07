package flow

import (
	"context"
	"fmt"
	"testing"

	"github.com/sapliy/fintech-ecosystem/internal/flow/domain"
)

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

func TestDebugSessionManager(t *testing.T) {
	manager := domain.NewDebugSessionManager()
	ctx := context.Background()

	t.Run("Create debug session", func(t *testing.T) {
		session, err := manager.CreateSession(ctx, "flow_123", "zone_456", domain.DebugLevelInfo)
		if err != nil {
			t.Fatalf("Failed to create debug session: %v", err)
		}

		if session.FlowID != "flow_123" {
			t.Errorf("Expected flow ID flow_123, got %s", session.FlowID)
		}
		if session.ZoneID != "zone_456" {
			t.Errorf("Expected zone ID zone_456, got %s", session.ZoneID)
		}
		if !session.Active {
			t.Error("Session should be active")
		}
		if len(session.Events) == 0 {
			t.Error("Session should have start event")
		}
	})

	t.Run("Get debug session", func(t *testing.T) {
		session, err := manager.CreateSession(ctx, "flow_789", "zone_101", domain.DebugLevelVerbose)
		if err != nil {
			t.Fatalf("Failed to create debug session: %v", err)
		}

		retrieved, err := manager.GetSession(session.ID)
		if err != nil {
			t.Fatalf("Failed to get debug session: %v", err)
		}

		if retrieved.ID != session.ID {
			t.Errorf("Expected session ID %s, got %s", session.ID, retrieved.ID)
		}
		if retrieved.Level != domain.DebugLevelVerbose {
			t.Errorf("Expected verbose level, got %s", retrieved.Level)
		}
	})

	t.Run("End debug session", func(t *testing.T) {
		session, err := manager.CreateSession(ctx, "flow_end", "zone_end", domain.DebugLevelInfo)
		if err != nil {
			t.Fatalf("Failed to create debug session: %v", err)
		}

		err = manager.EndSession(session.ID)
		if err != nil {
			t.Fatalf("Failed to end debug session: %v", err)
		}

		retrieved, err := manager.GetSession(session.ID)
		if err != nil {
			t.Fatalf("Failed to get debug session: %v", err)
		}

		if retrieved.Active {
			t.Error("Session should not be active")
		}
	})
}

func TestDebugEventLogging(t *testing.T) {
	manager := domain.NewDebugSessionManager()
	ctx := context.Background()

	session, err := manager.CreateSession(ctx, "flow_events", "zone_events", domain.DebugLevelInfo)
	if err != nil {
		t.Fatalf("Failed to create debug session: %v", err)
	}

	t.Run("Log node start", func(t *testing.T) {
		input := map[string]interface{}{"test": "value"}
		manager.LogNodeStart(session.ID, "node_1", "condition", input)

		events, err := manager.GetEvents(session.ID, nil)
		if err != nil {
			t.Fatalf("Failed to get events: %v", err)
		}

		// Should have start event + node start event
		if len(events) < 2 {
			t.Errorf("Expected at least 2 events, got %d", len(events))
		}

		// Find the node start event
		var nodeStartEvent *domain.DebugEvent
		for _, event := range events {
			if event.Type == domain.DebugEventNodeStart {
				nodeStartEvent = &event
				break
			}
		}

		if nodeStartEvent == nil {
			t.Error("Node start event not found")
		} else {
			t.Logf("Found node start event with NodeID: '%s'", nodeStartEvent.NodeID)
			if nodeStartEvent.NodeID != "node_1" {
				t.Errorf("Expected node ID node_1, got %s", nodeStartEvent.NodeID)
			}
		}
	})

	t.Run("Log condition evaluation", func(t *testing.T) {
		manager.LogConditionEval(session.ID, "node_2", "amount", "gt", 100.0, 150.0, true)

		events, err := manager.GetEvents(session.ID, nil)
		if err != nil {
			t.Fatalf("Failed to get events: %v", err)
		}

		// Find the condition eval event
		var conditionEvent *domain.DebugEvent
		for _, event := range events {
			if event.Type == domain.DebugEventConditionEval {
				conditionEvent = &event
				break
			}
		}

		if conditionEvent == nil {
			t.Error("Condition evaluation event not found")
		} else {
			if conditionEvent.Level != domain.DebugLevelVerbose {
				t.Errorf("Expected verbose level for successful condition, got %s", conditionEvent.Level)
			}
		}
	})

	t.Run("Log node error", func(t *testing.T) {
		testErr := fmt.Errorf("Test error")
		manager.LogNodeError(session.ID, "node_3", testErr)

		events, err := manager.GetEvents(session.ID, nil)
		if err != nil {
			t.Fatalf("Failed to get events: %v", err)
		}

		// Find the node error event
		var errorEvent *domain.DebugEvent
		for _, event := range events {
			if event.Type == domain.DebugEventNodeError {
				errorEvent = &event
				break
			}
		}

		if errorEvent == nil {
			t.Error("Node error event not found")
		}
	})
}

func TestDebugService(t *testing.T) {
	repo := NewMockFlowRepository()
	service := NewDebugService(repo)
	ctx := context.Background()

	// Create a test flow
	testFlow := &domain.Flow{
		ID:      "flow_debug_test",
		OrgID:   "org_123",
		ZoneID:  "zone_456",
		Name:    "Test Debug Flow",
		Enabled: true,
		Nodes: []domain.Node{
			{
				ID:   "node_trigger",
				Type: domain.NodeTrigger,
			},
		},
	}
	repo.CreateFlow(ctx, testFlow)

	t.Run("Start debug session", func(t *testing.T) {
		session, err := service.StartDebugSession(ctx, "flow_debug_test", "zone_456", domain.DebugLevelInfo)
		if err != nil {
			t.Fatalf("Failed to start debug session: %v", err)
		}

		if session.FlowID != "flow_debug_test" {
			t.Errorf("Expected flow ID flow_debug_test, got %s", session.FlowID)
		}
	})

	t.Run("Start debug session with invalid flow", func(t *testing.T) {
		_, err := service.StartDebugSession(ctx, "invalid_flow", "zone_456", domain.DebugLevelInfo)
		if err == nil {
			t.Error("Expected error for invalid flow")
		}
	})

	t.Run("Start debug session with wrong zone", func(t *testing.T) {
		_, err := service.StartDebugSession(ctx, "flow_debug_test", "wrong_zone", domain.DebugLevelInfo)
		if err == nil {
			t.Error("Expected error for wrong zone")
		}
	})
}

func TestDebugFlowRunner(t *testing.T) {
	repo := NewMockFlowRepository()
	service := NewDebugService(repo)
	ctx := context.Background()

	// Create a test flow with a condition node
	testFlow := &domain.Flow{
		ID:      "flow_runner_test",
		OrgID:   "org_123",
		ZoneID:  "zone_456",
		Name:    "Test Runner Flow",
		Enabled: true,
		Nodes: []domain.Node{
			{
				ID:   "node_trigger",
				Type: domain.NodeTrigger,
			},
			{
				ID:   "node_condition",
				Type: domain.NodeCondition,
				Data: []byte(`{"field":"amount","operator":"gt","value":100}`),
			},
		},
		Edges: []domain.Edge{
			{
				ID:     "edge_1",
				Source: "node_trigger",
				Target: "node_condition",
			},
		},
	}
	repo.CreateFlow(ctx, testFlow)

	t.Run("Execute flow with debug", func(t *testing.T) {
		// Start debug session
		session, err := service.StartDebugSession(ctx, "flow_runner_test", "zone_456", domain.DebugLevelInfo)
		if err != nil {
			t.Fatalf("Failed to start debug session: %v", err)
		}

		// Create debug runner
		baseRunner := domain.NewFlowRunner(repo)
		debugRunner := NewDebugFlowRunner(baseRunner, service, repo)

		// Execute with debug
		input := map[string]interface{}{"amount": 150.0}
		err = debugRunner.ExecuteWithDebug(ctx, testFlow, input, session.ID)
		if err != nil {
			t.Fatalf("Failed to execute flow with debug: %v", err)
		}

		// Check debug events
		events, err := service.GetDebugEvents(session.ID, nil)
		if err != nil {
			t.Fatalf("Failed to get debug events: %v", err)
		}

		// Should have multiple events (start, node start, condition eval, node end, etc.)
		if len(events) < 3 {
			t.Errorf("Expected at least 3 debug events, got %d", len(events))
		}

		// Verify condition evaluation was logged
		var conditionEvent *domain.DebugEvent
		for _, event := range events {
			if event.Type == domain.DebugEventConditionEval {
				conditionEvent = &event
				break
			}
		}

		if conditionEvent == nil {
			t.Error("Condition evaluation event not found")
		}
	})
}
