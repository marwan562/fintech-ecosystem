package main

import (
	"context"
	"log"
	"microservices/internal/auth"
	pb "microservices/proto/auth"
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

	return &pb.ValidateKeyResponse{
		Valid:       true,
		UserId:      key.UserID,
		Environment: key.Environment,
	}, nil
}
