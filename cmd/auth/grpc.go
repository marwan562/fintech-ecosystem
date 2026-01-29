package main

import (
	"context"
	"fmt"
	"log"
	"strings"
	"time"

	"github.com/marwan562/fintech-ecosystem/internal/auth"
	pb "github.com/marwan562/fintech-ecosystem/proto/auth"
)

type AuthGRPCServer struct {
	pb.UnimplementedAuthServiceServer
	repo *auth.Repository
}

func NewAuthGRPCServer(repo *auth.Repository) *AuthGRPCServer {
	return &AuthGRPCServer{repo: repo}
}

func (s *AuthGRPCServer) ValidateKey(ctx context.Context, req *pb.ValidateKeyRequest) (*pb.ValidateKeyResponse, error) {
	key, err := s.repo.GetAPIKeyByHash(ctx, req.KeyHash)
	if err != nil {
		log.Printf("GRPC ValidateKey error: %v", err)
		return &pb.ValidateKeyResponse{Valid: false}, nil
	}

	if key == nil || key.RevokedAt != nil {
		return &pb.ValidateKeyResponse{Valid: false}, nil
	}

	role := ""
	if key.OrgID != "" {
		role = "admin" // Default for API keys
	}

	return &pb.ValidateKeyResponse{
		Valid:       true,
		UserId:      key.UserID,
		Environment: key.Environment,
		Scopes:      key.Scopes,
		OrgId:       key.OrgID,
		Role:        role,
	}, nil
}

func (s *AuthGRPCServer) ValidateToken(ctx context.Context, req *pb.ValidateTokenRequest) (*pb.ValidateTokenResponse, error) {
	token, err := s.repo.ValidateOAuthToken(ctx, req.AccessToken)
	if err != nil {
		// Log error only if it's not "not found" or "expired" to avoid noise?
		// Repo returns error on expiry.
		return &pb.ValidateTokenResponse{Valid: false}, nil
	}
	if token == nil {
		return &pb.ValidateTokenResponse{Valid: false}, nil
	}

	return &pb.ValidateTokenResponse{
		Valid:     true,
		ClientId:  token.ClientID,
		UserId:    token.UserID,
		Scope:     token.Scope,
		ExpiresAt: token.ExpiresAt.Unix(),
	}, nil
}

func (s *AuthGRPCServer) CreateSSOProvider(ctx context.Context, req *pb.CreateSSOProviderRequest) (*pb.SSOProvider, error) {
	provider := &auth.SSOProvider{
		OrgID:        req.OrgId,
		Name:         req.Name,
		ProviderType: req.ProviderType,
		IssuerURL:    req.IssuerUrl,
		ClientID:     req.ClientId,
		ClientSecret: req.ClientSecret,
	}

	if err := s.repo.CreateSSOProvider(ctx, provider); err != nil {
		return nil, err
	}

	return &pb.SSOProvider{
		Id:           provider.ID,
		OrgId:        provider.OrgID,
		Name:         provider.Name,
		ProviderType: provider.ProviderType,
		IssuerUrl:    provider.IssuerURL,
		ClientId:     provider.ClientID,
		Active:       provider.Active,
	}, nil
}

func (s *AuthGRPCServer) GetSSOProvider(ctx context.Context, req *pb.GetSSOProviderRequest) (*pb.SSOProvider, error) {
	provider, err := s.repo.GetSSOProviderByID(ctx, req.Id)
	if err != nil {
		return nil, err
	}
	if provider == nil {
		return nil, fmt.Errorf("SSO provider not found")
	}

	return &pb.SSOProvider{
		Id:           provider.ID,
		OrgId:        provider.OrgID,
		Name:         provider.Name,
		ProviderType: provider.ProviderType,
		IssuerUrl:    provider.IssuerURL,
		ClientId:     provider.ClientID,
		Active:       provider.Active,
	}, nil
}

