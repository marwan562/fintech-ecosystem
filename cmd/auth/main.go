package main

import (
	"context"
	"fmt"
	"log"
	"net"
	"net/http"
	"os"
	"os/signal"
	"syscall"

	"github.com/redis/go-redis/v9"
	"github.com/sapliy/fintech-ecosystem/internal/auth/domain"
	"github.com/sapliy/fintech-ecosystem/internal/auth/infrastructure"
	"github.com/sapliy/fintech-ecosystem/internal/flow"
	flowDomain "github.com/sapliy/fintech-ecosystem/internal/flow/domain"
	flowInfra "github.com/sapliy/fintech-ecosystem/internal/flow/infrastructure"
	zone "github.com/sapliy/fintech-ecosystem/internal/zone"
	zoneDomain "github.com/sapliy/fintech-ecosystem/internal/zone/domain"
	zoneInfra "github.com/sapliy/fintech-ecosystem/internal/zone/infrastructure"
	"github.com/sapliy/fintech-ecosystem/pkg/database"
	"github.com/sapliy/fintech-ecosystem/pkg/jsonutil"
	"github.com/sapliy/fintech-ecosystem/pkg/monitoring"
	"github.com/sapliy/fintech-ecosystem/pkg/observability"
	pb "github.com/sapliy/fintech-ecosystem/proto/auth"
	ledgerPb "github.com/sapliy/fintech-ecosystem/proto/ledger"
	"go.opentelemetry.io/contrib/instrumentation/net/http/otelhttp"
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

		// Run automated migrations
		if err := database.Migrate(db, "microservices", "migrations/auth"); err != nil {
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

	hmacSecret := os.Getenv("API_KEY_HMAC_SECRET")
	if hmacSecret == "" {
		hmacSecret = "local-dev-secret-do-not-use-in-prod"
		log.Println("Warning: API_KEY_HMAC_SECRET not set, using default for dev")
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
		log.Printf("Warning: Redis connection failed in Auth: %v", err)
	}

	sqlRepo := infrastructure.NewSQLRepository(db)
	cachedRepo := infrastructure.NewCachedRepository(sqlRepo, rdb)
	authService := domain.NewAuthService(cachedRepo)

	zoneSQLRepo := zoneInfra.NewSQLRepository(db)
	zoneRepo := zoneInfra.NewCachedRepository(zoneSQLRepo, rdb)
	flowRepo := flowInfra.NewSQLRepository(db)

	providers := zoneDomain.TemplateProviders{
		CreateLedgerAccount: func(ctx context.Context, name, accType, currency string, zoneID, mode string) error {
			conn, err := grpc.NewClient("localhost:50052", grpc.WithInsecure())
			if err != nil {
				return err
			}
			defer conn.Close()
			client := ledgerPb.NewLedgerServiceClient(conn)
			_, err = client.CreateAccount(ctx, &ledgerPb.CreateAccountRequest{
				Name:     name,
				Type:     accType,
				Currency: currency,
				ZoneId:   zoneID,
				Mode:     mode,
			})
			return err
		},
		CreateFlow: func(ctx context.Context, zoneID string, name string, nodes interface{}, edges interface{}) error {
			// Basic template flow
			return flowRepo.CreateFlow(ctx, &flowDomain.Flow{
				ZoneID:  zoneID,
				Name:    name,
				Enabled: true,
			})
		},
	}
	zoneService := zone.NewService(zoneRepo, providers)
	templateService := zone.NewTemplateService(zoneService)

	handler := &AuthHandler{service: authService, hmacSecret: hmacSecret}
	zoneHandler := &ZoneHandler{service: zoneService, templateService: templateService}

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
	mux.HandleFunc("/oauth/token", handler.OAuthTokenHandler)
	mux.HandleFunc("/oauth/authorize", handler.AuthorizeHandler)
	mux.HandleFunc("/oauth/clients", handler.RegisterClientHandler)
	mux.HandleFunc("/oauth/introspect", handler.TokenIntrospectionHandler)
	// Internal endpoint for Gateway Validation
	mux.HandleFunc("/validate_key", handler.ValidateAPIKey)
	mux.HandleFunc("/events/trigger", handler.TriggerEvent)
	mux.HandleFunc("/sso/callback", handler.SSOCallback)

	// Zone Management
	mux.HandleFunc("/zones", func(w http.ResponseWriter, r *http.Request) {
		switch r.Method {
		case http.MethodPost:
			zoneHandler.CreateZone(w, r)
		case http.MethodGet:
			if r.URL.Query().Get("id") != "" {
				zoneHandler.GetZone(w, r)
			} else {
				zoneHandler.ListZones(w, r)
			}
		default:
			jsonutil.WriteErrorJSON(w, "Method not allowed")
		}
	})

	flowRunner := flowDomain.NewFlowRunner(flowRepo)
	debugService := flow.NewDebugService(flowRepo)
	flowHandler := &FlowHandler{repo: flowRepo, runner: flowRunner}
	debugHandler := NewDebugHandler(debugService)

	// ... rest of main ...
	mux.HandleFunc("/flows", func(w http.ResponseWriter, r *http.Request) {
		switch r.Method {
		case http.MethodPost:
			flowHandler.CreateFlow(w, r)
		case http.MethodGet:
			if r.URL.Query().Get("id") != "" {
				flowHandler.GetFlow(w, r)
			} else {
				flowHandler.ListFlows(w, r)
			}
		case http.MethodPut:
			flowHandler.UpdateFlow(w, r)
		default:
			jsonutil.WriteErrorJSON(w, "Method not allowed")
		}
	})

	mux.HandleFunc("/flows/executions", flowHandler.GetExecution)
	mux.HandleFunc("/flows/resume", flowHandler.ResumeExecution)
	mux.HandleFunc("/flows/bulk-update", flowHandler.BulkUpdateFlows)
	mux.HandleFunc("/zones/bulk-metadata", zoneHandler.BulkUpdateMetadata)

	// Template Management
	mux.HandleFunc("/templates", func(w http.ResponseWriter, r *http.Request) {
		switch r.Method {
		case http.MethodGet:
			if r.URL.Query().Get("type") != "" {
				zoneHandler.GetTemplate(w, r)
			} else {
				zoneHandler.ListTemplates(w, r)
			}
		default:
			jsonutil.WriteErrorJSON(w, "Method not allowed")
		}
	})
	mux.HandleFunc("/templates/apply", func(w http.ResponseWriter, r *http.Request) {
		if r.Method == http.MethodPost {
			zoneHandler.ApplyTemplate(w, r)
		} else {
			jsonutil.WriteErrorJSON(w, "Method not allowed")
		}
	})

	// Debug Mode endpoints
	mux.HandleFunc("/debug/sessions", func(w http.ResponseWriter, r *http.Request) {
		switch r.Method {
		case http.MethodPost:
			debugHandler.StartDebugSession(w, r)
		case http.MethodGet:
			if r.URL.Query().Get("session_id") != "" {
				debugHandler.GetDebugSession(w, r)
			} else {
				debugHandler.GetDebugEvents(w, r)
			}
		default:
			jsonutil.WriteErrorJSON(w, "Method not allowed")
		}
	})
	mux.HandleFunc("/debug/sessions/end", func(w http.ResponseWriter, r *http.Request) {
		if r.Method == http.MethodPost {
			debugHandler.EndDebugSession(w, r)
		} else {
			jsonutil.WriteErrorJSON(w, "Method not allowed")
		}
	})
	mux.HandleFunc("/debug/execute", func(w http.ResponseWriter, r *http.Request) {
		if r.Method == http.MethodPost {
			debugHandler.ExecuteFlowWithDebug(w, r)
		} else {
			jsonutil.WriteErrorJSON(w, "Method not allowed")
		}
	})
	mux.HandleFunc("/debug/ws", debugHandler.WebSocketDebug)

	// Initialize Logger
	logger := observability.NewLogger("auth-service")

	// Wrap handler with OpenTelemetry and Prometheus
	otelHandler := otelhttp.NewHandler(mux, "auth-request")
	promHandler := monitoring.PrometheusMiddleware(otelHandler)

	port := "8081"        // Define port for HTTP server
	router := promHandler // Define router for HTTP server

	logger.Info("Auth Service starting", "port", port)
	logger.Info("Debug API available", "url", fmt.Sprintf("http://localhost:%s/api/v1", port))
	logger.Info("WebSocket available", "url", fmt.Sprintf("ws://localhost:%s/api/v1/auth/debug/ws", port))

	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	srv := &http.Server{
		Addr:    ":" + port,
		Handler: router,
	}

	go func() {
		if err := srv.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			logger.Error("HTTP server error", "error", err)
			os.Exit(1)
		}
	}()

	<-ctx.Done()
	logger.Info("Shutting down Auth Service...")

	// Start gRPC Server
	lis, err := net.Listen("tcp", ":50051")
	if err != nil {
		logger.Error("Failed to listen for gRPC", "error", err)
		os.Exit(1)
	}
	s := grpc.NewServer(
		grpc.UnaryInterceptor(monitoring.UnaryServerInterceptor("auth")),
	)
	pb.RegisterAuthServiceServer(s, NewAuthGRPCServer(authService))

	log.Println("Auth service gRPC starting on :50051")
	if err := s.Serve(lis); err != nil {
		log.Fatalf("gRPC server failed: %v", err)
	}
}
