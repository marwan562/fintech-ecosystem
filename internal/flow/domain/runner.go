package domain

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"time"
)

// ErrExecutionPaused is a sentinel error used to signal that an execution
// has been paused (e.g. by an approval node). It propagates up the call
// stack to prevent Execute() from overwriting the paused status.
var ErrExecutionPaused = fmt.Errorf("execution paused")

type NodeHandler interface {
	Execute(ctx context.Context, node *Node, input map[string]interface{}) (map[string]interface{}, error)
}

type FlowRunner struct {
	repo     Repository
	handlers map[NodeType]NodeHandler
	hooks    []ExecutionHook
}

type ExecutionHook interface {
	BeforeNode(ctx context.Context, node *Node, input map[string]interface{})
	AfterNode(ctx context.Context, node *Node, output map[string]interface{}, err error)
}

func NewFlowRunner(repo Repository) *FlowRunner {
	r := &FlowRunner{
		repo:     repo,
		handlers: make(map[NodeType]NodeHandler),
		hooks:    make([]ExecutionHook, 0),
	}
	r.registerDefaultHandlers()
	return r
}

func (r *FlowRunner) AddHook(hook ExecutionHook) {
	r.hooks = append(r.hooks, hook)
}

func (r *FlowRunner) registerDefaultHandlers() {
	r.handlers[NodeCondition] = &ConditionHandler{}
	r.handlers[NodeWebhook] = &WebhookHandler{}
	r.handlers[NodeApproval] = &ApprovalHandler{}
	r.handlers[NodeAuditLog] = &AuditHandler{}
}

func (r *FlowRunner) Execute(ctx context.Context, flow *Flow, input map[string]interface{}) error {
	exec := &FlowExecution{
		ID:          fmt.Sprintf("exec_%d", time.Now().UnixNano()),
		FlowID:      flow.ID,
		FlowVersion: flow.Version,
		Status:      ExecutionRunning,
		StartedAt:   time.Now(),
	}
	inputBytes, _ := json.Marshal(input)
	exec.Input = inputBytes

	if err := r.repo.CreateExecution(ctx, exec); err != nil {
		return err
	}

	// Find trigger node
	var startNode *Node
	for _, n := range flow.Nodes {
		if n.Type == NodeTrigger {
			startNode = &n
			break
		}
	}

	if startNode == nil {
		return fmt.Errorf("no trigger node found in flow %s", flow.ID)
	}

	if err := r.executeNode(ctx, flow, startNode, input, exec); err != nil {
		if err == ErrExecutionPaused {
			return nil // Execution paused successfully; status already persisted
		}
		return err
	}

	exec.Status = ExecutionCompleted
	exec.EndedAt = time.Now()
	return r.repo.UpdateExecution(ctx, exec)
}

func (r *FlowRunner) executeNode(ctx context.Context, flow *Flow, node *Node, input map[string]interface{}, exec *FlowExecution) error {
	log.Printf("Executing node %s (%s)", node.ID, node.Type)
	exec.CurrentNodeID = node.ID

	step := ExecutionStep{
		NodeID: node.ID,
		Status: ExecutionRunning,
		Input:  func() json.RawMessage { b, _ := json.Marshal(input); return b }(),
	}
	exec.Steps = append(exec.Steps, step)

	var output map[string]interface{}
	var err error

	for _, hook := range r.hooks {
		hook.BeforeNode(ctx, node, input)
	}

	handler, ok := r.handlers[node.Type]
	if ok {
		output, err = handler.Execute(ctx, node, input)
	} else {
		output = input
	}

	for _, hook := range r.hooks {
		hook.AfterNode(ctx, node, output, err)
	}

	if err != nil {
		if err.Error() == "execution_paused" {
			log.Printf("Node %s paused execution", node.ID)
			exec.Status = ExecutionPaused
			exec.Steps[len(exec.Steps)-1].Status = ExecutionPaused
			if dbErr := r.repo.UpdateExecution(ctx, exec); dbErr != nil {
				return dbErr
			}
			return ErrExecutionPaused
		}
		log.Printf("Node %s failed: %v", node.ID, err)
		exec.Steps[len(exec.Steps)-1].Status = ExecutionFailed
		exec.Steps[len(exec.Steps)-1].Error = err.Error()
		return err
	}

	outputBytes, _ := json.Marshal(output)
	exec.Steps[len(exec.Steps)-1].Status = ExecutionCompleted
	exec.Steps[len(exec.Steps)-1].Output = outputBytes

	// Find next nodes
	var nextNodes []*Node
	for _, edge := range flow.Edges {
		if edge.Source == node.ID {
			if node.Type == NodeCondition {
				res, _ := output["result"].(bool)
				if (res && edge.SourceHandle == "true") || (!res && edge.SourceHandle == "false") {
					for _, n := range flow.Nodes {
						if n.ID == edge.Target {
							nextNodes = append(nextNodes, &n)
						}
					}
				}
				continue
			}

			for _, n := range flow.Nodes {
				if n.ID == edge.Target {
					nextNodes = append(nextNodes, &n)
				}
			}
		}
	}

	for _, next := range nextNodes {
		if err := r.executeNode(ctx, flow, next, output, exec); err != nil {
			return err
		}
	}

	return r.repo.UpdateExecution(ctx, exec)
}

