package api

import (
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/sapliy/sapliy-core/internal/payment/domain"
)

func TestPaymentHandler_CreatePaymentIntent(t *testing.T) {
	tests := []struct {
		name           string
		reqBody        string
		headers        map[string]string
		mockSetup      func(*domain.MockRepository)
		expectedStatus int
		expectedBody   string
	}{
		{
			name:    "Valid Request",
			reqBody: `{"amount":1000,"currency":"USD"}`,
			headers: map[string]string{"X-User-ID": "user_123", "X-Zone-ID": "zone_123"},
			mockSetup: func(m *domain.MockRepository) {
				m.CreatePaymentIntentFunc = func(ctx context.Context, intent *domain.PaymentIntent) error {
					intent.ID = "pi_123"
					return nil
				}
			},
			expectedStatus: http.StatusCreated,
			expectedBody:   `"id":"pi_123"`,
		},
		{
			name:           "Unauthorized",
			reqBody:        `{"amount":1000,"currency":"USD"}`,
			headers:        map[string]string{},
			mockSetup:      func(m *domain.MockRepository) {},
			expectedStatus: http.StatusUnauthorized,
			expectedBody:   "Authentication required",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			mRepo := &domain.MockRepository{}
			tt.mockSetup(mRepo)
			service := domain.NewPaymentService(mRepo)
			h := &PaymentHandler{service: service}

			req := httptest.NewRequest("POST", "/payment_intents", strings.NewReader(tt.reqBody))
			for k, v := range tt.headers {
				req.Header.Set(k, v)
			}
			w := httptest.NewRecorder()

			h.CreatePaymentIntent(w, req)

			if w.Code != tt.expectedStatus {
				t.Errorf("Expected status %d, got %d", tt.expectedStatus, w.Code)
			}
			if !strings.Contains(w.Body.String(), tt.expectedBody) {
				t.Errorf("Expected body to contain '%s', got '%s'", tt.expectedBody, w.Body.String())
			}
		})
	}
}

func TestPaymentHandler_IdempotencyMiddleware(t *testing.T) {
	tests := []struct {
		name           string
		headers        map[string]string
		mockSetup      func(*domain.MockRepository)
		expectedStatus int
		expectedHit    bool
	}{
		{
			name:    "New Request - Saves Key",
			headers: map[string]string{"Idempotency-Key": "key_1", "X-User-ID": "user_1"},
			mockSetup: func(m *domain.MockRepository) {
				m.GetIdempotencyKeyFunc = func(ctx context.Context, userID, key string) (*domain.IdempotencyRecord, error) {
					return nil, nil // Not found
				}
				m.SaveIdempotencyKeyFunc = func(ctx context.Context, userID, key string, statusCode int, body string) error {
					return nil
				}
			},
			expectedStatus: http.StatusOK,
			expectedHit:    false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			mRepo := &domain.MockRepository{}
			tt.mockSetup(mRepo)
			service := domain.NewPaymentService(mRepo)
			h := &PaymentHandler{service: service}

			next := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				w.WriteHeader(http.StatusOK)
				_, _ = w.Write([]byte("ok"))
			})

			req := httptest.NewRequest("POST", "/test", nil)
			for k, v := range tt.headers {
				req.Header.Set(k, v)
			}
			w := httptest.NewRecorder()

			h.IdempotencyMiddleware(next)(w, req)

			if w.Code != tt.expectedStatus {
				t.Errorf("Expected status %d, got %d", tt.expectedStatus, w.Code)
			}
			hit := w.Header().Get("X-Idempotency-Hit") == "true"
			if hit != tt.expectedHit {
				t.Errorf("Expected X-Idempotency-Hit %v, got %v", tt.expectedHit, hit)
			}
		})
	}
}
