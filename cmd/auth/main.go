package main

import (
	"log"
	"net"
	"net/http"
	"os"

	"microservices/internal/auth"
	"microservices/pkg/database"
	"microservices/pkg/jsonutil"
	pb "microservices/proto/auth"

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
		defer db.Close()
	}

	repo := auth.NewRepository(db)
	handler := &AuthHandler{repo: repo}

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
	go func() {
		if err := http.ListenAndServe(":8081", mux); err != nil {
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
