package main

import (
	"context"
	"database/sql"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/marwan562/fintech-ecosystem/internal/payment"
)

func TestPaymentHandler_CreatePaymentIntent(t *testing.T) {
	tests := []struct {
		name           string
		reqBody        string
		headers        map[string]string
		mockSetup      func(*payment.MockDB)
		expectedStatus int
		expectedBody   string
	}{
		{
			name:    "Valid Request",
			reqBody: `{"amount":1000,"currency":"USD"}`,
			headers: map[string]string{"X-User-ID": "user_123"},
			mockSetup: func(m *payment.MockDB) {
				m.QueryRowContextFunc = func(ctx context.Context, query string, args ...any) payment.Row {
					return &payment.MockRow{
						ScanFunc: func(dest ...any) error {
							*(dest[0].(*string)) = "pi_123"
							return nil
						},
					}
				}
			},
			expectedStatus: http.StatusCreated,
			expectedBody:   `"id":"pi_123"`,
		},
		{
			name:           "Unauthorized",
			reqBody:        `{"amount":1000,"currency":"USD"}`,
			headers:        map[string]string{},
			mockSetup:      func(m *payment.MockDB) {},
			expectedStatus: http.StatusUnauthorized,
			expectedBody:   "Authentication required",
		},
		{
			name:           "Invalid Amount",
			reqBody:        `{"amount":-50,"currency":"USD"}`,
			headers:        map[string]string{"X-User-ID": "user_123"},
			mockSetup:      func(m *payment.MockDB) {},
			expectedStatus: http.StatusBadRequest,
			expectedBody:   "Amount and Currency are required",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			mDB := &payment.MockDB{}
			tt.mockSetup(mDB)
			repo := payment.NewTestRepository(mDB)
			h := &PaymentHandler{repo: repo}

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
		mockSetup      func(*payment.MockDB)
		expectedStatus int
		expectedHit    bool
	}{
		{
			name:    "New Request - Saves Key",
			headers: map[string]string{"Idempotency-Key": "key_1", "X-User-ID": "user_1"},
			mockSetup: func(m *payment.MockDB) {
				// GetIdempotencyKey: return nil (not found)
				m.QueryRowContextFunc = func(ctx context.Context, query string, args ...any) payment.Row {
					return &payment.MockRow{
						ScanFunc: func(dest ...any) error {
							return sql.ErrNoRows
						},
					}
				}
				// SaveIdempotencyKey: success
				m.ExecContextFunc = func(ctx context.Context, query string, args ...any) (sql.Result, error) {
					return nil, nil
				}
			},
			expectedStatus: http.StatusOK,
			expectedHit:    false,
		},
		{
			name:    "Existing Request - Returns Cached",
			headers: map[string]string{"Idempotency-Key": "key_1", "X-User-ID": "user_1"},
			mockSetup: func(m *payment.MockDB) {
				// GetIdempotencyKey: return cached record
				m.QueryRowContextFunc = func(ctx context.Context, query string, args ...any) payment.Row {
					return &payment.MockRow{
						ScanFunc: func(dest ...any) error {
							// Order: user_id, key, response_body, status_code
							*(dest[0].(*string)) = "user_1"
							*(dest[1].(*string)) = "key_1"
							*(dest[2].(*string)) = `{"status":"succeeded"}`
							*(dest[3].(*int)) = http.StatusOK
							return nil
						},
					}
				}
			},
			expectedStatus: http.StatusOK,
			expectedHit:    true,
		},
		{
			name:    "Collision Prevention - Different User",
			headers: map[string]string{"Idempotency-Key": "key_1", "X-User-ID": "user_2"},
			mockSetup: func(m *payment.MockDB) {
				// GetIdempotencyKey for user_2: return nil
				m.QueryRowContextFunc = func(ctx context.Context, query string, args ...any) payment.Row {
					return &payment.MockRow{
						ScanFunc: func(dest ...any) error {
							return sql.ErrNoRows
						},
					}
				}
				m.ExecContextFunc = func(ctx context.Context, query string, args ...any) (sql.Result, error) {
					return nil, nil
				}
			},
			expectedStatus: http.StatusOK,
			expectedHit:    false,
		},
		{
			name:    "Transient Error - Not Cached",
			headers: map[string]string{"Idempotency-Key": "key_err", "X-User-ID": "user_1"},
			mockSetup: func(m *payment.MockDB) {
				// GetIdempotencyKey: return nil
				m.QueryRowContextFunc = func(ctx context.Context, query string, args ...any) payment.Row {
					return &payment.MockRow{
						ScanFunc: func(dest ...any) error {
							return sql.ErrNoRows
						},
					}
				}
				// ExecContext (Save) should NOT be called for 500
			},
			expectedStatus: http.StatusInternalServerError,
			expectedHit:    false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			mDB := &payment.MockDB{}
			tt.mockSetup(mDB)
			repo := payment.NewTestRepository(mDB)
			h := &PaymentHandler{repo: repo}

			// Dummy handler that returns 500 if the test name says so
			next := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				if strings.Contains(tt.name, "Transient Error") {
					w.WriteHeader(http.StatusInternalServerError)
					_, _ = w.Write([]byte("error"))
					return
				}
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
