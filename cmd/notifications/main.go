package main

import (
	"context"
	"database/sql"
	"encoding/json"
	"log"
	"os"

	"github.com/marwan562/fintech-ecosystem/internal/notification"
	"github.com/marwan562/fintech-ecosystem/pkg/database"
	"github.com/marwan562/fintech-ecosystem/pkg/messaging"
	"github.com/marwan562/fintech-ecosystem/pkg/monitoring"
	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promauto"
)

var (
	NotificationsSent = promauto.NewCounterVec(prometheus.CounterOpts{
		Name: "notifications_sent_total",
		Help: "Total number of notifications sent.",
	}, []string{"channel", "status"})

	NotificationsProcessed = promauto.NewCounterVec(prometheus.CounterOpts{
		Name: "notifications_processed_total",
		Help: "Total number of notifications processed from queue.",
	}, []string{"template", "status"})
)

func main() {
	rabbitURL := os.Getenv("RABBITMQ_URL")
	if rabbitURL == "" {
		rabbitURL = "amqp://user:password@localhost:5672/"
	}

	dbDSN := os.Getenv("DATABASE_URL")

	// Initialize RabbitMQ client
	client, err := messaging.NewRabbitMQClient(rabbitURL)
	if err != nil {
		log.Fatalf("Failed to connect to RabbitMQ: %v", err)
	}
	defer client.Close()

	queue, err := client.DeclareQueue("notifications")
	if err != nil {
		log.Fatalf("Failed to declare queue: %v", err)
	}

	// Initialize driver registry
	registry := notification.NewDriverRegistry()
	registry.Register(notification.NewEmailDriver())
	registry.Register(notification.NewSMSDriver())
	registry.Register(notification.NewWebDriver())

	// Initialize repository (optional - only if DB is configured)
	var repo *notification.Repository
	var db *sql.DB
	if dbDSN != "" {
		db, err = database.Connect(dbDSN)
		if err != nil {
			log.Printf("Warning: Failed to connect to database, running without persistence: %v", err)
		} else {
			repo = notification.NewRepository(db)
			log.Println("Database connected, notifications will be persisted")
		}
	} else {
		log.Println("DATABASE_URL not set, running without persistence")
	}

	// Initialize notification service
	svc := notification.NewService(repo, registry)

	// Start Metrics Server
	monitoring.StartMetricsServer(":8084")

	log.Println("Notifications Service started. Waiting for tasks...")

	client.Consume(queue.Name, func(body []byte) error {
		ctx := context.Background()

		// Try to parse as structured NotificationRequest first
		var req notification.NotificationRequest
		if err := json.Unmarshal(body, &req); err == nil && req.TemplateID != "" {
			// Structured request with template
			log.Printf("Processing notification request: template=%s, channel=%s, recipient=%s",
				req.TemplateID, req.Channel, req.Recipient)

			notif, err := svc.Send(ctx, &req)
			if err != nil {
				NotificationsProcessed.WithLabelValues(req.TemplateID, "error").Inc()
				NotificationsSent.WithLabelValues(string(req.Channel), "error").Inc()
				return err
			}

			NotificationsProcessed.WithLabelValues(req.TemplateID, "success").Inc()
			NotificationsSent.WithLabelValues(string(notif.Channel), "success").Inc()
			return nil
		}

		// Fallback: Legacy format (simple email/sms task)
		var legacyTask struct {
			Type    string `json:"type"`
			To      string `json:"to"`
			Subject string `json:"subject,omitempty"`
			Body    string `json:"body"`
		}
		if err := json.Unmarshal(body, &legacyTask); err != nil {
			log.Printf("Failed to parse notification task: %v", err)
			return err
		}

		log.Printf("Processing legacy notification: type=%s, to=%s", legacyTask.Type, legacyTask.To)

		channel := notification.Email
		if legacyTask.Type == "sms" {
			channel = notification.SMS
		}

		_, err := svc.SendSimple(ctx, "", legacyTask.To, channel, legacyTask.Subject, legacyTask.Body)
		if err != nil {
			NotificationsSent.WithLabelValues(legacyTask.Type, "error").Inc()
			return err
		}

		NotificationsSent.WithLabelValues(legacyTask.Type, "success").Inc()
		return nil
	})

	// Keep main running
	select {}
}
