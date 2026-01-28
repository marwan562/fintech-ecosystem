package main

import (
	"encoding/json"
	"log"
	"net/http"
	"strings"
	"time"

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
	fullKey, hash, err := apikey.GenerateKey(prefix, h.hmacSecret)
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
	Scopes      string `json:"scopes"`
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
		Scopes:      key.Scopes,
	})
}

// AuthHandler holds dependencies for authentication endpoints.
type AuthHandler struct {
	repo       *auth.Repository
	hmacSecret string
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

// OAuthTokenResponse represents the response for /oauth/token.
type OAuthTokenResponse struct {
	AccessToken  string `json:"access_token"`
	TokenType    string `json:"token_type"`
	ExpiresIn    int    `json:"expires_in"`
	RefreshToken string `json:"refresh_token,omitempty"`
	Scope        string `json:"scope,omitempty"`
}

// OAuthTokenHandler handles OAuth2 token requests (Client Credentials and Authorization Code).
func (h *AuthHandler) OAuthTokenHandler(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		jsonutil.WriteErrorJSON(w, "Method not allowed")
		return
	}

	grantType := r.FormValue("grant_type")

	switch grantType {
	case "client_credentials":
		h.handleClientCredentials(w, r)
	case "authorization_code":
		h.handleAuthorizationCode(w, r)
	default:
		writeOAuthError(w, "unsupported_grant_type", "Grant type not supported", http.StatusBadRequest)
	}
}

// handleClientCredentials processes client_credentials grant.
func (h *AuthHandler) handleClientCredentials(w http.ResponseWriter, r *http.Request) {
	clientID, clientSecret, ok := r.BasicAuth()
	if !ok {
		clientID = r.FormValue("client_id")
		clientSecret = r.FormValue("client_secret")
	}

	if clientID == "" || clientSecret == "" {
		w.Header().Set("WWW-Authenticate", `Basic realm="api"`)
		writeOAuthError(w, "invalid_client", "Client credentials required", http.StatusUnauthorized)
		return
	}

	client, err := h.repo.GetClientByID(r.Context(), clientID)
	if err != nil {
		log.Printf("handleClientCredentials: DB error: %v", err)
		writeOAuthError(w, "server_error", "Internal error", http.StatusInternalServerError)
		return
	}
	if client == nil {
		writeOAuthError(w, "invalid_client", "Unknown client", http.StatusUnauthorized)
		return
	}

	incomingHash := auth.HashString(clientSecret)
	if incomingHash != client.ClientSecretHash {
		writeOAuthError(w, "invalid_client", "Invalid credentials", http.StatusUnauthorized)
		return
	}

	scope := r.FormValue("scope")
	if scope == "" {
		scope = "default"
	}

	h.issueTokens(w, r, client.ID, client.UserID, scope)
}

