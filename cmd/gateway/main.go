package main

import (
	"fmt"
	"log"
	"microservices/pkg/jsonutil"
	"net/http"
	"net/http/httputil"
	"net/url"
	"strings"
	"time"
)

// GatewayHandler holds the configuration for upstream service URLs.
type GatewayHandler struct {
	authServiceURL    string
	paymentServiceURL string
	ledgerServiceURL  string
}

// NewGatewayHandler creates a new instance of GatewayHandler with the provided service URLs.
func NewGatewayHandler(authServiceURL, paymentServiceURL, ledgerServiceURL string) *GatewayHandler {
	return &GatewayHandler{
		authServiceURL:    authServiceURL,
		paymentServiceURL: paymentServiceURL,
		ledgerServiceURL:  ledgerServiceURL,
	}
}

// proxyRequest creates a reverse proxy to the target URL and serves the request.
func (h *GatewayHandler) proxyRequest(target string, w http.ResponseWriter, r *http.Request) {
	url, err := url.Parse(target)
	if err != nil {
		log.Printf("Error parsing target URL %s: %v", target, err)
		jsonutil.WriteErrorJSON(w, "Internal Server Error; Invalid Target")
		return
	}

	proxy := httputil.NewSingleHostReverseProxy(url)

	// Optional: Modify the director to update the request path if needed.
	// For now, we forward the path as is, but we might want to strip prefixes later.
	// Example: /auth/register -> /register on the auth service
	originalDirector := proxy.Director
	proxy.Director = func(req *http.Request) {
		originalDirector(req)
		// Set the Host header to the target host
		req.Host = url.Host
	}

	// logging the forward proxy request
	fmt.Printf("\nForward request:%v", r.URL)

	proxy.ServeHTTP(w, r)
}

// ServeHTTP implements the http.Handler interface to route requests to appropriate services.
func (h *GatewayHandler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	path := r.URL.Path

	switch {
	case strings.HasPrefix(path, "/auth"):
		// Strip the /auth prefix so the auth service sees /register instead of /auth/register
		// Is this desired? The user didn't specify, but it's common practice.
		// Let's assume the auth service expects /register.
		http.StripPrefix("/auth", http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			h.proxyRequest(h.authServiceURL, w, r)
		})).ServeHTTP(w, r)

	case strings.HasPrefix(path, "/payments"):
		http.StripPrefix("/payments", http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			h.proxyRequest(h.paymentServiceURL, w, r)
		})).ServeHTTP(w, r)

	case strings.HasPrefix(path, "/ledger"):
		http.StripPrefix("/ledger", http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			h.proxyRequest(h.ledgerServiceURL, w, r)
		})).ServeHTTP(w, r)

	case path == "/health":
		jsonutil.WriteJSON(w, http.StatusOK, map[string]string{
			"status":  "active",
			"service": "gateway",
			"date":    time.Now().Format(time.DateTime),
		})

	default:
		jsonutil.WriteErrorJSON(w, "Not Found")
	}
}

func main() {
	// Configuration (In a real app, load from env)
	// Note: Auth service is on :8081
	gateway := NewGatewayHandler(
		"http://127.0.0.1:8081",
		"http://127.0.0.1:8082",
		"http://127.0.0.1:8083",
	)

	server := &http.Server{
		Addr:    ":8080",
		Handler: gateway,
	}

	log.Println("Gateway service starting on :8080")
	if err := server.ListenAndServe(); err != nil {
		log.Fatalf("Gateway server failed: %v", err)
	}
}
