package main

import (
	"encoding/json"
	"net/http"
	"strings"

	"github.com/marwan562/fintech-ecosystem/internal/ledger"
	"github.com/marwan562/fintech-ecosystem/pkg/jsonutil"
)

type LedgerHandler struct {
	repo *ledger.Repository
}

func (h *LedgerHandler) CreateAccount(w http.ResponseWriter, r *http.Request) {
	var req struct {
		Name   string             `json:"name"`
		Type   ledger.AccountType `json:"type"`
		UserID *string            `json:"user_id"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		jsonutil.WriteErrorJSON(w, "Invalid request body")
		return
	}

	if req.Name == "" || req.Type == "" {
		jsonutil.WriteErrorJSON(w, "Name and Type are required")
		return
	}

	acc, err := h.repo.CreateAccount(r.Context(), req.Name, req.Type, req.UserID)
	if err != nil {
		jsonutil.WriteErrorJSON(w, "Failed to create account")
		return
	}

	jsonutil.WriteJSON(w, http.StatusCreated, acc)
}

func (h *LedgerHandler) GetAccount(w http.ResponseWriter, r *http.Request) {
	// Extract ID from URL path: /accounts/{id}
	// Simple parsing assuming strict routing
	// Format: /accounts/UUID

	parts := strings.Split(r.URL.Path, "/")
	if len(parts) < 3 {
		jsonutil.WriteErrorJSON(w, "Invalid URL")
		return
	}
	id := parts[len(parts)-1]

	acc, err := h.repo.GetAccount(r.Context(), id)
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
	var req ledger.TransactionRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		jsonutil.WriteErrorJSON(w, "Invalid request body")
		return
	}

	// Basic Validation
	if req.ReferenceID == "" || len(req.Entries) < 2 {
		jsonutil.WriteErrorJSON(w, "Invalid transaction: ReferenceID required, and at least 2 entries needed")
		return
	}

	if err := h.repo.RecordTransaction(r.Context(), req); err != nil {
		if strings.Contains(err.Error(), "transaction is not balanced") {
			jsonutil.WriteErrorJSON(w, err.Error()) // 400 Bad Request
		} else {
			// Check for unique constraint violation on reference_id if needed, but for now generic 500
			jsonutil.WriteErrorJSON(w, "Failed to record transaction: "+err.Error())
		}
		return
	}

	jsonutil.WriteJSON(w, http.StatusCreated, map[string]string{"status": "recorded"})
}
