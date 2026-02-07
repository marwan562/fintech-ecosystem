package nodes

import (
	"context"
	"fmt"
	"time"
)

// Note: NodeResult is defined in condition.go

// TransformNode maps and transforms input data
type TransformNode struct {
	NodeID   string            `json:"id"`
	Mappings map[string]string `json:"mappings"` // output_key -> input_path
	NextNode string            `json:"next,omitempty"`
}

// NewTransformNode creates a new transform node
func NewTransformNode(id string, mappings map[string]string) *TransformNode {
	return &TransformNode{
		NodeID:   id,
		Mappings: mappings,
	}
}

// ID returns the node ID
func (n *TransformNode) ID() string { return n.NodeID }

// Type returns the node type
func (n *TransformNode) Type() string { return "transform" }

// Execute transforms input data according to mappings
func (n *TransformNode) Execute(ctx context.Context, input map[string]interface{}) (*NodeResult, error) {
	output := make(map[string]interface{})

	for outputKey, inputPath := range n.Mappings {
		val, err := extractValue(input, inputPath)
		if err == nil {
			output[outputKey] = val
		}
	}

	return &NodeResult{
		Success: true,
		Output:  output,
		Next:    n.NextNode,
	}, nil
}

// DelayNode pauses execution for a specified duration
type DelayNode struct {
	NodeID   string        `json:"id"`
	Duration time.Duration `json:"duration"`
	NextNode string        `json:"next,omitempty"`
}

// NewDelayNode creates a new delay node
func NewDelayNode(id string, duration time.Duration) *DelayNode {
	return &DelayNode{
		NodeID:   id,
		Duration: duration,
	}
}

// ID returns the node ID
func (n *DelayNode) ID() string { return n.NodeID }

// Type returns the node type
func (n *DelayNode) Type() string { return "delay" }

// Execute pauses execution for the configured duration
func (n *DelayNode) Execute(ctx context.Context, input map[string]interface{}) (*NodeResult, error) {
	select {
	case <-ctx.Done():
		return &NodeResult{
			Success: false,
			Error:   "execution cancelled",
		}, ctx.Err()
	case <-time.After(n.Duration):
		return &NodeResult{
			Success: true,
			Output:  input, // Pass through
			Next:    n.NextNode,
		}, nil
	}
}

// LoopNode iterates over an array in the input
type LoopNode struct {
	NodeID    string `json:"id"`
	ArrayPath string `json:"array_path"` // Path to array in input
	ItemKey   string `json:"item_key"`   // Key to use for each item
	IndexKey  string `json:"index_key"`  // Key to use for index
	BodyNode  string `json:"body_node"`  // Node to execute for each item
	NextNode  string `json:"next,omitempty"`
}

// NewLoopNode creates a new loop node
func NewLoopNode(id, arrayPath, bodyNode string) *LoopNode {
	return &LoopNode{
		NodeID:    id,
		ArrayPath: arrayPath,
		ItemKey:   "item",
		IndexKey:  "index",
		BodyNode:  bodyNode,
	}
}

// ID returns the node ID
func (n *LoopNode) ID() string { return n.NodeID }

// Type returns the node type
func (n *LoopNode) Type() string { return "loop" }

// Execute iterates over the array (actual iteration handled by runner)
func (n *LoopNode) Execute(ctx context.Context, input map[string]interface{}) (*NodeResult, error) {
	array, err := extractValue(input, n.ArrayPath)
	if err != nil {
		return &NodeResult{
			Success: false,
			Error:   fmt.Sprintf("array not found at path %s", n.ArrayPath),
		}, nil
	}

	items, ok := array.([]interface{})
	if !ok {
		return &NodeResult{
			Success: false,
			Error:   "value at path is not an array",
		}, nil
	}

	// Return loop metadata for runner to handle iteration
	return &NodeResult{
		Success: true,
		Output: map[string]interface{}{
			"__loop":      true,
			"__items":     items,
			"__item_key":  n.ItemKey,
			"__index_key": n.IndexKey,
			"__body_node": n.BodyNode,
		},
		Next: n.NextNode,
	}, nil
}

// SubflowNode invokes another flow as a sub-process
type SubflowNode struct {
	NodeID      string            `json:"id"`
	FlowID      string            `json:"flow_id"`
	InputMap    map[string]string `json:"input_map,omitempty"` // Mapping of subflow input
	WaitForDone bool              `json:"wait_for_done"`
	NextNode    string            `json:"next,omitempty"`
}

// NewSubflowNode creates a new subflow node
func NewSubflowNode(id, flowID string, wait bool) *SubflowNode {
	return &SubflowNode{
		NodeID:      id,
		FlowID:      flowID,
		WaitForDone: wait,
		InputMap:    make(map[string]string),
	}
}

// ID returns the node ID
func (n *SubflowNode) ID() string { return n.NodeID }

// Type returns the node type
func (n *SubflowNode) Type() string { return "subflow" }

// Execute returns subflow execution metadata (actual execution handled by runner)
func (n *SubflowNode) Execute(ctx context.Context, input map[string]interface{}) (*NodeResult, error) {
	// Build subflow input from mapping
	subflowInput := make(map[string]interface{})
	if len(n.InputMap) > 0 {
		for key, path := range n.InputMap {
			val, err := extractValue(input, path)
			if err == nil {
				subflowInput[key] = val
			}
		}
	} else {
		// Pass through all input
		subflowInput = input
	}

	return &NodeResult{
		Success: true,
		Output: map[string]interface{}{
			"__subflow":       true,
			"__flow_id":       n.FlowID,
			"__subflow_input": subflowInput,
			"__wait":          n.WaitForDone,
		},
		Next: n.NextNode,
	}, nil
}

// Note: toString is defined in condition.go
