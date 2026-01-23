package main

import (
	"context"
	"encoding/json"
	"log"
	"os"
	"strings"
	"sync"
	"time"

	"github.com/marwan562/fintech-ecosystem/pkg/messaging"
	"github.com/marwan562/fintech-ecosystem/pkg/monitoring"
	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promauto"
)

var (
	RiskyPayments = promauto.NewCounterVec(prometheus.CounterOpts{
		Name: "fraud_risky_payments_total",
		Help: "Total number of payments flagged as risky.",
	}, []string{"reason"})
)

type PaymentEvent struct {
	Type string `json:"type"`
	Data struct {
		ID       string `json:"id"`
		Amount   int64  `json:"amount"`
		Currency string `json:"currency"`
		UserID   string `json:"user_id"`
	} `json:"data"`
}

type VelocityTracker struct {
	mu       sync.Mutex
	payments map[string][]time.Time
}

func NewVelocityTracker() *VelocityTracker {
	return &VelocityTracker{
		payments: make(map[string][]time.Time),
	}
}

func (v *VelocityTracker) AddAndCheck(userID string) bool {
	v.mu.Lock()
	defer v.mu.Unlock()

	now := time.Now()
	window := 1 * time.Minute
	threshold := 5

	// Add current
	v.payments[userID] = append(v.payments[userID], now)

	// Clean up old and count
	var fresh []time.Time
	for _, t := range v.payments[userID] {
		if now.Sub(t) < window {
			fresh = append(fresh, t)
		}
	}
	v.payments[userID] = fresh

	return len(fresh) > threshold
}

func main() {
	kafkaBrokers := os.Getenv("KAFKA_BROKERS")
	if kafkaBrokers == "" {
		kafkaBrokers = "localhost:9092"
	}
	brokers := strings.Split(kafkaBrokers, ",")

	consumer := messaging.NewKafkaConsumer(brokers, "payments", "fraud-group")

	// RabbitMQ for risk alerts (async tasks for human review)
	rabbitURL := os.Getenv("RABBITMQ_URL")
	if rabbitURL == "" {
		rabbitURL = "amqp://user:password@localhost:5672/"
	}
	rabbitClient, _ := messaging.NewRabbitMQClient(messaging.Config{
		URL:                   rabbitURL,
		ReconnectDelay:        time.Second,
		MaxReconnectDelay:     time.Minute,
		MaxRetries:            -1,
		CircuitBreakerEnabled: true,
	})
	if rabbitClient != nil {
		defer rabbitClient.Close()
		rabbitClient.DeclareQueue("risk_alerts")
	}

	tracker := NewVelocityTracker()

	// Start Metrics Server
	monitoring.StartMetricsServer(":8081") // Fraud service metrics

	log.Println("Fraud Detection Service started. Monitoring 'payments' topic...")

	consumer.Consume(context.Background(), func(key string, value []byte) error {
		var event PaymentEvent
		if err := json.Unmarshal(value, &event); err != nil {
			return err
		}

		if event.Type != "payment.succeeded" {
			return nil
		}

		if tracker.AddAndCheck(event.Data.UserID) {
			log.Printf("⚠️ FRAUD ALERT: High velocity detected for UserID %s", event.Data.UserID)
			RiskyPayments.WithLabelValues("velocity_high").Inc()

			if rabbitClient != nil {
				alert := map[string]string{
					"user_id": event.Data.UserID,
					"reason":  "Velocity high ( >5 per minute)",
					"time":    time.Now().Format(time.RFC3339),
				}
				body, _ := json.Marshal(alert)
				rabbitClient.Publish(context.Background(), "risk_alerts", body)
			}
		}

		return nil
	})
}
