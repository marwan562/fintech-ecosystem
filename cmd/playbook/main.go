package main

import (
	"encoding/json"
	"fmt"
	"log"
	"os"
	"strconv"
	"strings"
	"time"

	"github.com/sapliy/sapliy-core/internal/playbook"
)

// dunningConfigFromEnv loads a DunningConfig from environment variables,
// falling back to the sane defaults for anything unset.
//   - SAPLIY_MAX_RETRIES          int     (default 4)
//   - SAPLIY_FIRST_RETRY_DELAY_H  int     hours (default 5)
//   - SAPLIY_RETRY_STEP_DELAY_H   int     hours (default 48)
//   - SAPLIY_FINAL_RETRY_DELAY_H  int     hours (default 96)
//   - SAPLIY_CHANNELS             comma-separated (default email)
//   - SAPLIY_MAGIC_LINK           bool    (default true)
func dunningConfigFromEnv() playbook.DunningConfig {
	cfg := playbook.DefaultDunningConfig()

	if v, err := strconv.Atoi(os.Getenv("SAPLIY_MAX_RETRIES")); err == nil {
		cfg.MaxRetries = v
	}
	if v, err := strconv.Atoi(os.Getenv("SAPLIY_FIRST_RETRY_DELAY_H")); err == nil {
		cfg.FirstRetryDelay = time.Duration(v) * time.Hour
	}
	if v, err := strconv.Atoi(os.Getenv("SAPLIY_RETRY_STEP_DELAY_H")); err == nil {
		cfg.RetryStepDelay = time.Duration(v) * time.Hour
	}
	if v, err := strconv.Atoi(os.Getenv("SAPLIY_FINAL_RETRY_DELAY_H")); err == nil {
		cfg.FinalRetryDelay = time.Duration(v) * time.Hour
	}
	if v := os.Getenv("SAPLIY_CHANNELS"); v != "" {
		parts := strings.Split(v, ",")
		channels := make([]string, 0, len(parts))
		for _, c := range parts {
			if c = strings.TrimSpace(c); c != "" {
				channels = append(channels, c)
			}
		}
		if len(channels) > 0 {
			cfg.Channels = channels
		}
	}
	if v, err := strconv.ParseBool(os.Getenv("SAPLIY_MAGIC_LINK")); err == nil {
		cfg.MagicLink = v
	}

	return cfg
}

func main() {
	dunning := dunningConfigFromEnv()
	engine := playbook.NewEngineWithConfigs(dunning, playbook.DefaultRefundApprovalConfig())

	now := time.Now().UTC()

	// Sample PaymentFailed event (revenue recovery playbook).
	failed := playbook.FinancialEvent{
		ID:          "evt_payment_failed_1",
		Type:        playbook.PaymentFailed,
		AmountCents: 4900,
		Currency:    "usd",
		CustomerID:  "cus_123",
		InvoiceID:   "in_123",
		PaymentID:   "pi_123",
		OccurredAt:  now.Add(-1 * time.Hour),
		Metadata:    map[string]string{"last_outcome": "declined", "retry_attempt": "0"},
	}
	if _, err := engine.HandleEvent(failed, "tenant_demo"); err != nil {
		log.Fatalf("failed to process PaymentFailed event: %v", err)
	}

	// Sample RefundRequested event (refund approval playbook).
	refund := playbook.FinancialEvent{
		ID:          "evt_refund_requested_1",
		Type:        playbook.RefundRequested,
		AmountCents: 150000, // $1,500.00 -> requires finance_manager approval
		Currency:    "usd",
		CustomerID:  "cus_123",
		PaymentID:   "pi_456",
		OccurredAt:  now,
		Metadata:    map[string]string{"days_since_charge": "5"},
	}
	if _, err := engine.HandleEvent(refund, "tenant_demo"); err != nil {
		log.Fatalf("failed to process RefundRequested event: %v", err)
	}

	// Sample InvoiceOverdue event (invoice reminder playbook).
	overdue := playbook.FinancialEvent{
		ID:          "evt_invoice_overdue_1",
		Type:        playbook.InvoiceOverdue,
		AmountCents: 12000,
		Currency:    "usd",
		CustomerID:  "cus_456",
		InvoiceID:   "in_789",
		OccurredAt:  now,
	}
	if _, err := engine.HandleEvent(overdue, "tenant_demo"); err != nil {
		log.Fatalf("failed to process InvoiceOverdue event: %v", err)
	}

	// Verify the immutable hash chain before printing.
	if err := engine.Log().Verify(); err != nil {
		log.Fatalf("decision log verification failed: %v", err)
	}

	enc := json.NewEncoder(os.Stdout)
	enc.SetIndent("", "  ")
	if err := enc.Encode(engine.Log().Entries()); err != nil {
		log.Fatalf("failed to encode decision log: %v", err)
	}

	fmt.Fprintf(os.Stderr, "decision log entries: %d (chain verified)\n", len(engine.Log().Entries()))
}
