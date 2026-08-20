package playbook

import (
	"encoding/json"
	"time"
)

// PlaybookType identifies the operational playbook a Playbook belongs to.
type PlaybookType string

const (
	PlaybookRevenueRecovery  PlaybookType = "revenue_recovery"
	PlaybookRefundApproval   PlaybookType = "refund_approval"
	PlaybookInvoiceReminders PlaybookType = "invoice_reminders"
)

// PlaybookStatus is the lifecycle state of a Playbook.
type PlaybookStatus string

const (
	PlaybookStatusDraft    PlaybookStatus = "draft"
	PlaybookStatusActive   PlaybookStatus = "active"
	PlaybookStatusPaused   PlaybookStatus = "paused"
	PlaybookStatusArchived PlaybookStatus = "archived"
)

// Playbook is the persisted configuration of an operational playbook.
// Config holds the playbook-specific configuration (DunningConfig,
// RefundApprovalConfig or InvoiceReminderConfig) as opaque JSON.
type Playbook struct {
	ID          string          `json:"id"`
	Type        PlaybookType    `json:"type"`
	Name        string          `json:"name"`
	Description string          `json:"description,omitempty"`
	ZoneID      string          `json:"zoneId,omitempty"`
	OrgID       string          `json:"orgId,omitempty"`
	Status      PlaybookStatus  `json:"status"`
	Config      json.RawMessage `json:"config,omitempty"`
	CreatedAt   time.Time       `json:"createdAt"`
	UpdatedAt   time.Time       `json:"updatedAt"`
}

// DunningConfig controls the revenue-recovery (dunning) retry schedule.
type DunningConfig struct {
	MaxRetries      int           `json:"maxRetries"`
	FirstRetryDelay time.Duration `json:"firstRetryDelay"`
	RetryStepDelay  time.Duration `json:"retryStepDelay"`
	FinalRetryDelay time.Duration `json:"finalRetryDelay"`
	Channels        []string      `json:"channels"`
	MagicLink       bool          `json:"magicLink"`
}

// DefaultDunningConfig returns the sane default dunning schedule:
// a first retry at ~5h, then stepping by 48h (days 3/5/7) up to 4 attempts.
func DefaultDunningConfig() DunningConfig {
	return DunningConfig{
		MaxRetries:      4,
		FirstRetryDelay: 5 * time.Hour,
		RetryStepDelay:  48 * time.Hour,
		FinalRetryDelay: 96 * time.Hour,
		Channels:        []string{"email"},
		MagicLink:       true,
	}
}

// WithDefaults fills any zero-valued fields with the sane defaults.
func (c DunningConfig) WithDefaults() DunningConfig {
	d := DefaultDunningConfig()
	if c.MaxRetries <= 0 {
		c.MaxRetries = d.MaxRetries
	}
	if c.FirstRetryDelay <= 0 {
		c.FirstRetryDelay = d.FirstRetryDelay
	}
	if c.RetryStepDelay <= 0 {
		c.RetryStepDelay = d.RetryStepDelay
	}
	if c.FinalRetryDelay <= 0 {
		c.FinalRetryDelay = d.FinalRetryDelay
	}
	if len(c.Channels) == 0 {
		c.Channels = append([]string(nil), d.Channels...)
	}
	if !c.MagicLink {
		c.MagicLink = d.MagicLink
	}
	return c
}

// RefundApprovalConfig controls the refund approval policy gates.
type RefundApprovalConfig struct {
	AutoApproveUnderCents    int64    `json:"autoApproveUnderCents"`
	RequireApprovalOverCents int64    `json:"requireApprovalOverCents"`
	MaxRefundDays            int      `json:"maxRefundDays"`
	NotifyChannels           []string `json:"notifyChannels"`
}

// DefaultRefundApprovalConfig returns the sane defaults: auto-approve below
// $1,000.00, require manager approval above $1,000.00, 90-day refund window.
func DefaultRefundApprovalConfig() RefundApprovalConfig {
	return RefundApprovalConfig{
		AutoApproveUnderCents:    100000,
		RequireApprovalOverCents: 100000,
		MaxRefundDays:            90,
		NotifyChannels:           []string{"email", "slack"},
	}
}

// WithDefaults fills any zero-valued fields with the sane defaults.
func (c RefundApprovalConfig) WithDefaults() RefundApprovalConfig {
	d := DefaultRefundApprovalConfig()
	if c.AutoApproveUnderCents <= 0 {
		c.AutoApproveUnderCents = d.AutoApproveUnderCents
	}
	if c.RequireApprovalOverCents <= 0 {
		c.RequireApprovalOverCents = d.RequireApprovalOverCents
	}
	if c.MaxRefundDays <= 0 {
		c.MaxRefundDays = d.MaxRefundDays
	}
	if len(c.NotifyChannels) == 0 {
		c.NotifyChannels = append([]string(nil), d.NotifyChannels...)
	}
	return c
}

// InvoiceReminderConfig controls the invoice reminder cadence.
type InvoiceReminderConfig struct {
	DueDateDaysBefore   int      `json:"dueDateDaysBefore"`
	ReminderCadenceDays int      `json:"reminderCadenceDays"`
	MaxReminders        int      `json:"maxReminders"`
	Channels            []string `json:"channels"`
}

// DefaultInvoiceReminderConfig returns the sane defaults: start 3 days before
// due, remind every 7 days, up to 4 reminders via email.
func DefaultInvoiceReminderConfig() InvoiceReminderConfig {
	return InvoiceReminderConfig{
		DueDateDaysBefore:   3,
		ReminderCadenceDays: 7,
		MaxReminders:        4,
		Channels:            []string{"email"},
	}
}

// WithDefaults fills any zero-valued fields with the sane defaults.
func (c InvoiceReminderConfig) WithDefaults() InvoiceReminderConfig {
	d := DefaultInvoiceReminderConfig()
	if c.DueDateDaysBefore <= 0 {
		c.DueDateDaysBefore = d.DueDateDaysBefore
	}
	if c.ReminderCadenceDays <= 0 {
		c.ReminderCadenceDays = d.ReminderCadenceDays
	}
	if c.MaxReminders <= 0 {
		c.MaxReminders = d.MaxReminders
	}
	if len(c.Channels) == 0 {
		c.Channels = append([]string(nil), d.Channels...)
	}
	return c
}
