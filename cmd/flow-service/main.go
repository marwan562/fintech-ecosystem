package main

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"os"
	"os/signal"
	"strings"
	"syscall"
	"time"

	"github.com/gorilla/mux"
	"github.com/gorilla/websocket"
	"github.com/sapliy/fintech-ecosystem/internal/flow"
	"github.com/sapliy/fintech-ecosystem/internal/flow/domain"
	"github.com/sapliy/fintech-ecosystem/internal/flow/infrastructure"
	"github.com/sapliy/fintech-ecosystem/pkg/database"
	"github.com/sapliy/fintech-ecosystem/pkg/messaging"
	"github.com/sapliy/fintech-ecosystem/pkg/observability"
)

type FlowServer struct {
	debugService *flow.DebugService
	repo         domain.Repository
	runner       *domain.FlowRunner
	upgrader     websocket.Upgrader
}

func NewFlowServer(debugService *flow.DebugService, repo domain.Repository) *FlowServer {
	return &FlowServer{
		debugService: debugService,
		repo:         repo,
		runner:       domain.NewFlowRunner(repo),
		upgrader: websocket.Upgrader{
			CheckOrigin: func(r *http.Request) bool {
				return true // Allow all origins for development
			},
		},
	}
}

// Debug HTTP Handlers

func (s *FlowServer) StartDebugSession(w http.ResponseWriter, r *http.Request) {
	vars := mux.Vars(r)
	flowID := vars["flowId"]
	zoneID := vars["zoneId"]

	var req struct {
		Level domain.DebugLevel `json:"level"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "Invalid request body", http.StatusBadRequest)
		return
	}

	session, err := s.debugService.StartDebugSession(r.Context(), flowID, zoneID, req.Level)
	if err != nil {
		http.Error(w, fmt.Sprintf("Failed to start debug session: %v", err), http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(session)
}

func (s *FlowServer) GetDebugSession(w http.ResponseWriter, r *http.Request) {
	vars := mux.Vars(r)
	sessionID := vars["sessionId"]

	session, err := s.debugService.GetDebugSession(sessionID)
	if err != nil {
		http.Error(w, fmt.Sprintf("Debug session not found: %v", err), http.StatusNotFound)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(session)
}

func (s *FlowServer) EndDebugSession(w http.ResponseWriter, r *http.Request) {
	vars := mux.Vars(r)
	sessionID := vars["sessionId"]

	if err := s.debugService.EndDebugSession(sessionID); err != nil {
		http.Error(w, fmt.Sprintf("Failed to end debug session: %v", err), http.StatusInternalServerError)
		return
	}

	w.WriteHeader(http.StatusOK)
	json.NewEncoder(w).Encode(map[string]string{"message": "Debug session ended"})
}

func (s *FlowServer) GetDebugEvents(w http.ResponseWriter, r *http.Request) {
	vars := mux.Vars(r)
	sessionID := vars["sessionId"]

	var since *time.Time
	if sinceStr := r.URL.Query().Get("since"); sinceStr != "" {
		if parsed, err := time.Parse(time.RFC3339, sinceStr); err == nil {
			since = &parsed
		}
	}

	events, err := s.debugService.GetDebugEvents(sessionID, since)
	if err != nil {
		http.Error(w, fmt.Sprintf("Failed to get debug events: %v", err), http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(events)
}

// WebSocket handler for real-time debug events
func (s *FlowServer) DebugWebSocket(w http.ResponseWriter, r *http.Request) {
	vars := mux.Vars(r)
	sessionID := vars["sessionId"]

	// Upgrade to WebSocket
	conn, err := s.upgrader.Upgrade(w, r, nil)
	if err != nil {
		log.Printf("Failed to upgrade to WebSocket: %v", err)
		return
	}
	defer conn.Close()

	// Get debug session
	session, err := s.debugService.GetDebugSession(sessionID)
	if err != nil {
		log.Printf("Debug session not found: %v", err)
		return
	}

	// Send existing events
	events, err := s.debugService.GetDebugEvents(sessionID, nil)
	if err == nil {
		for _, event := range events {
			if err := conn.WriteJSON(event); err != nil {
				log.Printf("Failed to send event: %v", err)
				return
			}
		}
	}

	// Listen for new events
	ticker := time.NewTicker(100 * time.Millisecond)
	defer ticker.Stop()

	for {
		select {
		case <-ticker.C:
			// Check for new events
			newEvents, err := s.debugService.GetDebugEvents(sessionID, &session.StartTime)
			if err != nil {
				continue
			}

			// Send only new events
			if len(newEvents) > 0 {
				for _, event := range newEvents[len(events):] {
					if err := conn.WriteJSON(event); err != nil {
						log.Printf("Failed to send new event: %v", err)
						return
					}
				}
			}
		case <-r.Context().Done():
			return
		}
	}
}

// Webhook Replay Handlers

type WebhookReplayer struct {
	eventStore  EventStore
	retriggerer EventRetriggerer
	flowService *flow.DebugService
}

// EventStore interface for storing/retrieving past events
type EventStore interface {
	GetPastEvents(ctx context.Context, zoneID string, limit int, offset int) ([]*domain.Event, error)
	GetEventByID(ctx context.Context, eventID string) (*domain.Event, error)
}

// EventRetriggerer interface for re-triggering events
type EventRetriggerer interface {
	RetriggerEvent(ctx context.Context, event *domain.Event) error
}

func NewWebhookReplayer(eventStore EventStore, retriggerer EventRetriggerer, flowService *flow.DebugService) *WebhookReplayer {
	return &WebhookReplayer{
		eventStore:  eventStore,
		retriggerer: retriggerer,
		flowService: flowService,
	}
}

func (wr *WebhookReplayer) GetPastEvents(w http.ResponseWriter, r *http.Request) {
	vars := mux.Vars(r)
	zoneID := vars["zoneId"]

	// Parse query parameters
	limit := 50 // default
	offset := 0

	if limitStr := r.URL.Query().Get("limit"); limitStr != "" {
		if parsed, err := fmt.Sscanf(limitStr, "%d", &limit); err != nil || parsed != 1 {
			limit = 50
		}
	}

	if offsetStr := r.URL.Query().Get("offset"); offsetStr != "" {
		if parsed, err := fmt.Sscanf(offsetStr, "%d", &offset); err != nil || parsed != 1 {
			offset = 0
		}
	}

	events, err := wr.eventStore.GetPastEvents(r.Context(), zoneID, limit, offset)
	if err != nil {
		http.Error(w, fmt.Sprintf("Failed to get past events: %v", err), http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]interface{}{
		"events": events,
		"limit":  limit,
		"offset": offset,
	})
}

func (wr *WebhookReplayer) ReplayEvent(w http.ResponseWriter, r *http.Request) {
	vars := mux.Vars(r)
	eventID := vars["eventId"]

	var req struct {
		ZoneID string `json:"zoneId"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "Invalid request body", http.StatusBadRequest)
		return
	}

	// Get the original event
	event, err := wr.eventStore.GetEventByID(r.Context(), eventID)
	if err != nil {
		http.Error(w, fmt.Sprintf("Event not found: %v", err), http.StatusNotFound)
		return
	}

	// Create a replayed event with new timestamp
	replayedEvent := &domain.Event{
		ID:        fmt.Sprintf("replay_%d", time.Now().UnixNano()),
		Type:      event.Type,
		ZoneID:    req.ZoneID,
		Data:      event.Data,
		CreatedAt: time.Now(),
	}

	// Retrigger the event
	if err := wr.retriggerer.RetriggerEvent(r.Context(), replayedEvent); err != nil {
		http.Error(w, fmt.Sprintf("Failed to replay event: %v", err), http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]interface{}{
		"message":    "Event replayed successfully",
		"eventId":    replayedEvent.ID,
		"originalId": eventID,
		"replayedAt": replayedEvent.CreatedAt,
	})
}

