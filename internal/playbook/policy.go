package playbook

import (
	"fmt"
)

// RefundDecision is the outcome of the deterministic refund approval policy.
// Action is one of: "auto_approve", "require_approval" or "reject".
type RefundDecision struct {
	Action           string   `json:"action"`
	Reason           string   `json:"reason"`
	RequiresApproval bool     `json:"requiresApproval"`
	ApproverRole     string   `json:"approverRole,omitempty"`
	Evidence         []string `json:"evidence,omitempty"`
}

// EvaluateRefund applies the refund approval policy gates in deterministic
// order:
//
//  1. amountCents > RequireApprovalOverCents  -> require_approval (finance_manager)
//  2. amountCents > AutoApproveUnderCents     -> require_approval (team_lead)
//  3. daysSinceCharge > MaxRefundDays         -> reject
//  4. otherwise                               -> auto_approve
//
// isEnterprise is recorded as evidence and may tighten the decision in future
// policy revisions.
func EvaluateRefund(cfg RefundApprovalConfig, amountCents int64, daysSinceCharge int, isEnterprise bool) RefundDecision {
	cfg = cfg.WithDefaults()

	evidence := []string{fmt.Sprintf("amount_cents=%d", amountCents)}
	if isEnterprise {
		evidence = append(evidence, "enterprise_customer=true")
	}

	if amountCents > cfg.RequireApprovalOverCents {
		return RefundDecision{
			Action:           "require_approval",
			Reason:           fmt.Sprintf("amount %d exceeds require-approval threshold of %d cents", amountCents, cfg.RequireApprovalOverCents),
			RequiresApproval: true,
			ApproverRole:     "finance_manager",
			Evidence:         append(evidence, "policy=require_approval_over_cents"),
		}
	}

	if amountCents > cfg.AutoApproveUnderCents {
		return RefundDecision{
			Action:           "require_approval",
			Reason:           fmt.Sprintf("amount %d exceeds auto-approve threshold of %d cents", amountCents, cfg.AutoApproveUnderCents),
			RequiresApproval: true,
			ApproverRole:     "team_lead",
			Evidence:         append(evidence, "policy=auto_approve_over_cents"),
		}
	}

	if daysSinceCharge > cfg.MaxRefundDays {
		return RefundDecision{
			Action:   "reject",
			Reason:   fmt.Sprintf("refund requested %d days after charge, outside %d-day window", daysSinceCharge, cfg.MaxRefundDays),
			Evidence: append(evidence, fmt.Sprintf("policy=max_refund_days days_since_charge=%d", daysSinceCharge)),
		}
	}

	return RefundDecision{
		Action:   "auto_approve",
		Reason:   fmt.Sprintf("amount %d within auto-approve threshold of %d cents and within %d-day window", amountCents, cfg.AutoApproveUnderCents, cfg.MaxRefundDays),
		Evidence: append(evidence, "policy=auto_approve"),
	}
}