// handleAuthorizationCode processes authorization_code grant with PKCE.
func (h *AuthHandler) handleAuthorizationCode(w http.ResponseWriter, r *http.Request) {
	code := r.FormValue("code")
	redirectURI := r.FormValue("redirect_uri")
	codeVerifier := r.FormValue("code_verifier")
	clientID := r.FormValue("client_id")

	if code == "" {
		writeOAuthError(w, "invalid_request", "Authorization code required", http.StatusBadRequest)
		return
	}

	authCode, err := h.repo.GetAuthorizationCode(r.Context(), code)
	if err != nil {
		log.Printf("handleAuthorizationCode: DB error: %v", err)
		writeOAuthError(w, "server_error", "Internal error", http.StatusInternalServerError)
		return
	}
	if authCode == nil {
		writeOAuthError(w, "invalid_grant", "Invalid authorization code", http.StatusBadRequest)
		return
	}

	// Validate code hasn't been used
	if authCode.Used {
		writeOAuthError(w, "invalid_grant", "Authorization code already used", http.StatusBadRequest)
		return
	}

	// Validate code hasn't expired
	if time.Now().After(authCode.ExpiresAt) {
		writeOAuthError(w, "invalid_grant", "Authorization code expired", http.StatusBadRequest)
		return
	}

	// Validate client_id matches
	if clientID != "" && clientID != authCode.ClientID {
		writeOAuthError(w, "invalid_grant", "Client mismatch", http.StatusBadRequest)
		return
	}

	// Validate redirect_uri matches
	if redirectURI != "" && redirectURI != authCode.RedirectURI {
		writeOAuthError(w, "invalid_grant", "Redirect URI mismatch", http.StatusBadRequest)
		return
	}

	// Validate PKCE code_verifier if code_challenge was provided
	if authCode.CodeChallenge != "" {
		if codeVerifier == "" {
			writeOAuthError(w, "invalid_request", "Code verifier required", http.StatusBadRequest)
			return
		}
		if !auth.VerifyCodeChallenge(codeVerifier, authCode.CodeChallenge, authCode.CodeChallengeMethod) {
			writeOAuthError(w, "invalid_grant", "Invalid code verifier", http.StatusBadRequest)
			return
		}
	}

	// Mark code as used
	if err := h.repo.MarkAuthorizationCodeUsed(r.Context(), code); err != nil {
		log.Printf("handleAuthorizationCode: Failed to mark code used: %v", err)
		writeOAuthError(w, "server_error", "Internal error", http.StatusInternalServerError)
		return
	}

	h.issueTokens(w, r, authCode.ClientID, authCode.UserID, authCode.Scope)
}

// issueTokens generates and returns access and refresh tokens.
func (h *AuthHandler) issueTokens(w http.ResponseWriter, r *http.Request, clientID, userID, scope string) {
	accessToken, err := auth.GenerateRandomString(32)
	if err != nil {
		writeOAuthError(w, "server_error", "Failed to generate token", http.StatusInternalServerError)
		return
	}

	refreshToken, err := auth.GenerateRandomString(32)
	if err != nil {
		writeOAuthError(w, "server_error", "Failed to generate token", http.StatusInternalServerError)
		return
	}

	expiresIn := 3600 // 1 hour
	expiresAt := time.Now().Add(time.Duration(expiresIn) * time.Second)

	token := &auth.OAuthToken{
		AccessToken:  accessToken,
		RefreshToken: refreshToken,
		ClientID:     clientID,
		UserID:       userID,
		Scope:        scope,
		ExpiresAt:    expiresAt,
	}

	if err := h.repo.CreateOAuthToken(r.Context(), token); err != nil {
		log.Printf("issueTokens: Failed to create token: %v", err)
		writeOAuthError(w, "server_error", "Failed to store token", http.StatusInternalServerError)
		return
	}

	jsonutil.WriteJSON(w, http.StatusOK, OAuthTokenResponse{
		AccessToken:  accessToken,
		TokenType:    "Bearer",
		ExpiresIn:    expiresIn,
		RefreshToken: refreshToken,
		Scope:        scope,
	})
}

// AuthorizeRequest holds the parsed authorize endpoint parameters.
type AuthorizeRequest struct {
	ResponseType        string
	ClientID            string
	RedirectURI         string
	Scope               string
	State               string
	CodeChallenge       string
	CodeChallengeMethod string
}

