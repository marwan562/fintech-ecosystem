package main

import (
	"net/http"
	"os"
	"strings"
	"time"

	"github.com/redis/go-redis/v9"
	"github.com/sapliy/fintech-ecosystem/internal/payment/domain"
	"github.com/sapliy/fintech-ecosystem/internal/payment/infrastructure"
	"github.com/sapliy/fintech-ecosystem/pkg/bank"
	"github.com/sapliy/fintech-ecosystem/pkg/database"
	"github.com/sapliy/fintech-ecosystem/pkg/jsonutil"
	"github.com/sapliy/fintech-ecosystem/pkg/messaging"
	"github.com/sapliy/fintech-ecosystem/pkg/monitoring"
	"github.com/sapliy/fintech-ecosystem/pkg/observability"
	pb "github.com/sapliy/fintech-ecosystem/proto/ledger"
	"go.opentelemetry.io/contrib/instrumentation/net/http/otelhttp"

	"context"

	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"
)

func main() {
	logger := observability.NewLogger("payments-service")

	// Default DSN for local development (Port 5434 for payments)
	dsn := os.Getenv("DB_DSN")
	if dsn == "" {
		dsn = "postgres://user:password@127.0.0.1:5434/payments?sslmode=disable"
	}

	db, err := database.Connect(dsn)
	if err != nil {
		logger.Warn("Database connection failed", "error", err)
	} else {
		logger.Info("Database connection established")

		// Run automated migrations
		if err := database.Migrate(db, "payments", "migrations/payments"); err != nil {
			logger.Error("Failed to run migrations", "error", err)
			os.Exit(1)
		}
	}
	if db != nil {
		defer func() {
			if err := db.Close(); err != nil {
				logger.Error("Failed to close DB", "error", err)
			}
		}()
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
		logger.Warn("Redis connection failed in Payments", "error", err)
	}

	// Initialize dependencies
	repo := infrastructure.NewSQLRepository(db)
	service := domain.NewPaymentService(repo)
	bankClient := bank.NewMockClient()

	// Setup Ledger Service gRPC Client
	ledgerGRPCAddr := os.Getenv("LEDGER_GRPC_ADDR")
	if ledgerGRPCAddr == "" {
		ledgerGRPCAddr = "localhost:50052"
	}
	conn, err := grpc.NewClient(ledgerGRPCAddr,
		grpc.WithTransportCredentials(insecure.NewCredentials()),
		grpc.WithUnaryInterceptor(monitoring.UnaryClientInterceptor("payments")),
	)
	if err != nil {
		logger.Error("did not connect to ledger gRPC", "error", err)
		os.Exit(1)
	}
	defer func() {
		if err := conn.Close(); err != nil {
			logger.Error("Failed to close gRPC connection", "error", err)
		}
	}()
	ledgerClient := pb.NewLedgerServiceClient(conn)

	// Setup Kafka Producer
	kafkaBrokers := os.Getenv("KAFKA_BROKERS")
	if kafkaBrokers == "" {
		kafkaBrokers = "localhost:9092"
	}
	brokers := strings.Split(kafkaBrokers, ",")
	kafkaProducer := messaging.NewKafkaProducer(brokers, "payments")
	defer func() {
		if err := kafkaProducer.Close(); err != nil {
			logger.Error("Failed to close Kafka producer", "error", err)
		}
	}()

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
		logger.Warn("Failed to connect to RabbitMQ", "error", err)
	} else {
		defer rabbitClient.Close()
		if _, err := rabbitClient.DeclareQueue("notifications"); err != nil {
			logger.Error("Failed to declare queue", "error", err)
		}
	}

	// Initialize Tracer
	shutdown, err := observability.InitTracer(context.Background(), observability.Config{
		ServiceName:    "payments",
		ServiceVersion: "0.1.0",
		Endpoint:       os.Getenv("OTEL_EXPORTER_OTLP_ENDPOINT"),
		Environment:    "production",
	})
	if err != nil {
		logger.Error("Failed to init tracer", "error", err)
	} else {
		defer func() {
			if err := shutdown(context.Background()); err != nil {
				logger.Error("Failed to shutdown tracer", "error", err)
			}
		}()
	}

	// Start Metrics Server
	monitoring.StartMetricsServer(":8086") // Distinct from HTTP server on 8082 if preferred, but on separate port is standard

	handler := &PaymentHandler{
		service:       service,
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

	port := ":8082"
	logger.Info("Payments service starting", "port", port)

	// Wrap handler with OpenTelemetry and Prometheus
	otelHandler := otelhttp.NewHandler(mux, "payments-request")
	promHandler := monitoring.PrometheusMiddleware(otelHandler)

	if err := http.ListenAndServe(port, promHandler); err != nil {
		logger.Error("Server failed", "error", err)
		os.Exit(1)
	}
}
