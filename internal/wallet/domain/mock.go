package domain

import (
	"context"

	pb "github.com/sapliy/sapliy-core/proto/ledger"
)

type MockLedgerClient struct {
	GetAccountFunc        func(ctx context.Context, accountID string) (*pb.GetAccountResponse, error)
	RecordTransactionFunc func(ctx context.Context, req *pb.RecordTransactionRequest) (*pb.RecordTransactionResponse, error)
}

func (m *MockLedgerClient) GetAccount(ctx context.Context, accountID string) (*pb.GetAccountResponse, error) {
	return m.GetAccountFunc(ctx, accountID)
}

func (m *MockLedgerClient) RecordTransaction(ctx context.Context, req *pb.RecordTransactionRequest) (*pb.RecordTransactionResponse, error) {
	return m.RecordTransactionFunc(ctx, req)
}
