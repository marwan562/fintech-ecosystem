package main

import (
	"context"
	"encoding/json"
	"log"
	"os"
	"os/signal"
	"strings"
	"syscall"

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

	// Setup Kafka Consumer
	brokers := strings.Split(os.Getenv("KAFKA_BROKERS"), ",")
	if len(brokers) == 0 || brokers[0] == "" {
		brokers = []string{"localhost:9092"}
	}

	// One consumer per topic for now, or just focus on "payments"
	consumer := messaging.NewKafkaConsumer(brokers, "payments", "flow-runner-group")

	// Initialize Tracer
	shutdown, _ := observability.InitTracer(context.Background(), observability.Config{
		ServiceName: "flow-runner",
	})
	defer shutdown(context.Background())

	log.Println("Flow Runner service starting...")

	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

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

	// consumer.Consume is blocking
	log.Println("Flow Runner exited")
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
