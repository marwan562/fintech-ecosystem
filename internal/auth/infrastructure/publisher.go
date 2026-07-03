package infrastructure

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/google/uuid"
	"github.com/sapliy/sapliy-core/pkg/messaging"
)

type KafkaPublisher struct {
	producer *messaging.KafkaProducer
	topic    string
}

func NewKafkaPublisher(brokers []string, topic string) *KafkaPublisher {
	return &KafkaPublisher{
		producer: messaging.NewKafkaProducer(brokers, topic),
		topic:    topic,
	}
}

func (p *KafkaPublisher) Publish(ctx context.Context, topic string, event interface{}) error {
	payload, err := json.Marshal(event)
	if err != nil {
		return fmt.Errorf("failed to marshal event: %w", err)
	}

	// Generate a unique key for partitioning (optional, here using UUID)
	key := uuid.New().String()

	// Use the producer to publish
	// Note: The pkg/messaging/kafka.go Producer is simple and topic is fixed in constructor usually.
	// But our interface implementation might need to handle different topics or a single topic with event types.
	// The pkg/messaging KafkaProducer seems to be tied to a topic. So we ignore the 'topic' arg if pkg/messaging doesn't support dynamic topics per message cleanly without recreating producer.
	// However, usually all auth events go to one topic e.g. 'auth_events' (or 'payment_events' based on existing env vars in notification service).

	return p.producer.Publish(ctx, key, payload)
}

func (p *KafkaPublisher) Close() error {
	return p.producer.Close()
}
