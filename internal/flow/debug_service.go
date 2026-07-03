package flow

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"time"

	"github.com/gorilla/websocket"
	"github.com/sapliy/sapliy-core/internal/flow/domain"
)

// DebugService manages flow debugging functionality
type DebugService struct {
	sessionManager *domain.DebugSessionManager
	repo           domain.Repository
	upgrader       websocket.Upgrader
}

// NewDebugService creates a new debug service
func NewDebugService(repo domain.Repository) *DebugService {
	return &DebugService{
		sessionManager: domain.NewDebugSessionManager(),
		repo:           repo,
		upgrader: websocket.Upgrader{
			CheckOrigin: func(r *http.Request) bool {
				return true // Allow all origins for development
			},
		},
	}
}

// GetRepository returns the underlying repository
func (s *DebugService) GetRepository() domain.Repository {
	return s.repo
}

// StartDebugSession starts a debug session for a flow
func (s *DebugService) StartDebugSession(ctx context.Context, flowID, zoneID string, level domain.DebugLevel) (*domain.DebugSession, error) {
	// Verify flow exists
	flow, err := s.repo.GetFlow(ctx, flowID)
	if err != nil {
		log.Printf("Debug service: failed to get flow %s: %v", flowID, err)
		return nil, fmt.Errorf("flow not found: %w", err)
	}

	if flow.ZoneID != zoneID {
		return nil, fmt.Errorf("flow does not belong to zone %s", zoneID)
	}

	return s.sessionManager.CreateSession(ctx, flowID, zoneID, level)
}

// EndDebugSession ends a debug session
func (s *DebugService) EndDebugSession(sessionID string) error {
	return s.sessionManager.EndSession(sessionID)
}

// GetDebugSession retrieves a debug session
func (s *DebugService) GetDebugSession(sessionID string) (*domain.DebugSession, error) {
	return s.sessionManager.GetSession(sessionID)
}

// GetDebugEvents retrieves debug events for a session
func (s *DebugService) GetDebugEvents(sessionID string, since *time.Time) ([]domain.DebugEvent, error) {
	return s.sessionManager.GetEvents(sessionID, since)
}

// GetSessionManager returns the session manager for testing
func (s *DebugService) GetSessionManager() *domain.DebugSessionManager {
	return s.sessionManager
}

// DebugFlowRunner wraps a FlowRunner with debugging capabilities
type DebugFlowRunner struct {
	*domain.FlowRunner
	debugService *DebugService
	repo         domain.Repository
}

// NewDebugFlowRunner creates a new debug flow runner
func NewDebugFlowRunner(runner *domain.FlowRunner, debugService *DebugService, repo domain.Repository) *DebugFlowRunner {
	return &DebugFlowRunner{
		FlowRunner:   runner,
		debugService: debugService,
		repo:         repo,
	}
}

// DebugHook implements domain.ExecutionHook for debug logging
type DebugHook struct {
	sessionID    string
	debugService *DebugService
	startTime    map[string]time.Time
}

func NewDebugHook(sessionID string, debugService *DebugService) *DebugHook {
	return &DebugHook{
		sessionID:    sessionID,
		debugService: debugService,
		startTime:    make(map[string]time.Time),
	}
}

func (h *DebugHook) BeforeNode(ctx context.Context, node *domain.Node, input map[string]interface{}) {
	h.startTime[node.ID] = time.Now()
	h.debugService.sessionManager.LogNodeStart(h.sessionID, node.ID, string(node.Type), input)
}

func (h *DebugHook) AfterNode(ctx context.Context, node *domain.Node, output map[string]interface{}, err error) {
	duration := time.Since(h.startTime[node.ID])
	if err != nil {
		if err.Error() == "execution_paused" {
			h.debugService.sessionManager.LogNodePaused(h.sessionID, node.ID, "Paused")
		} else {
			h.debugService.sessionManager.LogNodeError(h.sessionID, node.ID, err)
		}
	} else {
		// Log specialized events for certain node types
		if node.Type == domain.NodeCondition {
			var config struct {
				Field    string      `json:"field"`
				Operator string      `json:"operator"`
				Value    interface{} `json:"value"`
			}
			json.Unmarshal(node.Data, &config)

			// We can't easily get the specific input value used by the handler here
			// without re-parsing it, but we can log the result
			result, _ := output["result"].(bool)
			h.debugService.sessionManager.LogConditionEval(h.sessionID, node.ID, config.Field, config.Operator, config.Value, nil, result)
		}

		h.debugService.sessionManager.LogNodeEnd(h.sessionID, node.ID, output, duration)
	}
}

// ExecuteWithDebug executes a flow with debug logging using hooks
func (d *DebugFlowRunner) ExecuteWithDebug(ctx context.Context, flow *domain.Flow, input map[string]interface{}, debugSessionID string) error {
	// Add debug hook to the runner
	hook := NewDebugHook(debugSessionID, d.debugService)
	d.FlowRunner.AddHook(hook)

	// Since we are using hooks, we can just call the standard Execute
	return d.FlowRunner.Execute(ctx, flow, input)
}
