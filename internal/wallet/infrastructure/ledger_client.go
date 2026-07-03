package infrastructure

import (
	"context"

	pb "github.com/sapliy/sapliy-core/proto/ledger"
)

type LedgerClient struct {
	client pb.LedgerServiceClient
}

func NewLedgerClient(client pb.LedgerServiceClient) *LedgerClient {
	return &LedgerClient{client: client}
}

func (c *LedgerClient) GetAccount(ctx context.Context, accountID string) (*pb.GetAccountResponse, error) {
	return c.client.GetAccount(ctx, &pb.GetAccountRequest{AccountId: accountID})
}

func (c *LedgerClient) RecordTransaction(ctx context.Context, req *pb.RecordTransactionRequest) (*pb.RecordTransactionResponse, error) {
	return c.client.RecordTransaction(ctx, req)
}
