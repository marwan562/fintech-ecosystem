package main

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"os"
	"strings"
	"sync"
	"time"

	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promauto"
	"github.com/sapliy/sapliy-core/internal/fraud"
	"github.com/sapliy/sapliy-core/pkg/messaging"
	"github.com/sapliy/sapliy-core/pkg/monitoring"
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
		defer func() {
			rabbitClient.Close()
		}()
		if _, err := rabbitClient.DeclareQueue("risk_alerts"); err != nil {
			log.Printf("Failed to declare risk_alerts queue: %v", err)
		}
	}

	engine := fraud.NewEngine(
		&fraud.AmountRule{Limit: 1000000}, // $10,000 in cents
		fraud.NewVelocityRule(1*time.Minute, 5),
	)

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

		tx := fraud.Transaction{
			ID:       event.Data.ID,
			Amount:   event.Data.Amount,
			Currency: event.Data.Currency,
			UserID:   event.Data.UserID,
		}

		results, isRisky := engine.Check(context.Background(), tx)
		if isRisky {
			for _, res := range results {
				if !res.Passed {
					log.Printf("⚠️ FRAUD ALERT: %s - %s (UserID: %s)", res.RuleName, res.Message, tx.UserID)
					RiskyPayments.WithLabelValues(res.RuleName).Inc()

					if rabbitClient != nil {
						alert := map[string]string{
							"user_id": tx.UserID,
							"reason":  fmt.Sprintf("%s: %s", res.RuleName, res.Message),
							"time":    time.Now().Format(time.RFC3339),
							"tx_id":   tx.ID,
						}
						body, _ := json.Marshal(alert)
						if err := rabbitClient.Publish(context.Background(), "risk_alerts", body); err != nil {
							log.Printf("Failed to publish risk alert: %v", err)
						}
					}
				}
			}
		}

		return nil
	})
}
