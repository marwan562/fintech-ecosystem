package api

import (
	"encoding/json"
	"log"
	"net/http"
	"time"

	"github.com/redis/go-redis/v9"
	"github.com/sapliy/sapliy-core/internal/auth/domain"
	"github.com/sapliy/sapliy-core/pkg/apierror"
	"github.com/sapliy/sapliy-core/pkg/apikey"
	"github.com/sapliy/sapliy-core/pkg/authutil"
	"github.com/sapliy/sapliy-core/pkg/bcryptutil"
	"github.com/sapliy/sapliy-core/pkg/jsonutil"
	"github.com/sapliy/sapliy-core/pkg/jwtutil"
)

// AuthHandler holds dependencies for authentication endpoints.
type AuthHandler struct {
	service     *domain.AuthService
	hmacSecret  string
	rdb         *redis.Client
	environment string
}

func NewAuthHandler(service *domain.AuthService, hmacSecret string, rdb *redis.Client, environment string) *AuthHandler {
	return &AuthHandler{
		service:     service,
		hmacSecret:  hmacSecret,
		rdb:         rdb,
		environment: environment,
	}
}

// GenerateAPIKeyRequest defines the payload for key generation.
type GenerateAPIKeyRequest struct {
	ZoneID      string `json:"zone_id"`
	Environment string `json:"environment"` // "test" or "live"
	Type        string `json:"type"`        // "secret" or "publishable"
}

// GenerateAPIKeyResponse defines the key generation response.
type GenerateAPIKeyResponse struct {
	Key          string `json:"key"`
	Environment  string `json:"environment"`
	ZoneID       string `json:"zone_id"`
	Mode         string `json:"mode"`
	Type         string `json:"type"`
	TruncatedKey string `json:"truncated_key"`
}

type OAuthTokenResponse struct {
	AccessToken  string `json:"access_token"`
	TokenType    string `json:"token_type"`
	ExpiresIn    int    `json:"expires_in"`
	RefreshToken string `json:"refresh_token,omitempty"`
	Scope        string `json:"scope,omitempty"`
}

type LoginResponse struct {
	Token string       `json:"token"`
	User  *domain.User `json:"user"`
}

type AuthorizeRequest struct {
	ResponseType        string
	ClientID            string
	RedirectURI         string
	Scope               string
	State               string
	CodeChallenge       string
	CodeChallengeMethod string
}

type RegisterClientRequest struct {
	Name         string   `json:"name"`
	RedirectURIs []string `json:"redirect_uris"`
	IsPublic     bool     `json:"is_public"`
}

type RegisterClientResponse struct {
	ClientID     string   `json:"client_id"`
	ClientSecret string   `json:"client_secret,omitempty"`
	Name         string   `json:"name"`
	RedirectURIs []string `json:"redirect_uris"`
	IsPublic     bool     `json:"is_public"`
}

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

type SSOCallbackRequest struct {
	Provider       string `json:"provider"`
	ProviderUserID string `json:"provider_user_id"`
	Email          string `json:"email"`
}

func (h *AuthHandler) GenerateAPIKey(w http.ResponseWriter, r *http.Request) {
	userID, err := authutil.ExtractUserID(r)
	if err != nil || userID == "" {
		apierror.Unauthorized("Unauthorized").Write(w)
		return
	}

	var req GenerateAPIKeyRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		apierror.BadRequest("Invalid request body").Write(w)
		return
	}

	if req.ZoneID == "" {
		apierror.BadRequest("zone_id is required").Write(w)
		return
	}

	if req.Type == "" {
		req.Type = "secret"
	}

	prefix := "sk_"
	if req.Type == "publishable" {
		prefix = "pk_"
	}
	prefix += req.Environment

	fullKey, hash, err := apikey.GenerateKey(prefix, h.hmacSecret)
	if err != nil {
		apierror.Internal("Failed to generate key").Write(w)
		return
	}

	truncated := fullKey[len(fullKey)-4:]

	key := &domain.APIKey{
		UserID:       userID,
		ZoneID:       req.ZoneID,
		Mode:         req.Environment,
		KeyPrefix:    prefix,
		KeyHash:      hash,
		TruncatedKey: truncated,
		Environment:  req.Environment,
		Type:         req.Type,
	}

	if err := h.service.CreateAPIKey(r.Context(), key); err != nil {
		apierror.Internal("Failed to save API key").Write(w)
		return
	}

	jsonutil.WriteJSON(w, http.StatusCreated, GenerateAPIKeyResponse{
		Key:          fullKey,
		Environment:  req.Environment,
		ZoneID:       req.ZoneID,
		Mode:         req.Environment,
		Type:         req.Type,
		TruncatedKey: truncated,
	})
}

