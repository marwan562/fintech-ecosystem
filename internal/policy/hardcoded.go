package policy

import (
	"context"
	"fmt"
)

// Action represents an action that can be policy-controlled
type Action string

const (
	ActionZoneCreate     Action = "zone.create"
	ActionZoneDelete     Action = "zone.delete"
	ActionFlowDeploy     Action = "flow.deploy"
	ActionFlowDeployLive Action = "flow.deploy.live"
	ActionPaymentCreate  Action = "payment.create"
	ActionRefundCreate   Action = "refund.create"
	ActionKeyCreate      Action = "key.create"
	ActionKeyRevoke      Action = "key.revoke"
	ActionUserInvite     Action = "user.invite"
	ActionUserRemove     Action = "user.remove"
	ActionSettingsUpdate Action = "settings.update"
)

// Role represents a user role
type Role string

const (
	RoleAdmin     Role = "admin"
	RoleFinance   Role = "finance"
	RoleDeveloper Role = "developer"
	RoleViewer    Role = "viewer"
)

// PolicyContext contains the context for policy evaluation
type PolicyContext struct {
	UserID   string
	OrgID    string
	ZoneID   string
	Roles    []Role
	Resource map[string]interface{}
	Action   Action
}

// PolicyResult contains the result of a policy check
type PolicyResult struct {
	Allowed bool
	Reason  string
	Rules   []string // Which rules matched
}

// PolicyEngine is the interface for policy evaluation
type PolicyEngine interface {
	Check(ctx context.Context, pctx *PolicyContext) (*PolicyResult, error)
}

// HardcodedPolicyEngine implements Phase 1 hardcoded policies
type HardcodedPolicyEngine struct{}

// NewHardcodedPolicyEngine creates a new hardcoded policy engine
func NewHardcodedPolicyEngine() *HardcodedPolicyEngine {
	return &HardcodedPolicyEngine{}
}

// Check evaluates hardcoded policies
func (e *HardcodedPolicyEngine) Check(ctx context.Context, pctx *PolicyContext) (*PolicyResult, error) {
	result := &PolicyResult{
		Allowed: false,
		Rules:   make([]string, 0),
	}

	// Check role-based permissions
	for _, role := range pctx.Roles {
		if e.roleAllowsAction(role, pctx.Action) {
			result.Allowed = true
			result.Reason = fmt.Sprintf("allowed by role: %s", role)
			result.Rules = append(result.Rules, fmt.Sprintf("role:%s", role))
			return result, nil
		}
	}

	result.Reason = "no matching policy found"
	return result, nil
}

// roleAllowsAction checks if a role permits an action
func (e *HardcodedPolicyEngine) roleAllowsAction(role Role, action Action) bool {
	// Admin can do everything
	if role == RoleAdmin {
		return true
	}

	// Role-based permissions matrix
	permissions := map[Role][]Action{
		RoleFinance: {
			ActionPaymentCreate,
			ActionRefundCreate,
			ActionFlowDeploy,
			ActionFlowDeployLive,
		},
		RoleDeveloper: {
			ActionZoneCreate,
			ActionFlowDeploy,
			ActionKeyCreate,
		},
		RoleViewer: {
			// Read-only - no write actions allowed
		},
	}

	allowedActions, ok := permissions[role]
	if !ok {
		return false
	}

	for _, allowed := range allowedActions {
		if allowed == action {
			return true
		}
	}

	return false
}

// RequireAdmin is a helper that checks if the user has admin role
func RequireAdmin(ctx context.Context, pctx *PolicyContext) error {
	for _, role := range pctx.Roles {
		if role == RoleAdmin {
			return nil
		}
	}
	return fmt.Errorf("admin role required")
}

// RequireRole is a helper that checks if the user has a specific role
func RequireRole(ctx context.Context, pctx *PolicyContext, required Role) error {
	for _, role := range pctx.Roles {
		if role == required || role == RoleAdmin {
			return nil
		}
	}
	return fmt.Errorf("role %s required", required)
}

// RequireAnyRole checks if the user has any of the specified roles
func RequireAnyRole(ctx context.Context, pctx *PolicyContext, roles ...Role) error {
	for _, role := range pctx.Roles {
		if role == RoleAdmin {
			return nil
		}
		for _, required := range roles {
			if role == required {
				return nil
			}
		}
	}
	return fmt.Errorf("one of roles %v required", roles)
}

// PolicyMiddleware wraps handlers with policy checks
type PolicyMiddleware struct {
	engine PolicyEngine
}

// NewPolicyMiddleware creates a new policy middleware
func NewPolicyMiddleware(engine PolicyEngine) *PolicyMiddleware {
	return &PolicyMiddleware{engine: engine}
}

// Check performs a policy check and returns an error if denied
func (m *PolicyMiddleware) Check(ctx context.Context, pctx *PolicyContext) error {
	result, err := m.engine.Check(ctx, pctx)
	if err != nil {
		return fmt.Errorf("policy check failed: %w", err)
	}

	if !result.Allowed {
		return fmt.Errorf("denied: %s", result.Reason)
	}

	return nil
}

// Audit logs policy decisions for compliance
type PolicyAuditLog struct {
	Timestamp string   `json:"timestamp"`
	UserID    string   `json:"userId"`
	OrgID     string   `json:"orgId"`
	Action    Action   `json:"action"`
	Resource  string   `json:"resource,omitempty"`
	Allowed   bool     `json:"allowed"`
	Reason    string   `json:"reason"`
	Rules     []string `json:"rules,omitempty"`
}
