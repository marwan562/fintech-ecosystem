package main

import (
	"context"
	"fmt"
	"net/http"
	"time"

	"github.com/gorilla/websocket"
	"github.com/redis/go-redis/v9"
	"github.com/sapliy/sapliy-core/pkg/observability"
	pb "github.com/sapliy/sapliy-core/proto/auth"
	walletpb "github.com/sapliy/sapliy-core/proto/wallet"
)

// GatewayHandler holds the configuration for upstream service URLs and Redis.
type GatewayHandler struct {
	authServiceURL         string
	paymentServiceURL      string
	ledgerServiceURL       string
	walletServiceURL       string
	billingServiceURL      string
	eventsServiceURL       string
	flowServiceURL         string
	notificationServiceURL string
	rdb                    *redis.Client
	upgrader               websocket.Upgrader
	authClient             pb.AuthServiceClient
	walletClient           walletpb.WalletServiceClient
	hmacSecret             string
	logger                 *observability.Logger
}

// NewGatewayHandler creates a new instance of GatewayHandler.
func NewGatewayHandler(cfg *Config, rdb *redis.Client, authClient pb.AuthServiceClient, walletClient walletpb.WalletServiceClient, logger *observability.Logger) *GatewayHandler {
	return &GatewayHandler{
		authServiceURL:         cfg.AuthServiceURL,
		paymentServiceURL:      cfg.PaymentServiceURL,
		ledgerServiceURL:       cfg.LedgerServiceURL,
		walletServiceURL:       cfg.WalletServiceURL,
		billingServiceURL:      cfg.BillingServiceURL,
		eventsServiceURL:       cfg.EventsServiceURL,
		flowServiceURL:         cfg.FlowServiceURL,
		notificationServiceURL: cfg.NotificationServiceURL,
		rdb:                    rdb,
		upgrader: websocket.Upgrader{
			CheckOrigin: func(r *http.Request) bool {
				origin := r.Header.Get("Origin")
				if origin == "" {
					return true
				}
				return cfg.AllowedOrigins[origin] || cfg.AllowedOrigins["*"]
			},
		},
		authClient:   authClient,
		walletClient: walletClient,
		hmacSecret:   cfg.HMACSecret,
		logger:       logger,
	}
}

func (h *GatewayHandler) validateKeyWithAuthService(ctx context.Context, keyHash string) (string, string, string, string, string, int32, string, string, string, bool) {
	res, err := h.authClient.ValidateKey(ctx, &pb.ValidateKeyRequest{KeyHash: keyHash})
	if err != nil {
		h.logger.Error("Auth service gRPC validation call failed", "error", err)
		return "", "", "", "", "", 0, "", "", "", false
	}

	return res.UserId, res.Environment, res.Scopes, res.OrgId, res.Role, res.RateLimitQuota, res.ZoneId, res.Mode, res.KeyType, res.Valid
}

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
