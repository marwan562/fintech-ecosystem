package main

import (
	"context"
	"log"

	"github.com/marwan562/fintech-ecosystem/internal/ledger/domain"
	pb "github.com/marwan562/fintech-ecosystem/proto/ledger"

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
	entry := domain.EntryRequest{
		AccountID: req.AccountId,
		Amount:    req.Amount,
		Direction: "credit",
	}

	balancingEntry := domain.EntryRequest{
		AccountID: "system_balancing",
		Amount:    -req.Amount,
		Direction: "debit",
	}

	txReq := domain.TransactionRequest{
		ReferenceID: req.ReferenceId,
		Description: req.Description,
		Entries:     []domain.EntryRequest{entry, balancingEntry},
	}

	err := s.service.RecordTransaction(ctx, txReq)
	if err != nil {
		log.Printf("GRPC RecordTransaction error: %v", err)
		return nil, err
	}

	return &pb.RecordTransactionResponse{
		Status: "recorded",
	}, nil
}

func (s *LedgerGRPCServer) GetAccount(ctx context.Context, req *pb.GetAccountRequest) (*pb.GetAccountResponse, error) {
	acc, err := s.service.GetAccount(ctx, req.AccountId)
	if err != nil {
		log.Printf("GRPC GetAccount error: %v", err)
		return nil, err
	}

	if acc == nil {
		return nil, nil
	}

	return &pb.GetAccountResponse{
		AccountId: acc.ID,
		Balance:   acc.Balance,
		CreatedAt: timestamppb.New(acc.CreatedAt),
	}, nil
}