func (wr *WebhookReplayer) BulkReplayEvents(w http.ResponseWriter, r *http.Request) {
	vars := mux.Vars(r)
	zoneID := vars["zoneId"]

	var req struct {
		EventIDs []string `json:"eventIds"`
		Delay    int      `json:"delay"` // Delay between replays in milliseconds
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "Invalid request body", http.StatusBadRequest)
		return
	}

	if len(req.EventIDs) == 0 {
		http.Error(w, "No event IDs provided", http.StatusBadRequest)
		return
	}

	results := make([]map[string]interface{}, 0, len(req.EventIDs))

	for i, eventID := range req.EventIDs {
		// Get the original event
		event, err := wr.eventStore.GetEventByID(r.Context(), eventID)
		if err != nil {
			results = append(results, map[string]interface{}{
				"eventId": eventID,
				"status":  "error",
				"error":   fmt.Sprintf("Event not found: %v", err),
			})
			continue
		}

		// Create replayed event
		replayedEvent := &domain.Event{
			ID:        fmt.Sprintf("replay_%d_%d", time.Now().UnixNano(), i),
			Type:      event.Type,
			ZoneID:    zoneID,
			Data:      event.Data,
			CreatedAt: time.Now(),
		}

		// Retrigger with delay if specified
		if req.Delay > 0 && i > 0 {
			time.Sleep(time.Duration(req.Delay) * time.Millisecond)
		}

		if err := wr.retriggerer.RetriggerEvent(r.Context(), replayedEvent); err != nil {
			results = append(results, map[string]interface{}{
				"eventId": eventID,
				"status":  "error",
				"error":   fmt.Sprintf("Failed to replay: %v", err),
			})
		} else {
			results = append(results, map[string]interface{}{
				"eventId":    eventID,
				"status":     "success",
				"replayedId": replayedEvent.ID,
				"replayedAt": replayedEvent.CreatedAt,
			})
		}
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]interface{}{
		"message": "Bulk replay completed",
		"results": results,
	})
}

