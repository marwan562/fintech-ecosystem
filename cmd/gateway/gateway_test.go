package main

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"strings"
	"testing"

	"github.com/alicebob/miniredis/v2"
	"github.com/redis/go-redis/v9"
	"github.com/sapliy/sapliy-core/pkg/apikey"
	"github.com/sapliy/sapliy-core/pkg/observability"
	pb "github.com/sapliy/sapliy-core/proto/auth"
	walletpb "github.com/sapliy/sapliy-core/proto/wallet"
	"google.golang.org/grpc"
)

// ─── Mock gRPC Clients ───────────────────────────────────────────────────────

// mockAuthClient implements pb.AuthServiceClient with configurable ValidateKey behaviour.
type mockAuthClient struct {
	ValidateKeyFunc func(ctx context.Context, in *pb.ValidateKeyRequest, opts ...grpc.CallOption) (*pb.ValidateKeyResponse, error)
}

func (m *mockAuthClient) ValidateKey(ctx context.Context, in *pb.ValidateKeyRequest, opts ...grpc.CallOption) (*pb.ValidateKeyResponse, error) {
	return m.ValidateKeyFunc(ctx, in, opts...)
}
func (m *mockAuthClient) ValidateToken(ctx context.Context, in *pb.ValidateTokenRequest, opts ...grpc.CallOption) (*pb.ValidateTokenResponse, error) {
	return nil, nil
}
func (m *mockAuthClient) CreateSSOProvider(ctx context.Context, in *pb.CreateSSOProviderRequest, opts ...grpc.CallOption) (*pb.SSOProvider, error) {
	return nil, nil
}
func (m *mockAuthClient) GetSSOProvider(ctx context.Context, in *pb.GetSSOProviderRequest, opts ...grpc.CallOption) (*pb.SSOProvider, error) {
	return nil, nil
}
func (m *mockAuthClient) InitiateSSO(ctx context.Context, in *pb.InitiateSSORequest, opts ...grpc.CallOption) (*pb.InitiateSSOResponse, error) {
	return nil, nil
}
func (m *mockAuthClient) GetAuditLogs(ctx context.Context, in *pb.GetAuditLogsRequest, opts ...grpc.CallOption) (*pb.GetAuditLogsResponse, error) {
	return nil, nil
}
func (m *mockAuthClient) AddTeamMember(ctx context.Context, in *pb.AddTeamMemberRequest, opts ...grpc.CallOption) (*pb.Membership, error) {
	return nil, nil
}
func (m *mockAuthClient) RemoveTeamMember(ctx context.Context, in *pb.RemoveTeamMemberRequest, opts ...grpc.CallOption) (*pb.RemoveTeamMemberResponse, error) {
	return nil, nil
}
func (m *mockAuthClient) ListTeamMembers(ctx context.Context, in *pb.ListTeamMembersRequest, opts ...grpc.CallOption) (*pb.ListTeamMembersResponse, error) {
	return nil, nil
}

// mockWalletClient implements walletpb.WalletServiceClient (no-op for gateway tests).
type mockWalletClient struct{}

func (m *mockWalletClient) CreateWallet(ctx context.Context, in *walletpb.CreateWalletRequest, opts ...grpc.CallOption) (*walletpb.Wallet, error) {
	return nil, nil
}
func (m *mockWalletClient) GetWallet(ctx context.Context, in *walletpb.GetWalletRequest, opts ...grpc.CallOption) (*walletpb.Wallet, error) {
	return nil, nil
}
func (m *mockWalletClient) TopUp(ctx context.Context, in *walletpb.TopUpRequest, opts ...grpc.CallOption) (*walletpb.TransactionResponse, error) {
	return nil, nil
}
func (m *mockWalletClient) Transfer(ctx context.Context, in *walletpb.TransferRequest, opts ...grpc.CallOption) (*walletpb.TransactionResponse, error) {
	return nil, nil
}

// ─── Helpers ─────────────────────────────────────────────────────────────────

const testHMACSecret = "test-hmac-secret"

