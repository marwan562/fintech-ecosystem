package infrastructure

import (
	"context"
	"database/sql"
	"errors"
	"fmt"

	"github.com/sapliy/sapliy-core/internal/auth/domain"
)

type SQLRepository struct {
	db *sql.DB
}

func toNullString(s string) sql.NullString {
	if s == "" {
		return sql.NullString{Valid: false}
	}
	return sql.NullString{String: s, Valid: true}
}

func NewSQLRepository(db *sql.DB) *SQLRepository {
	return &SQLRepository{db: db}
}

// User methods

func (r *SQLRepository) CreateUser(ctx context.Context, email, passwordHash string) (*domain.User, error) {
	var user domain.User
	err := r.db.QueryRowContext(ctx,
		"INSERT INTO users (email, password_hash) VALUES ($1, $2) RETURNING id, email, created_at",
		email, passwordHash).Scan(&user.ID, &user.Email, &user.CreatedAt)

	if err != nil {
		return nil, fmt.Errorf("failed to insert user: %w", err)
	}
	return &user, nil
}

func (r *SQLRepository) GetUserByEmail(ctx context.Context, email string) (*domain.User, error) {
	var user domain.User
	err := r.db.QueryRowContext(ctx,
		"SELECT id, email, password_hash, created_at FROM users WHERE email = $1",
		email).Scan(&user.ID, &user.Email, &user.Password, &user.CreatedAt)

	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, nil
		}
		return nil, fmt.Errorf("failed to get user: %w", err)
	}
	return &user, nil
}

func (r *SQLRepository) GetUserByID(ctx context.Context, id string) (*domain.User, error) {
	var user domain.User
	err := r.db.QueryRowContext(ctx,
		"SELECT id, email, created_at FROM users WHERE id = $1",
		id).Scan(&user.ID, &user.Email, &user.CreatedAt)

	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, nil
		}
		return nil, fmt.Errorf("failed to get user: %w", err)
	}
	return &user, nil
}

func (r *SQLRepository) GetUserByExternalID(ctx context.Context, provider, providerUserID string) (*domain.User, error) {
	var user domain.User
	var orgID sql.NullString
	err := r.db.QueryRowContext(ctx,
		`SELECT u.id, u.email, u.org_id, u.created_at 
		 FROM users u
		 JOIN external_identities e ON u.id = e.user_id
		 WHERE e.provider = $1 AND e.provider_user_id = $2`,
		provider, providerUserID).Scan(&user.ID, &user.Email, &orgID, &user.CreatedAt)
	user.OrgID = orgID.String

	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, nil
		}
		return nil, fmt.Errorf("failed to get user by external id: %w", err)
	}
	return &user, nil
}

func (r *SQLRepository) LinkExternalIdentity(ctx context.Context, userID, provider, providerUserID string) error {
	_, err := r.db.ExecContext(ctx,
		`INSERT INTO external_identities (user_id, provider, provider_user_id) 
		 VALUES ($1, $2, $3) ON CONFLICT (provider, provider_user_id) DO UPDATE SET user_id = EXCLUDED.user_id`,
		userID, provider, providerUserID)
	return err
}

func (r *SQLRepository) UpdateUserPassword(ctx context.Context, userID, passwordHash string) error {
	result, err := r.db.ExecContext(ctx,
		`UPDATE users SET password_hash = $1 WHERE id = $2`,
		passwordHash, userID)
	if err != nil {
		return fmt.Errorf("failed to update password: %w", err)
	}
	rows, _ := result.RowsAffected()
	if rows == 0 {
		return fmt.Errorf("user not found")
	}
	return nil
}

func (r *SQLRepository) SetEmailVerified(ctx context.Context, userID string) error {
	result, err := r.db.ExecContext(ctx,
		`UPDATE users SET email_verified = TRUE, email_verified_at = NOW() WHERE id = $1`,
		userID)
	if err != nil {
		return fmt.Errorf("failed to set email verified: %w", err)
	}
	rows, _ := result.RowsAffected()
	if rows == 0 {
		return fmt.Errorf("user not found")
	}
	return nil
}

// Password Reset Token methods

