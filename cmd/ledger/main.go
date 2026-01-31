package main

import (
	"context"
	"log"
	"net"
	"net/http"
	"os"
	"strings"
	"time"

	"github.com/marwan562/fintech-ecosystem/internal/ledger/domain"
	"github.com/marwan562/fintech-ecosystem/internal/ledger/infrastructure"
	"github.com/marwan562/fintech-ecosystem/pkg/database"
	"github.com/marwan562/fintech-ecosystem/pkg/jsonutil"
	"github.com/marwan562/fintech-ecosystem/pkg/observability"
	pb "github.com/marwan562/fintech-ecosystem/proto/ledger"
	"go.opentelemetry.io/contrib/instrumentation/net/http/otelhttp"

	"github.com/marwan562/fintech-ecosystem/pkg/messaging"
	"github.com/prometheus/client_golang/prometheus/promhttp"
	"google.golang.org/grpc"
)

func main() {
	dsn := os.Getenv("DB_DSN")
	if dsn == "" {
		dsn = "postgres://user:password@127.0.0.1:5435/ledger?sslmode=disable"
	}

	db, err := database.Connect(dsn)
	if err != nil {
		log.Printf("Warning: Database connection failed: %v", err)
	} else {
		log.Println("Database connection established")

		// Run automated migrations
		if err := database.Migrate(db, "ledger", "migrations/ledger"); err != nil {
			log.Fatalf("Failed to run migrations: %v", err)
		}
	}
	if db != nil {
		defer func() {
			if err := db.Close(); err != nil {
				log.Printf("Failed to close DB: %v", err)
			}
		}()
	}

	// Layered Architecture Setup
	repo := infrastructure.NewSQLRepository(db)
	metrics := &infrastructure.PrometheusMetrics{}
	service := domain.NewLedgerService(repo, metrics)

	// Initialize Tracer
	shutdown, err := observability.InitTracer(context.Background(), observability.Config{
		ServiceName:    "ledger",
		ServiceVersion: "0.1.0",
		Endpoint:       os.Getenv("OTEL_EXPORTER_OTLP_ENDPOINT"),
		Environment:    "production",
	})
	if err != nil {
		log.Printf("Failed to init tracer: %v", err)
	} else {
		defer func() {
			if err := shutdown(context.Background()); err != nil {
				log.Printf("Failed to shutdown tracer: %v", err)
			}
		}()
	}

	// Start Kafka Consumer for Event Sourcing
	kafkaBrokers := os.Getenv("KAFKA_BROKERS")
	if kafkaBrokers == "" {
		kafkaBrokers = "localhost:9092"
	}
	brokers := strings.Split(kafkaBrokers, ",")
	go StartKafkaConsumer(brokers, service)

	// Start Outbox Publisher for Reliable Event Delivery
	ledgerProducer := messaging.NewKafkaProducer(brokers, "ledger-events")
	publisher := infrastructure.NewOutboxPublisher(repo, ledgerProducer, 2*time.Second)
	go publisher.Start(context.Background())

	handler := &LedgerHandler{service: service}

	mux := http.NewServeMux()

	mux.HandleFunc("/health", func(w http.ResponseWriter, r *http.Request) {
		jsonutil.WriteJSON(w, http.StatusOK, map[string]string{
			"status":  "active",
			"service": "ledger",
		})
	})

	mux.Handle("/metrics", promhttp.Handler())

	mux.HandleFunc("/accounts", handler.CreateAccount)

	// Simple routing for /accounts/{id}
	mux.HandleFunc("/accounts/", func(w http.ResponseWriter, r *http.Request) {
		if r.Method == http.MethodGet {
			handler.GetAccount(w, r)
			return
		}
		jsonutil.WriteErrorJSON(w, "Not Found")
	})

	mux.HandleFunc("/transactions", handler.RecordTransaction)

	log.Println("Ledger service HTTP starting on :8083")

	// Wrap handler with OpenTelemetry
	otelHandler := otelhttp.NewHandler(mux, "ledger-request")

	// Standard metrics/tracing wrapper can be added here
	go func() {
		if err := http.ListenAndServe(":8083", otelHandler); err != nil {
			log.Fatalf("HTTP server failed: %v", err)
		}
	}()

	// Start gRPC Server
	lis, err := net.Listen("tcp", ":50052")
	if err != nil {
		log.Fatalf("failed to listen for gRPC: %v", err)
	}
	s := grpc.NewServer()
	pb.RegisterLedgerServiceServer(s, NewLedgerGRPCServer(service))

	log.Println("Ledger service gRPC starting on :50052")
	if err := s.Serve(lis); err != nil {
		log.Fatalf("gRPC server failed: %v", err)
	}
}