// validKeyResponse returns a fully-populated ValidateKeyResponse for a live sk_ key.
func validKeyResponse() *pb.ValidateKeyResponse {
	return &pb.ValidateKeyResponse{
		Valid:          true,
		UserId:         "user_test",
		Environment:    "sandbox",
		Scopes:         "payments:read payments:write ledger:read",
		OrgId:          "org_test",
		Role:           "admin",
		RateLimitQuota: 1000,
		ZoneId:         "zone_test",
		Mode:           "live",
		KeyType:        "secret",
	}
}

// newTestGateway creates a GatewayHandler wrapped in middlewares.
func newTestGateway(t *testing.T, authClient pb.AuthServiceClient) (http.Handler, *GatewayHandler, *miniredis.Miniredis) {
	t.Helper()
	mr := miniredis.RunT(t)
	rdb := redis.NewClient(&redis.Options{Addr: mr.Addr()})
	logger := observability.NewLogger("gateway-test")

	allowedOrigins := map[string]bool{"http://localhost:3000": true}
	cfg := &Config{
		AuthServiceURL:         "http://auth",
		PaymentServiceURL:      "http://payment",
		LedgerServiceURL:       "http://ledger",
		WalletServiceURL:       "http://wallet",
		BillingServiceURL:      "http://billing",
		EventsServiceURL:       "http://events",
		FlowServiceURL:         "http://flow",
		NotificationServiceURL: "http://notification",
		HMACSecret:             testHMACSecret,
		AllowedOrigins:         allowedOrigins,
		CORSOrigins:            "http://localhost:3000",
	}

	h := NewGatewayHandler(cfg, rdb, authClient, &mockWalletClient{}, logger)

	// Wrap with middlewares similar to main.go
	handler := CORSMiddleware(cfg.CORSOrigins, cfg.AllowedOrigins)(h)
	handler = h.AuthMiddleware(handler)

	return handler, h, mr
}

// apiErrorCode unmarshals the "code" field from an apierror JSON response.
func apiErrorCode(t *testing.T, body string) string {
	t.Helper()
	var envelope struct {
		Error struct {
			Code string `json:"code"`
		} `json:"error"`
	}
	if err := json.Unmarshal([]byte(body), &envelope); err != nil {
		t.Fatalf("failed to parse error body %q: %v", body, err)
	}
	return envelope.Error.Code
}

// ─── Tests: API Key Validation ────────────────────────────────────────────────

func TestServeHTTP_MissingAPIKey(t *testing.T) {
	handler, h, _ := newTestGateway(t, &mockAuthClient{
		ValidateKeyFunc: func(_ context.Context, _ *pb.ValidateKeyRequest, _ ...grpc.CallOption) (*pb.ValidateKeyResponse, error) {
			return &pb.ValidateKeyResponse{Valid: false}, nil
		},
	})
	_ = h

	tests := []struct {
		name        string
		authHeader  string
		wantStatus  int
		wantErrCode string
	}{
		{
			name:        "No Authorization header",
			authHeader:  "",
			wantStatus:  http.StatusUnauthorized,
			wantErrCode: "UNAUTHORIZED",
		},
		{
			name:        "Bearer token without sk_ or pk_ prefix",
			authHeader:  "Bearer invalid_key_format",
			wantStatus:  http.StatusUnauthorized,
			wantErrCode: "UNAUTHORIZED",
		},
		{
			name:        "Non-Bearer scheme",
			authHeader:  "Basic dXNlcjpwYXNz",
			wantStatus:  http.StatusUnauthorized,
			wantErrCode: "UNAUTHORIZED",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			req := httptest.NewRequest(http.MethodGet, "/v1/payments", nil)
			if tt.authHeader != "" {
				req.Header.Set("Authorization", tt.authHeader)
			}
			w := httptest.NewRecorder()
			handler.ServeHTTP(w, req)

			if w.Code != tt.wantStatus {
				t.Errorf("status: got %d, want %d", w.Code, tt.wantStatus)
			}
			if code := apiErrorCode(t, w.Body.String()); code != tt.wantErrCode {
				t.Errorf("error code: got %q, want %q", code, tt.wantErrCode)
			}
		})
	}
}

