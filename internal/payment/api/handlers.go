package api

import (
	"encoding/json"
	"fmt"
	"net/http"

	"github.com/prometheus/client_golang/prometheus"
	"github.com/redis/go-redis/v9"
	"github.com/sapliy/sapliy-core/internal/payment/domain"
	"github.com/sapliy/sapliy-core/internal/payment/infrastructure"
	"github.com/sapliy/sapliy-core/pkg/apierror"
	"github.com/sapliy/sapliy-core/pkg/audit"
	"github.com/sapliy/sapliy-core/pkg/authutil"
	"github.com/sapliy/sapliy-core/pkg/bank"
	"github.com/sapliy/sapliy-core/pkg/jsonutil"
	"github.com/sapliy/sapliy-core/pkg/messaging"
	pb "github.com/sapliy/sapliy-core/proto/ledger"
)

type PaymentHandler struct {
	service       *domain.PaymentService
	bankClient    bank.Client
	rdb           *redis.Client
	ledgerClient  pb.LedgerServiceClient
	kafkaProducer *messaging.KafkaProducer
	rabbitClient  *messaging.RabbitMQClient
}

func NewPaymentHandler(
	service *domain.PaymentService,
	bankClient bank.Client,
	rdb *redis.Client,
	ledgerClient pb.LedgerServiceClient,
	kafkaProducer *messaging.KafkaProducer,
	rabbitClient *messaging.RabbitMQClient,
) *PaymentHandler {
	return &PaymentHandler{
		service:       service,
		bankClient:    bankClient,
		rdb:           rdb,
		ledgerClient:  ledgerClient,
		kafkaProducer: kafkaProducer,
		rabbitClient:  rabbitClient,
	}
}

// IdempotencyMiddleware wraps a handler to ensure idempotency.
func (h *PaymentHandler) IdempotencyMiddleware(next http.HandlerFunc) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		key := r.Header.Get("Idempotency-Key")
		if key == "" {
			next(w, r)
			return
		}

		userID, err := authutil.ExtractUserID(r)
		if err != nil || userID == "" {
			apierror.Unauthorized("Authentication required for idempotent requests").Write(w)
			return
		}

		record, err := h.service.GetIdempotencyKey(r.Context(), userID, key)
		if err != nil {
			apierror.Internal("Internal Server Error").Write(w)
			return
		}
		if record != nil {
			w.Header().Set("X-Idempotency-Hit", "true")
			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(record.StatusCode)
			w.Write([]byte(record.ResponseBody))
			return
		}

		recorder := &jsonutil.ResponseRecorder{
			ResponseWriter: w,
			StatusCode:     http.StatusOK,
		}

		next(recorder, r)

		if recorder.StatusCode < 500 {
			_ = h.service.SaveIdempotencyKey(r.Context(), userID, key, recorder.StatusCode, recorder.Body.String())
		}
	}
}

type CreateIntentRequest struct {
	Amount               int64  `json:"amount"`
	Currency             string `json:"currency"`
	Description          string `json:"description"`
	ApplicationFeeAmount int64  `json:"application_fee_amount"`
	OnBehalfOf           string `json:"on_behalf_of"`
}

func (h *PaymentHandler) CreatePaymentIntent(w http.ResponseWriter, r *http.Request) {
	timer := prometheus.NewTimer(infrastructure.PaymentLatency.WithLabelValues("create"))
	defer timer.ObserveDuration()

	userID, err := authutil.ExtractUserID(r)
	if err != nil || userID == "" {
		apierror.Unauthorized("Authentication required").Write(w)
		return
	}

	var req CreateIntentRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		apierror.BadRequest("Invalid request body").Write(w)
		return
	}

	intent := &domain.PaymentIntent{
		Amount:               req.Amount,
		Currency:             req.Currency,
		Description:          req.Description,
		UserID:               userID,
		ZoneID:               r.Header.Get("X-Zone-ID"),
		Mode:                 r.Header.Get("X-Zone-Mode"),
		ApplicationFeeAmount: req.ApplicationFeeAmount,
		OnBehalfOf:           req.OnBehalfOf,
		Status:               "requires_payment_method",
	}

	if err := h.service.CreatePaymentIntent(r.Context(), intent); err != nil {
		infrastructure.PaymentRequests.WithLabelValues("create", "error").Inc()
		apierror.Internal("Failed to create payment intent").Write(w)
		return
	}

	infrastructure.PaymentRequests.WithLabelValues("create", "success").Inc()

	audit.Log(r.Context(), audit.AuditLog{
		ActorID:      userID,
		Action:       "payment.intent_created",
		ResourceType: "payment_intent",
		ResourceID:   intent.ID,
		Metadata: map[string]interface{}{
			"amount":   intent.Amount,
			"currency": intent.Currency,
		},
	})

	jsonutil.WriteJSON(w, http.StatusCreated, intent)
}

func (h *PaymentHandler) ConfirmPaymentIntent(w http.ResponseWriter, r *http.Request) {
	timer := prometheus.NewTimer(infrastructure.PaymentLatency.WithLabelValues("confirm"))
	defer timer.ObserveDuration()

	id := jsonutil.GetIDAfter(r, "intents")
	if id == "" {
		apierror.BadRequest("Missing Intent ID").Write(w)
		return
	}

	var req struct {
		PaymentMethodID string `json:"payment_method_id"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		apierror.BadRequest("Invalid request body").Write(w)
		return
	}

	intent, err := h.service.GetPaymentIntent(r.Context(), id)
	if err != nil || intent == nil {
		apierror.NotFound("Payment intent not found").Write(w)
		return
	}

	if _, err := h.bankClient.Charge(r.Context(), intent.Amount, intent.Currency, req.PaymentMethodID); err != nil {
		_ = h.service.UpdateStatus(r.Context(), id, "FAILED")
		jsonutil.WriteJSON(w, http.StatusOK, map[string]string{"status": "failed", "reason": "Bank declined"})
		return
	}

	if err := h.service.UpdateStatus(r.Context(), id, "SUCCEEDED"); err != nil {
		apierror.Internal("Failed to update status").Write(w)
		return
	}

	infrastructure.PaymentRequests.WithLabelValues("confirm", "success").Inc()
	jsonutil.WriteJSON(w, http.StatusOK, intent)
}

func (h *PaymentHandler) RefundPaymentIntent(w http.ResponseWriter, r *http.Request) {
	id := jsonutil.GetIDAfter(r, "intents")
	if id == "" {
		apierror.BadRequest("Missing Intent ID").Write(w)
		return
	}

	// For simplified logic, mapping refund to CANCELLED or a new REFUNDED if added to domain
	if err := h.service.UpdateStatus(r.Context(), id, "CANCELLED"); err != nil {
		apierror.Internal("Failed to update refund status").Write(w)
		return
	}

	jsonutil.WriteJSON(w, http.StatusOK, map[string]string{"status": "refunded"})
}

func (h *PaymentHandler) ListPaymentIntents(w http.ResponseWriter, r *http.Request) {
	zoneID := r.URL.Query().Get("zone")
	limitStr := r.URL.Query().Get("limit")
	limit := 50
	if limitStr != "" {
		fmt.Sscanf(limitStr, "%d", &limit)
	}

	intents, err := h.service.ListPaymentIntents(r.Context(), zoneID, limit)
	if err != nil {
		apierror.Internal("Failed to list payment intents").Write(w)
		return
	}

	jsonutil.WriteJSON(w, http.StatusOK, intents)
}