// Register handles user account creation.
func (h *AuthHandler) Register(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		apierror.BadRequest("Method not allowed").Write(w)
		return
	}

	var req struct {
		Email    string `json:"email"`
		Password string `json:"password"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		apierror.BadRequest("Invalid request body").Write(w)
		return
	}

	if req.Email == "" || req.Password == "" {
		apierror.BadRequest("Email and password are required").Write(w)
		return
	}

	b := &bcryptutil.BcryptUtilsImpl{}
	passwordHash, err := b.GenerateHash(req.Password)
	if err != nil {
		apierror.Internal("Failed to hash password").Write(w)
		return
	}

	user, err := h.service.CreateUser(r.Context(), req.Email, passwordHash)
	if err != nil {
		apierror.Conflict("Email already taken").Write(w)
		return
	}

	// Internal token creation for verification (logged but not failing request)
	// ONLY enabled in non-production environments
	if h.environment != "production" {
		token, _ := h.service.CreateEmailVerificationToken(r.Context(), user.ID)
		if token != "" && h.rdb != nil {
			h.rdb.Set(r.Context(), "debug:verify:"+user.Email, token, 1*time.Hour)
		}
	}

	// Auto-login after registration
	accessToken, _ := jwtutil.GenerateToken(user.ID, user.Email)
	refreshToken, _ := h.service.CreateRefreshToken(r.Context(), user.ID)

	if accessToken != "" && refreshToken != "" {
		h.setAuthCookies(w, accessToken, refreshToken)
	}

	user.Password = ""
	jsonutil.WriteJSON(w, http.StatusCreated, map[string]interface{}{
		"token": accessToken,
		"user":  user,
	})
}

// Login handles user authentication and JWT issuance.
func (h *AuthHandler) Login(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		apierror.BadRequest("Method not allowed").Write(w)
		return
	}

	var req struct {
		Email    string `json:"email"`
		Password string `json:"password"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		apierror.BadRequest("Invalid request body").Write(w)
		return
	}

	user, err := h.service.GetUserByEmail(r.Context(), req.Email)
	if err != nil || user == nil {
		apierror.Unauthorized("Invalid email or password").Write(w)
		return
	}

	b := &bcryptutil.BcryptUtilsImpl{}
	if !b.CompareHash(req.Password, user.Password) {
		apierror.Unauthorized("Invalid email or password").Write(w)
		return
	}

	accessToken, err := jwtutil.GenerateToken(user.ID, user.Email)
	refreshToken, err2 := h.service.CreateRefreshToken(r.Context(), user.ID)
	if err != nil || err2 != nil {
		apierror.Internal("Failed to create session").Write(w)
		return
	}

	h.setAuthCookies(w, accessToken, refreshToken)

	user.Password = ""
	jsonutil.WriteJSON(w, http.StatusOK, map[string]interface{}{
		"token": accessToken,
		"user":  user,
	})
}