func (r *SQLRepository) CreatePasswordResetToken(ctx context.Context, token *domain.PasswordResetToken) error {
	_, err := r.db.ExecContext(ctx,
		`INSERT INTO password_reset_tokens (id, user_id, token_hash, expires_at) VALUES ($1, $2, $3, $4)`,
		token.ID, token.UserID, token.Token, token.ExpiresAt)
	if err != nil {
		return fmt.Errorf("failed to create password reset token: %w", err)
	}
	return nil
}

func (r *SQLRepository) GetPasswordResetToken(ctx context.Context, tokenHash string) (*domain.PasswordResetToken, error) {
	var token domain.PasswordResetToken
	err := r.db.QueryRowContext(ctx,
		`SELECT id, user_id, token_hash, expires_at, used_at, created_at FROM password_reset_tokens WHERE token_hash = $1`,
		tokenHash).Scan(&token.ID, &token.UserID, &token.Token, &token.ExpiresAt, &token.UsedAt, &token.CreatedAt)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, nil
		}
		return nil, fmt.Errorf("failed to get password reset token: %w", err)
	}
	return &token, nil
}

func (r *SQLRepository) MarkPasswordResetTokenUsed(ctx context.Context, tokenHash string) error {
	result, err := r.db.ExecContext(ctx,
		`UPDATE password_reset_tokens SET used_at = NOW() WHERE token_hash = $1 AND used_at IS NULL`,
		tokenHash)
	if err != nil {
		return fmt.Errorf("failed to mark token used: %w", err)
	}
	rows, _ := result.RowsAffected()
	if rows == 0 {
		return fmt.Errorf("token already used or not found")
	}
	return nil
}

// Email Verification Token methods

func (r *SQLRepository) CreateEmailVerificationToken(ctx context.Context, token *domain.EmailVerificationToken) error {
	_, err := r.db.ExecContext(ctx,
		`INSERT INTO email_verification_tokens (id, user_id, token_hash, expires_at) VALUES ($1, $2, $3, $4)`,
		token.ID, token.UserID, token.Token, token.ExpiresAt)
	if err != nil {
		return fmt.Errorf("failed to create email verification token: %w", err)
	}
	return nil
}

func (r *SQLRepository) GetEmailVerificationToken(ctx context.Context, tokenHash string) (*domain.EmailVerificationToken, error) {
	var token domain.EmailVerificationToken
	err := r.db.QueryRowContext(ctx,
		`SELECT id, user_id, token_hash, expires_at, used_at, created_at FROM email_verification_tokens WHERE token_hash = $1`,
		tokenHash).Scan(&token.ID, &token.UserID, &token.Token, &token.ExpiresAt, &token.UsedAt, &token.CreatedAt)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, nil
		}
		return nil, fmt.Errorf("failed to get email verification token: %w", err)
	}
	return &token, nil
}

func (r *SQLRepository) MarkEmailVerificationTokenUsed(ctx context.Context, tokenHash string) error {
	result, err := r.db.ExecContext(ctx,
		`UPDATE email_verification_tokens SET used_at = NOW() WHERE token_hash = $1 AND used_at IS NULL`,
		tokenHash)
	if err != nil {
		return fmt.Errorf("failed to mark token used: %w", err)
	}
	rows, _ := result.RowsAffected()
	if rows == 0 {
		return fmt.Errorf("token already used or not found")
	}
	return nil
}

// RefreshToken methods

func (r *SQLRepository) CreateRefreshToken(ctx context.Context, token *domain.RefreshToken) error {
	_, err := r.db.ExecContext(ctx,
		`INSERT INTO refresh_tokens (id, user_id, token_hash, expires_at, revoked) VALUES ($1, $2, $3, $4, $5)`,
		token.ID, token.UserID, token.TokenHash, token.ExpiresAt, token.Revoked)
	if err != nil {
		return fmt.Errorf("failed to create refresh token: %w", err)
	}
	return nil
}

func (r *SQLRepository) GetRefreshToken(ctx context.Context, tokenHash string) (*domain.RefreshToken, error) {
	var token domain.RefreshToken
	err := r.db.QueryRowContext(ctx,
		`SELECT id, user_id, token_hash, expires_at, revoked, created_at FROM refresh_tokens WHERE token_hash = $1`,
		tokenHash).Scan(&token.ID, &token.UserID, &token.TokenHash, &token.ExpiresAt, &token.Revoked, &token.CreatedAt)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, nil
		}
		return nil, fmt.Errorf("failed to get refresh token: %w", err)
	}
	return &token, nil
}

