package main

import (
	"os"
	"strings"

	"github.com/sapliy/sapliy-core/pkg/observability"
)

type Config struct {
	Port                   string
	RedisAddr              string
	AuthServiceURL         string
	PaymentServiceURL      string
	LedgerServiceURL       string
	WalletServiceURL       string
	NotificationServiceURL string
	EventsServiceURL       string
	FlowServiceURL         string
	BillingServiceURL      string
	AuthGRPCAddr           string
	WalletGRPCAddr         string
	HMACSecret             string
	CORSOrigins            string
	AllowedOrigins         map[string]bool
	OTEL_Endpoint          string
	Environment            string
}

func LoadConfig(logger *observability.Logger) *Config {
	redisAddr := getEnv("REDIS_ADDR", "localhost:6379")
	authURL := getEnv("AUTH_SERVICE_URL", "http://127.0.0.1:8081")
	paymentURL := getEnv("PAYMENT_SERVICE_URL", "http://127.0.0.1:8082")
	ledgerURL := getEnv("LEDGER_SERVICE_URL", "http://127.0.0.1:8083")
	walletURL := getEnv("WALLET_SERVICE_URL", "http://127.0.0.1:8085")
	notificationURL := getEnv("NOTIFICATION_SERVICE_URL", "http://127.0.0.1:8084")
	eventsURL := getEnv("EVENTS_SERVICE_URL", "http://127.0.0.1:8089")
	flowURL := getEnv("FLOW_SERVICE_URL", "http://127.0.0.1:8088")
	billingURL := getEnv("BILLING_SERVICE_URL", "http://127.0.0.1:8090")

	authGRPCAddr := getEnv("AUTH_GRPC_ADDR", "localhost:50051")
	walletGRPCAddr := getEnv("WALLET_GRPC_ADDR", "localhost:50053")

	hmacSecret := os.Getenv("API_KEY_HMAC_SECRET")
	goEnv := getEnv("GO_ENV", "development")
	if hmacSecret == "" {
		if goEnv == "production" {
			logger.Error("FATAL: API_KEY_HMAC_SECRET must be set in production")
			os.Exit(1)
		}
		hmacSecret = "local-dev-secret-do-not-use-in-prod"
		logger.Warn("API_KEY_HMAC_SECRET not set, using default for dev")
	}

	corsOrigins := os.Getenv("CORS_ALLOWED_ORIGINS")
	if corsOrigins == "" {
		corsOrigins = "http://localhost:3000"
		logger.Info("CORS_ALLOWED_ORIGINS not set, defaulting to localhost:3000")
	}

	originsList := strings.Split(corsOrigins, ",")
	allowedOrigins := make(map[string]bool, len(originsList))
	for _, o := range originsList {
		allowedOrigins[strings.TrimSpace(o)] = true
	}

	return &Config{
		Port:                   ":8080",
		RedisAddr:              redisAddr,
		AuthServiceURL:         authURL,
		PaymentServiceURL:      paymentURL,
		LedgerServiceURL:       ledgerURL,
		WalletServiceURL:       walletURL,
		NotificationServiceURL: notificationURL,
		EventsServiceURL:       eventsURL,
		FlowServiceURL:         flowURL,
		BillingServiceURL:      billingURL,
		AuthGRPCAddr:           authGRPCAddr,
		WalletGRPCAddr:         walletGRPCAddr,
		HMACSecret:             hmacSecret,
		CORSOrigins:            corsOrigins,
		AllowedOrigins:         allowedOrigins,
		OTEL_Endpoint:          os.Getenv("OTEL_EXPORTER_OTLP_ENDPOINT"),
		Environment:            goEnv,
	}
}

func getEnv(key, fallback string) string {
	if value, ok := os.LookupEnv(key); ok {
		return value
	}
	return fallback
}
