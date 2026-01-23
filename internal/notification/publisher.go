package notification

import (
	"context"
	"encoding/json"
	"log"

	"github.com/marwan562/fintech-ecosystem/pkg/messaging"
)

// EventPublisher publishes business events to Kafka
type EventPublisher struct {
	producer *messaging.KafkaProducer
}

// NewEventPublisher creates a new event publisher
func NewEventPublisher(brokers []string, topic string) *EventPublisher {
	return &EventPublisher{
		producer: messaging.NewKafkaProducer(brokers, topic),
	}
}

// Publish publishes an event to Kafka
func (p *EventPublisher) Publish(ctx context.Context, event *Event) error {
	data, err := json.Marshal(event)
	if err != nil {
		return err
	}

	// Use event ID as key for ordering
	key := event.ID

	// For payment/refund events, use the payment/refund ID as key for ordering
	switch event.Type {
	case EventPaymentCreated, EventPaymentSucceeded, EventPaymentFailed:
		if paymentData, err := event.ParsePaymentEventData(); err == nil {
			key = paymentData.PaymentID
		}
	case EventRefundInitiated, EventRefundCompleted:
		if refundData, err := event.ParseRefundEventData(); err == nil {
			key = refundData.PaymentID // Order by payment ID for related events
		}
	}

	if err := p.producer.Publish(ctx, key, data); err != nil {
		log.Printf("Failed to publish event %s: %v", event.Type, err)
		return err
	}

	log.Printf("Published event: type=%s, id=%s", event.Type, event.ID)
	return nil
}

// PublishPaymentSucceeded is a convenience method for payment success events
func (p *EventPublisher) PublishPaymentSucceeded(ctx context.Context, paymentID, userID string, amount int64, currency, description string) error {
	event, err := NewEvent(EventPaymentSucceeded, PaymentEventData{
		PaymentID:   paymentID,
		UserID:      userID,
		Amount:      amount,
		Currency:    currency,
		Description: description,
		Status:      "succeeded",
	})
	if err != nil {
		return err
	}
	return p.Publish(ctx, event)
}

// PublishPaymentFailed is a convenience method for payment failure events
func (p *EventPublisher) PublishPaymentFailed(ctx context.Context, paymentID, userID string, amount int64, currency, failReason string) error {
	event, err := NewEvent(EventPaymentFailed, PaymentEventData{
		PaymentID:  paymentID,
		UserID:     userID,
		Amount:     amount,
		Currency:   currency,
		Status:     "failed",
		FailReason: failReason,
	})
	if err != nil {
		return err
	}
	return p.Publish(ctx, event)
}

// PublishRefundCompleted is a convenience method for refund completion events
func (p *EventPublisher) PublishRefundCompleted(ctx context.Context, refundID, paymentID, userID string, amount int64, currency string) error {
	event, err := NewEvent(EventRefundCompleted, RefundEventData{
		RefundID:  refundID,
		PaymentID: paymentID,
		UserID:    userID,
		Amount:    amount,
		Currency:  currency,
		Status:    "completed",
	})
	if err != nil {
		return err
	}
	return p.Publish(ctx, event)
}

// Close closes the event publisher
func (p *EventPublisher) Close() error {
	return p.producer.Close()
}
