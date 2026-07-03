package api

import (
	"encoding/json"
	"fmt"
	"net/http"
	"strings"

	"github.com/sapliy/sapliy-core/internal/ledger/domain"
	"github.com/sapliy/sapliy-core/pkg/apierror"
	"github.com/sapliy/sapliy-core/pkg/jsonutil"
)

type LedgerHandler struct {
	service *domain.LedgerService
}

func NewLedgerHandler(service *domain.LedgerService) *LedgerHandler {
	return &LedgerHandler{service: service}
}

func (h *LedgerHandler) CreateAccount(w http.ResponseWriter, r *http.Request) {
	var req struct {
		Name     string             `json:"name"`
		Type     domain.AccountType `json:"type"`
		Currency string             `json:"currency"`
		UserID   *string            `json:"user_id"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		apierror.BadRequest("Invalid request body").Write(w)
		return
	}

	if req.Name == "" || req.Type == "" {
		apierror.BadRequest("Name and Type are required").Write(w)
		return
	}

	if req.Currency == "" {
		req.Currency = "USD"
	}

	acc, err := h.service.CreateAccount(r.Context(), req.Name, req.Type, req.Currency, req.UserID, r.Header.Get("X-Zone-ID"), r.Header.Get("X-Zone-Mode"))
	if err != nil {
		apierror.Internal("Failed to create account").Write(w)
		return
	}

	jsonutil.WriteJSON(w, http.StatusCreated, acc)
}

func (h *LedgerHandler) GetAccount(w http.ResponseWriter, r *http.Request) {
	id := jsonutil.GetIDFromPath(r, "/v1/ledger/accounts/")
	if id == "" {
		apierror.BadRequest("Missing Account ID").Write(w)
		return
	}

	acc, err := h.service.GetAccount(r.Context(), id)
	if err != nil {
		apierror.Internal("Error retrieving account").Write(w)
		return
	}
	if acc == nil {
		apierror.NotFound("Account not found").Write(w)
		return
	}

	jsonutil.WriteJSON(w, http.StatusOK, acc)
}

func (h *LedgerHandler) RecordTransaction(w http.ResponseWriter, r *http.Request) {
	var req domain.TransactionRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		apierror.BadRequest("Invalid request body").Write(w)
		return
	}

	if req.ReferenceID == "" || len(req.Entries) < 2 {
		apierror.BadRequest("ReferenceID and at least 2 entries are required").Write(w)
		return
	}

	if err := h.service.RecordTransaction(r.Context(), req, r.Header.Get("X-Zone-ID"), r.Header.Get("X-Zone-Mode")); err != nil {
		if strings.Contains(err.Error(), "balanced") {
			apierror.BadRequest(err.Error()).Write(w)
		} else {
			apierror.Internal("Failed to record transaction").Write(w)
		}
		return
	}

	jsonutil.WriteJSON(w, http.StatusCreated, map[string]string{"status": "recorded"})
}

func (h *LedgerHandler) GetTransaction(w http.ResponseWriter, r *http.Request) {
	id := jsonutil.GetIDFromPath(r, "/v1/ledger/transactions/")
	tx, err := h.service.GetTransaction(r.Context(), id)
	if err != nil || tx == nil {
		apierror.NotFound("Transaction not found").Write(w)
		return
	}
	jsonutil.WriteJSON(w, http.StatusOK, tx)
}

func (h *LedgerHandler) BulkRecordTransactions(w http.ResponseWriter, r *http.Request) {
	var reqs []domain.TransactionRequest
	if err := json.NewDecoder(r.Body).Decode(&reqs); err != nil {
		apierror.BadRequest("Invalid request body").Write(w)
		return
	}

	zoneID := r.Header.Get("X-Zone-ID")
	mode := r.Header.Get("X-Zone-Mode")

	errs, err := h.service.BulkRecordTransactions(r.Context(), reqs, zoneID, mode)
	if err != nil {
		apierror.Internal("Internal server error: " + err.Error()).Write(w)
		return
	}

	results := make([]map[string]string, len(errs))
	for i, e := range errs {
		if e != nil {
			results[i] = map[string]string{"status": "error", "message": e.Error()}
		} else {
			results[i] = map[string]string{"status": "recorded"}
		}
	}

	jsonutil.WriteJSON(w, http.StatusMultiStatus, results)
}

func (h *LedgerHandler) ListTransactions(w http.ResponseWriter, r *http.Request) {
	zoneID := r.URL.Query().Get("zone")
	limitStr := r.URL.Query().Get("limit")
	limit := 50
	if limitStr != "" {
		fmt.Sscanf(limitStr, "%d", &limit)
	}

	txs, err := h.service.ListTransactions(r.Context(), zoneID, limit)
	if err != nil {
		apierror.Internal("Failed to list transactions").Write(w)
		return
	}

	jsonutil.WriteJSON(w, http.StatusOK, txs)
}