func TestServeHTTP_InvalidAPIKey(t *testing.T) {
	handler, h, _ := newTestGateway(t, &mockAuthClient{
		ValidateKeyFunc: func(_ context.Context, _ *pb.ValidateKeyRequest, _ ...grpc.CallOption) (*pb.ValidateKeyResponse, error) {
			return &pb.ValidateKeyResponse{Valid: false}, nil
		},
	})
	_ = h

	req := httptest.NewRequest(http.MethodGet, "/v1/payments", nil)
	req.Header.Set("Authorization", "Bearer sk_invalid_key_that_fails_validation")
	w := httptest.NewRecorder()
	handler.ServeHTTP(w, req)

	if w.Code != http.StatusUnauthorized {
		t.Errorf("status: got %d, want %d", w.Code, http.StatusUnauthorized)
	}
	if code := apiErrorCode(t, w.Body.String()); code != "UNAUTHORIZED" {
		t.Errorf("error code: got %q, want %q", code, "UNAUTHORIZED")
	}
}

// ─── Tests: Public Routes ─────────────────────────────────────────────────────

func TestServeHTTP_PublicRoutes(t *testing.T) {
	// Public routes should never call the auth gRPC client.
	authCalled := false
	handler, h, _ := newTestGateway(t, &mockAuthClient{
		ValidateKeyFunc: func(_ context.Context, _ *pb.ValidateKeyRequest, _ ...grpc.CallOption) (*pb.ValidateKeyResponse, error) {
			authCalled = true
			return &pb.ValidateKeyResponse{Valid: false}, nil
		},
	})

	// We need a real upstream to proxy to, so spin up a test server.
	upstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))
	defer upstream.Close()

	// Override auth service URL to point at our test server.
	h.authServiceURL = upstream.URL

	publicPaths := []string{"/auth/login", "/auth/register", "/health"}
	for _, path := range publicPaths {
		t.Run(path, func(t *testing.T) {
			authCalled = false
			req := httptest.NewRequest(http.MethodGet, path, nil)
			w := httptest.NewRecorder()
			handler.ServeHTTP(w, req)

			if authCalled {
				t.Errorf("auth gRPC client should not be called for public path %s", path)
			}
			// Public routes proxy to auth service — expect 200 from our stub.
			if w.Code != http.StatusOK {
				t.Errorf("status: got %d, want %d", w.Code, http.StatusOK)
			}
		})
	}
}

// ─── Tests: Scope Enforcement ─────────────────────────────────────────────────

func TestServeHTTP_ScopeEnforcement(t *testing.T) {
	// Auth returns a key with only ledger:read scope.
	handler, h, _ := newTestGateway(t, &mockAuthClient{
		ValidateKeyFunc: func(_ context.Context, _ *pb.ValidateKeyRequest, _ ...grpc.CallOption) (*pb.ValidateKeyResponse, error) {
			resp := validKeyResponse()
			resp.Scopes = "ledger:read" // no payments scope
			return resp, nil
		},
	})

	// Spin up a dummy upstream so the proxy doesn't fail on connection.
	upstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))
	defer upstream.Close()
	h.paymentServiceURL = upstream.URL

	apiKeyStr, _, _ := apikey.GenerateKey("sk", testHMACSecret)

	req := httptest.NewRequest(http.MethodPost, "/v1/payments", nil)
	req.Header.Set("Authorization", "Bearer "+apiKeyStr)
	w := httptest.NewRecorder()
	handler.ServeHTTP(w, req)

	if w.Code != http.StatusForbidden {
		t.Errorf("status: got %d, want %d (scope enforcement)", w.Code, http.StatusForbidden)
	}
	if code := apiErrorCode(t, w.Body.String()); code != "FORBIDDEN" {
		t.Errorf("error code: got %q, want %q", code, "FORBIDDEN")
	}
	if !strings.Contains(w.Body.String(), "required_scope") {
		t.Errorf("response should include required_scope in details, got: %s", w.Body.String())
	}
}

// ─── Tests: Publishable Key Restriction ──────────────────────────────────────

