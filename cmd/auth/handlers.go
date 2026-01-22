package main

import (
	"encoding/json"
	"log"
	"net/http"
	"strings"

	"github.com/marwan562/fintech-ecosystem/internal/auth"
	"github.com/marwan562/fintech-ecosystem/pkg/apikey"
	"github.com/marwan562/fintech-ecosystem/pkg/bcryptutil"
	"github.com/marwan562/fintech-ecosystem/pkg/jsonutil"
	"github.com/marwan562/fintech-ecosystem/pkg/jwtutil"
)

type GenerateAPIKeyRequest struct {
	Environment string `json:"environment"` // "test" or "live"
}

type GenerateAPIKeyResponse struct {
	Key          string `json:"key"` // Full key shown ONLY once
	Environment  string `json:"environment"`
	TruncatedKey string `json:"truncated_key"`
}

// Helper to extract UserID from JWT (duplicated from payments, common lib pending)
func extractUserIDFromToken(r *http.Request) (string, error) {
	authHeader := r.Header.Get("Authorization")
	if authHeader == "" {
		return "", nil
	}
	tokenString := strings.TrimPrefix(authHeader, "Bearer ")
	claims, err := jwtutil.ValidateToken(tokenString)
	if err != nil {
		return "", err
	}
	return claims.UserID, nil
}

func (h *AuthHandler) GenerateAPIKey(w http.ResponseWriter, r *http.Request) {
	userID, err := extractUserIDFromToken(r)
	if err != nil || userID == "" {
		log.Printf("GenerateAPIKey: Validation failed. userID: %s, err: %v", userID, err)
		jsonutil.WriteErrorJSON(w, "Unauthorized")
		return
	}
	log.Printf("GenerateAPIKey: Success for user %s", userID)

	var req GenerateAPIKeyRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		jsonutil.WriteErrorJSON(w, "Invalid request body")
		return
	}

	if req.Environment != "test" && req.Environment != "live" {
		jsonutil.WriteErrorJSON(w, "Invalid environment (use 'test' or 'live')")
		return
	}

	prefix := "sk_" + req.Environment
	fullKey, hash, err := apikey.GenerateKey(prefix)
	if err != nil {
		jsonutil.WriteErrorJSON(w, "Failed to generate key")
		return
	}

	truncated := fullKey[len(fullKey)-4:]

	apiKey := &auth.APIKey{
		UserID:       userID,
		KeyPrefix:    prefix,
		KeyHash:      hash,
		TruncatedKey: truncated,
		Environment:  req.Environment,
	}

	if err := h.repo.CreateAPIKey(r.Context(), apiKey); err != nil {
		jsonutil.WriteErrorJSON(w, "Failed to save API key")
		return
	}

	// Return the FULL key only once
	jsonutil.WriteJSON(w, http.StatusCreated, GenerateAPIKeyResponse{
		Key:          fullKey,
		Environment:  req.Environment,
		TruncatedKey: truncated,
	})
}

type ValidateAPIKeyRequest struct {
	KeyHash string `json:"key_hash"`
}

type ValidateAPIKeyResponse struct {
	Valid       bool   `json:"valid"`
	UserID      string `json:"user_id"`
	Environment string `json:"environment"`
}

func (h *AuthHandler) ValidateAPIKey(w http.ResponseWriter, r *http.Request) {
	// Internal endpoint, should be protected (e.g. valid only from localhost or via mTLS/secret)
	// For now, assuming network isolation or simple IP check in real world.

	var req ValidateAPIKeyRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		jsonutil.WriteErrorJSON(w, "Invalid request body")
		return
	}

	key, err := h.repo.GetAPIKeyByHash(r.Context(), req.KeyHash)
	if err != nil {
		log.Printf("Error validating key: %v", err)
		jsonutil.WriteErrorJSON(w, "Validation failed")
		return
	}

	if key == nil || key.RevokedAt != nil {
		jsonutil.WriteJSON(w, http.StatusOK, ValidateAPIKeyResponse{Valid: false})
		return
	}

	jsonutil.WriteJSON(w, http.StatusOK, ValidateAPIKeyResponse{
		Valid:       true,
		UserID:      key.UserID,
		Environment: key.Environment,
	})
}

// AuthHandler holds dependencies for authentication endpoints.
type AuthHandler struct {
	repo *auth.Repository
}

// RegisterRequest defines the payload for user registration.
type RegisterRequest struct {
	Email    string `json:"email"`
	Password string `json:"password"`
}

// LoginRequest defines the payload for user login.
type LoginRequest struct {
	Email    string `json:"email"`
	Password string `json:"password"`
}

// LoginResponse defines the successful response for login.
type LoginResponse struct {
	Token string `json:"token"`
}

// Register handles user account creation.
func (h *AuthHandler) Register(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		jsonutil.WriteErrorJSON(w, "Method not allowed")
		return
	}

	var req RegisterRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		jsonutil.WriteErrorJSON(w, "Invalid request body")
		return
	}

	if req.Email == "" || req.Password == "" {
		jsonutil.WriteErrorJSON(w, "Email and password are required")
		return
	}

	// Use bcryptutil to hash the password securely
	b := &bcryptutil.BcryptUtilsImpl{}
	passwordHash, err := b.GenerateHash(req.Password)
	if err != nil {
		jsonutil.WriteErrorJSON(w, "Failed to hash password")
		return
	}

	user, err := h.repo.CreateUser(r.Context(), req.Email, passwordHash)
	if err != nil {
		// Check for duplicate key error (generic check for now)
		// In a real app, parse the error code for unique constraint violation
		jsonutil.WriteErrorJSON(w, "Failed to create user (email might be taken)")
		return
	}

	jsonutil.WriteJSON(w, http.StatusCreated, user)
}

// Login handles user authentication and JWT issuance.
func (h *AuthHandler) Login(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		jsonutil.WriteErrorJSON(w, "Method not allowed")
		return
	}

	var req LoginRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		jsonutil.WriteErrorJSON(w, "Invalid request body")
		return
	}

	if req.Email == "" || req.Password == "" {
		jsonutil.WriteErrorJSON(w, "Email and password are required")
		return
	}

	// Fetch user by email
	user, err := h.repo.GetUserByEmail(r.Context(), req.Email)
	if err != nil {
		jsonutil.WriteErrorJSON(w, "Internal server error")
		return
	}
	if user == nil {
		jsonutil.WriteErrorJSON(w, "Invalid email or password") // Generic message for security
		return
	}

	// Verify password
	b := &bcryptutil.BcryptUtilsImpl{}
	match := b.CompareHash(req.Password, user.Password)
	if !match {
		jsonutil.WriteErrorJSON(w, "Invalid email or password")
		return
	}

	// Generate JWT
	token, err := jwtutil.GenerateToken(user.ID, user.Email)
	if err != nil {
		jsonutil.WriteErrorJSON(w, "Failed to generate token")
		return
	}

	log.Printf("Login: Success for user %s", user.Email)
	jsonutil.WriteJSON(w, http.StatusOK, LoginResponse{Token: token})
}
