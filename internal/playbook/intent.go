package playbook

import (
	"fmt"
	"regexp"
	"strconv"
	"strings"
)

// IntentStep is a single concrete action in an IntentPreview plan.
// Risk is one of "low", "medium" or "high".
type IntentStep struct {
	Title       string `json:"title"`
	Description string `json:"description"`
	Risk        string `json:"risk"`
}

// IntentPreview is the deterministic plan produced from a natural-language
// goal. It is derived by rules only — never from a raw LLM response — so that
// AI text can never steer execution.
type IntentPreview struct {
	PlaybookType     PlaybookType `json:"playbookType"`
	Summary          string       `json:"summary"`
	Steps            []IntentStep `json:"steps"`
	Confidence       float64      `json:"confidence"`
	RequiresApproval bool         `json:"requiresApproval"`
}

// keywordRule maps a set of goal keywords to a playbook type, summary and
// deterministic confidence.
type keywordRule struct {
	keywords   []string
	playbook   PlaybookType
	summary    string
	confidence float64
	steps      []IntentStep
}

// ParseIntent deterministically maps a natural-language goal to a playbook
// type and concrete plan. Matching is order-sensitive: revenue-recovery is
// checked first, then refund-approval, then invoice-reminders. Goals that
// match nothing fall back to a generic operational-workflow plan that asks
// for clarification. RequiresApproval is set when the goal mentions an
// amount above $1,000 or approval/escalation/enterprise concerns.
func ParseIntent(goal string) IntentPreview {
	g := strings.ToLower(strings.TrimSpace(goal))
	if g == "" {
		g = "clarify operational goal"
	}

	requiresApproval := mentionsAmountOver(g, 1000) || containsAnyWord(g, "approve", "approval", "escalate", "enterprise")

	rules := []keywordRule{
		{
			keywords:   []string{"recover", "failed", "payment", "churn", "retry", "dunning"},
			playbook:   PlaybookRevenueRecovery,
			summary:    "Recover failed subscription payments with smart retries and dunning",
			confidence: 0.92,
			steps: []IntentStep{
				{Title: "Detect failed payment", Description: "Consume the provider payment.failed event and enrich with customer context", Risk: "low"},
				{Title: "Evaluate policy", Description: "Run the dunning policy to decide whether the payment is retryable", Risk: "low"},
				{Title: "Schedule smart retry", Description: "Schedule the first retry at ~5h, then step by days 3/5/7 up to the retry cap", Risk: "medium"},
				{Title: "Notify customer via email/SMS", Description: "Send a magic-link recovery message over the configured channels", Risk: "low"},
				{Title: "Verify result", Description: "Confirm the retry succeeded or escalate terminal failures", Risk: "medium"},
				{Title: "Write audit decision", Description: "Append a hash-chained decision entry with the reason and policy applied", Risk: "low"},
			},
		},
		{
			keywords:   []string{"refund", "return", "chargeback"},
			playbook:   PlaybookRefundApproval,
			summary:    "Route refunds and financial adjustments through the refund approval policy",
			confidence: 0.88,
			steps: []IntentStep{
				{Title: "Capture refund request", Description: "Consume the provider refund.requested event and normalize the amount", Risk: "low"},
				{Title: "Evaluate refund policy", Description: "Check amount thresholds, refund window and customer status against policy", Risk: "medium"},
				{Title: "Route approval if over threshold", Description: "Require finance-manager approval for amounts above $1,000", Risk: "high"},
				{Title: "Execute refund", Description: "Dispatch the approved refund to the payment provider", Risk: "medium"},
				{Title: "Notify customer", Description: "Inform the customer of the refund decision", Risk: "low"},
				{Title: "Write audit decision", Description: "Append a hash-chained decision entry with the reason and policy applied", Risk: "low"},
			},
		},
		{
			keywords:   []string{"invoice", "overdue", "reminder", "collection", "past due"},
			playbook:   PlaybookInvoiceReminders,
			summary:    "Send automated reminders for overdue invoices",
			confidence: 0.86,
			steps: []IntentStep{
				{Title: "Detect overdue invoice", Description: "Consume the provider invoice.overdue event and compute days outstanding", Risk: "low"},
				{Title: "Evaluate reminder cadence", Description: "Apply the reminder schedule: first touch before due, then every 7 days", Risk: "low"},
				{Title: "Send email/SMS reminder", Description: "Deliver the reminder over the configured channels", Risk: "low"},
				{Title: "Track response", Description: "Watch for payment or customer reply to stop the cadence", Risk: "medium"},
				{Title: "Escalate to collection", Description: "Route persistent non-payment to the collections workflow", Risk: "high"},
				{Title: "Write audit decision", Description: "Append a hash-chained decision entry with the reason and policy applied", Risk: "low"},
			},
		},
	}

	for _, r := range rules {
		if containsAnyWord(g, r.keywords...) {
			return IntentPreview{
				PlaybookType:     r.playbook,
				Summary:          r.summary,
				Steps:            r.steps,
				Confidence:       r.confidence,
				RequiresApproval: requiresApproval,
			}
		}
	}

	return IntentPreview{
		PlaybookType:     PlaybookType("operational_workflow"),
		Summary:          "Generic operational workflow — clarify the business goal",
		Confidence:       0.72,
		RequiresApproval: requiresApproval,
		Steps: []IntentStep{
			{Title: "Clarify business goal", Description: "Ask the user to restate the outcome in terms of payments, refunds or invoicing", Risk: "low"},
			{Title: "Recommend a playbook", Description: "Map the clarified goal to the best-matching operational playbook", Risk: "medium"},
			{Title: "Write audit decision", Description: "Record the recommendation in the hash-chained decision log", Risk: "low"},
		},
	}
}

var amountRe = regexp.MustCompile(`\$?\d+(?:,\d{3})*(?:\.\d+)?`)

// containsAnyWord reports whether any keyword appears as a substring of the
// normalized goal. Normalization lowercases and collapses punctuation to
// spaces, so "past-due" matches the "past due" keyword and plural forms match
// their singular keyword ("payments" matches "payment").
func containsAnyWord(goal string, keywords ...string) bool {
	g := normalizeGoal(goal)
	for _, kw := range keywords {
		if strings.Contains(g, kw) {
			return true
		}
	}
	return false
}

// normalizeGoal lowercases goal and replaces every non-alphanumeric rune with
// a space so keyword substrings match across punctuation.
func normalizeGoal(goal string) string {
	var b strings.Builder
	for _, r := range strings.ToLower(goal) {
		if (r >= 'a' && r <= 'z') || (r >= '0' && r <= '9') {
			b.WriteRune(r)
		} else {
			b.WriteByte(' ')
		}
	}
	return b.String()
}

// mentionsAmountOver reports whether g contains a numeric amount (with
// optional $ prefix and thousands separators) strictly above threshold.
func mentionsAmountOver(g string, threshold float64) bool {
	for _, m := range amountRe.FindAllString(g, -1) {
		s := strings.ReplaceAll(strings.ReplaceAll(m, "$", ""), ",", "")
		if v, err := strconv.ParseFloat(s, 64); err == nil && v > threshold {
			return true
		}
	}
	return false
}

// String returns a compact human-readable summary of the preview, used for
// logging and CLI output.
func (p IntentPreview) String() string {
	return fmt.Sprintf("%s (confidence %.2f, requiresApproval=%t)", p.PlaybookType, p.Confidence, p.RequiresApproval)
}