func (r *SQLRepository) RevokeRefreshToken(ctx context.Context, id string) error {
	result, err := r.db.ExecContext(ctx,
		`UPDATE refresh_tokens SET revoked = TRUE WHERE id = $1`,
		id)
	if err != nil {
		return fmt.Errorf("failed to revoke refresh token: %w", err)
	}
	rows, _ := result.RowsAffected()
	if rows == 0 {
		return fmt.Errorf("refresh token not found")
	}
	return nil
}

// Organization methods

func (r *SQLRepository) CreateOrganization(ctx context.Context, name, domainName string) (*domain.Organization, error) {
	var org domain.Organization
	err := r.db.QueryRowContext(ctx,
		"INSERT INTO organizations (name, domain) VALUES ($1, $2) RETURNING id, name, domain, created_at",
		name, domainName).Scan(&org.ID, &org.Name, &org.Domain, &org.CreatedAt)

	if err != nil {
		return nil, fmt.Errorf("failed to create organization: %w", err)
	}
	return &org, nil
}

func (r *SQLRepository) GetOrganization(ctx context.Context, id string) (*domain.Organization, error) {
	var org domain.Organization
	err := r.db.QueryRowContext(ctx,
		"SELECT id, name, domain, created_at FROM organizations WHERE id = $1",
		id).Scan(&org.ID, &org.Name, &org.Domain, &org.CreatedAt)

	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, nil
		}
		return nil, fmt.Errorf("failed to get organization: %w", err)
	}
	return &org, nil
}

func (r *SQLRepository) AddMember(ctx context.Context, userID, orgID, role string) error {
	_, err := r.db.ExecContext(ctx,
		"INSERT INTO memberships (user_id, org_id, role) VALUES ($1, $2, $3) ON CONFLICT DO NOTHING",
		userID, orgID, role)
	return err
}

func (r *SQLRepository) RemoveMember(ctx context.Context, userID, orgID string) error {
	result, err := r.db.ExecContext(ctx,
		"DELETE FROM memberships WHERE user_id = $1 AND org_id = $2",
		userID, orgID)
	if err != nil {
		return err
	}
	rows, _ := result.RowsAffected()
	if rows == 0 {
		return fmt.Errorf("membership not found")
	}
	return nil
}

func (r *SQLRepository) UpdateMemberRole(ctx context.Context, userID, orgID, role string) error {
	result, err := r.db.ExecContext(ctx,
		"UPDATE memberships SET role = $1 WHERE user_id = $2 AND org_id = $3",
		role, userID, orgID)
	if err != nil {
		return err
	}
	rows, _ := result.RowsAffected()
	if rows == 0 {
		return fmt.Errorf("membership not found")
	}
	return nil
}

func (r *SQLRepository) ListOrgMembers(ctx context.Context, orgID string) ([]domain.Membership, error) {
	rows, err := r.db.QueryContext(ctx,
		"SELECT user_id, org_id, role, created_at FROM memberships WHERE org_id = $1",
		orgID)
	if err != nil {
		return nil, err
	}
	defer func() { _ = rows.Close() }()

	var memberships []domain.Membership
	for rows.Next() {
		var m domain.Membership
		if err := rows.Scan(&m.UserID, &m.OrgID, &m.Role, &m.CreatedAt); err != nil {
			return nil, err
		}
		memberships = append(memberships, m)
	}
	return memberships, nil
}

func (r *SQLRepository) GetUserMemberships(ctx context.Context, userID string) ([]domain.Membership, error) {
	rows, err := r.db.QueryContext(ctx,
		"SELECT user_id, org_id, role, created_at FROM memberships WHERE user_id = $1",
		userID)
	if err != nil {
		return nil, err
	}
	defer func() { _ = rows.Close() }()

	var memberships []domain.Membership
	for rows.Next() {
		var m domain.Membership
		if err := rows.Scan(&m.UserID, &m.OrgID, &m.Role, &m.CreatedAt); err != nil {
			return nil, err
		}
		memberships = append(memberships, m)
	}
	return memberships, nil
}

func (r *SQLRepository) GetMembership(ctx context.Context, userID, orgID string) (*domain.Membership, error) {
	var m domain.Membership
	err := r.db.QueryRowContext(ctx,
		"SELECT user_id, org_id, role, created_at FROM memberships WHERE user_id = $1 AND org_id = $2",
		userID, orgID).Scan(&m.UserID, &m.OrgID, &m.Role, &m.CreatedAt)

	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, nil
		}
		return nil, err
	}
	return &m, nil
}

