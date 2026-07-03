package main

import (
	"net/http"
	"strings"
	"time"

	"github.com/gorilla/websocket"
	"github.com/sapliy/sapliy-core/pkg/apierror"
	"github.com/sapliy/sapliy-core/pkg/jsonutil"
)

func (h *GatewayHandler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	path := r.URL.Path

	// Route to Service
	p := path
	if after, ok := strings.CutPrefix(p, "/v1"); ok {
		p = after
	}

	switch {
	case strings.HasPrefix(p, "/payments"):
		http.StripPrefix(path[:len(path)-len(p)]+"/payments", http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			h.proxyRequest(h.paymentServiceURL, w, r)
		})).ServeHTTP(w, r)

	case strings.HasPrefix(p, "/ledger"):
		http.StripPrefix(path[:len(path)-len(p)]+"/ledger", http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			h.proxyRequest(h.ledgerServiceURL, w, r)
		})).ServeHTTP(w, r)

	case strings.HasPrefix(p, "/wallets"):
		h.proxyRequest(h.walletServiceURL, w, r)

	case strings.HasPrefix(p, "/billing"):
		http.StripPrefix(path[:len(path)-len(p)]+"/billing", http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			h.proxyRequest(h.billingServiceURL, w, r)
		})).ServeHTTP(w, r)

	case strings.HasPrefix(p, "/webhooks") || strings.HasPrefix(p, "/notifications"):
		h.proxyRequest(h.notificationServiceURL, w, r)

	case strings.HasPrefix(p, "/events"):
		// Use internal WS handler for events stream, or proxy for others
		if p == "/events/stream" && websocket.IsWebSocketUpgrade(r) {
			h.handleWebSocket(w, r)
			return
		}
		if p == "/events/emit" && r.Method == http.MethodPost {
			h.handleEventEmit(w, r)
			return
		}
		h.proxyRequest(h.eventsServiceURL, w, r)

	case strings.HasPrefix(p, "/flows") || strings.HasPrefix(p, "/executions"):
		h.proxyRequest(h.flowServiceURL, w, r)

	case strings.HasPrefix(p, "/zones"):
		// Some /zones endpoints belong to flow-service and events-service
		if strings.Contains(p, "/flows") {
			h.proxyRequest(h.flowServiceURL, w, r)
			return
		}
		if strings.Contains(p, "/events") {
			h.proxyRequest(h.eventsServiceURL, w, r)
			return
		}
		// Default /zones goes to auth
		h.proxyRequest(h.authServiceURL, w, r)

	case p == "/ws": // Legacy or alternative WS path
		if websocket.IsWebSocketUpgrade(r) {
			h.handleWebSocket(w, r)
			return
		}
		apierror.BadRequest("WebSocket upgrade required").Write(w)

	default:
		// Public Routes or Root
		if strings.HasPrefix(path, "/auth") || path == "/health" {
			h.routePublic(w, r)
			return
		}

		// Fallback for root path if it's a WebSocket upgrade
		if (p == "/" || p == "") && websocket.IsWebSocketUpgrade(r) {
			h.handleWebSocket(w, r)
			return
		}
		h.logger.Warn("Route not found", "path", path)
		apierror.NotFound("Not Found").Write(w)
	}
}

func (h *GatewayHandler) routePublic(w http.ResponseWriter, r *http.Request) {
	path := r.URL.Path
	if strings.HasPrefix(path, "/auth") {
		http.StripPrefix("/auth", http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			h.proxyRequest(h.authServiceURL, w, r)
		})).ServeHTTP(w, r)
		return
	}
	// Health
	jsonutil.WriteJSON(w, http.StatusOK, map[string]string{
		"status":  "active",
		"service": "gateway",
		"date":    time.Now().Format(time.DateTime),
	})
}
