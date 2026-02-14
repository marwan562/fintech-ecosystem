package notification

import (
	"context"
	"encoding/json"
	"log"
)

// NotificationTask represents a task to be processed by workers
type NotificationTask struct {
	ID         string            `json:"id"`
	Channel    Channel           `json:"channel"`
	Recipient  string            `json:"recipient"`
	TemplateID string            `json:"template_id"`
	Data       map[string]string `json:"data"`
	EventID    string            `json:"event_id"`
	EventType  EventType         `json:"event_type"`
	RetryCount int               `json:"retry_count"`
	MaxRetries int               `json:"max_retries"`
}

// WebhookTask represents a webhook delivery task
type WebhookTask struct {
	ID         string          `json:"id"`
	PartnerID  string          `json:"partner_id"`
	URL        string          `json:"url"`
	Payload    json.RawMessage `json:"payload"`
	Secret     string          `json:"secret"`
	EventType  EventType       `json:"event_type"`
	RetryCount int             `json:"retry_count"`
	MaxRetries int             `json:"max_retries"`
}

// RoutingConfig defines which channels to notify for each event type
type RoutingConfig struct {
	EventType EventType
	Email     bool
	SMS       bool
	Web       bool
	Webhook   bool
}

// DefaultRoutingRules defines the default routing for each event type
var DefaultRoutingRules = map[EventType]RoutingConfig{
	EventPaymentSucceeded: {
		EventType: EventPaymentSucceeded,
		Email:     true,
		SMS:       false,
		Web:       true,
		Webhook:   true,
	},
	EventPaymentFailed: {
		EventType: EventPaymentFailed,
		Email:     true,
		SMS:       true, // SMS for failures is important
		Web:       true,
		Webhook:   true,
	},
	EventRefundInitiated: {
		EventType: EventRefundInitiated,
		Email:     true,
		SMS:       false,
		Web:       true,
		Webhook:   true,
	},
	EventRefundCompleted: {
		EventType: EventRefundCompleted,
		Email:     true,
		SMS:       true,
		Web:       true,
		Webhook:   true,
	},
	EventUserRegistered: {
		EventType: EventUserRegistered,
		Email:     true,
		SMS:       false,
		Web:       false,
		Webhook:   false,
	},
	EventPasswordReset: {
		EventType: EventPasswordReset,
		Email:     true,
		SMS:       true,
		Web:       false,
		Webhook:   false,
	},
}

// Router routes events to appropriate notification channels
type Router struct {
	rabbitClient RabbitPublisher
	rules        map[EventType]RoutingConfig
}

// RabbitPublisher interface for RabbitMQ publishing
type RabbitPublisher interface {
	Publish(ctx context.Context, queue string, body []byte) error
}

// NewRouter creates a new event router
func NewRouter(rabbitClient RabbitPublisher) *Router {
	return &Router{
		rabbitClient: rabbitClient,
		rules:        DefaultRoutingRules,
	}
}

// Route processes an event and routes it to appropriate queues
func (r *Router) Route(ctx context.Context, event *Event) error {
	config, ok := r.rules[event.Type]
	if !ok {
		log.Printf("No routing rules for event type: %s", event.Type)
		return nil
	}

	templateData := r.extractTemplateData(event)

	// Route to email queue
	if config.Email {
		task := r.createNotificationTask(event, Email, templateData)
		if err := r.publishTask(ctx, "email.notifications", task); err != nil {
			log.Printf("Failed to route to email queue: %v", err)
		}
	}

	// Route to SMS queue
	if config.SMS {
		task := r.createNotificationTask(event, SMS, templateData)
		if err := r.publishTask(ctx, "sms.notifications", task); err != nil {
			log.Printf("Failed to route to SMS queue: %v", err)
		}
	}

	// Route to Web Push queue
	if config.Web {
		task := r.createNotificationTask(event, Web, templateData)
		if err := r.publishTask(ctx, "web.notifications", task); err != nil {
			log.Printf("Failed to route to web queue: %v", err)
		}
	}

	// Route to Webhook queue
	if config.Webhook {
		webhookTask := r.createWebhookTask(event)
		if err := r.publishTask(ctx, "webhook.notifications", webhookTask); err != nil {
			log.Printf("Failed to route to webhook queue: %v", err)
		}
	}

	return nil
}

func (r *Router) createNotificationTask(event *Event, channel Channel, data map[string]string) *NotificationTask {
	return &NotificationTask{
		ID:         "task_" + event.ID,
		Channel:    channel,
		Recipient:  data["Recipient"],
		TemplateID: r.getTemplateID(event.Type),
		Data:       data,
		EventID:    event.ID,
		EventType:  event.Type,
		RetryCount: 0,
		MaxRetries: 3,
	}
}

func (r *Router) createWebhookTask(event *Event) *WebhookTask {
	return &WebhookTask{
		ID:         "webhook_" + event.ID,
		PartnerID:  "default", // Would be extracted from user preferences
		URL:        "",        // Would be fetched from partner config
		Payload:    event.Data,
		EventType:  event.Type,
		RetryCount: 0,
		MaxRetries: 5,
	}
}

func (r *Router) publishTask(ctx context.Context, queue string, task interface{}) error {
	data, err := json.Marshal(task)
	if err != nil {
		return err
	}
	return r.rabbitClient.Publish(ctx, queue, data)
}

func (r *Router) extractTemplateData(event *Event) map[string]string {
	data := make(map[string]string)

	switch event.Type {
	case EventPaymentSucceeded, EventPaymentFailed:
		if paymentData, err := event.ParsePaymentEventData(); err == nil {
			data["UserID"] = paymentData.UserID
			data["Recipient"] = "user_" + paymentData.UserID + "@example.com"
			data["Amount"] = formatAmount(paymentData.Amount)
			data["Currency"] = paymentData.Currency
			data["TransactionID"] = paymentData.PaymentID
			data["UserName"] = "User " + paymentData.UserID
			if paymentData.FailReason != "" {
				data["FailReason"] = paymentData.FailReason
			}
		}
	case EventRefundInitiated, EventRefundCompleted:
		if refundData, err := event.ParseRefundEventData(); err == nil {
			data["UserID"] = refundData.UserID
			data["Recipient"] = "user_" + refundData.UserID + "@example.com"
			data["Amount"] = formatAmount(refundData.Amount)
			data["Currency"] = refundData.Currency
			data["RefundID"] = refundData.RefundID
			data["UserName"] = "User " + refundData.UserID
		}
	case EventUserRegistered:
		if userData, err := event.ParseUserEventData(); err == nil {
			data["UserID"] = userData.UserID
			data["Recipient"] = userData.Email
			data["UserName"] = userData.FirstName + " " + userData.LastName
			data["link"] = userData.Link
			data["token"] = userData.Token
		}
	case EventPasswordReset:
		if userData, err := event.ParseUserEventData(); err == nil {
			data["UserID"] = userData.UserID
			data["Recipient"] = userData.Email
			data["link"] = userData.Link
			data["token"] = userData.Token
			// For templates expecting 'Code', map token to it if it's short/OTP
			data["Code"] = userData.Token
		}
	}

	return data
}

func (r *Router) getTemplateID(eventType EventType) string {
	switch eventType {
	case EventPaymentSucceeded:
		return "payment_success"
	case EventPaymentFailed:
		return "payment_failed"
	case EventRefundCompleted:
		return "payment_refund"
	case EventUserRegistered:
		return TemplateVerification
	case EventPasswordReset:
		return "otp"
	default:
		return "generic"
	}
}

func formatAmount(amount int64) string {
	return string(rune(amount/100)) + "." + string(rune(amount%100))
}
