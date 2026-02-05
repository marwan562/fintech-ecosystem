package main

import (
	"context"
	"fmt"
	"log"
	"net/http"
	"net/http/httputil"
	"net/url"
	"os"
	"strings"
	"time"

	"github.com/sapliy/fintech-ecosystem/pkg/apikey"
	"github.com/sapliy/fintech-ecosystem/pkg/jsonutil"
	"github.com/sapliy/fintech-ecosystem/pkg/observability"
	"github.com/sapliy/fintech-ecosystem/pkg/scopes"
	"go.opentelemetry.io/contrib/instrumentation/net/http/otelhttp"

	pb "github.com/sapliy/fintech-ecosystem/proto/auth"
	walletpb "github.com/sapliy/fintech-ecosystem/proto/wallet"

	"github.com/gorilla/websocket"
	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promauto"
	"github.com/redis/go-redis/v9"
	"github.com/sapliy/fintech-ecosystem/pkg/monitoring"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"
)

var (
	GatewayRequests = promauto.NewCounterVec(prometheus.CounterOpts{
		Name: "gateway_requests_total",
		Help: "Total number of requests handled by the gateway.",
	}, []string{"method", "path", "status"})
)

// GatewayHandler holds the configuration for upstream service URLs and Redis.
type GatewayHandler struct {
	authServiceURL         string
	paymentServiceURL      string
	ledgerServiceURL       string
	walletServiceURL       string
	billingServiceURL      string
	notificationServiceURL string
	rdb                    *redis.Client
	upgrader               websocket.Upgrader
	authClient             pb.AuthServiceClient
	walletClient           walletpb.WalletServiceClient
	hmacSecret             string
}

// NewGatewayHandler creates a new instance of GatewayHandler.
func NewGatewayHandler(auth, payment, ledger, wallet, billing, notification string, rdb *redis.Client, authClient pb.AuthServiceClient, walletClient walletpb.WalletServiceClient, hmacSecret string) *GatewayHandler {
	return &GatewayHandler{
		authServiceURL:         auth,
		paymentServiceURL:      payment,
		ledgerServiceURL:       ledger,
		walletServiceURL:       wallet,
		billingServiceURL:      billing,
		notificationServiceURL: notification,
		rdb:                    rdb,
		upgrader: websocket.Upgrader{
			CheckOrigin: func(r *http.Request) bool { return true },
		},
		authClient:   authClient,
		walletClient: walletClient,
		hmacSecret:   hmacSecret,
	}
}

// validateKeyWithAuthService calls the Auth service to validate the API key hash.
func (h *GatewayHandler) validateKeyWithAuthService(ctx context.Context, keyHash string) (string, string, string, string, string, int32, string, string, bool) {
	res, err := h.authClient.ValidateKey(ctx, &pb.ValidateKeyRequest{KeyHash: keyHash})
	if err != nil {
		log.Printf("Auth service gRPC validation call failed: %v", err)
		return "", "", "", "", "", 0, "", "", false
	}

	return res.UserId, res.Environment, res.Scopes, res.OrgId, res.Role, res.RateLimitQuota, res.ZoneId, res.Mode, res.Valid
}

// checkRateLimit checks if the key has exceeded its quota.
func (h *GatewayHandler) checkRateLimit(ctx context.Context, keyHash string, quota int32) (bool, error) {
	if quota <= 0 {
		quota = 100 // Default fallback
	}
	window := time.Now().Unix() / 60
	redisKey := fmt.Sprintf("rate_limit:%s:%d", keyHash, window)

	count, err := h.rdb.Incr(ctx, redisKey).Result()
	if err != nil {
		return false, err
	}

	if count == 1 {
		h.rdb.Expire(ctx, redisKey, 60*time.Second)
	}

	return count <= int64(quota), nil
}

// proxyRequest creates a reverse proxy to the target URL and serves the request.
func (h *GatewayHandler) proxyRequest(target string, w http.ResponseWriter, r *http.Request) {
	targetURL, err := url.Parse(target)
	if err != nil {
		log.Printf("Error parsing target URL %s: %v", target, err)
		jsonutil.WriteErrorJSON(w, "Internal Server Error; Invalid Target")
		return
	}

	proxy := httputil.NewSingleHostReverseProxy(targetURL)
	originalDirector := proxy.Director
	proxy.Director = func(req *http.Request) {
		originalDirector(req)
		req.Host = targetURL.Host
		// Ensure headers injected by middleware persist
		if userID := r.Header.Get("X-User-ID"); userID != "" {
			req.Header.Set("X-User-ID", userID)
		}
		if env := r.Header.Get("X-Environment"); env != "" {
			req.Header.Set("X-Environment", env)
		}
		if orgID := r.Header.Get("X-Org-ID"); orgID != "" {
			req.Header.Set("X-Org-ID", orgID)
		}
		if zoneID := r.Header.Get("X-Zone-ID"); zoneID != "" {
			req.Header.Set("X-Zone-ID", zoneID)
		}
		if mode := r.Header.Get("X-Zone-Mode"); mode != "" {
			req.Header.Set("X-Zone-Mode", mode)
		}
	}

	proxy.ServeHTTP(w, r)
}

