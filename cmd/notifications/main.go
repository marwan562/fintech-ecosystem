package main

import (
	"context"
	"encoding/json"
	"log"
	"os"
	"strings"

	"github.com/marwan562/fintech-ecosystem/internal/notification"
	"github.com/marwan562/fintech-ecosystem/pkg/database"
	"github.com/marwan562/fintech-ecosystem/pkg/messaging"
	"github.com/marwan562/fintech-ecosystem/pkg/monitoring"
	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promauto"
	"github.com/redis/go-redis/v9"
)

var (
	EventsProcessed = promauto.NewCounterVec(prometheus.CounterOpts{
		Name: "notification_events_processed_total",
		Help: "Total number of events processed from Kafka.",
	}, []string{"event_type", "status"})

	TasksRouted = promauto.NewCounterVec(prometheus.CounterOpts{
		Name: "notification_tasks_routed_total",
		Help: "Total number of tasks routed to RabbitMQ queues.",
	}, []string{"channel"})

	NotificationsSent = promauto.NewCounterVec(prometheus.CounterOpts{
		Name: "notifications_sent_total",
		Help: "Total number of notifications sent by workers.",
	}, []string{"channel", "status"})
)

func main() {
	// Environment variables
	kafkaBrokers := getEnv("KAFKA_BROKERS", "localhost:9092")
	kafkaTopic := getEnv("KAFKA_TOPIC", "payment_events")
	kafkaGroupID := getEnv("KAFKA_GROUP_ID", "notification-service")
	rabbitURL := getEnv("RABBITMQ_URL", "amqp://user:password@localhost:5672/")
	redisAddr := getEnv("REDIS_ADDR", "localhost:6379")
	dbDSN := getEnv("DATABASE_URL", "")

	ctx := context.Background()

	// Initialize Redis client
	rdb := redis.NewClient(&redis.Options{
		Addr: redisAddr,
	})
	if _, err := rdb.Ping(ctx).Result(); err != nil {
		log.Printf("Warning: Redis not available, running without deduplication: %v", err)
		rdb = nil
	}

	// Initialize RabbitMQ client
	rabbitClient, err := messaging.NewRabbitMQClient(rabbitURL)
	if err != nil {
		log.Fatalf("Failed to connect to RabbitMQ: %v", err)
	}
	defer rabbitClient.Close()

	// Declare notification queues with DLQ support
	queues := []string{"email.notifications", "sms.notifications", "web.notifications", "webhook.notifications"}
	for _, q := range queues {
		if _, err := rabbitClient.DeclareQueueWithDLQ(q); err != nil {
			log.Printf("Warning: Failed to declare queue %s with DLQ: %v", q, err)
			// Fallback to regular queue
			rabbitClient.DeclareQueue(q)
		}
	}

	// Initialize database (optional)
	var repo *notification.Repository
	if dbDSN != "" {
		db, err := database.Connect(dbDSN)
		if err != nil {
			log.Printf("Warning: Database not available: %v", err)
		} else {
			repo = notification.NewRepository(db)
			log.Println("Database connected for notification persistence")
		}
	}

	// Initialize driver registry for workers
	registry := notification.NewDriverRegistry()
	registry.Register(notification.NewEmailDriver())
	registry.Register(notification.NewSMSDriver())
	registry.Register(notification.NewWebDriver())

	// Initialize event router
	router := notification.NewRouter(rabbitClient)

	// Start notification workers (consume from RabbitMQ)
	startWorkers(rabbitClient, registry, rdb, repo)

	// Start Metrics Server
	monitoring.StartMetricsServer(":8084")

	log.Println("Notification Service started")
	log.Printf("  - Kafka: %s (topic: %s, group: %s)", kafkaBrokers, kafkaTopic, kafkaGroupID)
	log.Printf("  - RabbitMQ: connected")
	log.Printf("  - Redis: %v", rdb != nil)
	log.Println("Consuming events from Kafka...")

	// Initialize Kafka consumer
	brokers := strings.Split(kafkaBrokers, ",")
	kafkaConsumer := messaging.NewKafkaConsumer(brokers, kafkaTopic, kafkaGroupID)
	defer kafkaConsumer.Close()

	// Consume events from Kafka and route to RabbitMQ
	kafkaConsumer.Consume(ctx, func(key string, value []byte) error {
		var event notification.Event
		if err := json.Unmarshal(value, &event); err != nil {
			log.Printf("Failed to unmarshal event: %v", err)
			EventsProcessed.WithLabelValues("unknown", "error").Inc()
			return err
		}

		log.Printf("Received event: type=%s, id=%s", event.Type, event.ID)

		// Route event to appropriate RabbitMQ queues
		if err := router.Route(ctx, &event); err != nil {
			log.Printf("Failed to route event: %v", err)
			EventsProcessed.WithLabelValues(string(event.Type), "error").Inc()
			return err
		}

		EventsProcessed.WithLabelValues(string(event.Type), "success").Inc()
		return nil
	})

	// Keep main running
	select {}
}

func startWorkers(rabbitClient *messaging.RabbitMQClient, registry *notification.DriverRegistry, rdb *redis.Client, repo *notification.Repository) {
	// Email worker
	emailDriver, _ := registry.Get(notification.Email)
	emailWorker := notification.NewWorker(notification.Email, emailDriver, rdb)
	rabbitClient.Consume("email.notifications", func(body []byte) error {
		err := emailWorker.ProcessTask(context.Background(), body)
		if err != nil {
			NotificationsSent.WithLabelValues("email", "error").Inc()
		} else {
			NotificationsSent.WithLabelValues("email", "success").Inc()
		}
		return err
	})

	// SMS worker
	smsDriver, _ := registry.Get(notification.SMS)
	smsWorker := notification.NewWorker(notification.SMS, smsDriver, rdb)
	rabbitClient.Consume("sms.notifications", func(body []byte) error {
		err := smsWorker.ProcessTask(context.Background(), body)
		if err != nil {
			NotificationsSent.WithLabelValues("sms", "error").Inc()
		} else {
			NotificationsSent.WithLabelValues("sms", "success").Inc()
		}
		return err
	})

	// Web push worker
	webDriver, _ := registry.Get(notification.Web)
	webWorker := notification.NewWorker(notification.Web, webDriver, rdb)
	rabbitClient.Consume("web.notifications", func(body []byte) error {
		err := webWorker.ProcessTask(context.Background(), body)
		if err != nil {
			NotificationsSent.WithLabelValues("web", "error").Inc()
		} else {
			NotificationsSent.WithLabelValues("web", "success").Inc()
		}
		return err
	})

	// Webhook worker
	webhookWorker := notification.NewWebhookWorker(rdb)
	rabbitClient.Consume("webhook.notifications", func(body []byte) error {
		err := webhookWorker.ProcessWebhook(context.Background(), body)
		if err != nil {
			NotificationsSent.WithLabelValues("webhook", "error").Inc()
		} else {
			NotificationsSent.WithLabelValues("webhook", "success").Inc()
		}
		return err
	})

	log.Println("Workers started for: email, sms, web, webhook")
}

func getEnv(key, defaultValue string) string {
	if value := os.Getenv(key); value != "" {
		return value
	}
	return defaultValue
}
