package playbook_test

import (
	"testing"

	"github.com/sapliy/sapliy-core/internal/playbook"
)

func TestParseIntent(t *testing.T) {
	tests := []struct {
		name              string
		goal              string
		wantType          playbook.PlaybookType
		wantRequiresAppr  bool
		wantMinConfidence float64
		wantMaxConfidence float64
		wantMinSteps      int
	}{
		{
			name:              "revenue recovery goal",
			goal:              "recover failed subscription payments",
			wantType:          playbook.PlaybookRevenueRecovery,
			wantRequiresAppr:  false,
			wantMinConfidence: 0.72,
			wantMaxConfidence: 0.95,
			wantMinSteps:      5,
		},
		{
			name:              "revenue recovery via retry keyword",
			goal:              "set up smart retries and dunning for churn",
			wantType:          playbook.PlaybookRevenueRecovery,
			wantMinConfidence: 0.72,
			wantMaxConfidence: 0.95,
			wantMinSteps:      5,
		},
		{
			name:              "refund approval goal",
			goal:              "process refunds and returns for customers",
			wantType:          playbook.PlaybookRefundApproval,
			wantMinConfidence: 0.72,
			wantMaxConfidence: 0.95,
			wantMinSteps:      5,
		},
		{
			name:              "invoice reminders goal",
			goal:              "send reminders for overdue invoices",
			wantType:          playbook.PlaybookInvoiceReminders,
			wantMinConfidence: 0.72,
			wantMaxConfidence: 0.95,
			wantMinSteps:      5,
		},
		{
			name:              "invoice reminders past-due hyphenated",
			goal:              "collection workflow for past-due invoices",
			wantType:          playbook.PlaybookInvoiceReminders,
			wantMinConfidence: 0.72,
			wantMaxConfidence: 0.95,
			wantMinSteps:      5,
		},
		{
			name:              "requires approval on amount over 1000",
			goal:              "approve a refund of $2,500",
			wantType:          playbook.PlaybookRefundApproval,
			wantRequiresAppr:  true,
			wantMinConfidence: 0.72,
			wantMaxConfidence: 0.95,
			wantMinSteps:      5,
		},
		{
			name:              "requires approval on escalate keyword",
			goal:              "recover failed payments and escalate enterprise accounts",
			wantType:          playbook.PlaybookRevenueRecovery,
			wantRequiresAppr:  true,
			wantMinConfidence: 0.72,
			wantMaxConfidence: 0.95,
			wantMinSteps:      5,
		},
		{
			name:              "amount exactly 1000 does not require approval",
			goal:              "refund 1000 dollars",
			wantType:          playbook.PlaybookRefundApproval,
			wantMinConfidence: 0.72,
			wantMaxConfidence: 0.95,
			wantMinSteps:      5,
		},
		{
			name:              "unknown goal falls back to generic workflow",
			goal:              "optimize our north star metric",
			wantType:          playbook.PlaybookType("operational_workflow"),
			wantMinConfidence: 0.72,
			wantMaxConfidence: 0.72,
			wantMinSteps:      1,
		},
		{
			name:              "empty goal falls back to generic workflow",
			goal:              "",
			wantType:          playbook.PlaybookType("operational_workflow"),
			wantMinConfidence: 0.72,
			wantMaxConfidence: 0.72,
			wantMinSteps:      1,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := playbook.ParseIntent(tt.goal)

			if got.PlaybookType != tt.wantType {
				t.Errorf("PlaybookType = %q, want %q", got.PlaybookType, tt.wantType)
			}
			if got.RequiresApproval != tt.wantRequiresAppr {
				t.Errorf("RequiresApproval = %v, want %v", got.RequiresApproval, tt.wantRequiresAppr)
			}
			if got.Confidence < tt.wantMinConfidence || got.Confidence > tt.wantMaxConfidence {
				t.Errorf("Confidence = %v, want within [%v, %v]", got.Confidence, tt.wantMinConfidence, tt.wantMaxConfidence)
			}
			if len(got.Steps) < tt.wantMinSteps {
				t.Errorf("len(Steps) = %d, want >= %d", len(got.Steps), tt.wantMinSteps)
			}
			for _, s := range got.Steps {
				if s.Title == "" {
					t.Errorf("step has empty title: %+v", s)
				}
				switch s.Risk {
				case "low", "medium", "high":
				default:
					t.Errorf("step risk %q is not low|medium|high", s.Risk)
				}
			}
			if got.Summary == "" {
				t.Error("Summary should not be empty")
			}
		})
	}
}

func TestParseIntent_Deterministic(t *testing.T) {
	goal := "recover failed subscription payments"
	a := playbook.ParseIntent(goal)
	b := playbook.ParseIntent(goal)
	if a.PlaybookType != b.PlaybookType || a.Confidence != b.Confidence || len(a.Steps) != len(b.Steps) {
		t.Fatal("ParseIntent must be deterministic for the same goal")
	}
}

func TestParseIntent_RevenueRecoverySteps(t *testing.T) {
	got := playbook.ParseIntent("recover failed subscription payments")

	wantTitles := []string{
		"Detect failed payment",
		"Evaluate policy",
		"Schedule smart retry",
		"Notify customer via email/SMS",
		"Verify result",
		"Write audit decision",
	}
	if len(got.Steps) != len(wantTitles) {
		t.Fatalf("len(Steps) = %d, want %d", len(got.Steps), len(wantTitles))
	}
	for i, w := range wantTitles {
		if got.Steps[i].Title != w {
			t.Errorf("Steps[%d].Title = %q, want %q", i, got.Steps[i].Title, w)
		}
	}
}