func TestServeHTTP_PublishableKeyRestriction(t *testing.T) {
	handler, h, _ := newTestGateway(t, &mockAuthClient{
		ValidateKeyFunc: func(_ context.Context, _ *pb.ValidateKeyRequest, _ ...grpc.CallOption) (*pb.ValidateKeyResponse, error) {
			resp := validKeyResponse()
			resp.KeyType = "publishable"
			return resp, nil
		},
	})
	_ = h

	apiKeyStr, _, _ := apikey.GenerateKey("pk", testHMACSecret)

	req := httptest.NewRequest(http.MethodGet, "/v1/payments", nil)
	req.Header.Set("Authorization", "Bearer "+apiKeyStr)
	w := httptest.NewRecorder()
	handler.ServeHTTP(w, req)

	if w.Code != http.StatusForbidden {
		t.Errorf("status: got %d, want %d", w.Code, http.StatusForbidden)
	}
	if code := apiErrorCode(t, w.Body.String()); code != "FORBIDDEN" {
		t.Errorf("error code: got %q, want %q", code, "FORBIDDEN")
	}
}

// ─── Tests: Rate Limiting ─────────────────────────────────────────────────────

func TestServeHTTP_RateLimiting(t *testing.T) {
	const quota = int32(3)

	handler, h, mr := newTestGateway(t, &mockAuthClient{
		ValidateKeyFunc: func(_ context.Context, _ *pb.ValidateKeyRequest, _ ...grpc.CallOption) (*pb.ValidateKeyResponse, error) {
			resp := validKeyResponse()
			resp.RateLimitQuota = quota
			return resp, nil
		},
	})

	// Upstream for successful requests.
	upstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))
	defer upstream.Close()
	h.paymentServiceURL = upstream.URL

	apiKeyStr, _, _ := apikey.GenerateKey("sk", testHMACSecret)
	_ = mr // miniredis is already wired in

	makeRequest := func() int {
		req := httptest.NewRequest(http.MethodGet, "/v1/payments", nil)
		req.Header.Set("Authorization", "Bearer "+apiKeyStr)
		w := httptest.NewRecorder()
		handler.ServeHTTP(w, req)
		return w.Code
	}

	// First `quota` requests should succeed.
	for i := int32(0); i < quota; i++ {
		if code := makeRequest(); code != http.StatusOK {
			t.Errorf("request %d: got %d, want %d", i+1, code, http.StatusOK)
		}
	}

	// The (quota+1)th request should be rate-limited.
	if code := makeRequest(); code != http.StatusTooManyRequests {
		t.Errorf("rate-limited request: got %d, want %d", code, http.StatusTooManyRequests)
	}
	w := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/v1/payments", nil)
	req.Header.Set("Authorization", "Bearer "+apiKeyStr)
	handler.ServeHTTP(w, req)
	if w.Header().Get("Retry-After") == "" {
		t.Error("rate-limited response should include Retry-After header")
	}
	if code := apiErrorCode(t, w.Body.String()); code != "RATE_LIMIT_EXCEEDED" {
		t.Errorf("error code: got %q, want %q", code, "RATE_LIMIT_EXCEEDED")
	}
}

// ─── Tests: Event Emit ────────────────────────────────────────────────────────

func TestHandleEventEmit(t *testing.T) {
	tests := []struct {
		name        string
		body        string
		zoneID      string
		wantStatus  int
		wantErrCode string
	}{
		{
			name:       "Valid event emit",
			body:       `{"type":"payment.created","data":{"amount":100}}`,
			zoneID:     "zone_test",
			wantStatus: http.StatusAccepted,
		},
		{
			name:        "Missing zone ID",
			body:        `{"type":"payment.created"}`,
			zoneID:      "",
			wantStatus:  http.StatusBadRequest,
			wantErrCode: "BAD_REQUEST",
		},
		{
			name:        "Missing event type",
			body:        `{"data":{"amount":100}}`,
			zoneID:      "zone_test",
			wantStatus:  http.StatusBadRequest,
			wantErrCode: "BAD_REQUEST",
		},
		{
			name:        "Invalid JSON body",
			body:        `not-json`,
			zoneID:      "zone_test",
			wantStatus:  http.StatusBadRequest,
			wantErrCode: "BAD_REQUEST",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, h, _ := newTestGateway(t, &mockAuthClient{
				ValidateKeyFunc: func(_ context.Context, _ *pb.ValidateKeyRequest, _ ...grpc.CallOption) (*pb.ValidateKeyResponse, error) {
					return validKeyResponse(), nil
				},
			})

			req := httptest.NewRequest(http.MethodPost, "/v1/events/emit", strings.NewReader(tt.body))
			if tt.zoneID != "" {
				req.Header.Set("X-Zone-ID", tt.zoneID)
			}
			req.Header.Set("X-Org-ID", "org_test")
			w := httptest.NewRecorder()

			h.handleEventEmit(w, req)

			if w.Code != tt.wantStatus {
				t.Errorf("status: got %d, want %d. body: %s", w.Code, tt.wantStatus, w.Body.String())
			}
			if tt.wantErrCode != "" {
				if code := apiErrorCode(t, w.Body.String()); code != tt.wantErrCode {
					t.Errorf("error code: got %q, want %q", code, tt.wantErrCode)
				}
			}
		})
	}
}

