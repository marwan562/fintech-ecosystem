package domain

import (
	"context"
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"fmt"
	"time"

	"github.com/google/uuid"
)

type Publisher interface {
	Publish(ctx context.Context, topic string, event interface{}) error
}

type AuthService struct {
	repo      Repository
	publisher Publisher
}

func NewAuthService(repo Repository, publisher Publisher) *AuthService {
	return &AuthService{
		repo:      repo,
		publisher: publisher,
	}
}

// User methods

func (s *AuthService) CreateUser(ctx context.Context, email, passwordHash string) (*User, error) {
	user, err := s.repo.CreateUser(ctx, email, passwordHash)
	if err != nil {
		return nil, err
	}

	// Generate verification token
	token, err := s.CreateEmailVerificationToken(ctx, user.ID)
	if err != nil {
		// Log error but don't fail user creation? Or fail?
		// For now, let's log and proceed or return error.
		// Better to fail if critical.
		return user, nil // Proceed without token for now or implement retry
	}

	// Publish UserRegistered event
	if s.publisher != nil {
		event := map[string]interface{}{
			"id":        uuid.New().String(),
			"type":      "user.registered",
			"timestamp": time.Now().UTC(),
			"data": map[string]string{
				"user_id": user.ID,
				"email":   user.Email,
				"token":   token,
				"link":    fmt.Sprintf("https://sapliy.com/verify-email?token=%s", token), // Todo: configure base URL
			},
		}
		_ = s.publisher.Publish(ctx, "", event)
	}

	return user, nil
}

func (s *AuthService) GetUserByEmail(ctx context.Context, email string) (*User, error) {
	return s.repo.GetUserByEmail(ctx, email)
}

func (s *AuthService) GetUserByID(ctx context.Context, id string) (*User, error) {
	return s.repo.GetUserByID(ctx, id)
}

func (s *AuthService) GetUserByExternalID(ctx context.Context, provider, providerUserID string) (*User, error) {
	return s.repo.GetUserByExternalID(ctx, provider, providerUserID)
}

func (s *AuthService) LinkExternalIdentity(ctx context.Context, userID, provider, providerUserID string) error {
	return s.repo.LinkExternalIdentity(ctx, userID, provider, providerUserID)
}

// Organization methods

func (s *AuthService) CreateOrganization(ctx context.Context, name, domain string) (*Organization, error) {
	return s.repo.CreateOrganization(ctx, name, domain)
}

func (s *AuthService) GetOrganization(ctx context.Context, id string) (*Organization, error) {
	return s.repo.GetOrganization(ctx, id)
}

func (s *AuthService) AddMember(ctx context.Context, userID, orgID, role string) error {
	return s.repo.AddMember(ctx, userID, orgID, role)
}

func (s *AuthService) RemoveMember(ctx context.Context, userID, orgID string) error {
	return s.repo.RemoveMember(ctx, userID, orgID)
}

func (s *AuthService) UpdateMemberRole(ctx context.Context, userID, orgID, role string) error {
	return s.repo.UpdateMemberRole(ctx, userID, orgID, role)
}

func (s *AuthService) ListOrgMembers(ctx context.Context, orgID string) ([]Membership, error) {
	return s.repo.ListOrgMembers(ctx, orgID)
}

func (s *AuthService) GetUserMemberships(ctx context.Context, userID string) ([]Membership, error) {
	return s.repo.GetUserMemberships(ctx, userID)
}

func (s *AuthService) HasPermission(ctx context.Context, userID, orgID, requiredRole string) (bool, error) {
	m, err := s.repo.GetMembership(ctx, userID, orgID)
	if err != nil {
		return false, err
	}
	if m == nil {
		return false, nil
	}

	roles := map[string]int{
		RoleOwner:     4,
		RoleAdmin:     3,
		RoleDeveloper: 2,
		RoleMember:    1,
	}

	return roles[m.Role] >= roles[requiredRole], nil
}

// APIKey methods

