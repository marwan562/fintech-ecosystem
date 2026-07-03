package connect

import (
	"context"

	pb "github.com/sapliy/sapliy-core/proto/connect"
	"google.golang.org/protobuf/types/known/timestamppb"
)

type Service struct {
	pb.UnimplementedConnectServiceServer
	repo *Repository
}

func NewService(repo *Repository) *Service {
	return &Service{repo: repo}
}

func (s *Service) CreateAccount(ctx context.Context, req *pb.CreateAccountRequest) (*pb.Account, error) {
	acc := &Account{
		UserID:             req.UserId,
		Type:               req.Type,
		Country:            req.Country,
		Email:              req.Email,
		BusinessType:       req.BusinessType,
		Status:             "pending",
		PlatformFeePercent: 0.0,
		PayoutSettings: PayoutSettings{
			Interval: "daily",
		},
	}

	if err := s.repo.CreateAccount(ctx, acc); err != nil {
		return nil, err
	}

	return mapAccountToProto(acc), nil
}

func (s *Service) GetAccount(ctx context.Context, req *pb.GetAccountRequest) (*pb.Account, error) {
	acc, err := s.repo.GetAccount(ctx, req.Id)
	if err != nil {
		return nil, err
	}
	if acc == nil {
		return nil, nil // Or return Not Found error
	}

	return mapAccountToProto(acc), nil
}

func (s *Service) UpdateAccount(ctx context.Context, req *pb.UpdateAccountRequest) (*pb.Account, error) {
	acc, err := s.repo.GetAccount(ctx, req.Id)
	if err != nil {
		return nil, err
	}
	if acc == nil {
		return nil, nil
	}

	if req.Email != "" {
		acc.Email = req.Email
	}
	if req.PayoutSettings != nil {
		acc.PayoutSettings.Interval = req.PayoutSettings.Interval
		acc.PayoutSettings.BankAccountID = req.PayoutSettings.BankAccountId
	}
	if req.PlatformFeePercent != 0 {
		acc.PlatformFeePercent = req.PlatformFeePercent
	}

	if err := s.repo.UpdateAccount(ctx, acc); err != nil {
		return nil, err
	}

	return mapAccountToProto(acc), nil
}

func mapAccountToProto(acc *Account) *pb.Account {
	return &pb.Account{
		Id:                 acc.ID,
		UserId:             acc.UserID,
		Type:               acc.Type,
		Country:            acc.Country,
		Email:              acc.Email,
		BusinessType:       acc.BusinessType,
		Status:             acc.Status,
		PlatformFeePercent: acc.PlatformFeePercent,
		PayoutSettings: &pb.PayoutSettings{
			Interval:      acc.PayoutSettings.Interval,
			BankAccountId: acc.PayoutSettings.BankAccountID,
		},
		CreatedAt: timestamppb.New(acc.CreatedAt),
		UpdatedAt: timestamppb.New(acc.UpdatedAt),
	}
}
