package playbook

import (
	"fmt"
	"strconv"
	"time"
)

// Engine is the playbook orchestrator. It consumes unified FinancialEvents,
// applies deterministic policies, and records every decision in a hash-chained
// DecisionLog.
type Engine struct {
	log        *DecisionLog
	dunningCfg DunningConfig
	refundCfg  RefundApprovalConfig
}

// NewEngine creates an engine using default playbook configurations.
func NewEngine() *Engine {
	return &Engine{
		log:        NewDecisionLog(),
		dunningCfg: DefaultDunningConfig(),
		refundCfg:  DefaultRefundApprovalConfig(),
	}
}

// NewEngineWithConfigs creates an engine with explicit configurations.
// Zero-valued config fields fall back to the sane defaults.
func NewEngineWithConfigs(dunning DunningConfig, refund RefundApprovalConfig) *Engine {
	return &Engine{
		log:        NewDecisionLog(),
		dunningCfg: dunning.WithDefaults(),
		refundCfg:  refund.WithDefaults(),
	}
}

// Log exposes the engine's decision log for inspection.
func (e *Engine) Log() *DecisionLog {
	return e.log
}

// HandleEvent routes a financial event through the deterministic policy state
// machine, appends each decision to the decision log, and returns the newly
// recorded entries.
func (e *Engine) HandleEvent(ev FinancialEvent, tenantID string) ([]DecisionEntry, error) {
	if ev.OccurredAt.IsZero() {
		ev.OccurredAt = time.Now().UTC()
	}

	var entries []DecisionEntry
	switch ev.Type {
	case PaymentFailed:
		entry, err := e.handlePaymentFailed(ev, tenantID)
		if err != nil {
			return nil, err
		}
		entries = append(entries, entry)
	case RefundRequested:
		entry, err := e.handleRefundRequested(ev, tenantID)
		if err != nil {
			return nil, err
		}
		entries = append(entries, entry)
	case InvoiceOverdue:
		entry, err := e.handleInvoiceOverdue(ev, tenantID)
		if err != nil {
			return nil, err
		}
		entries = append(entries, entry)
	default:
		return nil, fmt.Errorf("unsupported event type: %s", ev.Type)
	}
	return entries, nil
}

// handlePaymentFailed applies the dunning retry policy.
func (e *Engine) handlePaymentFailed(ev FinancialEvent, tenantID string) (DecisionEntry, error) {
	attempt := parseIntMeta(ev.Metadata, "retry_attempt")
	lastOutcome := ev.Metadata["last_outcome"]
	if lastOutcome == "" {
		lastOutcome = "declined"
	}

	rd := DecideRetry(e.dunningCfg, ev, attempt, lastOutcome)

	action := "escalate"
	if rd.ShouldRetry {
		action = "schedule_retry"
	}

	aiReasoning := fmt.Sprintf(
		"Dunning policy evaluated failed payment for %s: attempt=%d last_outcome=%q -> %s.",
		ev.PaymentID, attempt, lastOutcome, rd.Reason,
	)
	if rd.NextAttemptAt != nil {
		aiReasoning = fmt.Sprintf(
			"Dunning policy scheduled next retry for %s at %s (attempt=%d, last_outcome=%q).",
			ev.PaymentID, rd.NextAttemptAt.UTC().Format(time.RFC3339), attempt+1, lastOutcome,
		)
	}

	entry := DecisionEntry{
		PlaybookID:    string(PlaybookRevenueRecovery),
		TenantID:      tenantID,
		Event:         PaymentFailed,
		Action:        action,
		Reason:        rd.Reason,
		PolicyApplied: "dunning-policy",
		AIReasoning:   aiReasoning,
		Confidence:    0.85,
		Actor:         "engine",
		CreatedAt:     time.Now().UTC(),
	}
	return e.log.Append(entry)
}

// handleRefundRequested applies the refund approval policy.
func (e *Engine) handleRefundRequested(ev FinancialEvent, tenantID string) (DecisionEntry, error) {
	daysSinceCharge := parseIntMeta(ev.Metadata, "days_since_charge")
	isEnterprise := ev.Metadata["enterprise"] == "true"

	dec := EvaluateRefund(e.refundCfg, ev.AmountCents, daysSinceCharge, isEnterprise)

	action := dec.Action
	if dec.RequiresApproval {
		action = "request_approval"
	}

	aiReasoning := fmt.Sprintf(
		"Refund approval policy evaluated refund of %d %s for %s: %s.",
		ev.AmountCents, ev.Currency, ev.PaymentID, dec.Reason,
	)
	if dec.ApproverRole != "" {
		aiReasoning = fmt.Sprintf(
			"Refund approval policy routed refund of %d %s to %s: %s.",
			ev.AmountCents, ev.Currency, dec.ApproverRole, dec.Reason,
		)
	}

	entry := DecisionEntry{
		PlaybookID:    string(PlaybookRefundApproval),
		TenantID:      tenantID,
		Event:         RefundRequested,
		Action:        action,
		Reason:        dec.Reason,
		PolicyApplied: "refund-approval-policy",
		AIReasoning:   aiReasoning,
		Confidence:    1.0,
		Actor:         "engine",
		CreatedAt:     time.Now().UTC(),
	}
	return e.log.Append(entry)
}

// handleInvoiceOverdue records an invoice reminder decision.
func (e *Engine) handleInvoiceOverdue(ev FinancialEvent, tenantID string) (DecisionEntry, error) {
	entry := DecisionEntry{
		PlaybookID:    string(PlaybookInvoiceReminders),
		TenantID:      tenantID,
		Event:         InvoiceOverdue,
		Action:        "send_reminder",
		Reason:        "invoice overdue; reminder scheduled per reminder cadence",
		PolicyApplied: "invoice-reminder-policy",
		AIReasoning: fmt.Sprintf(
			"Invoice reminder policy triggered for invoice %s (%d %s outstanding).",
			ev.InvoiceID, ev.AmountCents, ev.Currency,
		),
		Confidence: 0.9,
		Actor:      "engine",
		CreatedAt:  time.Now().UTC(),
	}
	return e.log.Append(entry)
}

// parseIntMeta parses an integer from event metadata, defaulting to 0.
func parseIntMeta(meta map[string]string, key string) int {
	if meta == nil {
		return 0
	}
	v, err := strconv.Atoi(meta[key])
	if err != nil {
		return 0
	}
	return v
}
