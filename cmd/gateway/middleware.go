package main

import (
	"net/http"
	"strings"

	"github.com/sapliy/sapliy-core/pkg/apierror"
	"github.com/sapliy/sapliy-core/pkg/apikey"
	"github.com/sapliy/sapliy-core/pkg/scopes"
)

type Middleware func(http.Handler) http.Handler

func CORSMiddleware(allowedOrigins string, allowedOriginsMap map[string]bool) Middleware {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			origin := r.Header.Get("Origin")

			// Set CORS headers only for explicitly allowed origins
			if origin != "" && allowedOriginsMap[origin] {
				w.Header().Set("Access-Control-Allow-Origin", origin)
			} else if allowedOriginsMap["*"] {
				w.Header().Set("Access-Control-Allow-Origin", "*")
			} else {
				if r.Method == http.MethodOptions {
					w.WriteHeader(http.StatusForbidden)
					return
				}
			}

			w.Header().Set("Access-Control-Allow-Methods", "GET, POST, PUT, DELETE, PATCH, OPTIONS")
			w.Header().Set("Access-Control-Allow-Headers", "Authorization, Content-Type, Idempotency-Key, X-Zone-ID")
			w.Header().Set("Access-Control-Allow-Credentials", "true")
			w.Header().Set("Access-Control-Max-Age", "86400")

			if r.Method == http.MethodOptions {
				w.WriteHeader(http.StatusNoContent)
				return
			}

			next.ServeHTTP(w, r)
		})
	}
}

func (h *GatewayHandler) AuthMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		path := r.URL.Path

		if strings.HasPrefix(path, "/auth") || path == "/health" {
			next.ServeHTTP(w, r)
			return
		}

		// Extract Secret Key
		authHeader := r.Header.Get("Authorization")
		apiKeyStr := ""
		if strings.HasPrefix(authHeader, "Bearer ") {
			apiKeyStr = strings.TrimPrefix(authHeader, "Bearer ")
		} else {
			apiKeyStr = r.URL.Query().Get("api_key")
		}

		if apiKeyStr == "" || (!strings.HasPrefix(apiKeyStr, "sk_") && !strings.HasPrefix(apiKeyStr, "pk_")) {
			apierror.Unauthorized("Missing or invalid API Key").Write(w)
			return
		}

		// Hash Key
		keyHash := apikey.HashKey(apiKeyStr, h.hmacSecret)

		// Validate with Auth Service
		userID, env, keyScopes, orgID, role, quota, zoneID, mode, keyType, valid := h.validateKeyWithAuthService(r.Context(), keyHash)
		if !valid {
			apierror.Unauthorized("Invalid or revoked API Key").Write(w)
			return
		}

		// Key Type Enforcement
		if keyType == "publishable" && !strings.HasPrefix(path, "/v1/events/emit") {
			apierror.Forbidden("Publishable keys only allowed for event emission").Write(w)
			return
		}

		// Scope Enforcement
		scopePath := path
		if after, ok := strings.CutPrefix(scopePath, "/v1"); ok {
			scopePath = after
		}
		requiredScope := scopes.GetRequiredScope(scopePath, r.Method)
		if requiredScope != "" && !scopes.HasScope(keyScopes, requiredScope) {
			apierror.ForbiddenWithDetails("Insufficient scope", map[string]string{
				"required_scope": requiredScope,
			}).Write(w)
			return
		}

		// Rate Limiting
		allowed, err := h.checkRateLimit(r.Context(), keyHash, quota)
		if err != nil {
			h.logger.Error("Redis error in rate limiter", "error", err)
			apierror.Internal("Internal Server Error").Write(w)
			return
		}
		if !allowed {
			w.Header().Set("Retry-After", "60")
			apierror.RateLimited("60").Write(w)
			return
		}

		// Inject Context
		r.Header.Set("X-User-ID", userID)
		r.Header.Set("X-Environment", env)
		r.Header.Set("X-Org-ID", orgID)
		r.Header.Set("X-Role", role)
		r.Header.Set("X-Zone-ID", zoneID)
		r.Header.Set("X-Zone-Mode", mode)

		next.ServeHTTP(w, r)
	})
}

func BodyLimitMiddleware(maxBytes int64) Middleware {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			r.Body = http.MaxBytesReader(w, r.Body, maxBytes)
			next.ServeHTTP(w, r)
		})
	}
}
