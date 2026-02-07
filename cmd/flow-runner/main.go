package main

import (
	"context"
	"encoding/json"
	"log"
	"os"
	"os/signal"
	"strings"
	"syscall"
	"time"

	"github.com/redis/go-redis/v9"
	"github.com/sapliy/fintech-ecosystem/internal/flow/domain"
	"github.com/sapliy/fintech-ecosystem/internal/flow/infrastructure"
	"github.com/sapliy/fintech-ecosystem/pkg/database"
	"github.com/sapliy/fintech-ecosystem/pkg/messaging"
	"github.com/sapliy/fintech-ecosystem/pkg/observability"
)

func main() {
	dsn := os.Getenv("DB_DSN")
	if dsn == "" {
		dsn = "postgres://user:password@127.0.0.1:5433/microservices?sslmode=disable"
	}

	db, err := database.Connect(dsn)
	if err != nil {
		log.Fatalf("Failed to connect to DB: %v", err)
	}
	defer db.Close()

	// Initialise internal plumbing
	repo := infrastructure.NewSQLRepository(db)
	runner := domain.NewFlowRunner(repo)

	// Setup Redis client for Streams
	redisAddr := os.Getenv("REDIS_ADDR")
	if redisAddr == "" {
		redisAddr = "localhost:6379"
	}
	rdb := redis.NewClient(&redis.Options{Addr: redisAddr})

	// Setup Kafka Consumer (legacy support)
	brokers := strings.Split(os.Getenv("KAFKA_BROKERS"), ",")
	if len(brokers) == 0 || brokers[0] == "" {
		brokers = []string{"localhost:9092"}
	}
	consumer := messaging.NewKafkaConsumer(brokers, "payments", "flow-runner-group")

	// Initialize Tracer
	shutdown, _ := observability.InitTracer(context.Background(), observability.Config{
		ServiceName: "flow-runner",
	})
	defer shutdown(context.Background())

	log.Println("Flow Runner service starting...")

	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	// Start Redis Streams consumer in a separate goroutine
	go consumeRedisStreams(ctx, rdb, repo, runner)

	// Kafka consumer (blocking)
	consumer.Consume(ctx, func(key string, value []byte) error {
		var event map[string]interface{}
		if err := json.Unmarshal(value, &event); err != nil {
			return nil // Skip invalid events
		}

		zoneID, _ := event["zone_id"].(string)
		if zoneID == "" {
			return nil
		}

		// Find flows for this zone
		flows, err := repo.ListFlows(ctx, zoneID)
		if err != nil {
			return err
		}

		for _, f := range flows {
			// Check if flow trigger matches event
			if matchesTrigger(f, "payments", event) {
				go func(flow *domain.Flow) {
					if err := runner.Execute(context.WithValue(ctx, "trace_id", string(key)), flow, event); err != nil {
						log.Printf("Flow %s failed: %v", flow.ID, err)
					}
				}(f)
			}
		}

		return nil
	})

	log.Println("Flow Runner exited")
}

// consumeRedisStreams listens to zone-scoped event streams and triggers flows
func consumeRedisStreams(ctx context.Context, rdb *redis.Client, repo domain.Repository, runner *domain.FlowRunner) {
	consumerGroup := "flow-runner-group"
	consumerName := "flow-runner-1"

	// Get all zones and subscribe to their event streams
	for {
		select {
		case <-ctx.Done():
			return
		default:
		}

		// Dynamically discover streams by pattern (zone.*.event.*)
		keys, err := rdb.Keys(ctx, "zone.*.event.*").Result()
		if err != nil {
			log.Printf("Redis Keys error: %v", err)
			time.Sleep(5 * time.Second)
			continue
		}

		for _, stream := range keys {
			// Create consumer group if not exists
			rdb.XGroupCreateMkStream(ctx, stream, consumerGroup, "0").Err()

			// Read new messages
			result, err := rdb.XReadGroup(ctx, &redis.XReadGroupArgs{
				Group:    consumerGroup,
				Consumer: consumerName,
				Streams:  []string{stream, ">"},
				Count:    10,
				Block:    time.Second,
			}).Result()

			if err != nil {
				if err != redis.Nil {
					log.Printf("XReadGroup error: %v", err)
				}
				continue
			}

			for _, xstream := range result {
				for _, msg := range xstream.Messages {
					processStreamMessage(ctx, xstream.Stream, msg, repo, runner)

					// Acknowledge message
					rdb.XAck(ctx, xstream.Stream, consumerGroup, msg.ID)
				}
			}
		}

		time.Sleep(100 * time.Millisecond)
	}
}