// APIKey methods

func (r *SQLRepository) CreateAPIKey(ctx context.Context, key *domain.APIKey) error {
	if key.Scopes == "" {
		key.Scopes = "*"
	}
	err := r.db.QueryRowContext(ctx,
		`INSERT INTO api_keys (user_id, org_id, zone_id, mode, key_prefix, key_hash, truncated_key, environment, scopes, type)
		 VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10) RETURNING id, created_at`,
		key.UserID, toNullString(key.OrgID), toNullString(key.ZoneID), key.Mode, key.KeyPrefix, key.KeyHash, key.TruncatedKey, key.Environment, key.Scopes, key.Type).
		Scan(&key.ID, &key.CreatedAt)

	if err != nil {
		return fmt.Errorf("failed to create api key: %w", err)
	}
	return nil
}

func (r *SQLRepository) GetAPIKeyByHash(ctx context.Context, hash string) (*domain.APIKey, error) {
	var key domain.APIKey
	var scopes sql.NullString
	var orgID sql.NullString
	var zoneID sql.NullString
	var mode sql.NullString
	var typeStr sql.NullString
	var revokedAt sql.NullTime
	err := r.db.QueryRowContext(ctx,
		"SELECT id, user_id, org_id, zone_id, mode, key_prefix, environment, scopes, type, revoked_at FROM api_keys WHERE key_hash = $1",
		hash).Scan(&key.ID, &key.UserID, &orgID, &zoneID, &mode, &key.KeyPrefix, &key.Environment, &scopes, &typeStr, &revokedAt)
	key.OrgID = orgID.String
	key.ZoneID = zoneID.String
	key.Mode = mode.String
	key.Type = typeStr.String
	if key.Type == "" {
		key.Type = "secret"
	}
	key.Scopes = scopes.String
	if key.Scopes == "" {
		key.Scopes = "*"
	}
	if revokedAt.Valid {
		key.RevokedAt = &revokedAt.Time
	}

	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, nil
		}
		return nil, fmt.Errorf("failed to get api key: %w", err)
	}
	return &key, nil
}

// OAuth methods

func (r *SQLRepository) GetClientByID(ctx context.Context, clientID string) (*domain.OAuthClient, error) {
	var client domain.OAuthClient
	err := r.db.QueryRowContext(ctx,
		"SELECT id, client_secret_hash, user_id, name, is_public, created_at FROM oauth_clients WHERE id = $1",
		clientID).Scan(&client.ID, &client.ClientSecretHash, &client.UserID, &client.Name, &client.IsPublic, &client.CreatedAt)

	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, nil
		}
		return nil, fmt.Errorf("failed to get client: %w", err)
	}
	return &client, nil
}

func (r *SQLRepository) CreateOAuthClient(ctx context.Context, client *domain.OAuthClient) error {
	_, err := r.db.ExecContext(ctx,
		`INSERT INTO oauth_clients (id, client_secret_hash, user_id, name, is_public)
		 VALUES ($1, $2, $3, $4, $5)`,
		client.ID, client.ClientSecretHash, client.UserID, client.Name, client.IsPublic)

	if err != nil {
		return fmt.Errorf("failed to create oauth client: %w", err)
	}
	return nil
}

func (r *SQLRepository) AddRedirectURI(ctx context.Context, clientID, redirectURI string) error {
	_, err := r.db.ExecContext(ctx,
		`INSERT INTO client_redirect_uris (client_id, redirect_uri) VALUES ($1, $2)
		 ON CONFLICT (client_id, redirect_uri) DO NOTHING`,
		clientID, redirectURI)

	if err != nil {
		return fmt.Errorf("failed to add redirect uri: %w", err)
	}
	return nil
}

func (r *SQLRepository) ValidateRedirectURI(ctx context.Context, clientID, redirectURI string) (bool, error) {
	var count int
	err := r.db.QueryRowContext(ctx,
		`SELECT COUNT(*) FROM client_redirect_uris WHERE client_id = $1 AND redirect_uri = $2`,
		clientID, redirectURI).Scan(&count)

	if err != nil {
		return false, fmt.Errorf("failed to validate redirect uri: %w", err)
	}

	return count > 0, nil
}

