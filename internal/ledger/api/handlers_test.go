package api

import (
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/sapliy/sapliy-core/internal/ledger/domain"
)

func TestLedgerHandler_CreateAccount(t *testing.T) {
	tests := []struct {
		name           string
		reqBody        string
		mockSetup      func(*domain.MockRepository)
		expectedStatus int
		expectedBody   string
	}{
		{
			name:    "Valid Request",
			reqBody: `{"name":"Checking","type":"asset","currency":"USD"}`,
			mockSetup: func(m *domain.MockRepository) {
				m.CreateAccountFunc = func(ctx context.Context, acc *domain.Account) error {
					acc.ID = "acc_123"
					return nil
				}
			},
			expectedStatus: http.StatusCreated,
			expectedBody:   `"id":"acc_123"`,
		},
		{
			name:           "Missing Name",
			reqBody:        `{"type":"asset"}`,
			mockSetup:      func(m *domain.MockRepository) {},
			expectedStatus: http.StatusBadRequest,
			expectedBody:   "Name and Type are required",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			mRepo := &domain.MockRepository{}
			tt.mockSetup(mRepo)
			service := domain.NewLedgerService(mRepo, nil)
			h := &LedgerHandler{service: service}

			req := httptest.NewRequest("POST", "/accounts", strings.NewReader(tt.reqBody))
			w := httptest.NewRecorder()

			h.CreateAccount(w, req)

			if w.Code != tt.expectedStatus {
				t.Errorf("Expected status %d, got %d", tt.expectedStatus, w.Code)
			}
			if !strings.Contains(w.Body.String(), tt.expectedBody) {
				t.Errorf("Expected body to contain '%s', got '%s'", tt.expectedBody, w.Body.String())
			}
		})
	}
}

func TestLedgerHandler_RecordTransaction(t *testing.T) {
	tests := []struct {
		name           string
		reqBody        string
		expectedStatus int
		expectedBody   string
	}{
		{
			name:           "Missing ReferenceID",
			reqBody:        `{"entries":[{"account_id":"acc_1","amount":100},{"account_id":"acc_2","amount":-100}]}`,
			expectedStatus: http.StatusBadRequest,
			expectedBody:   "ReferenceID and at least 2 entries are required",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			service := domain.NewLedgerService(&domain.MockRepository{}, nil)
			h := &LedgerHandler{service: service}

			req := httptest.NewRequest("POST", "/transactions", strings.NewReader(tt.reqBody))
			w := httptest.NewRecorder()

			h.RecordTransaction(w, req)

			if w.Code != tt.expectedStatus {
				t.Errorf("Expected status %d, got %d", tt.expectedStatus, w.Code)
			}
			if !strings.Contains(w.Body.String(), tt.expectedBody) {
				t.Errorf("Expected body to contain '%s', got '%s'", tt.expectedBody, w.Body.String())
			}
		})
	}
}
