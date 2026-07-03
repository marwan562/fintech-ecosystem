package domain

import (
	"context"
	"fmt"

	"github.com/sapliy/sapliy-core/internal/policy"
)

// PolicyEngine handles RBAC checks by wrapping the unified policy engine.
type PolicyEngine struct {
	engine policy.PolicyEngine
}

func NewPolicyEngine() *PolicyEngine {
	return &PolicyEngine{
		engine: policy.NewEngine(),
	}
}

// Can check if a user with a specific role can perform an action on a resource.
// Mapping old (action, resource) to new "resource.action" format.
func (e *PolicyEngine) Can(role string, action string, resource string) bool {
	// Special handling for Owner/Admin who bypass all checks
	if role == RoleOwner || role == RoleAdmin {
		return true
	}

	// Map old style to new style "resource.action"
	// This is a bridge for Phase 2
	unifiedAction := policy.Action(fmt.Sprintf("%s.%s", resource, action))

	pctx := &policy.PolicyContext{
		Roles:  []policy.Role{policy.Role(role)},
		Action: unifiedAction,
	}

	result, err := e.engine.Check(context.Background(), pctx)
	if err != nil {
		return false
	}

	return result.Allowed
}

func (e *PolicyEngine) ValidateAction(role string, action string, resource string) error {
	if !e.Can(role, action, resource) {
		return fmt.Errorf("role %s is not authorized to %s resource %s", role, action, resource)
	}
	return nil
}
