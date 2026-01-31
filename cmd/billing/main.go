package main

import (
	"context"
	"database/sql"
	"log"
	"net"
	"os"
	"os/signal"
	"syscall"
	"time"

	_ "github.com/lib/pq"
	"google.golang.org/grpc"
	"google.golang.org/grpc/reflection"

	"github.com/marwan562/fintech-ecosystem/internal/billing/infrastructure"
	"github.com/marwan562/fintech-ecosystem/internal/billing/service"
)

func main() {
	dbURL := os.Getenv("DATABASE_URL")
	if dbURL == "" {
		dbURL = "postgres://postgres:postgres@localhost:5432/fintech?sslmode=disable"
	}

	db, err := sql.Open("postgres", dbURL)
	if err != nil {
		log.Fatalf("failed to connect to database: %v", err)
	}
	defer func() { _ = db.Close() }()

	repo := infrastructure.NewSQLRepository(db)

	// Mock payment client for now - in reality this would be a gRPC client to Payment service
	paymentClient := &mockPaymentClient{}

	worker := service.NewSubscriptionWorker(repo, paymentClient, 1*time.Minute)

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	go worker.Start(ctx)

	lis, err := net.Listen("tcp", ":50054")
	if err != nil {
		log.Fatalf("failed to listen: %v", err)
	}

	s := grpc.NewServer()
	// pb.RegisterBillingServiceServer(s, billingService) // Not yet implemented gRPC handlers fully in domain.BillingService
	reflection.Register(s)

	go func() {
		log.Println("Billing service starting on :50054")
		if err := s.Serve(lis); err != nil {
			log.Fatalf("failed to serve: %v", err)
		}
	}()

	stop := make(chan os.Signal, 1)
	signal.Notify(stop, syscall.SIGINT, syscall.SIGTERM)
	<-stop

	log.Println("Shutting down billing service...")
	s.GracefulStop()
}

type mockPaymentClient struct{}

func (m *mockPaymentClient) CreatePayment(ctx context.Context, userID, orgID string, amount int64, currency string) (string, error) {
	log.Printf("Mock Payment: user=%s, org=%s, amount=%d %s", userID, orgID, amount, currency)
	return "pmt_mock_" + userID, nil
}
