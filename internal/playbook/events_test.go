package playbook_test

import (
	"testing"

	"github.com/sapliy/sapliy-core/internal/playbook"
)

func TestNormalizeProviderEvent_Stripe(t *testing.T) {
	tests := []struct {
		name     string
		raw      map[string]any
		wantType playbook.FinancialEventType
		wantAmt  int64
	}{
		{
			name: "invoice.payment_failed",
			raw: map[string]any{
				"id":   "evt_1",
				"type": "invoice.payment_failed",
				"data": map[string]any{
					"object": map[string]any{
						"id":           "in_1",
						"amount_due":   4900,
						"amount":       4900,
						"currency":     "usd",
						"customer":     "cus_1",
						"subscription": "sub_1",
						"created":      1724000000,
					},
				},
			},
			wantType: playbook.PaymentFailed,
			wantAmt:  4900,
		},
		{
			name: "payment_intent.succeeded",
			raw: map[string]any{
				"type": "payment_intent.succeeded",
				"data": map[string]any{
					"object": map[string]any{
						"id":             "pi_1",
						"amount":         10000,
						"currency":       "usd",
						"customer":       "cus_1",
						"payment_method": "pm_1",
					},
				},
			},
			wantType: playbook.PaymentSucceeded,
			wantAmt:  10000,
		},
		{
			name: "invoice.upcoming maps to card expiring signal",
			raw: map[string]any{
				"type": "invoice.upcoming",
				"data": map[string]any{
					"object": map[string]any{
						"amount": 9900,
					},
				},
			},
			wantType: playbook.CardExpiring,
			wantAmt:  9900,
		},
		{
			name: "robust to missing keys",
			raw: map[string]any{
				"type": "invoice.payment_failed",
				"data": map[string]any{},
			},
			wantType: playbook.PaymentFailed,
			wantAmt:  0,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := playbook.NormalizeProviderEvent("stripe", tt.raw)
			if err != nil {
				t.Fatalf("NormalizeProviderEvent: %v", err)
			}
			if got.Type != tt.wantType {
				t.Errorf("Type = %q, want %q", got.Type, tt.wantType)
			}
			if got.AmountCents != tt.wantAmt {
				t.Errorf("AmountCents = %d, want %d", got.AmountCents, tt.wantAmt)
			}
		})
	}
}

func TestNormalizeProviderEvent_PayPal(t *testing.T) {
	tests := []struct {
		name     string
		raw      map[string]any
		wantType playbook.FinancialEventType
		wantAmt  int64
	}{
		{
			name: "PAYMENT.SALE.DENIED",
			raw: map[string]any{
				"id":         "WH-1",
				"event_type": "PAYMENT.SALE.DENIED",
				"resource": map[string]any{
					"id":                   "1FY123",
					"state":                "denied",
					"billing_agreement_id": "I-ABC123",
					"amount": map[string]any{
						"total":    "29.99",
						"currency": "USD",
					},
				},
			},
			wantType: playbook.PaymentFailed,
			wantAmt:  2999,
		},
		{
			name: "BILLING.SUBSCRIPTION.ACTIVATED",
			raw: map[string]any{
				"event_type": "BILLING.SUBSCRIPTION.ACTIVATED",
				"resource": map[string]any{
					"amount": map[string]any{
						"total":    "25.5",
						"currency": "USD",
					},
				},
			},
			wantType: playbook.PaymentSucceeded,
			wantAmt:  2550,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := playbook.NormalizeProviderEvent("paypal", tt.raw)
			if err != nil {
				t.Fatalf("NormalizeProviderEvent: %v", err)
			}
			if got.Type != tt.wantType {
				t.Errorf("Type = %q, want %q", got.Type, tt.wantType)
			}
			if got.AmountCents != tt.wantAmt {
				t.Errorf("AmountCents = %d, want %d", got.AmountCents, tt.wantAmt)
			}
		})
	}
}

func TestNormalizeProviderEvent_Unsupported(t *testing.T) {
	if _, err := playbook.NormalizeProviderEvent("unknown", map[string]any{}); err == nil {
		t.Fatalf("expected error for unknown provider")
	}
	if _, err := playbook.NormalizeProviderEvent("stripe", map[string]any{"type": "nope"}); err == nil {
		t.Fatalf("expected error for unknown stripe event type")
	}
}
