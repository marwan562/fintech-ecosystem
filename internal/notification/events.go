package notification

import (
	"encoding/json"
	"time"
)

// EventType represents the type of business event
type EventType string

const (
	// Payment events
	EventPaymentCreated   EventType = "payment.created"
	EventPaymentSucceeded EventType = "payment.succeeded"
	EventPaymentFailed    EventType = "payment.failed"

	// Refund events
	EventRefundInitiated EventType = "refund.initiated"
	EventRefundCompleted EventType = "refund.completed"

	// Auth events
	EventUserRegistered EventType = "user.registered"
	EventPasswordReset  EventType = "password.reset"

	// Webhook events
	EventWebhookDelivery EventType = "webhook.delivery"
)

// Event is the envelope for all business events
type Event struct {
	ID        string          `json:"id"`
	Type      EventType       `json:"type"`
	Timestamp time.Time       `json:"timestamp"`
	Data      json.RawMessage `json:"data"`
}

// PaymentEventData contains payment-related event data
type PaymentEventData struct {
	PaymentID   string `json:"payment_id"`
	UserID      string `json:"user_id"`
	Amount      int64  `json:"amount"`
	Currency    string `json:"currency"`
	Description string `json:"description,omitempty"`
	Status      string `json:"status"`
	FailReason  string `json:"fail_reason,omitempty"`
}

// RefundEventData contains refund-related event data
type RefundEventData struct {
	RefundID  string `json:"refund_id"`
	PaymentID string `json:"payment_id"`
	UserID    string `json:"user_id"`
	Amount    int64  `json:"amount"`
	Currency  string `json:"currency"`
	Reason    string `json:"reason,omitempty"`
	Status    string `json:"status"`
}

// UserEventData contains user-related event data
type UserEventData struct {
	UserID    string `json:"user_id"`
	Email     string `json:"email"`
	Phone     string `json:"phone,omitempty"`
	FirstName string `json:"first_name,omitempty"`
	LastName  string `json:"last_name,omitempty"`
	Link      string `json:"link,omitempty"`
	Token     string `json:"token,omitempty"`
}

// WebhookDeliveryData contains webhook delivery task data
type WebhookDeliveryData struct {
	WebhookID  string            `json:"webhook_id"`
	PartnerID  string            `json:"partner_id"`
	URL        string            `json:"url"`
	EventType  EventType         `json:"event_type"`
	Payload    json.RawMessage   `json:"payload"`
	Headers    map[string]string `json:"headers,omitempty"`
	RetryCount int               `json:"retry_count"`
	MaxRetries int               `json:"max_retries"`
}

// NewEvent creates a new event with the given type and data
func NewEvent(eventType EventType, data interface{}) (*Event, error) {
	dataBytes, err := json.Marshal(data)
	if err != nil {
		return nil, err
	}

	return &Event{
		ID:        generateEventID(),
		Type:      eventType,
		Timestamp: time.Now().UTC(),
		Data:      dataBytes,
	}, nil
}

// ParsePaymentEventData parses the event data as PaymentEventData
func (e *Event) ParsePaymentEventData() (*PaymentEventData, error) {
	var data PaymentEventData
	if err := json.Unmarshal(e.Data, &data); err != nil {
		return nil, err
	}
	return &data, nil
}

// ParseRefundEventData parses the event data as RefundEventData
func (e *Event) ParseRefundEventData() (*RefundEventData, error) {
	var data RefundEventData
	if err := json.Unmarshal(e.Data, &data); err != nil {
		return nil, err
	}
	return &data, nil
}

// ParseUserEventData parses the event data as UserEventData
func (e *Event) ParseUserEventData() (*UserEventData, error) {
	var data UserEventData
	if err := json.Unmarshal(e.Data, &data); err != nil {
		return nil, err
	}
	return &data, nil
}

// generateEventID creates a unique event ID
func generateEventID() string {
	return "evt_" + time.Now().Format("20060102150405") + "_" + randomString(8)
}

// randomString generates a random alphanumeric string
func randomString(n int) string {
	const letters = "abcdefghijklmnopqrstuvwxyz0123456789"
	b := make([]byte, n)
	for i := range b {
		b[i] = letters[time.Now().UnixNano()%int64(len(letters))]
		time.Sleep(time.Nanosecond) // Ensure different values
	}
	return string(b)
}
