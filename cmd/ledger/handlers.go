package main

import (
	"encoding/json"
	"net/http"
	"strings"

	"github.com/marwan562/fintech-ecosystem/internal/ledger/domain"
	"github.com/marwan562/fintech-ecosystem/pkg/jsonutil"
)

type LedgerHandler struct {
	service *domain.LedgerService
}

func (h *LedgerHandler) CreateAccount(w http.ResponseWriter, r *http.Request) {
	var req struct {
		Name     string             `json:"name"`
		Type     domain.AccountType `json:"type"`
		Currency string             `json:"currency"`
		UserID   *string            `json:"user_id"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		jsonutil.WriteErrorJSON(w, "Invalid request body")
		return
	}

	if req.Name == "" || req.Type == "" {
		jsonutil.WriteErrorJSON(w, "Name and Type are required")
		return
	}

	if req.Currency == "" {
		req.Currency = "USD" // Default
	}

	acc, err := h.service.CreateAccount(r.Context(), req.Name, req.Type, req.Currency, req.UserID)
	if err != nil {
		jsonutil.WriteErrorJSON(w, "Failed to create account")
		return
	}

	jsonutil.WriteJSON(w, http.StatusCreated, acc)
}

func (h *LedgerHandler) GetAccount(w http.ResponseWriter, r *http.Request) {
	parts := strings.Split(r.URL.Path, "/")
	if len(parts) < 3 {
		jsonutil.WriteErrorJSON(w, "Invalid URL")
		return
	}
	id := parts[len(parts)-1]

	acc, err := h.service.GetAccount(r.Context(), id)
	if err != nil {
		jsonutil.WriteErrorJSON(w, "Error retrieving account")
		return
	}
	if acc == nil {
		jsonutil.WriteErrorJSON(w, "Account not found")
		return
	}

	jsonutil.WriteJSON(w, http.StatusOK, acc)
}

func (h *LedgerHandler) RecordTransaction(w http.ResponseWriter, r *http.Request) {
	var req domain.TransactionRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		jsonutil.WriteErrorJSON(w, "Invalid request body")
		return
	}

	// Basic Validation
	if req.ReferenceID == "" || len(req.Entries) < 2 {
		jsonutil.WriteErrorJSON(w, "Invalid transaction: ReferenceID required, and at least 2 entries needed")
		return
	}

	if err := h.service.RecordTransaction(r.Context(), req); err != nil {
		if strings.Contains(err.Error(), "transaction is not balanced") {
			jsonutil.WriteErrorJSON(w, err.Error()) // 400 Bad Request
		} else {
			jsonutil.WriteErrorJSON(w, "Failed to record transaction: "+err.Error())
		}
		return
	}

	jsonutil.WriteJSON(w, http.StatusCreated, map[string]string{"status": "recorded"})
}
