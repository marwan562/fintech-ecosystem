package main

import (
	"context"
	"log"
	"microservices/internal/ledger"
	pb "microservices/proto/ledger"

	"google.golang.org/protobuf/types/known/timestamppb"
)

type LedgerGRPCServer struct {
	pb.UnimplementedLedgerServiceServer
	repo *ledger.Repository
}

func NewLedgerGRPCServer(repo *ledger.Repository) *LedgerGRPCServer {
	return &LedgerGRPCServer{repo: repo}
}

func (s *LedgerGRPCServer) RecordTransaction(ctx context.Context, req *pb.RecordTransactionRequest) (*pb.RecordTransactionResponse, error) {
	// For simplicity, we create a balanced transaction with two entries:
	// One for the account (asset/revenue) and one for a balancing account (e.g. system equity or cash)
	// In a real system, the caller would provide the full transaction structure.
	// For here, we'll implement what the payments service needs: recording a payment.

	// Let's assume the request amount is what needs to be added (positive) or subtracted (negative)
	// from the account_id provided.

	// Since the internal RecordTransaction expects a balanced set of entries,
	// we'll simulate a double-entry for demonstration if only one account is provided.
	// In a professional ledger, we'd have a 'balancing' account.

	entry := ledger.EntryRequest{
		AccountID: req.AccountId,
		Amount:    req.Amount,
		Direction: "credit", // Defaulting to credit for payment received? Ledger logic varies.
	}

	// To balance it, we'd need another entry. Let's look for a system account or just use a dummy 'system' account.
	balancingEntry := ledger.EntryRequest{
		AccountID: "system_balancing", // This should ideally exist or be configurable
		Amount:    -req.Amount,
		Direction: "debit",
	}

	txReq := ledger.TransactionRequest{
		ReferenceID: req.ReferenceId,
		Description: req.Description,
		Entries:     []ledger.EntryRequest{entry, balancingEntry},
	}

	err := s.repo.RecordTransaction(ctx, txReq)
	if err != nil {
		log.Printf("GRPC RecordTransaction error: %v", err)
		return nil, err
	}

	return &pb.RecordTransactionResponse{
		Status: "recorded",
	}, nil
}

func (s *LedgerGRPCServer) GetAccount(ctx context.Context, req *pb.GetAccountRequest) (*pb.GetAccountResponse, error) {
	acc, err := s.repo.GetAccount(ctx, req.AccountId)
	if err != nil {
		log.Printf("GRPC GetAccount error: %v", err)
		return nil, err
	}

	if acc == nil {
		return nil, nil // Or a specific gRPC error
	}

	return &pb.GetAccountResponse{
		AccountId: acc.ID,
		Balance:   acc.Balance,
		CreatedAt: timestamppb.New(acc.CreatedAt),
	}, nil
}