func (r *FlowRunner) Resume(ctx context.Context, execID string, overrides map[string]interface{}) error {
	exec, err := r.repo.GetExecution(ctx, execID)
	if err != nil {
		return err
	}
	if exec.Status != ExecutionPaused {
		return fmt.Errorf("execution is not paused")
	}

	flow, err := r.repo.GetFlow(ctx, exec.FlowID)
	if err != nil {
		return err
	}

	exec.Status = ExecutionRunning

	var currentNode *Node
	for _, n := range flow.Nodes {
		if n.ID == exec.CurrentNodeID {
			currentNode = &n
			break
		}
	}

	if currentNode == nil {
		return fmt.Errorf("current node %s not found in flow", exec.CurrentNodeID)
	}

	var lastOutput map[string]interface{}
	if len(exec.Steps) > 0 {
		json.Unmarshal(exec.Steps[len(exec.Steps)-1].Output, &lastOutput)
	}
	if lastOutput == nil {
		lastOutput = make(map[string]interface{})
	}

	for k, v := range overrides {
		lastOutput[k] = v
	}

	var nextNodes []*Node
	for _, edge := range flow.Edges {
		if edge.Source == currentNode.ID {
			for _, n := range flow.Nodes {
				if n.ID == edge.Target {
					nextNodes = append(nextNodes, &n)
				}
			}
		}
	}

	log.Printf("Resuming execution %s from node %s", execID, currentNode.ID)

	for _, next := range nextNodes {
		if err := r.executeNode(ctx, flow, next, lastOutput, exec); err != nil {
			return err
		}
	}

	exec.Status = ExecutionCompleted
	exec.EndedAt = time.Now()
	return r.repo.UpdateExecution(ctx, exec)
}

// GetHandler returns the handler for a specific node type
func (r *FlowRunner) GetHandler(nodeType NodeType) NodeHandler {
	return r.handlers[nodeType]
}

// --- Specific Handlers ---

type ConditionHandler struct{}

func (h *ConditionHandler) Execute(ctx context.Context, node *Node, input map[string]interface{}) (map[string]interface{}, error) {
	var config struct {
		Field    string      `json:"field"`
		Operator string      `json:"operator"`
		Value    interface{} `json:"value"`
	}
	json.Unmarshal(node.Data, &config)

	inputValue, ok := input[config.Field]
	if !ok {
		return map[string]interface{}{"result": false}, nil
	}

	result := false
	switch config.Operator {
	case "equals":
		result = fmt.Sprintf("%v", inputValue) == fmt.Sprintf("%v", config.Value)
	case "gt":
		iv, _ := inputValue.(float64)
		cv, _ := config.Value.(float64)
		result = iv > cv
	}

	return map[string]interface{}{"result": result}, nil
}

type WebhookHandler struct{}

func (h *WebhookHandler) Execute(ctx context.Context, node *Node, input map[string]interface{}) (map[string]interface{}, error) {
	log.Printf("Sending webhook for node %s", node.ID)
	return map[string]interface{}{"status": "sent"}, nil
}

type ApprovalHandler struct{}

func (h *ApprovalHandler) Execute(ctx context.Context, node *Node, input map[string]interface{}) (map[string]interface{}, error) {
	log.Printf("Approval required for node %s", node.ID)
	return nil, fmt.Errorf("execution_paused")
}

type AuditHandler struct{}

func (h *AuditHandler) Execute(ctx context.Context, node *Node, input map[string]interface{}) (map[string]interface{}, error) {
	log.Printf("Auditing node %s: %v", node.ID, input)
	return input, nil
}