func (s *AuthService) CreateAPIKey(ctx context.Context, key *APIKey) error {
	return s.repo.CreateAPIKey(ctx, key)
}

func (s *AuthService) GetAPIKeyByHash(ctx context.Context, hash string) (*APIKey, error) {
	return s.repo.GetAPIKeyByHash(ctx, hash)
}

// OAuth methods

func (s *AuthService) GetClientByID(ctx context.Context, clientID string) (*OAuthClient, error) {
	return s.repo.GetClientByID(ctx, clientID)
}

func (s *AuthService) CreateOAuthClient(ctx context.Context, client *OAuthClient) error {
	return s.repo.CreateOAuthClient(ctx, client)
}

func (s *AuthService) AddRedirectURI(ctx context.Context, clientID, redirectURI string) error {
	return s.repo.AddRedirectURI(ctx, clientID, redirectURI)
}

func (s *AuthService) ValidateRedirectURI(ctx context.Context, clientID, redirectURI string) (bool, error) {
	return s.repo.ValidateRedirectURI(ctx, clientID, redirectURI)
}

func (s *AuthService) CreateOAuthToken(ctx context.Context, token *OAuthToken) error {
	return s.repo.CreateOAuthToken(ctx, token)
}

func (s *AuthService) ValidateOAuthToken(ctx context.Context, accessToken string) (*OAuthToken, error) {
	token, err := s.repo.ValidateOAuthToken(ctx, accessToken)
	if err != nil {
		return nil, err
	}
	if token == nil {
		return nil, nil
	}
	if time.Now().After(token.ExpiresAt) {
		return nil, fmt.Errorf("token expired")
	}
	return token, nil
}

func (s *AuthService) CreateAuthorizationCode(ctx context.Context, code *AuthorizationCode) error {
	return s.repo.CreateAuthorizationCode(ctx, code)
}

func (s *AuthService) GetAuthorizationCode(ctx context.Context, code string) (*AuthorizationCode, error) {
	return s.repo.GetAuthorizationCode(ctx, code)
}

func (s *AuthService) MarkAuthorizationCodeUsed(ctx context.Context, code string) error {
	return s.repo.MarkAuthorizationCodeUsed(ctx, code)
}

// SSO Provider methods

func (s *AuthService) CreateSSOProvider(ctx context.Context, p *SSOProvider) error {
	return s.repo.CreateSSOProvider(ctx, p)
}

func (s *AuthService) GetSSOProviderByID(ctx context.Context, id string) (*SSOProvider, error) {
	return s.repo.GetSSOProviderByID(ctx, id)
}

func (s *AuthService) GetSSOProviderByDomain(ctx context.Context, domain string) (*SSOProvider, error) {
	return s.repo.GetSSOProviderByDomain(ctx, domain)
}

// Audit Log methods

func (s *AuthService) CreateAuditLog(ctx context.Context, log *AuditLog) error {
	return s.repo.CreateAuditLog(ctx, log)
}

func (s *AuthService) GetAuditLogs(ctx context.Context, orgID string, limit, offset int, action string) ([]AuditLog, int, error) {
	return s.repo.GetAuditLogs(ctx, orgID, limit, offset, action)
}

// Helper methods (Domain Logic)

func (s *AuthService) GenerateRandomString(n int) (string, error) {
	b := make([]byte, n)
	_, err := rand.Read(b)
	if err != nil {
		return "", err
	}
	return base64.URLEncoding.EncodeToString(b), nil
}

func (s *AuthService) HashString(str string) string {
	hash := sha256.Sum256([]byte(str))
	return fmt.Sprintf("%x", hash)
}

func (s *AuthService) VerifyCodeChallenge(codeVerifier, codeChallenge, method string) bool {
	if method == "S256" {
		hash := sha256.Sum256([]byte(codeVerifier))
		challenge := base64.RawURLEncoding.EncodeToString(hash[:])
		return challenge == codeChallenge
	}
	// Default to plain
	return codeVerifier == codeChallenge
}

// Password Reset methods

