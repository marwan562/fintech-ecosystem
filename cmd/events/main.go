package main

import (
	"context"
	"encoding/json"
	"log"
	"net/http"
	"os"

	"github.com/gorilla/websocket"
	"github.com/marwan562/fintech-ecosystem/pkg/jsonutil"
	"github.com/redis/go-redis/v9"
)

var upgrader = websocket.Upgrader{
	CheckOrigin: func(r *http.Request) bool {
		return true // Allow all origins for CLI/dev tools
	},
}

type EventServer struct {
	rdb *redis.Client
}

func main() {
	// Initialize Redis
	redisAddr := os.Getenv("REDIS_ADDR")
	if redisAddr == "" {
		redisAddr = "localhost:6379"
	}
	rdb := redis.NewClient(&redis.Options{
		Addr: redisAddr,
	})
	if err := rdb.Ping(context.Background()).Err(); err != nil {
		log.Printf("Warning: Redis connection failed in Events service: %v", err)
	}

	server := &EventServer{rdb: rdb}

	http.HandleFunc("/health", func(w http.ResponseWriter, r *http.Request) {
		jsonutil.WriteJSON(w, http.StatusOK, map[string]string{"status": "active", "service": "events"})
	})

	http.HandleFunc("/v1/events/stream", server.handleWebSocket)

	port := os.Getenv("PORT")
	if port == "" {
		port = "8089"
	}

	log.Printf("Events WebSocket service starting on :%s", port)
	if err := http.ListenAndServe(":"+port, nil); err != nil {
		log.Fatal(err)
	}
}

func (s *EventServer) handleWebSocket(w http.ResponseWriter, r *http.Request) {
	// 1. Authenticate (Simple API Key check for now)
	apiKey := r.URL.Query().Get("api_key")
	if apiKey == "" {
		http.Error(w, "Unauthorized: api_key required", http.StatusUnauthorized)
		return
	}

	// 2. Filter params
	zoneID := r.URL.Query().Get("zone")

	// 3. Upgrade to WebSocket
	conn, err := upgrader.Upgrade(w, r, nil)
	if err != nil {
		log.Printf("Upgrade error: %v", err)
		return
	}
	defer conn.Close()

	log.Printf("Client connected. Zone: %s", zoneID)

	// 4. Subscribe to Redis
	// We subscribe to a general channel "webhook_events" where all service emit events
	pubsub := s.rdb.Subscribe(r.Context(), "webhook_events")
	defer pubsub.Close()

	// 5. Stream Loop
	ch := pubsub.Channel()

	// Handle client disconnect via ping/pong or read failure
	done := make(chan struct{})
	go func() {
		for {
			if _, _, err := conn.ReadMessage(); err != nil {
				close(done)
				return
			}
		}
	}()

	for {
		select {
		case msg := <-ch:
			var event map[string]interface{}
			if err := json.Unmarshal([]byte(msg.Payload), &event); err != nil {
				continue
			}

			// Filter by zone if specified
			if zoneID != "" {
				if eventZone, ok := event["zone_id"].(string); ok && eventZone != zoneID {
					continue
				}
			}

			// Send to client
			if err := conn.WriteJSON(event); err != nil {
				log.Printf("Write error: %v", err)
				return
			}

		case <-done:
			log.Println("Client disconnected")
			return
		}
	}
}