func (s *AuthGRPCServer) InitiateSSO(ctx context.Context, req *pb.InitiateSSORequest) (*pb.InitiateSSOResponse, error) {
	parts := strings.Split(req.Email, "@")
	if len(parts) != 2 {
		return nil, fmt.Errorf("invalid email format")
	}
	domain := parts[1]

	provider, err := s.repo.GetSSOProviderByDomain(ctx, domain)
	if err != nil {
		return nil, err
	}
	if provider == nil {
		return nil, fmt.Errorf("no SSO provider configured for domain %s", domain)
	}

	var authURL string
	switch provider.ProviderType {
	case "oidc":
		// Basic OIDC initiation URL
		authURL = fmt.Sprintf("%s?client_id=%s&redirect_uri=%s&response_type=code&scope=openid email profile&state=sso_%s",
			provider.IssuerURL, provider.ClientID, req.RedirectUri, provider.ID)
	case "saml":
		// Basic SAML/Redirect binding initiation placeholder
		authURL = fmt.Sprintf("%s?SAMLRequest=...", provider.SSOURL)
	}

	return &pb.InitiateSSOResponse{AuthUrl: authURL}, nil
}

func (s *AuthGRPCServer) GetAuditLogs(ctx context.Context, req *pb.GetAuditLogsRequest) (*pb.GetAuditLogsResponse, error) {
	logs, total, err := s.repo.GetAuditLogs(ctx, req.OrgId, int(req.Limit), int(req.Offset), req.Action)
	if err != nil {
		return nil, err
	}

	var pbLogs []*pb.AuditLog
	for _, l := range logs {
		pbLogs = append(pbLogs, &pb.AuditLog{
			Id:           l.ID,
			OrgId:        l.OrgID,
			UserId:       l.UserID,
			Action:       l.Action,
			ResourceType: l.ResourceType,
			ResourceId:   l.ResourceID,
			Metadata:     string(l.Metadata),
			IpAddress:    l.IPAddress,
			CreatedAt:    l.CreatedAt.Format(time.RFC3339),
		})
	}

	return &pb.GetAuditLogsResponse{
		Logs:       pbLogs,
		TotalCount: int32(total),
	}, nil
}

func (s *AuthGRPCServer) AddTeamMember(ctx context.Context, req *pb.AddTeamMemberRequest) (*pb.Membership, error) {
	err := s.repo.AddMember(ctx, req.UserId, req.OrgId, req.Role)
	if err != nil {
		return nil, err
	}

	// Fetch to return full object (could be optimized)
	memberships, err := s.repo.GetUserMemberships(ctx, req.UserId)
	if err != nil {
		return nil, err
	}

	for _, m := range memberships {
		if m.OrgID == req.OrgId {
			return &pb.Membership{
				UserId:    m.UserID,
				OrgId:     m.OrgID,
				Role:      m.Role,
				CreatedAt: m.CreatedAt.Format(time.RFC3339),
			}, nil
		}
	}

	return nil, fmt.Errorf("failed to retrieve added membership")
}

func (s *AuthGRPCServer) RemoveTeamMember(ctx context.Context, req *pb.RemoveTeamMemberRequest) (*pb.RemoveTeamMemberResponse, error) {
	if err := s.repo.RemoveMember(ctx, req.UserId, req.OrgId); err != nil {
		return nil, err
	}
	return &pb.RemoveTeamMemberResponse{Success: true}, nil
}

func (s *AuthGRPCServer) ListTeamMembers(ctx context.Context, req *pb.ListTeamMembersRequest) (*pb.ListTeamMembersResponse, error) {
	memberships, err := s.repo.ListOrgMembers(ctx, req.OrgId)
	if err != nil {
		return nil, err
	}

	var pbMemberships []*pb.Membership
	for _, m := range memberships {
		pbMemberships = append(pbMemberships, &pb.Membership{
			UserId:    m.UserID,
			OrgId:     m.OrgID,
			Role:      m.Role,
			CreatedAt: m.CreatedAt.Format(time.RFC3339),
		})
	}

	return &pb.ListTeamMembersResponse{Memberships: pbMemberships}, nil
}