// AuthorizeHandler handles the /oauth/authorize endpoint for Authorization Code flow.
func (h *AuthHandler) AuthorizeHandler(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet && r.Method != http.MethodPost {
		jsonutil.WriteErrorJSON(w, "Method not allowed")
		return
	}

	// Parse request parameters
	req := AuthorizeRequest{
		ResponseType:        r.FormValue("response_type"),
		ClientID:            r.FormValue("client_id"),
		RedirectURI:         r.FormValue("redirect_uri"),
		Scope:               r.FormValue("scope"),
		State:               r.FormValue("state"),
		CodeChallenge:       r.FormValue("code_challenge"),
		CodeChallengeMethod: r.FormValue("code_challenge_method"),
	}

	// Validate response_type
	if req.ResponseType != "code" {
		writeOAuthError(w, "unsupported_response_type", "Only 'code' response type supported", http.StatusBadRequest)
		return
	}

	// Validate client
	client, err := h.repo.GetClientByID(r.Context(), req.ClientID)
	if err != nil {
		log.Printf("AuthorizeHandler: DB error: %v", err)
		writeOAuthError(w, "server_error", "Internal error", http.StatusInternalServerError)
		return
	}
	if client == nil {
		writeOAuthError(w, "invalid_request", "Unknown client", http.StatusBadRequest)
		return
	}

	// Validate redirect_uri
	if req.RedirectURI == "" {
		writeOAuthError(w, "invalid_request", "Redirect URI required", http.StatusBadRequest)
		return
	}

	validURI, err := h.repo.ValidateRedirectURI(r.Context(), req.ClientID, req.RedirectURI)
	if err != nil {
		log.Printf("AuthorizeHandler: Failed to validate redirect URI: %v", err)
		writeOAuthError(w, "server_error", "Internal error", http.StatusInternalServerError)
		return
	}
	if !validURI {
		writeOAuthError(w, "invalid_request", "Invalid redirect URI", http.StatusBadRequest)
		return
	}

	// Validate PKCE for public clients
	if client.IsPublic && req.CodeChallenge == "" {
		writeOAuthError(w, "invalid_request", "PKCE required for public clients", http.StatusBadRequest)
		return
	}

	// Default code_challenge_method to S256
	if req.CodeChallenge != "" && req.CodeChallengeMethod == "" {
		req.CodeChallengeMethod = "S256"
	}

	// For now, auto-approve (no consent screen) - get user from JWT
	userID, err := extractUserIDFromToken(r)
	if err != nil || userID == "" {
		// Redirect to login or return error
		writeOAuthError(w, "access_denied", "User not authenticated", http.StatusUnauthorized)
		return
	}

	// Generate authorization code
	codeValue, err := auth.GenerateRandomString(32)
	if err != nil {
		writeOAuthError(w, "server_error", "Failed to generate code", http.StatusInternalServerError)
		return
	}

	authCode := &auth.AuthorizationCode{
		Code:                codeValue,
		ClientID:            req.ClientID,
		UserID:              userID,
		RedirectURI:         req.RedirectURI,
		Scope:               req.Scope,
		CodeChallenge:       req.CodeChallenge,
		CodeChallengeMethod: req.CodeChallengeMethod,
		ExpiresAt:           time.Now().Add(10 * time.Minute), // Codes expire in 10 minutes
	}

	if err := h.repo.CreateAuthorizationCode(r.Context(), authCode); err != nil {
		log.Printf("AuthorizeHandler: Failed to create auth code: %v", err)
		writeOAuthError(w, "server_error", "Failed to create authorization code", http.StatusInternalServerError)
		return
	}

	// Build redirect URL with code and state
	redirectURL := req.RedirectURI + "?code=" + codeValue
	if req.State != "" {
		redirectURL += "&state=" + req.State
	}

	http.Redirect(w, r, redirectURL, http.StatusFound)
}

// RegisterClientRequest defines the payload for client registration.
type RegisterClientRequest struct {
	Name         string   `json:"name"`
	RedirectURIs []string `json:"redirect_uris"`
	IsPublic     bool     `json:"is_public"`
}

// RegisterClientResponse returns the new client credentials.
type RegisterClientResponse struct {
	ClientID     string   `json:"client_id"`
	ClientSecret string   `json:"client_secret,omitempty"` // Only shown once, empty for public clients
	Name         string   `json:"name"`
	RedirectURIs []string `json:"redirect_uris"`
	IsPublic     bool     `json:"is_public"`
}

