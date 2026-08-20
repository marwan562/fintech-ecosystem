package playbook_test

import (
	"testing"
	"time"

	"github.com/sapliy/sapliy-core/internal/playbook"
)

func TestRetrySchedule_CountAndOffsets(t *testing.T) {
	failedAt := time.Date(2026, 8, 19, 12, 0, 0, 0, time.UTC)
	cfg := playbook.DefaultDunningConfig() // MaxRetries=4, First=5h, Step=48h

	got := playbook.RetrySchedule(cfg, failedAt)

	if len(got) != cfg.MaxRetries {
		t.Fatalf("RetrySchedule() count = %d, want %d", len(got), cfg.MaxRetries)
	}

	want := []time.Time{
		failedAt.Add(5 * time.Hour),
		failedAt.Add(5 * time.Hour).Add(48 * time.Hour),
		failedAt.Add(5 * time.Hour).Add(48 * time.Hour).Add(48 * time.Hour),
		failedAt.Add(5 * time.Hour).Add(48 * time.Hour).Add(48 * time.Hour).Add(48 * time.Hour),
	}
	for i, w := range want {
		if !got[i].Equal(w) {
			t.Errorf("attempt %d = %v, want %v", i, got[i], w)
		}
	}
}

func TestRetrySchedule_RespectsMaxRetries(t *testing.T) {
	failedAt := time.Date(2026, 8, 19, 12, 0, 0, 0, time.UTC)

	cfg := playbook.DefaultDunningConfig()
	cfg.MaxRetries = 2
	if got := playbook.RetrySchedule(cfg, failedAt); len(got) != 2 {
		t.Errorf("RetrySchedule() count = %d, want 2", len(got))
	}
}

func TestRetrySchedule_AppliesSaneDefaults(t *testing.T) {
	failedAt := time.Date(2026, 8, 19, 12, 0, 0, 0, time.UTC)

	got := playbook.RetrySchedule(playbook.DunningConfig{}, failedAt)
	if len(got) != 4 {
		t.Fatalf("RetrySchedule() with zero config count = %d, want 4 (default MaxRetries)", len(got))
	}
	if !got[0].Equal(failedAt.Add(5 * time.Hour)) {
		t.Errorf("attempt 1 = %v, want failedAt+5h (default FirstRetryDelay)", got[0])
	}
}

func TestDecideRetry_Table(t *testing.T) {
	failedAt := time.Date(2026, 8, 19, 12, 0, 0, 0, time.UTC)
	baseCfg := playbook.DefaultDunningConfig()

	tests := []struct {
		name        string
		cfg         playbook.DunningConfig
		attempt     int
		lastOutcome string
		wantRetry   bool
		wantReason  string
		wantNextAt  *time.Time
	}{
		{
			name:        "first failure schedules retry",
			cfg:         baseCfg,
			attempt:     0,
			lastOutcome: "declined",
			wantRetry:   true,
			wantReason:  "retry_scheduled",
			wantNextAt:  timePtr(failedAt.Add(5 * time.Hour)),
		},
		{
			name:        "second retry steps forward",
			cfg:         baseCfg,
			attempt:     1,
			lastOutcome: "insufficient_funds",
			wantRetry:   true,
			wantReason:  "retry_scheduled",
			wantNextAt:  timePtr(failedAt.Add(5 * time.Hour).Add(48 * time.Hour)),
		},
		{
			name:        "hard decline is terminal",
			cfg:         baseCfg,
			attempt:     0,
			lastOutcome: "hard_decline",
			wantRetry:   false,
			wantReason:  "terminal_outcome:hard_decline",
		},
		{
			name:        "fraud block is terminal",
			cfg:         baseCfg,
			attempt:     0,
			lastOutcome: "fraud_block",
			wantRetry:   false,
			wantReason:  "terminal_outcome:fraud_block",
		},
		{
			name:        "max retries exceeded",
			cfg:         mustCfg(baseCfg, 2),
			attempt:     2,
			lastOutcome: "declined",
			wantRetry:   false,
			wantReason:  "max_retries_exceeded",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ev := playbook.FinancialEvent{OccurredAt: failedAt}
			got := playbook.DecideRetry(tt.cfg, ev, tt.attempt, tt.lastOutcome)

			if got.ShouldRetry != tt.wantRetry {
				t.Errorf("ShouldRetry = %v, want %v", got.ShouldRetry, tt.wantRetry)
			}
			if got.Reason != tt.wantReason {
				t.Errorf("Reason = %q, want %q", got.Reason, tt.wantReason)
			}
			if tt.wantNextAt != nil {
				if got.NextAttemptAt == nil || !got.NextAttemptAt.Equal(*tt.wantNextAt) {
					t.Errorf("NextAttemptAt = %v, want %v", got.NextAttemptAt, tt.wantNextAt)
				}
			} else if got.NextAttemptAt != nil {
				t.Errorf("NextAttemptAt = %v, want nil", got.NextAttemptAt)
			}
		})
	}
}

func mustCfg(c playbook.DunningConfig, maxRetries int) playbook.DunningConfig {
	c.MaxRetries = maxRetries
	return c
}

func timePtr(t time.Time) *time.Time { return &t }
