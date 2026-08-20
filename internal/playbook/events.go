package playbook

import (
	"fmt"
	"strconv"
	"strings"
	"time"
)

// FinancialEventType is the unified, provider-agnostic financial event type.
type FinancialEventType string

const (
	PaymentFailed    FinancialEventType = "payment.failed"
	PaymentSucceeded FinancialEventType = "payment.succeeded"
	InvoiceOverdue   FinancialEventType = "invoice.overdue"
	InvoicePaid      FinancialEventType = "invoice.paid"
	RefundRequested  FinancialEventType = "refund.requested"
	RefundApproved   FinancialEventType = "refund.approved"
	RefundRejected   FinancialEventType = "refund.rejected"
	CardExpiring     FinancialEventType = "card.expiring"
)

// FinancialEvent is the unified financial event model consumed by the
// playbook engine. AmountCents is fixed-point int64 (never a float).
type FinancialEvent struct {
	ID             string             `json:"id"`
	TenantID       string             `json:"tenantId"`
	Type           FinancialEventType `json:"type"`
	AmountCents    int64              `json:"amountCents"`
	Currency       string             `json:"currency"`
	CustomerID     string             `json:"customerId,omitempty"`
	SubscriptionID string             `json:"subscriptionId,omitempty"`
	InvoiceID      string             `json:"invoiceId,omitempty"`
	PaymentID      string             `json:"paymentId,omitempty"`
	OccurredAt     time.Time          `json:"occurredAt"`
	Metadata       map[string]string  `json:"metadata,omitempty"`
}

// NormalizeProviderEvent maps a raw provider webhook payload into the unified
// FinancialEvent model. It is robust to missing keys (zero values are used).
// Known providers and event types:
//
//	stripe: invoice.payment_failed | payment_intent.succeeded | invoice.upcoming
//	paypal: PAYMENT.SALE.DENIED | BILLING.SUBSCRIPTION.ACTIVATED
func NormalizeProviderEvent(provider string, raw map[string]any) (FinancialEvent, error) {
	switch provider {
	case "stripe":
		return normalizeStripeEvent(raw)
	case "paypal":
		return normalizePayPalEvent(raw)
	default:
		return FinancialEvent{}, fmt.Errorf("unsupported provider: %s", provider)
	}
}

// normalizeStripeEvent maps a Stripe webhook payload.
func normalizeStripeEvent(raw map[string]any) (FinancialEvent, error) {
	evType := strVal(raw["type"])
	var fType FinancialEventType
	switch evType {
	case "invoice.payment_failed":
		fType = PaymentFailed
	case "payment_intent.succeeded":
		fType = PaymentSucceeded
	case "invoice.upcoming":
		fType = CardExpiring
	default:
		return FinancialEvent{}, fmt.Errorf("unsupported stripe event type: %q", evType)
	}

	// Stripe nests the payload under data.object; fall back to data itself.
	obj := mapVal(raw["data"], "")
	if nested := mapVal(obj["object"], ""); len(nested) > 0 {
		obj = nested
	}

	ev := FinancialEvent{
		ID:             strVal(raw["id"]),
		Type:           fType,
		AmountCents:    int64Val(obj["amount"]),
		Currency:       strVal(obj["currency"]),
		CustomerID:     strVal(obj["customer"]),
		SubscriptionID: strVal(obj["subscription"]),
		InvoiceID:      strVal(obj["invoice"]),
		PaymentID:      strVal(obj["payment_intent"]),
	}
	if ev.InvoiceID == "" {
		// invoice.* events carry the id on the object itself.
		ev.InvoiceID = strVal(obj["id"])
	}
	if ev.PaymentID == "" && evType == "payment_intent.succeeded" {
		ev.PaymentID = strVal(obj["id"])
	}
	ev.OccurredAt = time.Unix(int64Val(obj["created"]), 0).UTC()

	meta := map[string]string{}
	if rc := mapVal(obj["outcome"], ""); strVal(rc["type"]) != "" {
		meta["last_outcome"] = strVal(rc["type"])
	}
	if rs := strVal(obj["failure_code"]); rs != "" {
		meta["failure_code"] = rs
	}
	if len(meta) > 0 {
		ev.Metadata = meta
	}
	return ev, nil
}

// normalizePayPalEvent maps a PayPal webhook payload.
func normalizePayPalEvent(raw map[string]any) (FinancialEvent, error) {
	evType := strVal(raw["event_type"])
	var fType FinancialEventType
	switch evType {
	case "PAYMENT.SALE.DENIED":
		fType = PaymentFailed
	case "BILLING.SUBSCRIPTION.ACTIVATED":
		fType = PaymentSucceeded
	default:
		return FinancialEvent{}, fmt.Errorf("unsupported paypal event type: %q", evType)
	}

	resource := mapVal(raw["resource"], "")

	ev := FinancialEvent{
		ID:          strVal(raw["id"]),
		Type:        fType,
		AmountCents: payPalAmountToCents(resource),
		Currency:    strVal(mapVal(resource["amount"], "")["currency"]),
		CustomerID:  strVal(resource["payer"]),
	}
	if subID := strVal(resource["billing_agreement_id"]); subID != "" {
		ev.SubscriptionID = subID
	}
	if paymentID := strVal(resource["id"]); paymentID != "" {
		ev.PaymentID = paymentID
	}
	ev.OccurredAt = time.Unix(int64Val(resource["create_time"]), 0).UTC()

	meta := map[string]string{}
	if sc := strVal(resource["state"]); sc != "" {
		meta["paypal_state"] = sc
	}
	if len(meta) > 0 {
		ev.Metadata = meta
	}
	return ev, nil
}

// payPalAmountToCents converts a PayPal decimal amount string ("25.00") into
// fixed-point cents (2500). Malformed values map to 0.
func payPalAmountToCents(resource map[string]any) int64 {
	amt := mapVal(resource["amount"], "")
	total := strVal(amt["total"])
	return decimalToCents(total)
}

// decimalToCents parses a decimal money string ("25.00", "25.5", "25") into
// int64 cents. Non-numeric input returns 0.
func decimalToCents(s string) int64 {
	s = strings.TrimSpace(s)
	if s == "" {
		return 0
	}
	parts := strings.Split(s, ".")
	whole, err := strconv.ParseInt(parts[0], 10, 64)
	if err != nil {
		return 0
	}
	frac := int64(0)
	if len(parts) > 1 {
		f := parts[1]
		if len(f) > 2 {
			f = f[:2]
		}
		frac, _ = strconv.ParseInt(f, 10, 64)
		switch len(f) {
		case 1:
			frac *= 10
		case 2:
			// already hundredths
		default:
			frac = 0
		}
	}
	return whole*100 + frac
}

// strVal extracts a string value from a generic map, returning "" if missing.
func strVal(v any) string {
	if v == nil {
		return ""
	}
	if s, ok := v.(string); ok {
		return s
	}
	return ""
}

// int64Val extracts an int64 value from a generic map, returning 0 if missing.
// Stripe amounts arrive as JSON numbers which decode as float64.
func int64Val(v any) int64 {
	switch n := v.(type) {
	case int64:
		return n
	case int:
		return int64(n)
	case float64:
		return int64(n)
	case string:
		i, _ := strconv.ParseInt(n, 10, 64)
		return i
	default:
		return 0
	}
}

// mapVal extracts a nested map[string]any from a generic map.
// Returns an empty map when the value is absent or not an object.
func mapVal(v any, _ string) map[string]any {
	if m, ok := v.(map[string]any); ok {
		return m
	}
	return map[string]any{}
}