// RegisterClientHandler allows creating new OAuth2 clients.
func (h *AuthHandler) RegisterClientHandler(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		jsonutil.WriteErrorJSON(w, "Method not allowed")
		return
	}

	// Require authentication
	userID, err := extractUserIDFromToken(r)
	if err != nil || userID == "" {
		jsonutil.WriteErrorJSON(w, "Unauthorized")
		return
	}

	var req RegisterClientRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		jsonutil.WriteErrorJSON(w, "Invalid request body")
		return
	}

	if req.Name == "" {
		jsonutil.WriteErrorJSON(w, "Client name required")
		return
	}

	if len(req.RedirectURIs) == 0 {
		jsonutil.WriteErrorJSON(w, "At least one redirect URI required")
		return
	}

	// Generate client ID
	clientID, err := auth.GenerateRandomString(16)
	if err != nil {
		jsonutil.WriteErrorJSON(w, "Failed to generate client ID")
		return
	}

	var clientSecret, clientSecretHash string
	if !req.IsPublic {
		clientSecret, err = auth.GenerateRandomString(32)
		if err != nil {
			jsonutil.WriteErrorJSON(w, "Failed to generate client secret")
			return
		}
		clientSecretHash = auth.HashString(clientSecret)
	}

	client := &auth.OAuthClient{
		ID:               clientID,
		ClientSecretHash: clientSecretHash,
		UserID:           userID,
		Name:             req.Name,
		IsPublic:         req.IsPublic,
	}

	if err := h.repo.CreateOAuthClient(r.Context(), client); err != nil {
		log.Printf("RegisterClientHandler: Failed to create client: %v", err)
		jsonutil.WriteErrorJSON(w, "Failed to create client")
		return
	}

	// Add redirect URIs
	for _, uri := range req.RedirectURIs {
		if err := h.repo.AddRedirectURI(r.Context(), clientID, uri); err != nil {
			log.Printf("RegisterClientHandler: Failed to add redirect URI: %v", err)
		}
	}

	jsonutil.WriteJSON(w, http.StatusCreated, RegisterClientResponse{
		ClientID:     clientID,
		ClientSecret: clientSecret,
		Name:         req.Name,
		RedirectURIs: req.RedirectURIs,
		IsPublic:     req.IsPublic,
	})
}

// TokenIntrospectionRequest defines RFC 7662 introspection request.
type TokenIntrospectionRequest struct {
	Token         string `json:"token"`
	TokenTypeHint string `json:"token_type_hint,omitempty"`
}

// TokenIntrospectionResponse defines RFC 7662 introspection response.
type TokenIntrospectionResponse struct {
	Active    bool   `json:"active"`
	Scope     string `json:"scope,omitempty"`
	ClientID  string `json:"client_id,omitempty"`
	Username  string `json:"username,omitempty"`
	TokenType string `json:"token_type,omitempty"`
	Exp       int64  `json:"exp,omitempty"`
	Iat       int64  `json:"iat,omitempty"`
	Sub       string `json:"sub,omitempty"`
}

// TokenIntrospectionHandler implements RFC 7662 token introspection.
func (h *AuthHandler) TokenIntrospectionHandler(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		jsonutil.WriteErrorJSON(w, "Method not allowed")
		return
	}

	token := r.FormValue("token")
	if token == "" {
		jsonutil.WriteJSON(w, http.StatusOK, TokenIntrospectionResponse{Active: false})
		return
	}

	oauthToken, err := h.repo.ValidateOAuthToken(r.Context(), token)
	if err != nil || oauthToken == nil {
		jsonutil.WriteJSON(w, http.StatusOK, TokenIntrospectionResponse{Active: false})
		return
	}

	jsonutil.WriteJSON(w, http.StatusOK, TokenIntrospectionResponse{
		Active:    true,
		Scope:     oauthToken.Scope,
		ClientID:  oauthToken.ClientID,
		Sub:       oauthToken.UserID,
		TokenType: "Bearer",
		Exp:       oauthToken.ExpiresAt.Unix(),
		Iat:       oauthToken.CreatedAt.Unix(),
	})
}

// writeOAuthError writes an OAuth2-compliant error response.
func writeOAuthError(w http.ResponseWriter, errorCode, description string, statusCode int) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(statusCode)
	_ = json.NewEncoder(w).Encode(map[string]string{
		"error":             errorCode,
		"error_description": description,
	})
}
