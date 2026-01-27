package main

import (
	"context"
	"log"
	"net"
	"net/http"
	"os"

	"github.com/marwan562/fintech-ecosystem/internal/auth"
	"github.com/marwan562/fintech-ecosystem/pkg/database"
	"github.com/marwan562/fintech-ecosystem/pkg/jsonutil"
	"github.com/marwan562/fintech-ecosystem/pkg/observability" // NEW
	pb "github.com/marwan562/fintech-ecosystem/proto/auth"
	"go.opentelemetry.io/contrib/instrumentation/net/http/otelhttp" // NEW

	"github.com/marwan562/fintech-ecosystem/pkg/monitoring"
	"google.golang.org/grpc"
)

func main() {
	// Default DSN for local development
	dsn := os.Getenv("DB_DSN")
	if dsn == "" {
		dsn = "postgres://user:password@127.0.0.1:5433/microservices?sslmode=disable"
	}

	db, err := database.Connect(dsn)
	if err != nil {
		log.Printf("Warning: Database connection failed (ensure Docker is running): %v", err)
	} else {
		log.Println("Database connection established")

		// Run migration explicitly
		schema, err := os.ReadFile("internal/auth/schema.sql")
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
		defer func() {
			if err := db.Close(); err != nil {
				log.Printf("Failed to close DB: %v", err)
			}
		}()
	}

	hmacSecret := os.Getenv("API_KEY_HMAC_SECRET")
	if hmacSecret == "" {
		hmacSecret = "local-dev-secret-do-not-use-in-prod"
		log.Println("Warning: API_KEY_HMAC_SECRET not set, using default for dev")
	}

	repo := auth.NewRepository(db)
	handler := &AuthHandler{repo: repo, hmacSecret: hmacSecret}

	// Initialize Tracer
	shutdown, err := observability.InitTracer(context.Background(), observability.Config{
		ServiceName:    "auth",
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

	// Start Metrics Server
	monitoring.StartMetricsServer(":8085")

	mux := http.NewServeMux()

	mux.HandleFunc("/health", func(w http.ResponseWriter, r *http.Request) {
		jsonutil.WriteJSON(w, http.StatusOK, map[string]string{
			"status":  "active",
			"service": "auth",
			"db_connected": func() string {
				if db != nil {
					return "true"
				}
				return "false"
			}(),
		})
	})

	mux.HandleFunc("/register", handler.Register)
	mux.HandleFunc("/login", handler.Login)
	mux.HandleFunc("/api_keys", handler.GenerateAPIKey)
	// Internal endpoint for Gateway Validation
	mux.HandleFunc("/validate_key", handler.ValidateAPIKey)

	log.Println("Auth service HTTP starting on :8081")

	// Wrap handler with OpenTelemetry
	otelHandler := otelhttp.NewHandler(mux, "auth-request")

	go func() {
		if err := http.ListenAndServe(":8081", otelHandler); err != nil {
			log.Fatalf("HTTP server failed: %v", err)
		}
	}()

	// Start gRPC Server
	lis, err := net.Listen("tcp", ":50051")
	if err != nil {
		log.Fatalf("failed to listen for gRPC: %v", err)
	}
	s := grpc.NewServer()
	pb.RegisterAuthServiceServer(s, NewAuthGRPCServer(repo))

	log.Println("Auth service gRPC starting on :50051")
	if err := s.Serve(lis); err != nil {
		log.Fatalf("gRPC server failed: %v", err)
	}
}