func (r *SQLRepository) CreateOAuthToken(ctx context.Context, token *domain.OAuthToken) error {
	_, err := r.db.ExecContext(ctx,
		`INSERT INTO oauth_tokens (access_token, refresh_token, client_id, user_id, scope, expires_at)
		 VALUES ($1, $2, $3, $4, $5, $6)`,
		token.AccessToken, token.RefreshToken, token.ClientID, token.UserID, token.Scope, token.ExpiresAt)

	if err != nil {
		return fmt.Errorf("failed to create token: %w", err)
	}
	return nil
}

func (r *SQLRepository) ValidateOAuthToken(ctx context.Context, accessToken string) (*domain.OAuthToken, error) {
	var token domain.OAuthToken
	err := r.db.QueryRowContext(ctx,
		"SELECT access_token, refresh_token, client_id, user_id, scope, expires_at, created_at FROM oauth_tokens WHERE access_token = $1",
		accessToken).Scan(&token.AccessToken, &token.RefreshToken, &token.ClientID, &token.UserID, &token.Scope, &token.ExpiresAt, &token.CreatedAt)

	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, nil
		}
		return nil, fmt.Errorf("failed to validate token: %w", err)
	}

	return &token, nil
}

func (r *SQLRepository) CreateAuthorizationCode(ctx context.Context, code *domain.AuthorizationCode) error {
	_, err := r.db.ExecContext(ctx,
		`INSERT INTO authorization_codes (code, client_id, user_id, redirect_uri, scope, code_challenge, code_challenge_method, expires_at)
		 VALUES ($1, $2, $3, $4, $5, $6, $7, $8)`,
		code.Code, code.ClientID, code.UserID, code.RedirectURI, code.Scope,
		code.CodeChallenge, code.CodeChallengeMethod, code.ExpiresAt)

	if err != nil {
		return fmt.Errorf("failed to create authorization code: %w", err)
	}
	return nil
}

func (r *SQLRepository) GetAuthorizationCode(ctx context.Context, code string) (*domain.AuthorizationCode, error) {
	var authCode domain.AuthorizationCode
	var codeChallenge, codeChallengeMethod sql.NullString

	err := r.db.QueryRowContext(ctx,
		`SELECT code, client_id, user_id, redirect_uri, scope, code_challenge, code_challenge_method, expires_at, used, created_at
		 FROM authorization_codes WHERE code = $1`,
		code).Scan(
		&authCode.Code, &authCode.ClientID, &authCode.UserID, &authCode.RedirectURI,
		&authCode.Scope, &codeChallenge, &codeChallengeMethod,
		&authCode.ExpiresAt, &authCode.Used, &authCode.CreatedAt)

	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, nil
		}
		return nil, fmt.Errorf("failed to get authorization code: %w", err)
	}

	authCode.CodeChallenge = codeChallenge.String
	authCode.CodeChallengeMethod = codeChallengeMethod.String

	return &authCode, nil
}

func (r *SQLRepository) MarkAuthorizationCodeUsed(ctx context.Context, code string) error {
	result, err := r.db.ExecContext(ctx,
		`UPDATE authorization_codes SET used = TRUE WHERE code = $1 AND used = FALSE`,
		code)

	if err != nil {
		return fmt.Errorf("failed to mark code as used: %w", err)
	}

	rowsAffected, _ := result.RowsAffected()
	if rowsAffected == 0 {
		return fmt.Errorf("code already used or not found")
	}

	return nil
}

// SSO Provider methods

func (r *SQLRepository) CreateSSOProvider(ctx context.Context, p *domain.SSOProvider) error {
	err := r.db.QueryRowContext(ctx,
		`INSERT INTO sso_providers (org_id, name, provider_type, issuer_url, client_id, client_secret, metadata_url, sso_url, certificate)
		 VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9)
		 RETURNING id, created_at, updated_at`,
		p.OrgID, p.Name, p.ProviderType, p.IssuerURL, p.ClientID, p.ClientSecret, p.MetadataURL, p.SSOURL, p.Certificate).
		Scan(&p.ID, &p.CreatedAt, &p.UpdatedAt)

	if err != nil {
		return fmt.Errorf("failed to create sso provider: %w", err)
	}
	return nil
}

