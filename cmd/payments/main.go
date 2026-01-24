package main

import (
	"log"
	"net/http"
	"os"
	"strings"
	"time"

	"github.com/marwan562/fintech-ecosystem/internal/payment"
	"github.com/marwan562/fintech-ecosystem/pkg/bank"
	"github.com/marwan562/fintech-ecosystem/pkg/database"
	"github.com/marwan562/fintech-ecosystem/pkg/jsonutil"
	"github.com/marwan562/fintech-ecosystem/pkg/messaging"
	"github.com/marwan562/fintech-ecosystem/pkg/monitoring"
	"github.com/marwan562/fintech-ecosystem/pkg/observability"
	pb "github.com/marwan562/fintech-ecosystem/proto/ledger"

	"context"

	// NEW
	// NEW

	"github.com/redis/go-redis/v9"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"
)

func main() {
	// Default DSN for local development (Port 5434 for payments)
	dsn := os.Getenv("DB_DSN")
	if dsn == "" {
		dsn = "postgres://user:password@127.0.0.1:5434/payments?sslmode=disable"
	}

	db, err := database.Connect(dsn)
	if err != nil {
		log.Printf("Warning: Database connection failed (ensure Docker is running): %v", err)
	} else {
		log.Println("Database connection established")

		// Run migration explicitly
		schema, err := os.ReadFile("internal/payment/schema.sql")
		if err != nil {
			log.Printf("Failed to read schema file: %v", err)
		} else {
			if _, err := db.Exec(string(schema)); err != nil {
				log.Printf("Failed to run migration: %v", err)
			} else {
				log.Println("Schema migration executed successfully")
			}
		}
	}
	if db != nil {
		defer db.Close()
	}

	// Initialize Redis
	redisAddr := os.Getenv("REDIS_ADDR")
	if redisAddr == "" {
		redisAddr = "localhost:6379"
	}
	rdb := redis.NewClient(&redis.Options{
		Addr: redisAddr,
	})
	if err := rdb.Ping(context.Background()).Err(); err != nil {
		log.Printf("Warning: Redis connection failed in Payments: %v", err)
	}

	// Initialize dependencies
	repo := payment.NewRepository(db)
	bankClient := bank.NewMockClient()

	// Setup Ledger Service gRPC Client
	ledgerGRPCAddr := os.Getenv("LEDGER_GRPC_ADDR")
	if ledgerGRPCAddr == "" {
		ledgerGRPCAddr = "localhost:50052"
	}
	conn, err := grpc.Dial(ledgerGRPCAddr, grpc.WithTransportCredentials(insecure.NewCredentials()))
	if err != nil {
		log.Fatalf("did not connect to ledger gRPC: %v", err)
	}
	defer conn.Close()
	ledgerClient := pb.NewLedgerServiceClient(conn)

	// Setup Kafka Producer
	kafkaBrokers := os.Getenv("KAFKA_BROKERS")
	if kafkaBrokers == "" {
		kafkaBrokers = "localhost:9092"
	}
	brokers := strings.Split(kafkaBrokers, ",")
	kafkaProducer := messaging.NewKafkaProducer(brokers, "payments")
	defer kafkaProducer.Close()

	// Setup RabbitMQ Client
	rabbitURL := os.Getenv("RABBITMQ_URL")
	if rabbitURL == "" {
		rabbitURL = "amqp://user:password@localhost:5672/"
	}
	rabbitClient, err := messaging.NewRabbitMQClient(messaging.Config{
		URL:                   rabbitURL,
		ReconnectDelay:        time.Second,
		MaxReconnectDelay:     time.Minute,
		MaxRetries:            -1, // infinite retries
		CircuitBreakerEnabled: true,
	})
	if err != nil {
		log.Printf("Warning: Failed to connect to RabbitMQ: %v", err)
	} else {
		defer rabbitClient.Close()
		rabbitClient.DeclareQueue("notifications")
	}

	// Initialize Tracer
	shutdown, err := observability.InitTracer(context.Background(), observability.Config{
		ServiceName:    "payments",
		ServiceVersion: "0.1.0",
		Endpoint:       os.Getenv("OTEL_EXPORTER_OTLP_ENDPOINT"),
		Environment:    "production",
	})
	if err != nil {
		log.Printf("Failed to init tracer: %v", err)
	} else {
		defer shutdown(context.Background())
	}

	// Start Metrics Server
	monitoring.StartMetricsServer(":8086") // Distinct from HTTP server on 8082 if preferred, but on separate port is standard

	handler := &PaymentHandler{
		repo:          repo,
		bankClient:    bankClient,
		rdb:           rdb,
		ledgerClient:  ledgerClient,
		kafkaProducer: kafkaProducer,
		rabbitClient:  rabbitClient,
	}

	mux := http.NewServeMux()

	mux.HandleFunc("/health", func(w http.ResponseWriter, r *http.Request) {
		jsonutil.WriteJSON(w, http.StatusOK, map[string]string{
			"status":  "active",
			"service": "payments",
			"db_connected": func() string {
				if db != nil {
					return "true"
				}
				return "false"
			}(),
		})
	})

	// Register Handlers
	// Gateway forwards /payments/* -> /*
	// So /payments/payment_intents -> /payment_intents
	mux.HandleFunc("/payment_intents", handler.IdempotencyMiddleware(handler.CreatePaymentIntent))

	// For /confirm, we need to match the path prefix because of the ID parameter
	// /payment_intents/{id}/confirm
	mux.HandleFunc("/payment_intents/", func(w http.ResponseWriter, r *http.Request) {
		path := r.URL.Path
		if r.Method == http.MethodPost && strings.HasSuffix(path, "/confirm") {
			handler.IdempotencyMiddleware(handler.ConfirmPaymentIntent)(w, r)
			return
		}
		if r.Method == http.MethodPost && strings.HasSuffix(path, "/refund") {
			handler.IdempotencyMiddleware(handler.RefundPaymentIntent)(w, r)
			return
		}
		// Fallback or other sub-resources could go here.
		jsonutil.WriteErrorJSON(w, "Not Found")
	})

	log.Println("Payments service starting on :8082")

	// Wrap handler with OpenTelemetry
	otelHandler := otelhttp.NewHandler(mux, "payments-request")

	if err := http.ListenAndServe(":8082", otelHandler); err != nil {
		log.Fatalf("Server failed: %v", err)
	}
}