// ServeHTTP implements the http.Handler interface with Middleware.
func (h *GatewayHandler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	path := r.URL.Path
	log.Printf("Gateway: Incoming %s %s", r.Method, path)

	// Public Endpoints / Auth Management (JWT based or public)
	if path == "/ws" {
		h.handleWebSocket(w, r)
		return
	}

	if strings.HasPrefix(path, "/auth") || path == "/health" {
		log.Printf("Gateway: Routing public path %s", path)
		h.routePublic(w, r)
		return
	}

	// Protected Endpoints (API Key Required)
	// Extract Secret Key
	authHeader := r.Header.Get("Authorization")
	if !strings.HasPrefix(authHeader, "Bearer sk_") {
		jsonutil.WriteJSON(w, http.StatusUnauthorized, map[string]string{"error": "Missing or invalid API Key"})
		return
	}
	apiKey := strings.TrimPrefix(authHeader, "Bearer ")

	// Hash Key
	keyHash := apikey.HashKey(apiKey, h.hmacSecret)

	// Validate with Auth Service
	userID, env, keyScopes, orgID, role, quota, zoneID, mode, valid := h.validateKeyWithAuthService(r.Context(), keyHash)
	if !valid {
		jsonutil.WriteJSON(w, http.StatusUnauthorized, map[string]string{"error": "Invalid or revoked API Key"})
		return
	}

	// Scope Enforcement
	requiredScope := scopes.GetRequiredScope(path, r.Method)
	if requiredScope != "" && !scopes.HasScope(keyScopes, requiredScope) {
		jsonutil.WriteJSON(w, http.StatusForbidden, map[string]string{
			"error":          "Insufficient scope",
			"required_scope": requiredScope,
		})
		return
	}

	// Rate Limiting
	allowed, err := h.checkRateLimit(r.Context(), keyHash, quota)
	if err != nil {
		log.Printf("Redis error: %v", err)
		// Fail open or closed? Closed for security.
		jsonutil.WriteErrorJSON(w, "Internal Server Error")
		return
	}
	if !allowed {
		w.Header().Set("Retry-After", "60")
		jsonutil.WriteJSON(w, http.StatusTooManyRequests, map[string]string{"error": "Rate limit exceeded"})
		return
	}

	// Inject Context
	r.Header.Set("X-User-ID", userID)
	r.Header.Set("X-Environment", env)
	r.Header.Set("X-Org-ID", orgID)
	r.Header.Set("X-Role", role)
	r.Header.Set("X-Zone-ID", zoneID)
	r.Header.Set("X-Zone-Mode", mode)

	// Route to Service
	switch {
	case strings.HasPrefix(path, "/payments"):
		http.StripPrefix("/payments", http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			h.proxyRequest(h.paymentServiceURL, w, r)
		})).ServeHTTP(w, r)

	case strings.HasPrefix(path, "/ledger"):
		http.StripPrefix("/ledger", http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			h.proxyRequest(h.ledgerServiceURL, w, r)
		})).ServeHTTP(w, r)

	case strings.HasPrefix(path, "/wallets"):
		h.proxyRequest(h.walletServiceURL, w, r)

	case strings.HasPrefix(path, "/billing"):
		http.StripPrefix("/billing", http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			h.proxyRequest(h.billingServiceURL, w, r)
		})).ServeHTTP(w, r)

	case strings.HasPrefix(path, "/webhooks") || strings.HasPrefix(path, "/notifications"):
		h.proxyRequest(h.notificationServiceURL, w, r)

	default:
		jsonutil.WriteErrorJSON(w, "Not Found")
	}
}

func (h *GatewayHandler) handleWebSocket(w http.ResponseWriter, r *http.Request) {
	conn, err := h.upgrader.Upgrade(w, r, nil)
	if err != nil {
		log.Printf("WS upgrade failed: %v", err)
		return
	}
	defer func() {
		if err := conn.Close(); err != nil {
			log.Printf("Failed to close WS connection: %v", err)
		}
	}()

	pubsub := h.rdb.Subscribe(r.Context(), "webhook_events")
	defer func() {
		if err := pubsub.Close(); err != nil {
			log.Printf("Failed to close Redis PubSub: %v", err)
		}
	}()

	ch := pubsub.Channel()
	for msg := range ch {
		if err := conn.WriteMessage(websocket.TextMessage, []byte(msg.Payload)); err != nil {
			log.Printf("WS write failed: %v", err)
			break
		}
	}
}

