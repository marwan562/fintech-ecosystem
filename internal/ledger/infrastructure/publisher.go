package infrastructure

import (
	"context"
	"log"
	"time"

	"github.com/marwan562/fintech-ecosystem/internal/ledger/domain"
	"github.com/marwan562/fintech-ecosystem/pkg/messaging"
)

type OutboxPublisher struct {
	repo          domain.Repository
	kafkaProducer *messaging.KafkaProducer
	pollInterval  time.Duration
}

func NewOutboxPublisher(repo domain.Repository, kafkaProducer *messaging.KafkaProducer, interval time.Duration) *OutboxPublisher {
	return &OutboxPublisher{
		repo:          repo,
		kafkaProducer: kafkaProducer,
		pollInterval:  interval,
	}
}

func (p *OutboxPublisher) Start(ctx context.Context) {
	ticker := time.NewTicker(p.pollInterval)
	defer ticker.Stop()

	log.Printf("Outbox Publisher started (pooling every %v)", p.pollInterval)

	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			p.processOutbox(ctx)
		}
	}
}

func (p *OutboxPublisher) processOutbox(ctx context.Context) {
	events, err := p.repo.GetUnprocessedEvents(ctx, 50)
	if err != nil {
		log.Printf("Failed to fetch outbox events: %v", err)
		return
	}

	// Update lag metric
	OutboxLag.Set(float64(len(events)))

	for _, e := range events {
		log.Printf("Publishing outbox event %s (%s)", e.ID, e.Type)

		// Publish to Kafka
		if err := p.kafkaProducer.Publish(ctx, e.ID, e.Payload); err != nil {
			log.Printf("Failed to publish event %s to Kafka: %v", e.ID, err)
			continue // Will retry on next poll
		}

		// Mark as processed in DB
		if err := p.repo.MarkEventProcessed(ctx, e.ID); err != nil {
			log.Printf("Failed to mark event %s as processed: %v", e.ID, err)
		}
	}
}