func TestHandleEventEmit_Idempotency(t *testing.T) {
	_, h, _ := newTestGateway(t, &mockAuthClient{
		ValidateKeyFunc: func(_ context.Context, _ *pb.ValidateKeyRequest, _ ...grpc.CallOption) (*pb.ValidateKeyResponse, error) {
			return validKeyResponse(), nil
		},
	})

	body := `{"type":"payment.created","idempotency_key":"idem_key_123","data":{"amount":100}}`

	makeEmit := func() *httptest.ResponseRecorder {
		req := httptest.NewRequest(http.MethodPost, "/v1/events/emit", strings.NewReader(body))
		req.Header.Set("X-Zone-ID", "zone_test")
		req.Header.Set("X-Org-ID", "org_test")
		w := httptest.NewRecorder()
		h.handleEventEmit(w, req)
		return w
	}

	// First call: ingested.
	w1 := makeEmit()
	if w1.Code != http.StatusAccepted {
		t.Fatalf("first emit: got %d, want %d. body: %s", w1.Code, http.StatusAccepted, w1.Body.String())
	}
	var r1 EventEmitResponse
	if err := json.Unmarshal(w1.Body.Bytes(), &r1); err != nil {
		t.Fatalf("failed to parse first response: %v", err)
	}
	if r1.Status != "ingested" {
		t.Errorf("first emit status: got %q, want %q", r1.Status, "ingested")
	}

	// Second call with same idempotency key: duplicate.
	w2 := makeEmit()
	if w2.Code != http.StatusAccepted {
		t.Fatalf("duplicate emit: got %d, want %d. body: %s", w2.Code, http.StatusAccepted, w2.Body.String())
	}
	var r2 EventEmitResponse
	if err := json.Unmarshal(w2.Body.Bytes(), &r2); err != nil {
		t.Fatalf("failed to parse duplicate response: %v", err)
	}
	if r2.Status != "duplicate" {
		t.Errorf("duplicate emit status: got %q, want %q", r2.Status, "duplicate")
	}
	if r2.EventID != r1.EventID {
		t.Errorf("duplicate should return same event ID: got %q, want %q", r2.EventID, r1.EventID)
	}
}

// ─── Tests: CORS Middleware ───────────────────────────────────────────────────

func TestCORSMiddleware(t *testing.T) {
	next := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	})

	tests := []struct {
		name           string
		allowedOrigins string
		requestOrigin  string
		method         string
		wantStatus     int
		wantCORSHeader string // expected Access-Control-Allow-Origin value, "" means absent
	}{
		{
			name:           "Allowed origin",
			allowedOrigins: "http://localhost:3000",
			requestOrigin:  "http://localhost:3000",
			method:         http.MethodGet,
			wantStatus:     http.StatusOK,
			wantCORSHeader: "http://localhost:3000",
		},
		{
			name:           "Disallowed origin",
			allowedOrigins: "http://localhost:3000",
			requestOrigin:  "http://evil.com",
			method:         http.MethodGet,
			wantStatus:     http.StatusOK,
			wantCORSHeader: "", // no CORS header for unknown origin
		},
		{
			name:           "Preflight from disallowed origin returns 403",
			allowedOrigins: "http://localhost:3000",
			requestOrigin:  "http://evil.com",
			method:         http.MethodOptions,
			wantStatus:     http.StatusForbidden,
			wantCORSHeader: "",
		},
		{
			name:           "Preflight from allowed origin returns 204",
			allowedOrigins: "http://localhost:3000",
			requestOrigin:  "http://localhost:3000",
			method:         http.MethodOptions,
			wantStatus:     http.StatusNoContent,
			wantCORSHeader: "http://localhost:3000",
		},
		{
			name:           "Wildcard allows any origin",
			allowedOrigins: "*",
			requestOrigin:  "http://any.com",
			method:         http.MethodGet,
			wantStatus:     http.StatusOK,
			wantCORSHeader: "*",
		},
		{
			name:           "No origin header (same-origin)",
			allowedOrigins: "http://localhost:3000",
			requestOrigin:  "",
			method:         http.MethodGet,
			wantStatus:     http.StatusOK,
			wantCORSHeader: "", // no CORS header needed for same-origin
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			allowedOriginsMap := make(map[string]bool)
			for _, o := range strings.Split(tt.allowedOrigins, ",") {
				allowedOriginsMap[strings.TrimSpace(o)] = true
			}
			handler := CORSMiddleware(tt.allowedOrigins, allowedOriginsMap)(next)
			req := httptest.NewRequest(tt.method, "/v1/test", nil)
			if tt.requestOrigin != "" {
				req.Header.Set("Origin", tt.requestOrigin)
			}
			w := httptest.NewRecorder()
			handler.ServeHTTP(w, req)

			if w.Code != tt.wantStatus {
				t.Errorf("status: got %d, want %d", w.Code, tt.wantStatus)
			}
			gotCORS := w.Header().Get("Access-Control-Allow-Origin")
			if gotCORS != tt.wantCORSHeader {
				t.Errorf("CORS header: got %q, want %q", gotCORS, tt.wantCORSHeader)
			}
		})
	}
}

