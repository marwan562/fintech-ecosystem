package playbook_test

import (
	"testing"

	"github.com/sapliy/sapliy-core/internal/playbook"
)

func TestEvaluateRefund_Table(t *testing.T) {
	def := playbook.DefaultRefundApprovalConfig()

	tests := []struct {
		name            string
		cfg             playbook.RefundApprovalConfig
		amountCents     int64
		daysSinceCharge int
		isEnterprise    bool
		wantAction      string
		wantApproval    bool
		wantRole        string
	}{
		{
			name:            "small refund auto-approves",
			cfg:             def,
			amountCents:     5000, // $50.00
			daysSinceCharge: 5,
			wantAction:      "auto_approve",
			wantApproval:    false,
		},
		{
			name:            "exactly at threshold auto-approves",
			cfg:             def,
			amountCents:     100000, // $1,000.00
			daysSinceCharge: 5,
			wantAction:      "auto_approve",
			wantApproval:    false,
		},
		{
			name:            "large refund requires finance manager",
			cfg:             def,
			amountCents:     150000, // $1,500.00
			daysSinceCharge: 5,
			wantAction:      "require_approval",
			wantApproval:    true,
			wantRole:        "finance_manager",
		},
		{
			name:            "above auto-approve but under manager threshold routes to team lead",
			cfg:             playbook.RefundApprovalConfig{AutoApproveUnderCents: 5000, RequireApprovalOverCents: 100000, MaxRefundDays: 90},
			amountCents:     6000,
			daysSinceCharge: 5,
			wantAction:      "require_approval",
			wantApproval:    true,
			wantRole:        "team_lead",
		},
		{
			name:            "refund beyond window rejected",
			cfg:             def,
			amountCents:     5000,
			daysSinceCharge: 91,
			wantAction:      "reject",
			wantApproval:    false,
		},
		{
			name:            "enterprise large refund still requires manager",
			cfg:             def,
			amountCents:     150000,
			daysSinceCharge: 5,
			isEnterprise:    true,
			wantAction:      "require_approval",
			wantApproval:    true,
			wantRole:        "finance_manager",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := playbook.EvaluateRefund(tt.cfg, tt.amountCents, tt.daysSinceCharge, tt.isEnterprise)

			if got.Action != tt.wantAction {
				t.Errorf("Action = %q, want %q", got.Action, tt.wantAction)
			}
			if got.RequiresApproval != tt.wantApproval {
				t.Errorf("RequiresApproval = %v, want %v", got.RequiresApproval, tt.wantApproval)
			}
			if got.ApproverRole != tt.wantRole {
				t.Errorf("ApproverRole = %q, want %q", got.ApproverRole, tt.wantRole)
			}
			if got.Reason == "" {
				t.Errorf("Reason should not be empty")
			}
		})
	}
}
