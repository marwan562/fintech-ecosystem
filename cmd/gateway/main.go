package main

import (
	"context"
	"fmt"
	"log"
	"microservices/pkg/apikey"
	"microservices/pkg/jsonutil"
	"net/http"
	"net/http/httputil"
	"net/url"
	"os"
	"strings"
	"time"

	pb "microservices/proto/auth"

	"github.com/gorilla/websocket"
	"github.com/redis/go-redis/v9"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"
)

// GatewayHandler holds the configuration for upstream service URLs and Redis.
type GatewayHandler struct {
	authServiceURL    string
	paymentServiceURL string
	ledgerServiceURL  string
	rdb               *redis.Client
	upgrader          websocket.Upgrader
	authClient        pb.AuthServiceClient
}

// NewGatewayHandler creates a new instance of GatewayHandler.
func NewGatewayHandler(auth, payment, ledger string, rdb *redis.Client, authClient pb.AuthServiceClient) *GatewayHandler {
	return &GatewayHandler{
		authServiceURL:    auth,
		paymentServiceURL: payment,
		ledgerServiceURL:  ledger,
		rdb:               rdb,
		upgrader: websocket.Upgrader{
			CheckOrigin: func(r *http.Request) bool { return true },
		},
		authClient: authClient,
	}
}

// validateKeyWithAuthService calls the Auth service to validate the API key hash.
func (h *GatewayHandler) validateKeyWithAuthService(ctx context.Context, keyHash string) (string, string, bool) {
	res, err := h.authClient.ValidateKey(ctx, &pb.ValidateKeyRequest{KeyHash: keyHash})
	if err != nil {
		log.Printf("Auth service gRPC validation call failed: %v", err)
		return "", "", false
	}

	return res.UserId, res.Environment, res.Valid
}

// checkRateLimit checks if the key has exceeded 100 req/min.
func (h *GatewayHandler) checkRateLimit(ctx context.Context, keyHash string) (bool, error) {
	// Simple fixed window for demonstration, user asked for sliding window or similar.
	// Let's use a simple counter with expiration for the minute.
	// Key: rate_limit:{key_hash}:{minute_timestamp}
	window := time.Now().Unix() / 60
	redisKey := fmt.Sprintf("rate_limit:%s:%d", keyHash, window)

	count, err := h.rdb.Incr(ctx, redisKey).Result()
	if err != nil {
		return false, err
	}

	if count == 1 {
		h.rdb.Expire(ctx, redisKey, 60*time.Second)
	}

	return count <= 100, nil
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
	keyHash := apikey.HashKey(apiKey)

	// Validate with Auth Service
	userID, env, valid := h.validateKeyWithAuthService(r.Context(), keyHash)
	if !valid {
		jsonutil.WriteJSON(w, http.StatusUnauthorized, map[string]string{"error": "Invalid or revoked API Key"})
		return
	}

	// Rate Limiting
	allowed, err := h.checkRateLimit(r.Context(), keyHash)
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
	defer conn.Close()

	pubsub := h.rdb.Subscribe(r.Context(), "webhook_events")
	defer pubsub.Close()

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
	conn, err := grpc.Dial(authGRPCAddr, grpc.WithTransportCredentials(insecure.NewCredentials()))
	if err != nil {
		log.Fatalf("did not connect to auth gRPC: %v", err)
	}
	defer conn.Close()
	authClient := pb.NewAuthServiceClient(conn)

	gateway := NewGatewayHandler(authURL, paymentURL, ledgerURL, rdb, authClient)

	server := &http.Server{
		Addr:    ":8080",
		Handler: gateway,
	}

	log.Println("Gateway service starting on :8080")
	if err := server.ListenAndServe(); err != nil {
		log.Fatalf("Gateway server failed: %v", err)
	}
}
