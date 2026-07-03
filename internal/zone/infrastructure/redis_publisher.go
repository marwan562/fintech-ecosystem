package infrastructure

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/redis/go-redis/v9"
	"github.com/sapliy/sapliy-core/internal/zone/domain"
)

type RedisEventPublisher struct {
	client *redis.Client
}

func NewRedisEventPublisher(client *redis.Client) *RedisEventPublisher {
	return &RedisEventPublisher{client: client}
}

func (p *RedisEventPublisher) PublishZoneCreated(ctx context.Context, event domain.ZoneCreatedEvent) error {
	payload, err := json.Marshal(event)
	if err != nil {
		return fmt.Errorf("failed to marshal event: %w", err)
	}

	// Publish to Org-scoped channel
	channel := fmt.Sprintf("events:org:%s", event.OrgID)

	// Also publish to a dedicated zone creation channel if needed,
	// but the requirement is to integrate with Gateway/Frontend which likely listens to the Org channel.

	// We wrap it in a standard envelope structure if the system expects one,
	// but for now we'll publish the raw event or a simple wrapper.
	// Based on Gateway's `handleWebSocket`, it subscribes to `webhook_events`.
	// We are changing this to `events:org:{orgID}`.

	// Let's stick to a simple JSON payload first.
	return p.client.Publish(ctx, channel, payload).Err()
}