func (h *AuthHandler) setAuthCookies(w http.ResponseWriter, access, refresh string) {
	http.SetCookie(w, &http.Cookie{
		Name:     "auth-token",
		Value:    access,
		HttpOnly: true,
		Secure:   true,
		Path:     "/",
		Expires:  time.Now().Add(15 * time.Minute),
		SameSite: http.SameSiteLaxMode,
	})
	http.SetCookie(w, &http.Cookie{
		Name:     "refreshtoken",
		Value:    refresh,
		HttpOnly: true,
		Secure:   true,
		Path:     "/",
		Expires:  time.Now().Add(7 * 24 * time.Hour),
		SameSite: http.SameSiteLaxMode,
	})
}

func (h *AuthHandler) Refresh(w http.ResponseWriter, r *http.Request) {
	cookie, err := r.Cookie("refreshtoken")
	if err != nil {
		apierror.Unauthorized("No refresh token").Write(w)
		return
	}

	refreshToken, err := h.service.ValidateRefreshToken(r.Context(), cookie.Value)
	if err != nil || refreshToken == nil {
		h.setAuthCookies(w, "", "")
		apierror.Unauthorized("Invalid or expired refresh token").Write(w)
		return
	}

	user, err := h.service.GetUserByID(r.Context(), refreshToken.UserID)
	if err != nil || user == nil {
		apierror.NotFound("User not found").Write(w)
		return
	}

	accessToken, err := jwtutil.GenerateToken(user.ID, user.Email)
	if err != nil {
		apierror.Internal("Failed to generate token").Write(w)
		return
	}

	h.setAuthCookies(w, accessToken, cookie.Value)
	jsonutil.WriteJSON(w, http.StatusOK, map[string]string{"token": accessToken})
}

func (h *AuthHandler) CreateOrganization(w http.ResponseWriter, r *http.Request) {
	userID, err := authutil.ExtractUserID(r)
	if err != nil || userID == "" {
		apierror.Unauthorized("Unauthorized").Write(w)
		return
	}

	var req struct {
		Name   string `json:"name"`
		Domain string `json:"domain"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		apierror.BadRequest("Invalid request body").Write(w)
		return
	}

	org, err := h.service.CreateOrganization(r.Context(), req.Name, req.Domain)
	if err != nil {
		apierror.Internal("Failed to create organization").Write(w)
		return
	}

	_ = h.service.AddMember(r.Context(), userID, org.ID, domain.RoleOwner)
	jsonutil.WriteJSON(w, http.StatusCreated, org)
}

func (h *AuthHandler) ValidateAPIKey(w http.ResponseWriter, r *http.Request) {
	var req struct {
		KeyHash string `json:"key_hash"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		apierror.BadRequest("Invalid request body").Write(w)
		return
	}

	key, err := h.service.GetAPIKeyByHash(r.Context(), req.KeyHash)
	if err != nil || key == nil || key.RevokedAt != nil {
		jsonutil.WriteJSON(w, http.StatusOK, map[string]interface{}{"valid": false})
		return
	}

	jsonutil.WriteJSON(w, http.StatusOK, map[string]interface{}{
		"valid":       true,
		"user_id":     key.UserID,
		"org_id":      key.OrgID,
		"zone_id":     key.ZoneID,
		"mode":        key.Mode,
		"environment": key.Environment,
		"scopes":      key.Scopes,
		"type":        key.Type,
	})
}

func (h *AuthHandler) OAuthTokenHandler(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		apierror.BadRequest("Method not allowed").Write(w)
		return
	}

	grantType := r.FormValue("grant_type")

	switch grantType {
	case "client_credentials":
		h.handleClientCredentials(w, r)
	case "authorization_code":
		h.handleAuthorizationCode(w, r)
	default:
		h.writeOAuthError(w, "unsupported_grant_type", "Grant type not supported", http.StatusBadRequest)
	}
}