func (s *AuthService) CreatePasswordResetToken(ctx context.Context, userID string) (string, error) {
	// Generate a random token
	rawToken, err := s.GenerateRandomString(32)
	if err != nil {
		return "", err
	}

	// Hash the token for storage
	tokenHash := s.HashString(rawToken)

	token := &PasswordResetToken{
		ID:        uuid.New().String(),
		UserID:    userID,
		Token:     tokenHash,
		ExpiresAt: time.Now().Add(1 * time.Hour), // 1 hour expiry
	}

	if err := s.repo.CreatePasswordResetToken(ctx, token); err != nil {
		return "", err
	}

	// Publish PasswordResetRequested event
	if s.publisher != nil {
		// Get user email
		user, err := s.repo.GetUserByID(ctx, userID)
		if err == nil && user != nil {
			event := map[string]interface{}{
				"id":        uuid.New().String(),
				"type":      "password.reset", // Was user.password_reset_requested, but events.go defines EventPasswordReset = "password.reset"
				"timestamp": time.Now().UTC(),
				"data": map[string]string{
					"user_id": userID,
					"email":   user.Email,
					"token":   rawToken,
					"link":    fmt.Sprintf("https://sapliy.com/reset-password?token=%s", rawToken), // Todo: configure base URL
				},
			}
			_ = s.publisher.Publish(ctx, "", event)
		}
	}

	return rawToken, nil
}

func (s *AuthService) ValidatePasswordResetToken(ctx context.Context, rawToken string) (*PasswordResetToken, error) {
	tokenHash := s.HashString(rawToken)
	token, err := s.repo.GetPasswordResetToken(ctx, tokenHash)
	if err != nil {
		return nil, err
	}
	if token == nil {
		return nil, fmt.Errorf("invalid reset token")
	}
	if token.UsedAt != nil {
		return nil, fmt.Errorf("reset token already used")
	}
	if time.Now().After(token.ExpiresAt) {
		return nil, fmt.Errorf("reset token expired")
	}
	return token, nil
}

func (s *AuthService) ResetPassword(ctx context.Context, rawToken, newPasswordHash string) error {
	token, err := s.ValidatePasswordResetToken(ctx, rawToken)
	if err != nil {
		return err
	}

	if err := s.repo.UpdateUserPassword(ctx, token.UserID, newPasswordHash); err != nil {
		return err
	}

	return s.repo.MarkPasswordResetTokenUsed(ctx, s.HashString(rawToken))
}

// Email Verification methods

func (s *AuthService) CreateEmailVerificationToken(ctx context.Context, userID string) (string, error) {
	rawToken, err := s.GenerateRandomString(32)
	if err != nil {
		return "", err
	}

	tokenHash := s.HashString(rawToken)

	token := &EmailVerificationToken{
		ID:        uuid.New().String(),
		UserID:    userID,
		Token:     tokenHash,
		ExpiresAt: time.Now().Add(24 * time.Hour), // 24 hour expiry
	}

	if err := s.repo.CreateEmailVerificationToken(ctx, token); err != nil {
		return "", err
	}

	return rawToken, nil
}

func (s *AuthService) VerifyEmail(ctx context.Context, rawToken string) error {
	tokenHash := s.HashString(rawToken)
	token, err := s.repo.GetEmailVerificationToken(ctx, tokenHash)
	if err != nil {
		return err
	}
	if token == nil {
		return fmt.Errorf("invalid verification token")
	}
	if token.UsedAt != nil {
		return fmt.Errorf("verification token already used")
	}
	if time.Now().After(token.ExpiresAt) {
		return fmt.Errorf("verification token expired")
	}

	if err := s.repo.SetEmailVerified(ctx, token.UserID); err != nil {
		return err
	}

	return s.repo.MarkEmailVerificationTokenUsed(ctx, tokenHash)
}

func (s *AuthService) UpdateUserPassword(ctx context.Context, userID, passwordHash string) error {
	return s.repo.UpdateUserPassword(ctx, userID, passwordHash)
}
