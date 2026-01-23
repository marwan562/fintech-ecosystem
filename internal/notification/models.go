package notification

import (
	"time"
)

type Channel string

const (
	Email Channel = "email"
	SMS   Channel = "sms"
	Web   Channel = "web"
)

type Status string

const (
	StatusPending Status = "pending"
	StatusSent    Status = "sent"
	StatusFailed  Status = "failed"
)

type Notification struct {
	ID        string     `json:"id"`
	UserID    string     `json:"user_id"`
	Recipient string     `json:"recipient"`
	Channel   Channel    `json:"channel"`
	Title     string     `json:"title,omitempty"`
	Content   string     `json:"content"`
	Status    Status     `json:"status"`
	CreatedAt time.Time  `json:"created_at"`
	SentAt    *time.Time `json:"sent_at,omitempty"`
}

type NotificationRequest struct {
	UserID     string            `json:"user_id"`
	Recipient  string            `json:"recipient"`
	Channel    Channel           `json:"channel"`
	TemplateID string            `json:"template_id"`
	Data       map[string]string `json:"data"`
}
