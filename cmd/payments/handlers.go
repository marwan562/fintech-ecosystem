package main

import (
	"encoding/json"
	"log"
	"microservices/internal/payment"
	"microservices/pkg/bank"
	"microservices/pkg/jsonutil"
	"microservices/pkg/jwtutil"
	"net/http"
	"strings"

	pb "microservices/proto/ledger"

	"github.com/redis/go-redis/v9"
)

type PaymentHandler struct {
	repo         *payment.Repository
	bankClient   bank.Client
	rdb          *redis.Client
	ledgerClient pb.LedgerServiceClient
}

// IdempotencyMiddleware wraps a handler to ensure idempotency.
func (h *PaymentHandler) IdempotencyMiddleware(next http.HandlerFunc) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		key := r.Header.Get("Idempotency-Key")
		if key == "" {
			next(w, r)
			return
		}

		// Check if key exists
		record, err := h.repo.GetIdempotencyKey(r.Context(), key)
		if err != nil {
			log.Printf("Error checking idempotency key: %v", err)
			jsonutil.WriteErrorJSON(w, "Internal Server Error")
			return
		}
		if record != nil {
			w.Header().Set("X-Idempotency-Hit", "true")
			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(record.StatusCode)
			w.Write([]byte(record.ResponseBody))
			return
		}

		// Record response
		recorder := &jsonutil.ResponseRecorder{
			ResponseWriter: w,
			StatusCode:     http.StatusOK, // Default
		}

		next(recorder, r)

		// Save key asynchronously or synchronously?
		// Synchronously is safer for strict idempotency to ensure it's there before client retries.
		if err := h.repo.SaveIdempotencyKey(r.Context(), key, recorder.StatusCode, recorder.Body.String()); err != nil {
			log.Printf("Failed to save idempotency key: %v", err)
			// We don't fail the request if saving the key fails, but we log it.
			// Or maybe we should?
		}
	}
}

type CreateIntentRequest struct {
	Amount      int64  `json:"amount"`
	Currency    string `json:"currency"`
	Description string `json:"description"`
}

type ConfirmIntentRequest struct {
	PaymentMethodID string `json:"payment_method_id"` // e.g., "tok_visa"
}

// extractUserIDFromToken is a helper to get UserID.
// It checks X-User-ID header (injected by Gateway) first.
func extractUserIDFromToken(r *http.Request) (string, error) {
	if userID := r.Header.Get("X-User-ID"); userID != "" {
		return userID, nil
	}

	// Fallback if needed, or remove JWT logic entirely if we strictly enforce Gateway
	authHeader := r.Header.Get("Authorization")
	if authHeader == "" {
		return "", nil // No auth
	}
	// ... logic mostly redundant now but kept as fallback if direct access needed
	tokenString := strings.TrimPrefix(authHeader, "Bearer ")
	claims, err := jwtutil.ValidateToken(tokenString)
	if err != nil {
		return "", err
	}
	return claims.UserID, nil
}

func (h *PaymentHandler) CreatePaymentIntent(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		jsonutil.WriteErrorJSON(w, "Method not allowed")
		return
	}

	userID, err := extractUserIDFromToken(r)
	if err != nil {
		jsonutil.WriteJSON(w, http.StatusUnauthorized, map[string]string{"error": "Invalid token"})
		return
	}
	if userID == "" {
		// optionally enforce auth
		// jsonutil.WriteJSON(w, http.StatusUnauthorized, map[string]string{"error": "Authentication required"})
		// return
		// For verification purposes without complex auth setup in curl, we might allow anonymous or mock it?
		// No, let's enforce it to be correct.
		jsonutil.WriteJSON(w, http.StatusUnauthorized, map[string]string{"error": "Authentication required"})
		return
	}

	var req CreateIntentRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		jsonutil.WriteErrorJSON(w, "Invalid request body")
		return
	}

	if req.Amount <= 0 || req.Currency == "" {
		jsonutil.WriteErrorJSON(w, "Amount and Currency are required")
		return
	}

	intent := &payment.PaymentIntent{
		Amount:      req.Amount,
		Currency:    req.Currency,
		Status:      "requires_payment_method",
		Description: req.Description,
		UserID:      userID,
	}

	if err := h.repo.CreatePaymentIntent(r.Context(), intent); err != nil {
		jsonutil.WriteErrorJSON(w, "Failed to create payment intent")
		return
	}

	jsonutil.WriteJSON(w, http.StatusCreated, intent)
}

func (h *PaymentHandler) ConfirmPaymentIntent(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		jsonutil.WriteErrorJSON(w, "Method not allowed")
		return
	}

	// Extract ID from URL path (e.g. /payment_intents/{id}/confirm)
	// Simple parsing since we use ServeMux
	pathParts := strings.Split(r.URL.Path, "/")
	if len(pathParts) < 3 {
		jsonutil.WriteErrorJSON(w, "Invalid path")
		return
	}
	// Expected path: /payment_intents/{id}/confirm
	// parts: ["", "payment_intents", "{id}", "confirm"]
	// Wait, in main.go we will likely mount this handler at a specific path.
	// Let's assume we handle the ID extraction there or here properly.
	// If mounted at /payment_intents/, then parts are correct.
	id := pathParts[2]

	var req ConfirmIntentRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		jsonutil.WriteErrorJSON(w, "Invalid request body")
		return
	}

	if req.PaymentMethodID == "" {
		jsonutil.WriteErrorJSON(w, "payment_method_id is required")
		return
	}

	intent, err := h.repo.GetPaymentIntent(r.Context(), id)
	if err != nil || intent == nil {
		jsonutil.WriteErrorJSON(w, "Payment intent not found")
		return
	}

	if intent.Status == "succeeded" {
		jsonutil.WriteErrorJSON(w, "Payment already succeeded")
		return
	}

	// Call Mock Bank
	result, err := h.bankClient.Charge(r.Context(), intent.Amount, intent.Currency, req.PaymentMethodID)
	if err != nil || result.Status != "succeeded" {
		h.repo.UpdateStatus(r.Context(), id, "failed")
		jsonutil.WriteErrorJSON(w, "Payment failed: "+result.ErrorCode)
		return
	}

	// Update Status
	if err := h.repo.UpdateStatus(r.Context(), id, "succeeded"); err != nil {
		// Critical: In real world, we need to handle state consistency here
		jsonutil.WriteErrorJSON(w, "Failed to update payment status")
		return
	}

	// Publish Webhook Event
	event := map[string]interface{}{
		"type": "payment.succeeded",
		"data": intent,
	}
	eventBody, _ := json.Marshal(event)
	h.rdb.Publish(r.Context(), "webhook_events", eventBody)

	// Record in Ledger via gRPC
	_, err = h.ledgerClient.RecordTransaction(r.Context(), &pb.RecordTransactionRequest{
		AccountId:   "user_" + intent.UserID, // Derived account ID
		Amount:      intent.Amount,
		Currency:    intent.Currency,
		Description: "Payment for intent " + intent.ID,
		ReferenceId: intent.ID,
	})
	if err != nil {
		log.Printf("Failed to record transaction in ledger: %v", err)
		// We log it, but the payment itself succeeded in the bank and our DB.
		// In a production system, we might want to use a transactional outbox
		// or retry mechanism to ensure the ledger is eventually consistent.
	}

	intent.Status = "succeeded"
	jsonutil.WriteJSON(w, http.StatusOK, intent)
}
