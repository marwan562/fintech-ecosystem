package nodes

import (
	"context"
	"encoding/json"
	"fmt"
	"regexp"
	"strconv"
	"strings"
)

// Node is the interface for all flow nodes
type Node interface {
	ID() string
	Type() string
	Execute(ctx context.Context, input map[string]interface{}) (*NodeResult, error)
}

// NodeResult represents the output of a node execution
type NodeResult struct {
	Success bool                   `json:"success"`
	Output  map[string]interface{} `json:"output,omitempty"`
	Error   string                 `json:"error,omitempty"`
	Next    string                 `json:"next,omitempty"` // ID of next node to execute
}

// ConditionNode evaluates conditions to determine flow path
type ConditionNode struct {
	NodeID      string `json:"id"`
	Conditions  []Rule `json:"conditions"`
	TrueNext    string `json:"trueNext"`    // Node ID if condition is true
	FalseNext   string `json:"falseNext"`   // Node ID if condition is false
	CombineWith string `json:"combineWith"` // "and" or "or"
}

// Rule represents a single condition rule
type Rule struct {
	Field    string `json:"field"`    // JSONPath to field in input
	Operator string `json:"operator"` // eq, neq, gt, gte, lt, lte, contains, matches
	Value    string `json:"value"`    // Expected value (can use {{variables}})
}

// NewConditionNode creates a new condition node
func NewConditionNode(id string, rules []Rule, trueNext, falseNext string) *ConditionNode {
	return &ConditionNode{
		NodeID:      id,
		Conditions:  rules,
		TrueNext:    trueNext,
		FalseNext:   falseNext,
		CombineWith: "and",
	}
}

// ID returns the node ID
func (n *ConditionNode) ID() string {
	return n.NodeID
}

// Type returns the node type
func (n *ConditionNode) Type() string {
	return "condition"
}

// Execute evaluates the conditions
func (n *ConditionNode) Execute(ctx context.Context, input map[string]interface{}) (*NodeResult, error) {
	allPassed := n.CombineWith == "and"

	for _, rule := range n.Conditions {
		passed, err := n.evaluateRule(rule, input)
		if err != nil {
			return &NodeResult{
				Success: false,
				Error:   fmt.Sprintf("failed to evaluate rule: %v", err),
			}, nil
		}

		if n.CombineWith == "and" {
			allPassed = allPassed && passed
		} else { // or
			allPassed = allPassed || passed
		}
	}

	next := n.FalseNext
	if allPassed {
		next = n.TrueNext
	}

	return &NodeResult{
		Success: true,
		Output: map[string]interface{}{
			"conditionMet": allPassed,
		},
		Next: next,
	}, nil
}

// evaluateRule evaluates a single rule
func (n *ConditionNode) evaluateRule(rule Rule, input map[string]interface{}) (bool, error) {
	// Extract field value from input
	fieldValue, err := extractValue(input, rule.Field)
	if err != nil {
		return false, err
	}

	// Resolve variables in expected value
	expectedValue := n.resolveVariables(rule.Value, input)

	// Compare based on operator
	switch rule.Operator {
	case "eq", "==", "equals":
		return compareEqual(fieldValue, expectedValue), nil
	case "neq", "!=", "not_equals":
		return !compareEqual(fieldValue, expectedValue), nil
	case "gt", ">":
		return compareNumeric(fieldValue, expectedValue, ">"), nil
	case "gte", ">=":
		return compareNumeric(fieldValue, expectedValue, ">="), nil
	case "lt", "<":
		return compareNumeric(fieldValue, expectedValue, "<"), nil
	case "lte", "<=":
		return compareNumeric(fieldValue, expectedValue, "<="), nil
	case "contains":
		return strings.Contains(toString(fieldValue), expectedValue), nil
	case "matches":
		re, err := regexp.Compile(expectedValue)
		if err != nil {
			return false, fmt.Errorf("invalid regex: %w", err)
		}
		return re.MatchString(toString(fieldValue)), nil
	case "exists":
		return fieldValue != nil, nil
	case "not_exists":
		return fieldValue == nil, nil
	default:
		return false, fmt.Errorf("unknown operator: %s", rule.Operator)
	}
}

// resolveVariables replaces {{var}} placeholders with actual values
func (n *ConditionNode) resolveVariables(template string, input map[string]interface{}) string {
	re := regexp.MustCompile(`\{\{([^}]+)\}\}`)
	return re.ReplaceAllStringFunc(template, func(match string) string {
		path := strings.TrimSpace(match[2 : len(match)-2])
		val, err := extractValue(input, path)
		if err != nil {
			return match
		}
		return toString(val)
	})
}

// extractValue extracts a value from a nested map using dot notation
func extractValue(data map[string]interface{}, path string) (interface{}, error) {
	parts := strings.Split(path, ".")
	current := interface{}(data)

	for _, part := range parts {
		switch v := current.(type) {
		case map[string]interface{}:
			var ok bool
			current, ok = v[part]
			if !ok {
				return nil, nil // Field doesn't exist
			}
		default:
			return nil, fmt.Errorf("cannot traverse into %T", current)
		}
	}

	return current, nil
}

// compareEqual compares two values for equality
func compareEqual(a, b interface{}) bool {
	// Handle JSON comparison
	aj, _ := json.Marshal(a)
	bj, _ := json.Marshal(b)
	if string(aj) == string(bj) {
		return true
	}

	// String comparison
	return toString(a) == toString(b)
}

// compareNumeric compares two values numerically
func compareNumeric(a interface{}, b string, op string) bool {
	aFloat, err1 := toFloat(a)
	bFloat, err2 := strconv.ParseFloat(b, 64)

	if err1 != nil || err2 != nil {
		return false
	}

	switch op {
	case ">":
		return aFloat > bFloat
	case ">=":
		return aFloat >= bFloat
	case "<":
		return aFloat < bFloat
	case "<=":
		return aFloat <= bFloat
	}
	return false
}

// toString converts any value to string
func toString(v interface{}) string {
	switch val := v.(type) {
	case string:
		return val
	case nil:
		return ""
	default:
		b, _ := json.Marshal(val)
		return string(b)
	}
}

// toFloat converts any value to float64
func toFloat(v interface{}) (float64, error) {
	switch val := v.(type) {
	case float64:
		return val, nil
	case float32:
		return float64(val), nil
	case int:
		return float64(val), nil
	case int64:
		return float64(val), nil
	case string:
		return strconv.ParseFloat(val, 64)
	default:
		return 0, fmt.Errorf("cannot convert %T to float", v)
	}
}
