package domain

import (
	"context"
	"crypto/hmac"
	"crypto/sha256"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"log"
	"strings"
	"time"

	authdomain "github.com/sapliy/sapliy-core/internal/auth/domain"
)

// ApprovalNodeData represents the configuration for an approval node
type ApprovalNodeData struct {
	ApproverRole  string `json:"approverRole"`  // Required role: "admin", "finance", "owner"
	TimeoutHours  int    `json:"timeoutHours"`  // Hours before approval expires
	Message       string `json:"message"`       // Template message for approval request
	AllowMultiple bool   `json:"allowMultiple"` // Allow multiple approvers
}

// ApprovalToken represents a signed JWT-like token for approval validation
type ApprovalToken struct {
	ExecutionID  string    `json:"executionId"`
	NodeID       string    `json:"nodeId"`
	RequiredRole string    `json:"requiredRole"`
	OrgID        string    `json:"orgId"`
	ExpiresAt    time.Time `json:"expiresAt"`
}

// EnhancedApprovalHandler handles approval nodes with role-based validation
type EnhancedApprovalHandler struct {
	authService *authdomain.AuthService
	secret      string // Secret for signing approval tokens
}

func NewEnhancedApprovalHandler(authService *authdomain.AuthService, secret string) *EnhancedApprovalHandler {
	return &EnhancedApprovalHandler{
		authService: authService,
		secret:      secret,
	}
}

// Execute pauses the flow execution and creates an approval requirement
func (h *EnhancedApprovalHandler) Execute(ctx context.Context, node *Node, input map[string]interface{}) (map[string]interface{}, error) {
	// Parse approval node configuration
	var approvalData ApprovalNodeData
	if err := json.Unmarshal(node.Data, &approvalData); err != nil {
		log.Printf("Failed to parse approval node data: %v", err)
		// Fallback to basic approval without role validation
		approvalData = ApprovalNodeData{
			ApproverRole: "admin",
			TimeoutHours: 24,
			Message:      "Approval required",
		}
	}

	// Validate required role is specified
	if approvalData.ApproverRole == "" {
		approvalData.ApproverRole = "admin" // Default to admin
	}

	// Normalize role to lowercase
	approvalData.ApproverRole = strings.ToLower(approvalData.ApproverRole)

	// Validate role is one of the known roles
	validRoles := map[string]bool{
		authdomain.RoleOwner:     true,
		authdomain.RoleAdmin:     true,
		authdomain.RoleFinance:   true,
		authdomain.RoleMember:    true,
		authdomain.RoleDeveloper: true,
	}

	if !validRoles[approvalData.ApproverRole] {
		return nil, fmt.Errorf("invalid approver role: %s", approvalData.ApproverRole)
	}

	log.Printf("Approval required for node %s: role=%s, timeout=%dh, message=%s",
		node.ID, approvalData.ApproverRole, approvalData.TimeoutHours, approvalData.Message)

	// Store approval metadata in execution context
	// This will be used by Resume() to validate the approver
	approvalMetadata := map[string]interface{}{
		"requiredRole":  approvalData.ApproverRole,
		"timeoutHours":  approvalData.TimeoutHours,
		"message":       approvalData.Message,
		"allowMultiple": approvalData.AllowMultiple,
		"requestedAt":   time.Now().UTC().Format(time.RFC3339),
	}

	// Return execution_paused error to signal flow should pause
	return approvalMetadata, ErrExecutionPaused
}

// ValidateApproval validates that the user has the required role to approve
func (h *EnhancedApprovalHandler) ValidateApproval(ctx context.Context, userID, orgID, requiredRole string) error {
	hasPermission, err := h.authService.HasPermission(ctx, userID, orgID, requiredRole)
	if err != nil {
		return fmt.Errorf("failed to check user permissions: %w", err)
	}

	if !hasPermission {
		return fmt.Errorf("user does not have required role: %s", requiredRole)
	}

	return nil
}

// GenerateApprovalToken creates a signed token for approval validation
func (h *EnhancedApprovalHandler) GenerateApprovalToken(executionID, nodeID, requiredRole, orgID string, expiresAt time.Time) (string, error) {
	token := ApprovalToken{
		ExecutionID:  executionID,
		NodeID:       nodeID,
		RequiredRole: requiredRole,
		OrgID:        orgID,
		ExpiresAt:    expiresAt,
	}

	// Serialize token
	tokenBytes, err := json.Marshal(token)
	if err != nil {
		return "", fmt.Errorf("failed to marshal token: %w", err)
	}

	// Create HMAC signature
	mac := hmac.New(sha256.New, []byte(h.secret))
	mac.Write(tokenBytes)
	signature := mac.Sum(nil)

	// Combine token and signature
	combined := append(tokenBytes, signature...)
	return base64.URLEncoding.EncodeToString(combined), nil
}

// ValidateApprovalToken verifies the token signature and expiration
func (h *EnhancedApprovalHandler) ValidateApprovalToken(tokenString string) (*ApprovalToken, error) {
	// Decode base64
	combined, err := base64.URLEncoding.DecodeString(tokenString)
	if err != nil {
		return nil, fmt.Errorf("invalid token encoding: %w", err)
	}

	if len(combined) < 32 {
		return nil, fmt.Errorf("token too short")
	}

	// Split token and signature (last 32 bytes are signature)
	tokenBytes := combined[:len(combined)-32]
	signature := combined[len(combined)-32:]

	// Verify signature
	mac := hmac.New(sha256.New, []byte(h.secret))
	mac.Write(tokenBytes)
	expectedSignature := mac.Sum(nil)

	if !hmac.Equal(signature, expectedSignature) {
		return nil, fmt.Errorf("invalid token signature")
	}

	// Deserialize token
	var token ApprovalToken
	if err := json.Unmarshal(tokenBytes, &token); err != nil {
		return nil, fmt.Errorf("failed to unmarshal token: %w", err)
	}

	// Check expiration
	if time.Now().After(token.ExpiresAt) {
		return nil, fmt.Errorf("approval token expired")
	}

	return &token, nil
}

// ProcessApproval handles the approval action from a user
func (h *EnhancedApprovalHandler) ProcessApproval(ctx context.Context, tokenString, userID string, approved bool) error {
	// Validate token
	token, err := h.ValidateApprovalToken(tokenString)
	if err != nil {
		return fmt.Errorf("invalid approval token: %w", err)
	}

	// Validate user has required role
	if err := h.ValidateApproval(ctx, userID, token.OrgID, token.RequiredRole); err != nil {
		return fmt.Errorf("approval denied: %w", err)
	}

	log.Printf("Approval processed: executionId=%s, nodeId=%s, userId=%s, approved=%v",
		token.ExecutionID, token.NodeID, userID, approved)

	// TODO: Update execution status and trigger flow resume
	// This would typically:
	// 1. Update FlowExecution with approval decision
	// 2. Create audit log entry
	// 3. Trigger flow resume if approved
	// 4. Create ledger entry (implemented in next phase)

	return nil
}
