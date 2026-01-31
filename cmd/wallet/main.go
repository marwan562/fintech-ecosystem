package main

import (
	"context"
	"log"
	"net"
	"net/http"
	"os"

	"github.com/marwan562/fintech-ecosystem/internal/wallet/api"
	"github.com/marwan562/fintech-ecosystem/internal/wallet/domain"
	"github.com/marwan562/fintech-ecosystem/internal/wallet/infrastructure"
	"github.com/marwan562/fintech-ecosystem/pkg/monitoring"
	"github.com/marwan562/fintech-ecosystem/pkg/observability"
	ledgerpb "github.com/marwan562/fintech-ecosystem/proto/ledger"
	walletpb "github.com/marwan562/fintech-ecosystem/proto/wallet"
	"go.opentelemetry.io/contrib/instrumentation/net/http/otelhttp"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"
)

func main() {
	// Setup Ledger gRPC Client
	ledgerGRPCAddr := os.Getenv("LEDGER_GRPC_ADDR")
	if ledgerGRPCAddr == "" {
		ledgerGRPCAddr = "localhost:50052"
	}
	conn, err := grpc.NewClient(ledgerGRPCAddr,
		grpc.WithTransportCredentials(insecure.NewCredentials()),
		grpc.WithUnaryInterceptor(monitoring.UnaryClientInterceptor("wallet")),
	)
	if err != nil {
		log.Fatalf("did not connect to ledger gRPC: %v", err)
	}
	defer func() { _ = conn.Close() }()
	ledgerClient := ledgerpb.NewLedgerServiceClient(conn)

	// Initialize Domain Service
	infraLedger := infrastructure.NewLedgerClient(ledgerClient)
	walletService := domain.NewWalletService(infraLedger)

	// Initialize Tracer
	shutdown, err := observability.InitTracer(context.Background(), observability.Config{
		ServiceName:    "wallet",
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
	monitoring.StartMetricsServer(":8088")

	handler := api.NewWalletHandler(walletService)

	mux := http.NewServeMux()
	mux.HandleFunc("/v1/wallets/top-up", handler.TopUp)
	mux.HandleFunc("/v1/wallets/transfer", handler.Transfer)
	mux.HandleFunc("/v1/wallets/", handler.GetWallet)

	// Wrap handler with OpenTelemetry and Prometheus
	otelHandler := otelhttp.NewHandler(mux, "wallet-request")
	promHandler := monitoring.PrometheusMiddleware(otelHandler)

	go func() {
		log.Println("Wallet service HTTP starting on :8084")
		if err := http.ListenAndServe(":8084", promHandler); err != nil {
			log.Fatalf("HTTP server failed: %v", err)
		}
	}()

	// Start gRPC Server
	lis, err := net.Listen("tcp", ":50053")
	if err != nil {
		log.Fatalf("failed to listen for gRPC: %v", err)
	}
	s := grpc.NewServer(
		grpc.UnaryInterceptor(monitoring.UnaryServerInterceptor("wallet")),
	)
	walletpb.RegisterWalletServiceServer(s, api.NewWalletGRPCServer(walletService))

	log.Println("Wallet service gRPC starting on :50053")
	if err := s.Serve(lis); err != nil {
		log.Fatalf("gRPC server failed: %v", err)
	}
}
