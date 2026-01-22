package main

import (
	"log"
	"microservices/internal/payment"
	"microservices/pkg/bank"
	"microservices/pkg/database"
	"microservices/pkg/jsonutil"
	pb "microservices/proto/ledger"
	"net/http"
	"os"

	"context"

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

	handler := &PaymentHandler{
		repo:         repo,
		bankClient:   bankClient,
		rdb:          rdb,
		ledgerClient: ledgerClient,
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
		if r.Method == http.MethodPost && len(r.URL.Path) > len("/payment_intents/") && r.URL.Path[len(r.URL.Path)-8:] == "/confirm" {
			handler.IdempotencyMiddleware(handler.ConfirmPaymentIntent)(w, r)
			return
		}
		// Fallback or other sub-resources could go here.
		// For now, if it's not confirm, return 404 or Method Not Allowed
		jsonutil.WriteErrorJSON(w, "Not Found")
	})

	log.Println("Payments service starting on :8082")
	if err := http.ListenAndServe(":8082", mux); err != nil {
		log.Fatalf("Server failed: %v", err)
	}
}
