package domain

import (
	"context"
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"time"

	ledgerdomain "github.com/sapliy/sapliy-core/internal/ledger/domain"
)

// ApprovalLedgerEntry represents an immutable approval decision record
type ApprovalLedgerEntry struct {
	ID             string    `json:"id"`
	ExecutionID    string    `json:"executionId"`
	NodeID         string    `json:"nodeId"`
	FlowID         string    `json:"flowId"`
	ApproverUserID string    `json:"approverUserId"`
	RequiredRole   string    `json:"requiredRole"`
	Approved       bool      `json:"approved"`
	Reason         string    `json:"reason,omitempty"`
	Timestamp      time.Time `json:"timestamp"`
	Signature      string    `json:"signature"` // HMAC-SHA256 signature for integrity
}

// ApprovalLedgerService manages immutable approval records
type ApprovalLedgerService struct {
	ledgerService *ledgerdomain.LedgerService
	secret        string // Secret for signing approval entries
}

func NewApprovalLedgerService(ledgerService *ledgerdomain.LedgerService, secret string) *ApprovalLedgerService {
	return &ApprovalLedgerService{
		ledgerService: ledgerService,
		secret:        secret,
	}
}

// RecordApprovalDecision creates an immutable ledger entry for an approval decision
func (s *ApprovalLedgerService) RecordApprovalDecision(ctx context.Context, entry ApprovalLedgerEntry, zoneID, mode string) error {
	// Set timestamp if not provided
	if entry.Timestamp.IsZero() {
		entry.Timestamp = time.Now().UTC()
	}

	// Generate cryptographic signature
	signature, err := s.signEntry(entry)
	if err != nil {
		return fmt.Errorf("failed to sign approval entry: %w", err)
	}
	entry.Signature = signature

	// Create ledger transaction
	// In a double-entry bookkeeping system, approvals are recorded as:
	// - Debit: Approval Pending Account (reduces pending approvals)
	// - Credit: Approval Completed Account (increases completed approvals)
	// This maintains the ledger balance while creating an immutable audit trail

	description := fmt.Sprintf("Approval %s: execution=%s, approver=%s, role=%s",
		map[bool]string{true: "GRANTED", false: "REJECTED"}[entry.Approved],
		entry.ExecutionID,
		entry.ApproverUserID,
		entry.RequiredRole,
	)

	txRequest := ledgerdomain.TransactionRequest{
		ReferenceID: fmt.Sprintf("approval_%s_%s", entry.ExecutionID, entry.NodeID),
		Description: description,
		Entries: []ledgerdomain.EntryRequest{
			{
				AccountID: "approval_pending", // System account for pending approvals
				Amount:    1,
				Direction: "debit",
			},
			{
				AccountID: "approval_completed", // System account for completed approvals
				Amount:    1,
				Direction: "credit",
			},
		},
	}

	// Record transaction in ledger
	if err := s.ledgerService.RecordTransaction(ctx, txRequest, zoneID, mode); err != nil {
		return fmt.Errorf("failed to record approval in ledger: %w", err)
	}

	// TODO: Store the signed approval entry in a separate approvals table
	// This would allow querying approval history without scanning the entire ledger
	// For now, the ledger transaction description contains the key information

	return nil
}

// signEntry creates an HMAC-SHA256 signature of the approval entry
func (s *ApprovalLedgerService) signEntry(entry ApprovalLedgerEntry) (string, error) {
	// Create canonical representation for signing
	canonical := fmt.Sprintf("%s|%s|%s|%s|%s|%t|%s",
		entry.ExecutionID,
		entry.NodeID,
		entry.FlowID,
		entry.ApproverUserID,
		entry.RequiredRole,
		entry.Approved,
		entry.Timestamp.UTC().Format(time.RFC3339),
	)

	// Generate HMAC signature
	mac := hmac.New(sha256.New, []byte(s.secret))
	mac.Write([]byte(canonical))
	signature := hex.EncodeToString(mac.Sum(nil))

	return signature, nil
}

// VerifyEntrySignature verifies the cryptographic signature of an approval entry
func (s *ApprovalLedgerService) VerifyEntrySignature(entry ApprovalLedgerEntry) (bool, error) {
	expectedSignature, err := s.signEntry(entry)
	if err != nil {
		return false, err
	}

	return hmac.Equal([]byte(entry.Signature), []byte(expectedSignature)), nil
}

// GetApprovalHistory retrieves approval history for a specific execution
// Note: This would require a dedicated approvals table in production
func (s *ApprovalLedgerService) GetApprovalHistory(ctx context.Context, executionID string) ([]ApprovalLedgerEntry, error) {
	// TODO: Implement by querying a dedicated approvals table
	// For now, this is a placeholder that would need to be implemented
	// when the approvals table is added to the database schema
	return nil, fmt.Errorf("not implemented: requires approvals table")
}
