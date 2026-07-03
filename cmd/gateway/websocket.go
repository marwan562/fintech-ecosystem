package main

import (
	"fmt"
	"net/http"

	"github.com/gorilla/websocket"
	"github.com/sapliy/sapliy-core/pkg/apierror"
)

func (h *GatewayHandler) handleWebSocket(w http.ResponseWriter, r *http.Request) {
	orgID := r.Header.Get("X-Org-ID")
	if orgID == "" {
		h.logger.Warn("WebSocket attempt without Org ID context")
		apierror.Unauthorized("Organization context required for WebSocket").Write(w)
		return
	}

	conn, err := h.upgrader.Upgrade(w, r, nil)
	if err != nil {
		h.logger.Error("WS upgrade failed", "error", err)
		return
	}
	defer func() {
		if err := conn.Close(); err != nil {
			h.logger.Warn("Failed to close WS connection", "error", err)
		}
	}()

	// Subscribe to Org-scoped events
	channel := fmt.Sprintf("events:org:%s", orgID)
	h.logger.Info("Subscribing WS to channel", "channel", channel)

	pubsub := h.rdb.Subscribe(r.Context(), channel)
	defer func() {
		if err := pubsub.Close(); err != nil {
			h.logger.Warn("Failed to close Redis PubSub", "error", err)
		}
	}()

	ch := pubsub.Channel()
	for msg := range ch {
		if err := conn.WriteMessage(websocket.TextMessage, []byte(msg.Payload)); err != nil {
			h.logger.Error("WS write failed", "error", err)
			break
		}
	}
}
