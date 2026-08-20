package playbook_test

import (
	"testing"
	"time"

	"github.com/sapliy/sapliy-core/internal/playbook"
)

func TestEngine_HandleEvent_PaymentFailed(t *testing.T) {
	engine := playbook.NewEngine()
	failedAt := time.Date(2026, 8, 19, 12, 0, 0, 0, time.UTC)

	ev := playbook.FinancialEvent{
		ID:          "evt_1",
		Type:        playbook.PaymentFailed,
		AmountCents: 4900,
		Currency:    "usd",
		PaymentID:   "pi_123",
		OccurredAt:  failedAt,
		Metadata:    map[string]string{"last_outcome": "declined"},
	}

	entries, err := engine.HandleEvent(ev, "tenant-1")
	if err != nil {
		t.Fatalf("HandleEvent: %v", err)
	}
	if len(entries) != 1 {
		t.Fatalf("entries = %d, want 1", len(entries))
	}

	e := entries[0]
	if e.Event != playbook.PaymentFailed {
		t.Errorf("Event = %q", e.Event)
	}
	if e.Action != "schedule_retry" {
		t.Errorf("Action = %q, want schedule_retry", e.Action)
	}
	if e.PolicyApplied != "dunning-policy" {
		t.Errorf("PolicyApplied = %q", e.PolicyApplied)
	}
	if e.Reason != "retry_scheduled" {
		t.Errorf("Reason = %q", e.Reason)
	}
	if e.Confidence != 0.85 {
		t.Errorf("Confidence = %v, want 0.85", e.Confidence)
	}
	if e.TenantID != "tenant-1" {
		t.Errorf("TenantID = %q", e.TenantID)
	}

	// Decision was appended to the engine's log with a valid hash.
	log := engine.Log()
	if got := len(log.Entries()); got != 1 {
		t.Errorf("log length = %d, want 1", got)
	}
	if err := log.Verify(); err != nil {
		t.Errorf("log Verify(): %v", err)
	}
}

func TestEngine_HandleEvent_PaymentFailedEscalatesAfterMaxRetries(t *testing.T) {
	engine := playbook.NewEngine()
	failedAt := time.Date(2026, 8, 19, 12, 0, 0, 0, time.UTC)

	ev := playbook.FinancialEvent{
		Type:        playbook.PaymentFailed,
		AmountCents: 4900,
		PaymentID:   "pi_123",
		OccurredAt:  failedAt,
		Metadata:    map[string]string{"last_outcome": "declined", "retry_attempt": "4"},
	}

	entries, err := engine.HandleEvent(ev, "tenant-1")
	if err != nil {
		t.Fatalf("HandleEvent: %v", err)
	}
	if entries[0].Action != "escalate" {
		t.Errorf("Action = %q, want escalate", entries[0].Action)
	}
	if entries[0].Reason != "max_retries_exceeded" {
		t.Errorf("Reason = %q, want max_retries_exceeded", entries[0].Reason)
	}
}

func TestEngine_HandleEvent_PaymentFailedHardDeclineEscalates(t *testing.T) {
	engine := playbook.NewEngine()
	failedAt := time.Date(2026, 8, 19, 12, 0, 0, 0, time.UTC)

	ev := playbook.FinancialEvent{
		Type:        playbook.PaymentFailed,
		AmountCents: 4900,
		PaymentID:   "pi_123",
		OccurredAt:  failedAt,
		Metadata:    map[string]string{"last_outcome": "hard_decline"},
	}

	entries, err := engine.HandleEvent(ev, "tenant-1")
	if err != nil {
		t.Fatalf("HandleEvent: %v", err)
	}
	if entries[0].Action != "escalate" {
		t.Errorf("Action = %q, want escalate", entries[0].Action)
	}
	if entries[0].Reason != "terminal_outcome:hard_decline" {
		t.Errorf("Reason = %q", entries[0].Reason)
	}
}

func TestEngine_HandleEvent_RefundRequested(t *testing.T) {
	engine := playbook.NewEngine()

	tests := []struct {
		name        string
		amountCents int64
		meta        map[string]string
		wantAction  string
		wantRole    string
	}{
		{
			name:        "small refund auto-approves",
			amountCents: 5000,
			meta:        map[string]string{"days_since_charge": "5"},
			wantAction:  "auto_approve",
		},
		{
			name:        "large refund requests finance manager approval",
			amountCents: 150000,
			meta:        map[string]string{"days_since_charge": "5"},
			wantAction:  "request_approval",
			wantRole:    "finance_manager",
		},
		{
			name:        "refund beyond window rejected",
			amountCents: 5000,
			meta:        map[string]string{"days_since_charge": "120"},
			wantAction:  "reject",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ev := playbook.FinancialEvent{
				Type:        playbook.RefundRequested,
				AmountCents: tt.amountCents,
				Currency:    "usd",
				PaymentID:   "pi_456",
				OccurredAt:  time.Now().UTC(),
				Metadata:    tt.meta,
			}
			entries, err := engine.HandleEvent(ev, "tenant-1")
			if err != nil {
				t.Fatalf("HandleEvent: %v", err)
			}
			if len(entries) != 1 {
				t.Fatalf("entries = %d, want 1", len(entries))
			}
			e := entries[0]
			if e.Event != playbook.RefundRequested {
				t.Errorf("Event = %q", e.Event)
			}
			if e.Action != tt.wantAction {
				t.Errorf("Action = %q, want %q", e.Action, tt.wantAction)
			}
			if e.PolicyApplied != "refund-approval-policy" {
				t.Errorf("PolicyApplied = %q", e.PolicyApplied)
			}
			if tt.wantRole != "" && e.AIReasoning == "" {
				t.Errorf("AIReasoning should mention approval routing for role %q", tt.wantRole)
			}
		})
	}

	// All refund decisions were appended to the same log, in order.
	if got := len(engine.Log().Entries()); got != len(tests) {
		t.Errorf("log length = %d, want %d", got, len(tests))
	}
}

func TestEngine_HandleEvent_InvoiceOverdue(t *testing.T) {
	engine := playbook.NewEngine()

	ev := playbook.FinancialEvent{
		Type:        playbook.InvoiceOverdue,
		AmountCents: 12000,
		Currency:    "usd",
		InvoiceID:   "in_789",
		OccurredAt:  time.Now().UTC(),
	}

	entries, err := engine.HandleEvent(ev, "tenant-1")
	if err != nil {
		t.Fatalf("HandleEvent: %v", err)
	}
	e := entries[0]
	if e.Event != playbook.InvoiceOverdue {
		t.Errorf("Event = %q", e.Event)
	}
	if e.Action != "send_reminder" {
		t.Errorf("Action = %q, want send_reminder", e.Action)
	}
	if e.PolicyApplied != "invoice-reminder-policy" {
		t.Errorf("PolicyApplied = %q", e.PolicyApplied)
	}

	if err := engine.Log().Verify(); err != nil {
		t.Errorf("log Verify(): %v", err)
	}
}

func TestEngine_HandleEvent_UnsupportedType(t *testing.T) {
	engine := playbook.NewEngine()

	ev := playbook.FinancialEvent{
		Type:       playbook.FinancialEventType("some.unknown.event"),
		OccurredAt: time.Now().UTC(),
	}

	if _, err := engine.HandleEvent(ev, "tenant-1"); err == nil {
		t.Fatalf("HandleEvent should error on unsupported event type")
	}
	if got := len(engine.Log().Entries()); got != 0 {
		t.Errorf("log length = %d, want 0 (no entry appended for unknown type)", got)
	}
}