// ─── Tests: Route Not Found ───────────────────────────────────────────────────

func TestServeHTTP_RouteNotFound(t *testing.T) {
	handler, h, _ := newTestGateway(t, &mockAuthClient{
		ValidateKeyFunc: func(_ context.Context, _ *pb.ValidateKeyRequest, _ ...grpc.CallOption) (*pb.ValidateKeyResponse, error) {
			return validKeyResponse(), nil
		},
	})
	_ = h

	apiKeyStr, _, _ := apikey.GenerateKey("sk", testHMACSecret)

	req := httptest.NewRequest(http.MethodGet, "/v1/nonexistent-route", nil)
	req.Header.Set("Authorization", "Bearer "+apiKeyStr)
	w := httptest.NewRecorder()
	handler.ServeHTTP(w, req)

	if w.Code != http.StatusNotFound {
		t.Errorf("status: got %d, want %d", w.Code, http.StatusNotFound)
	}
	if code := apiErrorCode(t, w.Body.String()); code != "NOT_FOUND" {
		t.Errorf("error code: got %q, want %q", code, "NOT_FOUND")
	}
}

// ─── Tests: apierror package ──────────────────────────────────────────────────

func TestAPIKeyHashKey(t *testing.T) {
	// HashKey should be deterministic and non-empty.
	h1 := apikey.HashKey("sk_test_key", "secret")
	h2 := apikey.HashKey("sk_test_key", "secret")
	if h1 != h2 {
		t.Errorf("HashKey is not deterministic: %q != %q", h1, h2)
	}
	if h1 == "" {
		t.Error("HashKey returned empty string")
	}
	// Different keys should produce different hashes.
	h3 := apikey.HashKey("sk_other_key", "secret")
	if h1 == h3 {
		t.Error("HashKey collision: different keys produced same hash")
	}
}

func TestLoadConfig(t *testing.T) {
	logger := observability.NewLogger("test")

	// Set some env vars
	os.Setenv("AUTH_SERVICE_URL", "http://auth:8081")
	os.Setenv("API_KEY_HMAC_SECRET", "super-secret")
	os.Setenv("GO_ENV", "production")
	defer os.Unsetenv("AUTH_SERVICE_URL")
	defer os.Unsetenv("API_KEY_HMAC_SECRET")
	defer os.Unsetenv("GO_ENV")

	cfg := LoadConfig(logger)

	if cfg.AuthServiceURL != "http://auth:8081" {
		t.Errorf("expected auth URL http://auth:8081, got %s", cfg.AuthServiceURL)
	}
	if cfg.HMACSecret != "super-secret" {
		t.Errorf("expected secret super-secret, got %s", cfg.HMACSecret)
	}
	if cfg.Environment != "production" {
		t.Errorf("expected env production, got %s", cfg.Environment)
	}
}
