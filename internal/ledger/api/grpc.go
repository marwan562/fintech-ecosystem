package api

import (
	"context"
	"fmt"

	"github.com/sapliy/sapliy-core/internal/ledger/domain"
	pb "github.com/sapliy/sapliy-core/proto/ledger"
	"google.golang.org/protobuf/types/known/timestamppb"
)

type LedgerGRPCServer struct {
	pb.UnimplementedLedgerServiceServer
	service *domain.LedgerService
}

func NewLedgerGRPCServer(service *domain.LedgerService) *LedgerGRPCServer {
	return &LedgerGRPCServer{service: service}
}

func (s *LedgerGRPCServer) RecordTransaction(ctx context.Context, req *pb.RecordTransactionRequest) (*pb.RecordTransactionResponse, error) {
	txReq := domain.TransactionRequest{
		ReferenceID: req.ReferenceId,
		Description: req.Description,
		Entries: []domain.EntryRequest{
			{AccountID: req.AccountId, Amount: req.Amount, Direction: "credit"},
			{AccountID: "system_balancing", Amount: -req.Amount, Direction: "debit"},
		},
	}

	if err := s.service.RecordTransaction(ctx, txReq, req.ZoneId, req.Mode); err != nil {
		return nil, err
	}

	return &pb.RecordTransactionResponse{Status: "recorded"}, nil
}

func (s *LedgerGRPCServer) GetAccount(ctx context.Context, req *pb.GetAccountRequest) (*pb.GetAccountResponse, error) {
	acc, err := s.service.GetAccount(ctx, req.AccountId)
	if err != nil || acc == nil {
		return nil, fmt.Errorf("account not found")
	}

	return &pb.GetAccountResponse{
		AccountId: acc.ID,
		Balance:   acc.Balance,
		Currency:  acc.Currency,
		CreatedAt: timestamppb.New(acc.CreatedAt),
	}, nil
}
