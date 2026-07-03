package api

import (
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/sapliy/sapliy-core/internal/auth/domain"
	"github.com/sapliy/sapliy-core/pkg/bcryptutil"
)

func TestAuthHandler_Login(t *testing.T) {
	tests := []struct {
		name           string
		reqBody        string
		mockSetup      func(*domain.MockRepository)
		expectedStatus int
		expectedBody   string
	}{
		{
			name:    "Valid Login",
			reqBody: `{"email":"test@example.com","password":"password123"}`,
			mockSetup: func(m *domain.MockRepository) {
				m.GetUserByEmailFunc = func(ctx context.Context, email string) (*domain.User, error) {
					b := &bcryptutil.BcryptUtilsImpl{}
					hash, _ := b.GenerateHash("password123")
					return &domain.User{
						ID:       "user_123",
						Email:    "test@example.com",
						Password: hash,
					}, nil
				}
			},
			expectedStatus: http.StatusOK,
			expectedBody:   "token",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			mRepo := &domain.MockRepository{}
			tt.mockSetup(mRepo)
			service := domain.NewAuthService(mRepo, nil)
			h := &AuthHandler{service: service}

			req := httptest.NewRequest("POST", "/login", strings.NewReader(tt.reqBody))
			w := httptest.NewRecorder()

			h.Login(w, req)

			if w.Code != tt.expectedStatus {
				t.Errorf("%s: Expected status %d, got %d. Body: %s", tt.name, tt.expectedStatus, w.Code, w.Body.String())
			}
			if !strings.Contains(w.Body.String(), tt.expectedBody) {
				t.Errorf("%s: Expected body to contain '%s', got '%s'", tt.name, tt.expectedBody, w.Body.String())
			}
		})
	}
}

func TestAuthHandler_Register(t *testing.T) {
	tests := []struct {
		name           string
		reqBody        string
		mockSetup      func(*domain.MockRepository)
		expectedStatus int
		expectedBody   string
	}{
		{
			name:    "Successful Registration",
			reqBody: `{"email":"new@example.com","password":"password123"}`,
			mockSetup: func(m *domain.MockRepository) {
				m.CreateUserFunc = func(ctx context.Context, email, passwordHash string) (*domain.User, error) {
					return &domain.User{
						ID:    "user_456",
						Email: "new@example.com",
					}, nil
				}
				m.CreateEmailVerificationTokenFunc = func(ctx context.Context, token *domain.EmailVerificationToken) error {
					return nil
				}
			},
			expectedStatus: http.StatusCreated,
			expectedBody:   "new@example.com",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			mRepo := &domain.MockRepository{}
			tt.mockSetup(mRepo)
			service := domain.NewAuthService(mRepo, nil)
			h := &AuthHandler{service: service}

			req := httptest.NewRequest("POST", "/register", strings.NewReader(tt.reqBody))
			w := httptest.NewRecorder()

			h.Register(w, req)

			if w.Code != tt.expectedStatus {
				t.Errorf("%s: Expected status %d, got %d", tt.name, tt.expectedStatus, w.Code)
			}
			if !strings.Contains(w.Body.String(), tt.expectedBody) {
				t.Errorf("%s: Expected body to contain '%s', got '%s'", tt.name, tt.expectedBody, w.Body.String())
			}
		})
	}
}

func TestAuthHandler_VerifyEmail(t *testing.T) {
	tests := []struct {
		name           string
		reqBody        string
		mockSetup      func(*domain.MockRepository)
		expectedStatus int
	}{
		{
			name:    "Successful Verification",
			reqBody: `{"token":"valid_token"}`,
			mockSetup: func(m *domain.MockRepository) {
				m.GetEmailVerificationTokenFunc = func(ctx context.Context, tokenHash string) (*domain.EmailVerificationToken, error) {
					return &domain.EmailVerificationToken{
						ID:        "token_123",
						UserID:    "user_123",
						Token:     tokenHash,
						ExpiresAt: time.Now().Add(1 * time.Hour),
					}, nil
				}
				m.SetEmailVerifiedFunc = func(ctx context.Context, userID string) error {
					return nil
				}
				m.MarkEmailVerificationTokenUsedFunc = func(ctx context.Context, tokenHash string) error {
					return nil
				}
			},
			expectedStatus: http.StatusOK,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			mRepo := &domain.MockRepository{}
			tt.mockSetup(mRepo)
			service := domain.NewAuthService(mRepo, nil)
			h := &AuthHandler{service: service}

			req := httptest.NewRequest("POST", "/verify-email", strings.NewReader(tt.reqBody))
			w := httptest.NewRecorder()

			h.VerifyEmail(w, req)

			if w.Code != tt.expectedStatus {
				t.Errorf("%s: Expected status %d, got %d. Body: %s", tt.name, tt.expectedStatus, w.Code, w.Body.String())
			}
		})
	}
}

func TestAuthHandler_ResendVerification(t *testing.T) {
	tests := []struct {
		name           string
		reqBody        string
		mockSetup      func(*domain.MockRepository)
		expectedStatus int
	}{
		{
			name:    "Successful Resend",
			reqBody: `{"email":"test@example.com"}`,
			mockSetup: func(m *domain.MockRepository) {
				m.GetUserByEmailFunc = func(ctx context.Context, email string) (*domain.User, error) {
					return &domain.User{
						ID:            "user_123",
						Email:         email,
						EmailVerified: false,
					}, nil
				}
				m.CreateEmailVerificationTokenFunc = func(ctx context.Context, token *domain.EmailVerificationToken) error {
					return nil
				}
				m.GetUserByIDFunc = func(ctx context.Context, id string) (*domain.User, error) {
					return &domain.User{ID: id, Email: "test@example.com"}, nil
				}
			},
			expectedStatus: http.StatusOK,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			mRepo := &domain.MockRepository{}
			tt.mockSetup(mRepo)
			service := domain.NewAuthService(mRepo, nil)
			h := &AuthHandler{service: service}

			req := httptest.NewRequest("POST", "/resend-verification", strings.NewReader(tt.reqBody))
			w := httptest.NewRecorder()

			h.ResendVerification(w, req)

			if w.Code != tt.expectedStatus {
				t.Errorf("%s: Expected status %d, got %d. Body: %s", tt.name, tt.expectedStatus, w.Code, w.Body.String())
			}
		})
	}
}
