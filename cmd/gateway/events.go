package main

import (
	"crypto/sha256"
	"encoding/json"
	"fmt"
	"net/http"
	"time"

	"github.com/redis/go-redis/v9"
	"github.com/sapliy/sapliy-core/pkg/apierror"
	"github.com/sapliy/sapliy-core/pkg/jsonutil"
)

// EventEnvelope is the full event structure following the spec
type EventEnvelope struct {
	ID             string                 `json:"id"`
	Type           string                 `json:"type"`
	ZoneID         string                 `json:"zone_id"`
	OrgID          string                 `json:"org_id"`
	Timestamp      string                 `json:"timestamp"`
	IdempotencyKey string                 `json:"idempotency_key"`
	Payload        map[string]interface{} `json:"payload"`
	Meta           map[string]string      `json:"meta"`
}

type EventEmitRequest struct {
	Type           string                 `json:"type"`
	IdempotencyKey string                 `json:"idempotency_key,omitempty"`
	Data           map[string]interface{} `json:"data"`
	Meta           map[string]string      `json:"meta,omitempty"`
}

type EventEmitResponse struct {
	Status  string `json:"status"`
	EventID string `json:"event_id,omitempty"`
	Topic   string `json:"topic,omitempty"`
	Message string `json:"message,omitempty"`
}

// generateEventID creates a unique event ID
func generateEventID() string {
	return fmt.Sprintf("evt_%d", time.Now().UnixNano())
}

// hashPayload creates a SHA256 hash of the payload for dedup verification
func hashPayload(data map[string]interface{}) string {
	bytes, _ := json.Marshal(data)
	hash := sha256.Sum256(bytes)
	return fmt.Sprintf("%x", hash[:8])
}

func (h *GatewayHandler) handleEventEmit(w http.ResponseWriter, r *http.Request) {
	zoneID := r.Header.Get("X-Zone-ID")
	orgID := r.Header.Get("X-Org-ID")
	if zoneID == "" {
		apierror.BadRequest("Zone context missing").Write(w)
		return
	}

	var req EventEmitRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		apierror.BadRequest("Invalid request body").Write(w)
		return
	}

	if req.Type == "" {
		apierror.BadRequest("Event type required").Write(w)
		return
	}

	// Generate or use provided idempotency key
	idempotencyKey := req.IdempotencyKey
	if idempotencyKey == "" {
		idempotencyKey = r.Header.Get("Idempotency-Key")
	}
	if idempotencyKey == "" {
		// Auto-generate but warn (non-idempotent)
		idempotencyKey = fmt.Sprintf("auto_%d", time.Now().UnixNano())
	}

	// Build dedup key: zone_id:idempotency_key
	dedupKey := fmt.Sprintf("dedup:%s:%s", zoneID, idempotencyKey)
	payloadHash := hashPayload(req.Data)

	// Check for existing idempotency record
	cachedResult, err := h.rdb.Get(r.Context(), dedupKey).Result()
	if err == nil && cachedResult != "" {
		// Found cached result - parse and return
		var cached EventEmitResponse
		if json.Unmarshal([]byte(cachedResult), &cached) == nil {
			cached.Status = "duplicate"
			cached.Message = "Event already processed"
			jsonutil.WriteJSON(w, http.StatusAccepted, cached)
			return
		}
	}

	// Generate event envelope
	eventID := generateEventID()
	envelope := EventEnvelope{
		ID:             eventID,
		Type:           req.Type,
		ZoneID:         zoneID,
		OrgID:          orgID,
		Timestamp:      time.Now().UTC().Format(time.RFC3339),
		IdempotencyKey: idempotencyKey,
		Payload:        req.Data,
		Meta: map[string]string{
			"source":       req.Meta["source"],
			"env":          req.Meta["env"],
			"payload_hash": payloadHash,
		},
	}

	// Set default meta values
	if envelope.Meta["source"] == "" {
		envelope.Meta["source"] = "gateway"
	}

	topic := fmt.Sprintf("zone.%s.event.%s", zoneID, req.Type)
	envelopeBytes, _ := json.Marshal(envelope)

	// Publish to Redis Stream
	err = h.rdb.XAdd(r.Context(), &redis.XAddArgs{
		Stream: topic,
		Values: map[string]interface{}{
			"envelope": envelopeBytes,
			"data":     envelopeBytes, // For backward compatibility
			"ts":       time.Now().Unix(),
		},
	}).Err()

	if err != nil {
		h.logger.Error("Failed to publish to Redis Stream", "error", err)
		apierror.Internal("Failed to ingest event").Write(w)
		return
	}

	// Build response
	response := EventEmitResponse{
		Status:  "ingested",
		EventID: eventID,
		Topic:   topic,
	}

	// Cache result for dedup (30 days TTL)
	resultBytes, _ := json.Marshal(response)
	h.rdb.Set(r.Context(), dedupKey, resultBytes, 30*24*time.Hour)

	h.logger.Info("Event ingested", "event_id", eventID, "topic", topic, "zone", zoneID)
	jsonutil.WriteJSON(w, http.StatusAccepted, response)
}