// Flow CRUD Handlers

func (s *FlowServer) CreateFlow(w http.ResponseWriter, r *http.Request) {
	var flow domain.Flow
	if err := json.NewDecoder(r.Body).Decode(&flow); err != nil {
		http.Error(w, "Invalid request body", http.StatusBadRequest)
		return
	}

	if flow.ID == "" {
		flow.ID = fmt.Sprintf("flow_%d", time.Now().UnixNano())
	}
	flow.CreatedAt = time.Now()
	flow.UpdatedAt = time.Now()

	if err := s.repo.CreateFlow(r.Context(), &flow); err != nil {
		http.Error(w, fmt.Sprintf("Failed to create flow: %v", err), http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusCreated)
	json.NewEncoder(w).Encode(flow)
}

func (s *FlowServer) GetFlow(w http.ResponseWriter, r *http.Request) {
	vars := mux.Vars(r)
	flowID := vars["flowId"]

	flow, err := s.repo.GetFlow(r.Context(), flowID)
	if err != nil {
		http.Error(w, fmt.Sprintf("Flow not found: %v", err), http.StatusNotFound)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(flow)
}

func (s *FlowServer) ListFlows(w http.ResponseWriter, r *http.Request) {
	vars := mux.Vars(r)
	zoneID := vars["zoneId"]

	flows, err := s.repo.ListFlows(r.Context(), zoneID)
	if err != nil {
		http.Error(w, fmt.Sprintf("Failed to list flows: %v", err), http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]interface{}{
		"flows": flows,
		"count": len(flows),
	})
}

func (s *FlowServer) UpdateFlow(w http.ResponseWriter, r *http.Request) {
	vars := mux.Vars(r)
	flowID := vars["flowId"]

	// Get existing flow
	existing, err := s.repo.GetFlow(r.Context(), flowID)
	if err != nil {
		http.Error(w, fmt.Sprintf("Flow not found: %v", err), http.StatusNotFound)
		return
	}

	// Decode update
	var update domain.Flow
	if err := json.NewDecoder(r.Body).Decode(&update); err != nil {
		http.Error(w, "Invalid request body", http.StatusBadRequest)
		return
	}

	// Preserve immutable fields
	update.ID = existing.ID
	update.CreatedAt = existing.CreatedAt
	update.UpdatedAt = time.Now()

	if err := s.repo.UpdateFlow(r.Context(), &update); err != nil {
		http.Error(w, fmt.Sprintf("Failed to update flow: %v", err), http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(update)
}

func (s *FlowServer) DeleteFlow(w http.ResponseWriter, r *http.Request) {
	vars := mux.Vars(r)
	flowID := vars["flowId"]

	// Use BulkUpdateFlowsEnabled to disable, as there's no DeleteFlow in Repository
	if err := s.repo.BulkUpdateFlowsEnabled(r.Context(), []string{flowID}, false); err != nil {
		http.Error(w, fmt.Sprintf("Failed to delete flow: %v", err), http.StatusInternalServerError)
		return
	}

	w.WriteHeader(http.StatusNoContent)
}

func (s *FlowServer) EnableFlow(w http.ResponseWriter, r *http.Request) {
	vars := mux.Vars(r)
	flowID := vars["flowId"]

	if err := s.repo.BulkUpdateFlowsEnabled(r.Context(), []string{flowID}, true); err != nil {
		http.Error(w, fmt.Sprintf("Failed to enable flow: %v", err), http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]string{"message": "Flow enabled", "flowId": flowID})
}

func (s *FlowServer) DisableFlow(w http.ResponseWriter, r *http.Request) {
	vars := mux.Vars(r)
	flowID := vars["flowId"]

	if err := s.repo.BulkUpdateFlowsEnabled(r.Context(), []string{flowID}, false); err != nil {
		http.Error(w, fmt.Sprintf("Failed to disable flow: %v", err), http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]string{"message": "Flow disabled", "flowId": flowID})
}

func (s *FlowServer) BulkEnableFlows(w http.ResponseWriter, r *http.Request) {
	var req struct {
		FlowIDs []string `json:"flowIds"`
		Enabled bool     `json:"enabled"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "Invalid request body", http.StatusBadRequest)
		return
	}

	if err := s.repo.BulkUpdateFlowsEnabled(r.Context(), req.FlowIDs, req.Enabled); err != nil {
		http.Error(w, fmt.Sprintf("Failed to update flows: %v", err), http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]interface{}{
		"message": "Flows updated",
		"flowIds": req.FlowIDs,
		"enabled": req.Enabled,
	})
}

func setupRoutes(server *FlowServer, replayer *WebhookReplayer) *mux.Router {
	r := mux.NewRouter()

	// Flow CRUD API routes
	r.HandleFunc("/api/v1/flows", server.CreateFlow).Methods("POST")
	r.HandleFunc("/api/v1/flows/{flowId}", server.GetFlow).Methods("GET")
	r.HandleFunc("/api/v1/flows/{flowId}", server.UpdateFlow).Methods("PUT")
	r.HandleFunc("/api/v1/flows/{flowId}", server.DeleteFlow).Methods("DELETE")
	r.HandleFunc("/api/v1/zones/{zoneId}/flows", server.ListFlows).Methods("GET")
	r.HandleFunc("/api/v1/flows/{flowId}/enable", server.EnableFlow).Methods("POST")
	r.HandleFunc("/api/v1/flows/{flowId}/disable", server.DisableFlow).Methods("POST")
	r.HandleFunc("/api/v1/flows/bulk", server.BulkEnableFlows).Methods("POST")

	// Debug API routes
	r.HandleFunc("/api/v1/flows/{flowId}/zones/{zoneId}/debug", server.StartDebugSession).Methods("POST")
	r.HandleFunc("/api/v1/debug/sessions/{sessionId}", server.GetDebugSession).Methods("GET")
	r.HandleFunc("/api/v1/debug/sessions/{sessionId}", server.EndDebugSession).Methods("DELETE")
	r.HandleFunc("/api/v1/debug/sessions/{sessionId}/events", server.GetDebugEvents).Methods("GET")
	r.HandleFunc("/api/v1/debug/sessions/{sessionId}/ws", server.DebugWebSocket).Methods("GET")

	// Webhook Replay API routes
	r.HandleFunc("/api/v1/zones/{zoneId}/events/past", replayer.GetPastEvents).Methods("GET")
	r.HandleFunc("/api/v1/events/{eventId}/replay", replayer.ReplayEvent).Methods("POST")
	r.HandleFunc("/api/v1/zones/{zoneId}/events/bulk-replay", replayer.BulkReplayEvents).Methods("POST")

	return r
}

func main() {
	dsn := os.Getenv("DB_DSN")
	if dsn == "" {
		dsn = "postgres://user:password@127.0.0.1:5433/microservices?sslmode=disable"
	}

	db, err := database.Connect(dsn)
	if err != nil {
		log.Fatalf("Failed to connect to DB: %v", err)
	}
	defer db.Close()

	// Initialize repositories and services
	repo := infrastructure.NewSQLRepository(db)
	debugService := flow.NewDebugService(repo)

	// Setup Kafka Producer for retriggering
	brokers := strings.Split(os.Getenv("KAFKA_BROKERS"), ",")
	if len(brokers) == 0 || brokers[0] == "" {
		brokers = []string{"localhost:9092"}
	}
	kafkaProducer := messaging.NewKafkaProducer(brokers, "payments")
	defer kafkaProducer.Close()

	// Initialize real event store and retriggerer
	eventStore := repo // SQLRepository implements EventStore methods
	retriggerer := infrastructure.NewKafkaEventRetriggerer(kafkaProducer)

	server := NewFlowServer(debugService, repo)
	replayer := NewWebhookReplayer(eventStore, retriggerer, debugService)

	router := setupRoutes(server, replayer)

	port := os.Getenv("PORT")
	if port == "" {
		port = "8084"
	}

	// Initialize Tracer
	shutdown, _ := observability.InitTracer(context.Background(), observability.Config{
		ServiceName: "flow-service",
	})
	defer shutdown(context.Background())

	// Initialize Logger
	logger := observability.NewLogger("flow-service")

	logger.Info("Flow Service starting", "port", port)
	logger.Info("Debug API available", "url", fmt.Sprintf("http://localhost:%s/api/v1", port))
	logger.Info("WebSocket available", "url", fmt.Sprintf("ws://localhost:%s/api/v1/debug/sessions/{sessionId}/ws", port))

	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	srv := &http.Server{
		Addr:    ":" + port,
		Handler: router,
	}

	go func() {
		if err := srv.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			logger.Error("HTTP server error", "error", err)
			os.Exit(1)
		}
	}()

	<-ctx.Done()
	logger.Info("Shutting down Flow Service...")

	shutdownCtx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	if err := srv.Shutdown(shutdownCtx); err != nil {
		log.Printf("Server shutdown error: %v", err)
	}

	log.Println("Flow Service stopped")
}