func (r *SQLRepository) GetSSOProviderByID(ctx context.Context, id string) (*domain.SSOProvider, error) {
	var p domain.SSOProvider
	err := r.db.QueryRowContext(ctx,
		`SELECT id, org_id, name, provider_type, issuer_url, client_id, client_secret, metadata_url, sso_url, certificate, active, created_at, updated_at
		 FROM sso_providers WHERE id = $1`,
		id).Scan(&p.ID, &p.OrgID, &p.Name, &p.ProviderType, &p.IssuerURL, &p.ClientID, &p.ClientSecret, &p.MetadataURL, &p.SSOURL, &p.Certificate, &p.Active, &p.CreatedAt, &p.UpdatedAt)

	if err != nil {
		if err == sql.ErrNoRows {
			return nil, nil
		}
		return nil, fmt.Errorf("failed to get sso provider: %w", err)
	}
	return &p, nil
}

func (r *SQLRepository) GetSSOProviderByDomain(ctx context.Context, domainName string) (*domain.SSOProvider, error) {
	var p domain.SSOProvider
	err := r.db.QueryRowContext(ctx,
		`SELECT p.id, p.org_id, p.name, p.provider_type, p.issuer_url, p.client_id, p.client_secret, p.metadata_url, p.sso_url, p.certificate, p.active, p.created_at, p.updated_at
		 FROM sso_providers p
		 JOIN organizations o ON p.org_id = o.id
		 WHERE o.domain = $1 AND p.active = TRUE
		 LIMIT 1`,
		domainName).Scan(&p.ID, &p.OrgID, &p.Name, &p.ProviderType, &p.IssuerURL, &p.ClientID, &p.ClientSecret, &p.MetadataURL, &p.SSOURL, &p.Certificate, &p.Active, &p.CreatedAt, &p.UpdatedAt)

	if err != nil {
		if err == sql.ErrNoRows {
			return nil, nil
		}
		return nil, fmt.Errorf("failed to get sso provider by domain: %w", err)
	}
	return &p, nil
}

// Audit Log methods

func (r *SQLRepository) CreateAuditLog(ctx context.Context, log *domain.AuditLog) error {
	_, err := r.db.ExecContext(ctx,
		`INSERT INTO audit_logs (org_id, user_id, action, resource_type, resource_id, metadata, ip_address, user_agent)
		 VALUES ($1, $2, $3, $4, $5, $6, $7, $8)`,
		log.OrgID, log.UserID, log.Action, log.ResourceType, log.ResourceID, log.Metadata, log.IPAddress, log.UserAgent)

	if err != nil {
		return fmt.Errorf("failed to create audit log: %w", err)
	}
	return nil
}

func (r *SQLRepository) GetAuditLogs(ctx context.Context, orgID string, limit, offset int, action string) ([]domain.AuditLog, int, error) {
	query := `SELECT id, org_id, user_id, action, resource_type, resource_id, metadata, ip_address, created_at 
			  FROM audit_logs WHERE org_id = $1`
	args := []interface{}{orgID}
	placeholder := 2

	if action != "" {
		query += fmt.Sprintf(" AND action = $%d", placeholder)
		args = append(args, action)
		placeholder++
	}

	query += fmt.Sprintf(" ORDER BY created_at DESC LIMIT $%d OFFSET $%d", placeholder, placeholder+1)
	args = append(args, limit, offset)

	rows, err := r.db.QueryContext(ctx, query, args...)
	if err != nil {
		return nil, 0, err
	}
	defer func() { _ = rows.Close() }()

	var logs []domain.AuditLog
	for rows.Next() {
		var l domain.AuditLog
		if err := rows.Scan(&l.ID, &l.OrgID, &l.UserID, &l.Action, &l.ResourceType, &l.ResourceID, &l.Metadata, &l.IPAddress, &l.CreatedAt); err != nil {
			return nil, 0, err
		}
		logs = append(logs, l)
	}

	var total int
	countQuery := "SELECT COUNT(*) FROM audit_logs WHERE org_id = $1"
	countArgs := []interface{}{orgID}
	if action != "" {
		countQuery += " AND action = $2"
		countArgs = append(countArgs, action)
	}
	err = r.db.QueryRowContext(ctx, countQuery, countArgs...).Scan(&total)
	if err != nil {
		return nil, 0, err
	}

	return logs, total, nil
}
