package infrastructure

import (
	"context"

	"github.com/sapliy/sapliy-core/internal/flow/domain"
	"github.com/sapliy/sapliy-core/pkg/messaging"
)

type KafkaEventRetriggerer struct {
	producer *messaging.KafkaProducer
}

func NewKafkaEventRetriggerer(producer *messaging.KafkaProducer) *KafkaEventRetriggerer {
	return &KafkaEventRetriggerer{
		producer: producer,
	}
}

func (r *KafkaEventRetriggerer) RetriggerEvent(ctx context.Context, event *domain.Event) error {
	// Re-publish the event to Kafka to trigger standard flow processing
	return r.producer.Publish(ctx, event.ID, event.Data)
}
