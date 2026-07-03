package main

import (
	"context"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/sapliy/sapliy-core/pkg/authutil"
	"github.com/sapliy/sapliy-core/pkg/observability"
	"go.opentelemetry.io/contrib/instrumentation/net/http/otelhttp"

	pb "github.com/sapliy/sapliy-core/proto/auth"
	walletpb "github.com/sapliy/sapliy-core/proto/wallet"

	"github.com/redis/go-redis/v9"
	"github.com/sapliy/sapliy-core/pkg/monitoring"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"
)

func main() {
	logger := observability.NewLogger("gateway-service")
	cfg := LoadConfig(logger)

	rdb := redis.NewClient(&redis.Options{
		Addr: cfg.RedisAddr,
	})
	if err := rdb.Ping(context.Background()).Err(); err != nil {
		logger.Warn("Redis connection failed", "error", err)
	} else {
		logger.Info("Redis connection established")
	}

	// Setup Auth Service gRPC Client
	conn, err := grpc.NewClient(cfg.AuthGRPCAddr,
		grpc.WithTransportCredentials(insecure.NewCredentials()),
		grpc.WithChainUnaryInterceptor(
			monitoring.UnaryClientInterceptor("gateway"),
			authutil.UnaryInternalTokenClientInterceptor(),
		),
	)
	if err != nil {
		logger.Error("did not connect to auth gRPC", "error", err)
		os.Exit(1)
	}
	defer conn.Close()
	authClient := pb.NewAuthServiceClient(conn)

	// Setup Wallet Service gRPC Client
	connWallet, err := grpc.NewClient(cfg.WalletGRPCAddr,
		grpc.WithTransportCredentials(insecure.NewCredentials()),
		grpc.WithChainUnaryInterceptor(
			monitoring.UnaryClientInterceptor("gateway"),
			authutil.UnaryInternalTokenClientInterceptor(),
		),
	)
	if err != nil {
		logger.Error("did not connect to wallet gRPC", "error", err)
		os.Exit(1)
	}
	defer connWallet.Close()
	walletClient := walletpb.NewWalletServiceClient(connWallet)

	// Initialize Tracer
	shutdown, err := observability.InitTracer(context.Background(), observability.Config{
		ServiceName:    "gateway",
		ServiceVersion: "0.1.0",
		Endpoint:       cfg.OTEL_Endpoint,
		Environment:    cfg.Environment,
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
	monitoring.StartMetricsServer(":8087")

	gateway := NewGatewayHandler(cfg, rdb, authClient, walletClient, logger)

	// Middlewares
	limitHandler := BodyLimitMiddleware(1 * 1024 * 1024)(gateway) // 1MB limit
	authHandler := gateway.AuthMiddleware(limitHandler)
	corsHandler := CORSMiddleware(cfg.CORSOrigins, cfg.AllowedOrigins)(authHandler)
	otelHandler := otelhttp.NewHandler(corsHandler, "gateway-request")
	promHandler := monitoring.PrometheusMiddleware(otelHandler)

	server := &http.Server{
		Addr:              cfg.Port,
		Handler:           promHandler,
		ReadTimeout:       15 * time.Second,
		WriteTimeout:      30 * time.Second,
		IdleTimeout:       60 * time.Second,
		ReadHeaderTimeout: 5 * time.Second,
	}

	logger.Info("Gateway service starting", "port", cfg.Port)

	go func() {
		if err := server.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			logger.Error("Gateway server failed", "error", err)
			os.Exit(1)
		}
	}()

	quit := make(chan os.Signal, 1)
	signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)
	sig := <-quit
	logger.Info("Shutdown signal received, draining connections...", "signal", sig.String())

	shutdownCtx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
	defer cancel()

	if err := server.Shutdown(shutdownCtx); err != nil {
		logger.Error("Gateway forced to shutdown", "error", err)
		os.Exit(1)
	}
	logger.Info("Gateway shutdown complete")
}