func (h *AuthHandler) handleClientCredentials(w http.ResponseWriter, r *http.Request) {
	clientID, clientSecret, ok := r.BasicAuth()
	if !ok {
		clientID = r.FormValue("client_id")
		clientSecret = r.FormValue("client_secret")
	}

	if clientID == "" || clientSecret == "" {
		w.Header().Set("WWW-Authenticate", `Basic realm="api"`)
		h.writeOAuthError(w, "invalid_client", "Client credentials required", http.StatusUnauthorized)
		return
	}

	client, err := h.service.GetClientByID(r.Context(), clientID)
	if err != nil {
		log.Printf("handleClientCredentials: DB error: %v", err)
		h.writeOAuthError(w, "server_error", "Internal error", http.StatusInternalServerError)
		return
	}
	if client == nil {
		h.writeOAuthError(w, "invalid_client", "Unknown client", http.StatusUnauthorized)
		return
	}

	incomingHash := h.service.HashString(clientSecret)
	if incomingHash != client.ClientSecretHash {
		h.writeOAuthError(w, "invalid_client", "Invalid credentials", http.StatusUnauthorized)
		return
	}

	scope := r.FormValue("scope")
	if scope == "" {
		scope = "default"
	}

	h.issueTokens(w, r, client.ID, client.UserID, scope)
}

func (h *AuthHandler) handleAuthorizationCode(w http.ResponseWriter, r *http.Request) {
	code := r.FormValue("code")
	redirectURI := r.FormValue("redirect_uri")
	codeVerifier := r.FormValue("code_verifier")
	clientID := r.FormValue("client_id")

	if code == "" {
		h.writeOAuthError(w, "invalid_request", "Authorization code required", http.StatusBadRequest)
		return
	}

	authCode, err := h.service.GetAuthorizationCode(r.Context(), code)
	if err != nil {
		log.Printf("handleAuthorizationCode: DB error: %v", err)
		h.writeOAuthError(w, "server_error", "Internal error", http.StatusInternalServerError)
		return
	}
	if authCode == nil {
		h.writeOAuthError(w, "invalid_grant", "Invalid authorization code", http.StatusBadRequest)
		return
	}

	if authCode.Used {
		h.writeOAuthError(w, "invalid_grant", "Authorization code already used", http.StatusBadRequest)
		return
	}

	if time.Now().After(authCode.ExpiresAt) {
		h.writeOAuthError(w, "invalid_grant", "Authorization code expired", http.StatusBadRequest)
		return
	}

	if clientID != "" && clientID != authCode.ClientID {
		h.writeOAuthError(w, "invalid_grant", "Client mismatch", http.StatusBadRequest)
		return
	}

	if redirectURI != "" && redirectURI != authCode.RedirectURI {
		h.writeOAuthError(w, "invalid_grant", "Redirect URI mismatch", http.StatusBadRequest)
		return
	}

	if authCode.CodeChallenge != "" {
		if codeVerifier == "" {
			h.writeOAuthError(w, "invalid_request", "Code verifier required", http.StatusBadRequest)
			return
		}
		if !h.service.VerifyCodeChallenge(codeVerifier, authCode.CodeChallenge, authCode.CodeChallengeMethod) {
			h.writeOAuthError(w, "invalid_grant", "Invalid code verifier", http.StatusBadRequest)
			return
		}
	}

	if err := h.service.MarkAuthorizationCodeUsed(r.Context(), code); err != nil {
		log.Printf("handleAuthorizationCode: Failed to mark code used: %v", err)
		h.writeOAuthError(w, "server_error", "Internal error", http.StatusInternalServerError)
		return
	}

	h.issueTokens(w, r, authCode.ClientID, authCode.UserID, authCode.Scope)
}

