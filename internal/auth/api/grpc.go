package api

import (
	"context"
	"fmt"
	"log"
	"strings"
	"time"

	"github.com/sapliy/sapliy-core/internal/auth/domain"
	pb "github.com/sapliy/sapliy-core/proto/auth"
)

type AuthGRPCServer struct {
	pb.UnimplementedAuthServiceServer
	service *domain.AuthService
}

func NewAuthGRPCServer(service *domain.AuthService) *AuthGRPCServer {
	return &AuthGRPCServer{service: service}
}

func (s *AuthGRPCServer) ValidateKey(ctx context.Context, req *pb.ValidateKeyRequest) (*pb.ValidateKeyResponse, error) {
	key, err := s.service.GetAPIKeyByHash(ctx, req.KeyHash)
	if err != nil {
		log.Printf("GRPC ValidateKey error: %v", err)
		return &pb.ValidateKeyResponse{Valid: false}, nil
	}

	if key == nil || key.RevokedAt != nil {
		return &pb.ValidateKeyResponse{Valid: false}, nil
	}

	role := ""
	if key.OrgID != "" {
		role = "admin"
	}

	return &pb.ValidateKeyResponse{
		Valid:       true,
		UserId:      key.UserID,
		Environment: key.Environment,
		Scopes:      key.Scopes,
		OrgId:       key.OrgID,
		Role:        role,
		ZoneId:      key.ZoneID,
		Mode:        key.Mode,
		KeyType:     key.Type,
	}, nil
}

func (s *AuthGRPCServer) ValidateToken(ctx context.Context, req *pb.ValidateTokenRequest) (*pb.ValidateTokenResponse, error) {
	token, err := s.service.ValidateOAuthToken(ctx, req.AccessToken)
	if err != nil || token == nil {
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
	provider := &domain.SSOProvider{
		OrgID:        req.OrgId,
		Name:         req.Name,
		ProviderType: req.ProviderType,
		IssuerURL:    req.IssuerUrl,
		ClientID:     req.ClientId,
		ClientSecret: req.ClientSecret,
	}

	if err := s.service.CreateSSOProvider(ctx, provider); err != nil {
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
	provider, err := s.service.GetSSOProviderByID(ctx, req.Id)
	if err != nil || provider == nil {
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
	domainName := parts[1]

	provider, err := s.service.GetSSOProviderByDomain(ctx, domainName)
	if err != nil || provider == nil {
		return nil, fmt.Errorf("no SSO provider configured for domain %s", domainName)
	}

	var authURL string
	switch provider.ProviderType {
	case "oidc":
		authURL = fmt.Sprintf("%s?client_id=%s&redirect_uri=%s&response_type=code&scope=openid email profile&state=sso_%s",
			provider.IssuerURL, provider.ClientID, req.RedirectUri, provider.ID)
	case "saml":
		authURL = fmt.Sprintf("%s?SAMLRequest=...", provider.SSOURL)
	}

	return &pb.InitiateSSOResponse{AuthUrl: authURL}, nil
}

func (s *AuthGRPCServer) GetAuditLogs(ctx context.Context, req *pb.GetAuditLogsRequest) (*pb.GetAuditLogsResponse, error) {
	logs, total, err := s.service.GetAuditLogs(ctx, req.OrgId, int(req.Limit), int(req.Offset), req.Action)
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
	if err := s.service.AddMember(ctx, req.UserId, req.OrgId, req.Role); err != nil {
		return nil, err
	}

	memberships, err := s.service.GetUserMemberships(ctx, req.UserId)
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
	if err := s.service.RemoveMember(ctx, req.UserId, req.OrgId); err != nil {
		return nil, err
	}
	return &pb.RemoveTeamMemberResponse{Success: true}, nil
}

func (s *AuthGRPCServer) ListTeamMembers(ctx context.Context, req *pb.ListTeamMembersRequest) (*pb.ListTeamMembersResponse, error) {
	memberships, err := s.service.ListOrgMembers(ctx, req.OrgId)
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