func (h *GatewayHandler) routePublic(w http.ResponseWriter, r *http.Request) {
	path := r.URL.Path
	if strings.HasPrefix(path, "/auth") {
		http.StripPrefix("/auth", http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			h.proxyRequest(h.authServiceURL, w, r)
		})).ServeHTTP(w, r)
		return
	}
	// Health
	jsonutil.WriteJSON(w, http.StatusOK, map[string]string{
		"status":  "active",
		"service": "gateway",
		"date":    time.Now().Format(time.DateTime),
	})
}

func main() {
	// configuration
	redisAddr := os.Getenv("REDIS_ADDR")
	if redisAddr == "" {
		redisAddr = "localhost:6379"
	}

	authURL := os.Getenv("AUTH_SERVICE_URL")
	if authURL == "" {
		authURL = "http://127.0.0.1:8081"
	}

	paymentURL := os.Getenv("PAYMENT_SERVICE_URL")
	if paymentURL == "" {
		paymentURL = "http://127.0.0.1:8082"
	}

	ledgerURL := os.Getenv("LEDGER_SERVICE_URL")
	if ledgerURL == "" {
		ledgerURL = "http://127.0.0.1:8083"
	}

	walletURL := os.Getenv("WALLET_SERVICE_URL")
	if walletURL == "" {
		walletURL = "http://127.0.0.1:8085"
	}

	notificationURL := os.Getenv("NOTIFICATION_SERVICE_URL")
	if notificationURL == "" {
		notificationURL = "http://127.0.0.1:8084"
	}

	billingURL := os.Getenv("BILLING_SERVICE_URL")
	if billingURL == "" {
		billingURL = "http://127.0.0.1:8089" // Assuming a port for billing REST if added, or proxying gRPC?
		// Note: Billing seems gRPC only currently, but Gateway proxies HTTP.
		// For now, I'll point it to where billing might listen or keep it for future REST expansion.
		// If Billing has no REST handlers, this will 404/502 which is expected for now.
	}

	rdb := redis.NewClient(&redis.Options{
		Addr: redisAddr,
	})
	// Ping Redis
	if err := rdb.Ping(context.Background()).Err(); err != nil {
		log.Printf("Warning: Redis connection failed: %v", err)
	} else {
		log.Println("Redis connection established")
	}

	// Setup Auth Service gRPC Client
	authGRPCAddr := os.Getenv("AUTH_GRPC_ADDR")
	if authGRPCAddr == "" {
		authGRPCAddr = "localhost:50051"
	}
	conn, err := grpc.NewClient(authGRPCAddr,
		grpc.WithTransportCredentials(insecure.NewCredentials()),
		grpc.WithUnaryInterceptor(monitoring.UnaryClientInterceptor("gateway")),
	)
	if err != nil {
		log.Fatalf("did not connect to auth gRPC: %v", err)
	}
	defer func() {
		if err := conn.Close(); err != nil {
			log.Printf("Failed to close gRPC connection: %v", err)
		}
	}()
	authClient := pb.NewAuthServiceClient(conn)

	// Setup Wallet Service gRPC Client
	walletGRPCAddr := os.Getenv("WALLET_GRPC_ADDR")
	if walletGRPCAddr == "" {
		walletGRPCAddr = "localhost:50053"
	}
	connWallet, err := grpc.NewClient(walletGRPCAddr,
		grpc.WithTransportCredentials(insecure.NewCredentials()),
		grpc.WithUnaryInterceptor(monitoring.UnaryClientInterceptor("gateway")),
	)
	if err != nil {
		log.Fatalf("did not connect to wallet gRPC: %v", err)
	}
	defer func() { _ = connWallet.Close() }()
	walletClient := walletpb.NewWalletServiceClient(connWallet)

	// Initialize Tracer
	shutdown, err := observability.InitTracer(context.Background(), observability.Config{
		ServiceName:    "gateway",
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
	monitoring.StartMetricsServer(":8087")

	// HMAC Secret
	hmacSecret := os.Getenv("API_KEY_HMAC_SECRET")
	if hmacSecret == "" {
		hmacSecret = "local-dev-secret-do-not-use-in-prod"
		log.Println("Warning: API_KEY_HMAC_SECRET not set, using default for dev")
	}

	gateway := NewGatewayHandler(authURL, paymentURL, ledgerURL, walletURL, billingURL, notificationURL, rdb, authClient, walletClient, hmacSecret)

	// Wrap handler with OpenTelemetry and Prometheus
	otelHandler := otelhttp.NewHandler(gateway, "gateway-request")
	promHandler := monitoring.PrometheusMiddleware(otelHandler)

	server := &http.Server{
		Addr:    ":8080",
		Handler: promHandler,
	}

	log.Println("Gateway service starting on :8080")
	if err := server.ListenAndServe(); err != nil {
		log.Fatalf("Gateway server failed: %v", err)
	}
}