func processStreamMessage(ctx context.Context, stream string, msg redis.XMessage, repo domain.Repository, runner *domain.FlowRunner) {
	// Parse stream name: zone.{zone_id}.event.{event_type}
	parts := strings.Split(stream, ".")
	if len(parts) < 4 {
		return
	}

	zoneID := parts[1]
	eventType := parts[3]

	// Try to parse as full envelope first (new format)
	var envelope struct {
		ID             string                 `json:"id"`
		Type           string                 `json:"type"`
		ZoneID         string                 `json:"zone_id"`
		OrgID          string                 `json:"org_id"`
		Timestamp      string                 `json:"timestamp"`
		IdempotencyKey string                 `json:"idempotency_key"`
		Payload        map[string]interface{} `json:"payload"`
		Meta           map[string]string      `json:"meta"`
	}

	var payload map[string]interface{}
	var eventID string

	// Check for envelope (new format)
	if envelopeStr, ok := msg.Values["envelope"].(string); ok {
		if json.Unmarshal([]byte(envelopeStr), &envelope) == nil && envelope.ID != "" {
			eventID = envelope.ID
			eventType = envelope.Type
			payload = envelope.Payload
		}
	}

	// Fallback to data field (legacy format)
	if payload == nil {
		if data, ok := msg.Values["data"].(string); ok {
			json.Unmarshal([]byte(data), &payload)
		}
	}

	if payload == nil {
		payload = make(map[string]interface{})
	}

	if eventID == "" {
		eventID = msg.ID
	}

	// Create domain event
	domainEvent := &domain.Event{
		ID:             eventID,
		Type:           eventType,
		ZoneID:         zoneID,
		OrgID:          envelope.OrgID,
		Meta:           envelope.Meta,
		IdempotencyKey: envelope.IdempotencyKey,
		CreatedAt:      time.Now(), // approximate
	}

	if payloadJSON, err := json.Marshal(payload); err == nil {
		domainEvent.Data = payloadJSON
	}

	// Persist event to Postgres
	if err := repo.CreateEvent(ctx, domainEvent); err != nil {
		// Log error but continue (don't block execution for metrics/logging failures)
		log.Printf("Failed to persist event %s: %v", eventID, err)
	}

	event := map[string]interface{}{
		"event_id":        eventID,
		"zone_id":         zoneID,
		"type":            eventType,
		"payload":         payload,
		"idempotency_key": envelope.IdempotencyKey,
		"org_id":          envelope.OrgID,
		"timestamp":       envelope.Timestamp,
	}

	log.Printf("Processing event %s from stream %s: type=%s", eventID, stream, eventType)

	// Find matching flows for this zone
	flows, err := repo.ListFlows(ctx, zoneID)
	if err != nil {
		log.Printf("Failed to list flows for zone %s: %v", zoneID, err)
		return
	}

	for _, f := range flows {
		if f.Enabled && matchesTrigger(f, eventType, event) {
			go func(flow *domain.Flow) {
				log.Printf("Executing flow %s for event %s", flow.ID, eventType)
				if err := runner.Execute(ctx, flow, event); err != nil {
					log.Printf("Flow %s failed: %v", flow.ID, err)
				}
			}(f)
		}
	}
}

func matchesTrigger(flow *domain.Flow, topic string, event map[string]interface{}) bool {
	eventType, _ := event["type"].(string)

	for _, n := range flow.Nodes {
		if n.Type == domain.NodeTrigger {
			var data struct {
				EventType string `json:"eventType"`
			}
			json.Unmarshal(n.Data, &data)

			if data.EventType == eventType {
				return true
			}
			// Fallback for generic triggers
			if data.EventType == "" || data.EventType == "*" {
				return true
			}
		}
	}
	return false
}
