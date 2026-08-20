package playbook

import (
	"time"
)

// RetrySchedule computes the retry attempt times for a failed payment.
// Attempt 1 fires at failedAt + FirstRetryDelay, each subsequent attempt
// steps by RetryStepDelay, up to MaxRetries total attempts.
func RetrySchedule(cfg DunningConfig, failedAt time.Time) []time.Time {
	cfg = cfg.WithDefaults()
	if cfg.MaxRetries <= 0 {
		return nil
	}
	times := make([]time.Time, 0, cfg.MaxRetries)
	next := failedAt.Add(cfg.FirstRetryDelay)
	for i := 0; i < cfg.MaxRetries; i++ {
		times = append(times, next)
		next = next.Add(cfg.RetryStepDelay)
	}
	return times
}

// RetryDecision is the outcome of evaluating whether a failed payment should
// be retried. Confidence reflects how strongly the rule favours the decision.
type RetryDecision struct {
	ShouldRetry   bool       `json:"shouldRetry"`
	NextAttemptAt *time.Time `json:"nextAttemptAt,omitempty"`
	Reason        string     `json:"reason"`
	Confidence    float64    `json:"confidence"`
}

// terminalOutcomes are payment outcomes that a retry will never succeed past.
var terminalOutcomes = map[string]bool{
	"hard_decline": true,
	"fraud_block":  true,
}

// DecideRetry deterministically decides whether to retry a failed payment.
// attempt is the number of retries already performed (0 for the first
// failure). lastOutcome is the raw provider outcome for the last attempt.
func DecideRetry(cfg DunningConfig, ev FinancialEvent, attempt int, lastOutcome string) RetryDecision {
	cfg = cfg.WithDefaults()

	if attempt >= cfg.MaxRetries {
		return RetryDecision{
			ShouldRetry: false,
			Reason:      "max_retries_exceeded",
			Confidence:  1.0,
		}
	}

	if terminalOutcomes[lastOutcome] {
		return RetryDecision{
			ShouldRetry: false,
			Reason:      "terminal_outcome:" + lastOutcome,
			Confidence:  0.98,
		}
	}

	schedule := RetrySchedule(cfg, ev.OccurredAt)
	if attempt >= len(schedule) {
		return RetryDecision{
			ShouldRetry: false,
			Reason:      "max_retries_exceeded",
			Confidence:  1.0,
		}
	}

	next := schedule[attempt]
	return RetryDecision{
		ShouldRetry:   true,
		NextAttemptAt: &next,
		Reason:        "retry_scheduled",
		Confidence:    0.85,
	}
}
