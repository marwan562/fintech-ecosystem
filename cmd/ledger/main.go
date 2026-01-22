package main

import (
	"log"
	"microservices/internal/ledger"
	"microservices/pkg/database"
	"microservices/pkg/jsonutil"
	pb "microservices/proto/ledger"
	"net"
	"net/http"
	"os"

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

		// Run migration explicitly
		schema, err := os.ReadFile("internal/ledger/schema.sql")
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

	repo := ledger.NewRepository(db)
	handler := &LedgerHandler{repo: repo}

	mux := http.NewServeMux()

	mux.HandleFunc("/health", func(w http.ResponseWriter, r *http.Request) {
		jsonutil.WriteJSON(w, http.StatusOK, map[string]string{
			"status":  "active",
			"service": "ledger",
		})
	})

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
	go func() {
		if err := http.ListenAndServe(":8083", mux); err != nil {
			log.Fatalf("HTTP server failed: %v", err)
		}
	}()

	// Start gRPC Server
	lis, err := net.Listen("tcp", ":50052")
	if err != nil {
		log.Fatalf("failed to listen for gRPC: %v", err)
	}
	s := grpc.NewServer()
	pb.RegisterLedgerServiceServer(s, NewLedgerGRPCServer(repo))

	log.Println("Ledger service gRPC starting on :50052")
	if err := s.Serve(lis); err != nil {
		log.Fatalf("gRPC server failed: %v", err)
	}
}