func (h *AuthHandler) issueTokens(w http.ResponseWriter, r *http.Request, clientID, userID, scope string) {
	accessToken, err := h.service.GenerateRandomString(32)
	if err != nil {
		h.writeOAuthError(w, "server_error", "Failed to generate token", http.StatusInternalServerError)
		return
	}

	refreshToken, err := h.service.GenerateRandomString(32)
	if err != nil {
		h.writeOAuthError(w, "server_error", "Failed to generate token", http.StatusInternalServerError)
		return
	}

	expiresIn := 3600 // 1 hour
	expiresAt := time.Now().Add(time.Duration(expiresIn) * time.Second)

	token := &domain.OAuthToken{
		AccessToken:  accessToken,
		RefreshToken: refreshToken,
		ClientID:     clientID,
		UserID:       userID,
		Scope:        scope,
		ExpiresAt:    expiresAt,
	}

	if err := h.service.CreateOAuthToken(r.Context(), token); err != nil {
		log.Printf("issueTokens: Failed to create token: %v", err)
		h.writeOAuthError(w, "server_error", "Failed to store token", http.StatusInternalServerError)
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

func (h *AuthHandler) AuthorizeHandler(w http.ResponseWriter, r *http.Request) {
	req := AuthorizeRequest{
		ResponseType:        r.FormValue("response_type"),
		ClientID:            r.FormValue("client_id"),
		RedirectURI:         r.FormValue("redirect_uri"),
		Scope:               r.FormValue("scope"),
		State:               r.FormValue("state"),
		CodeChallenge:       r.FormValue("code_challenge"),
		CodeChallengeMethod: r.FormValue("code_challenge_method"),
	}

	if req.ResponseType != "code" {
		h.writeOAuthError(w, "unsupported_response_type", "Only 'code' response type supported", http.StatusBadRequest)
		return
	}

	client, err := h.service.GetClientByID(r.Context(), req.ClientID)
	if err != nil {
		log.Printf("AuthorizeHandler: DB error: %v", err)
		h.writeOAuthError(w, "server_error", "Internal error", http.StatusInternalServerError)
		return
	}
	if client == nil {
		h.writeOAuthError(w, "invalid_request", "Unknown client", http.StatusBadRequest)
		return
	}

	if req.RedirectURI == "" {
		h.writeOAuthError(w, "invalid_request", "Redirect URI required", http.StatusBadRequest)
		return
	}

	validURI, err := h.service.ValidateRedirectURI(r.Context(), req.ClientID, req.RedirectURI)
	if err != nil {
		log.Printf("AuthorizeHandler: Failed to validate redirect URI: %v", err)
		h.writeOAuthError(w, "server_error", "Internal error", http.StatusInternalServerError)
		return
	}
	if !validURI {
		h.writeOAuthError(w, "invalid_request", "Invalid redirect URI", http.StatusBadRequest)
		return
	}

	if client.IsPublic && req.CodeChallenge == "" {
		h.writeOAuthError(w, "invalid_request", "PKCE required for public clients", http.StatusBadRequest)
		return
	}

	if req.CodeChallenge != "" && req.CodeChallengeMethod == "" {
		req.CodeChallengeMethod = "S256"
	}

	userID, err := authutil.ExtractUserID(r)
	if err != nil || userID == "" {
		h.writeOAuthError(w, "access_denied", "User not authenticated", http.StatusUnauthorized)
		return
	}

	codeValue, err := h.service.GenerateRandomString(32)
	if err != nil {
		h.writeOAuthError(w, "server_error", "Failed to generate code", http.StatusInternalServerError)
		return
	}

	authCode := &domain.AuthorizationCode{
		Code:                codeValue,
		ClientID:            req.ClientID,
		UserID:              userID,
		RedirectURI:         req.RedirectURI,
		Scope:               req.Scope,
		CodeChallenge:       req.CodeChallenge,
		CodeChallengeMethod: req.CodeChallengeMethod,
		ExpiresAt:           time.Now().Add(10 * time.Minute),
	}

	if err := h.service.CreateAuthorizationCode(r.Context(), authCode); err != nil {
		log.Printf("AuthorizeHandler: Failed to create auth code: %v", err)
		h.writeOAuthError(w, "server_error", "Failed to create authorization code", http.StatusInternalServerError)
		return
	}

	redirectURL := req.RedirectURI + "?code=" + codeValue
	if req.State != "" {
		redirectURL += "&state=" + req.State
	}

	http.Redirect(w, r, redirectURL, http.StatusFound)
}

func (h *AuthHandler) RegisterClientHandler(w http.ResponseWriter, r *http.Request) {
	userID, err := authutil.ExtractUserID(r)
	if err != nil || userID == "" {
		apierror.Unauthorized("Unauthorized").Write(w)
		return
	}

	var req RegisterClientRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		apierror.BadRequest("Invalid request body").Write(w)
		return
	}

	if req.Name == "" {
		apierror.BadRequest("Client name required").Write(w)
		return
	}

	if len(req.RedirectURIs) == 0 {
		apierror.BadRequest("At least one redirect URI required").Write(w)
		return
	}

	clientID, err := h.service.GenerateRandomString(16)
	if err != nil {
		apierror.Internal("Failed to generate client ID").Write(w)
		return
	}

	var clientSecret, clientSecretHash string
	if !req.IsPublic {
		clientSecret, err = h.service.GenerateRandomString(32)
		if err != nil {
			apierror.Internal("Failed to generate client secret").Write(w)
			return
		}
		clientSecretHash = h.service.HashString(clientSecret)
	}

	client := &domain.OAuthClient{
		ID:               clientID,
		ClientSecretHash: clientSecretHash,
		UserID:           userID,
		Name:             req.Name,
		IsPublic:         req.IsPublic,
	}

	if err := h.service.CreateOAuthClient(r.Context(), client); err != nil {
		log.Printf("RegisterClientHandler: Failed to create client: %v", err)
		apierror.Internal("Failed to create client").Write(w)
		return
	}

	for _, uri := range req.RedirectURIs {
		if err := h.service.AddRedirectURI(r.Context(), clientID, uri); err != nil {
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

func (h *AuthHandler) TokenIntrospectionHandler(w http.ResponseWriter, r *http.Request) {
	token := r.FormValue("token")
	if token == "" {
		jsonutil.WriteJSON(w, http.StatusOK, TokenIntrospectionResponse{Active: false})
		return
	}

	oauthToken, err := h.service.ValidateOAuthToken(r.Context(), token)
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

func (h *AuthHandler) SSOCallback(w http.ResponseWriter, r *http.Request) {
	var req SSOCallbackRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		apierror.BadRequest("Invalid request body").Write(w)
		return
	}

	user, err := h.service.GetUserByExternalID(r.Context(), req.Provider, req.ProviderUserID)
	if err != nil {
		log.Printf("SSOCallback: DB error: %v", err)
		apierror.Internal("Internal server error").Write(w)
		return
	}

	if user == nil {
		user, err = h.service.CreateUser(r.Context(), req.Email, "SSO_MANAGED")
		if err != nil {
			log.Printf("SSOCallback: Failed to create user: %v", err)
			apierror.Internal("Failed to provision user").Write(w)
			return
		}

		if err := h.service.LinkExternalIdentity(r.Context(), user.ID, req.Provider, req.ProviderUserID); err != nil {
			log.Printf("SSOCallback: Failed to link identity: %v", err)
		}
	}

	token, err := jwtutil.GenerateToken(user.ID, user.Email)
	if err != nil {
		apierror.Internal("Failed to generate token").Write(w)
		return
	}

	user.Password = ""
	jsonutil.WriteJSON(w, http.StatusOK, LoginResponse{Token: token, User: user})
}

func (h *AuthHandler) writeOAuthError(w http.ResponseWriter, errorCode, description string, statusCode int) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(statusCode)
	_ = json.NewEncoder(w).Encode(map[string]string{
		"error":             errorCode,
		"error_description": description,
	})
}

func (h *AuthHandler) TriggerEvent(w http.ResponseWriter, r *http.Request) {
	var req struct {
		Type   string                 `json:"type"`
		ZoneID string                 `json:"zone_id"`
		Data   map[string]interface{} `json:"data"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		apierror.BadRequest("Invalid request body").Write(w)
		return
	}

	if req.Type == "" || req.ZoneID == "" {
		apierror.BadRequest("Type and ZoneID are required").Write(w)
		return
	}

	log.Printf("Manually triggering event %s for zone %s", req.Type, req.ZoneID)
	jsonutil.WriteJSON(w, http.StatusAccepted, map[string]string{"status": "event_queued"})
}

func (h *AuthHandler) ForgotPassword(w http.ResponseWriter, r *http.Request) {
	var req struct {
		Email string `json:"email"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		apierror.BadRequest("Invalid request").Write(w)
		return
	}
	user, err := h.service.GetUserByEmail(r.Context(), req.Email)
	if err != nil || user == nil {
		jsonutil.WriteJSON(w, http.StatusOK, map[string]string{"message": "Check your email."})
		return
	}
	// ONLY enabled in non-production environments
	if h.environment != "production" {
		token, _ := h.service.CreatePasswordResetToken(r.Context(), user.ID)
		if token != "" && h.rdb != nil {
			h.rdb.Set(r.Context(), "debug:reset:"+user.Email, token, 1*time.Hour)
		}
	}
	jsonutil.WriteJSON(w, http.StatusOK, map[string]string{"message": "Check your email."})
}

func (h *AuthHandler) ResetPassword(w http.ResponseWriter, r *http.Request) {
	var req struct {
		Token       string `json:"token"`
		NewPassword string `json:"new_password"`
	}
	json.NewDecoder(r.Body).Decode(&req)
	b := &bcryptutil.BcryptUtilsImpl{}
	hashed, _ := b.GenerateHash(req.NewPassword)
	if err := h.service.ResetPassword(r.Context(), req.Token, hashed); err != nil {
		apierror.BadRequest("Invalid token").Write(w)
		return
	}
	jsonutil.WriteJSON(w, http.StatusOK, map[string]string{"message": "Success"})
}

func (h *AuthHandler) VerifyEmail(w http.ResponseWriter, r *http.Request) {
	var req struct {
		Token string `json:"token"`
	}
	json.NewDecoder(r.Body).Decode(&req)
	if err := h.service.VerifyEmail(r.Context(), req.Token); err != nil {
		apierror.BadRequest("Invalid token").Write(w)
		return
	}
	jsonutil.WriteJSON(w, http.StatusOK, map[string]string{"message": "Verified"})
}

func (h *AuthHandler) ResendVerification(w http.ResponseWriter, r *http.Request) {
	var req struct {
		Email string `json:"email"`
	}
	json.NewDecoder(r.Body).Decode(&req)
	user, _ := h.service.GetUserByEmail(r.Context(), req.Email)
	if user != nil {
		token, _ := h.service.ResendEmailVerification(r.Context(), user.ID)
		if token != "" && h.rdb != nil {
			h.rdb.Set(r.Context(), "debug:verify:"+user.Email, token, 1*time.Hour)
		}
	}
	jsonutil.WriteJSON(w, http.StatusOK, map[string]string{"message": "Sent"})
}

func (h *AuthHandler) Logout(w http.ResponseWriter, r *http.Request) {
	cookie, err := r.Cookie("refreshtoken")
	if err == nil {
		token, err := h.service.ValidateRefreshToken(r.Context(), cookie.Value)
		if err == nil && token != nil {
			_ = h.service.RevokeRefreshToken(r.Context(), token.ID)
		}
	}

	// Clear cookies
	http.SetCookie(w, &http.Cookie{Name: "auth-token", Value: "", Path: "/", MaxAge: -1})
	http.SetCookie(w, &http.Cookie{Name: "refreshtoken", Value: "", Path: "/", MaxAge: -1})

	jsonutil.WriteJSON(w, http.StatusOK, map[string]string{"message": "Logged out"})
}

func (h *AuthHandler) DebugGetTokens(w http.ResponseWriter, r *http.Request) {
	email := r.URL.Query().Get("email")
	tokenType := r.URL.Query().Get("type")
	token, _ := h.rdb.Get(r.Context(), "debug:"+tokenType+":"+email).Result()
	jsonutil.WriteJSON(w, http.StatusOK, map[string]string{"token": token})
}
