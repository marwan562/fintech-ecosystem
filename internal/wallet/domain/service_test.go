package domain

import (
	"context"
	"errors"
	"testing"

	pb "github.com/sapliy/sapliy-core/proto/ledger"
	walletpb "github.com/sapliy/sapliy-core/proto/wallet"
)

func TestWalletService_GetWallet(t *testing.T) {
	ctx := context.Background()
	userID := "user-123"

	mockLedger := &MockLedgerClient{
		GetAccountFunc: func(ctx context.Context, id string) (*pb.GetAccountResponse, error) {
			return &pb.GetAccountResponse{
				AccountId: id,
				Balance:   1000,
				Currency:  "USD",
			}, nil
		},
	}

	service := NewWalletService(mockLedger)
	wallet, err := service.GetWallet(ctx, userID)

	if err != nil {
		t.Fatalf("GetWallet failed: %v", err)
	}
	if wallet.UserId != userID {
		t.Errorf("expected userID %s, got %s", userID, wallet.UserId)
	}
	if wallet.Balance != 1000 {
		t.Errorf("expected balance 1000, got %d", wallet.Balance)
	}
}

func TestWalletService_TopUp(t *testing.T) {
	ctx := context.Background()
	req := &walletpb.TopUpRequest{
		UserId:      "user-123",
		Amount:      500,
		Currency:    "USD",
		ReferenceId: "ref-123",
	}

	mockLedger := &MockLedgerClient{
		RecordTransactionFunc: func(ctx context.Context, ledgerReq *pb.RecordTransactionRequest) (*pb.RecordTransactionResponse, error) {
			if ledgerReq.AccountId != req.UserId || ledgerReq.Amount != req.Amount {
				return nil, errors.New("unexpected ledger request")
			}
			return &pb.RecordTransactionResponse{
				TransactionId: "tx-123",
				Status:        "COMPLETED",
			}, nil
		},
	}

	service := NewWalletService(mockLedger)
	res, err := service.TopUp(ctx, req)

	if err != nil {
		t.Fatalf("TopUp failed: %v", err)
	}
	if res.TransactionId != "tx-123" {
		t.Errorf("expected txID tx-123, got %s", res.TransactionId)
	}
}

func TestWalletService_Transfer(t *testing.T) {
	ctx := context.Background()
	req := &walletpb.TransferRequest{
		FromUserId:  "user-A",
		ToUserId:    "user-B",
		Amount:      200,
		Currency:    "USD",
		ReferenceId: "ref-456",
	}

	callCount := 0
	mockLedger := &MockLedgerClient{
		RecordTransactionFunc: func(ctx context.Context, ledgerReq *pb.RecordTransactionRequest) (*pb.RecordTransactionResponse, error) {
			callCount++
			if callCount == 1 {
				// Debit
				if ledgerReq.AccountId != req.FromUserId || ledgerReq.Amount != -req.Amount {
					return nil, errors.New("unexpected debit request")
				}
			} else if callCount == 2 {
				// Credit
				if ledgerReq.AccountId != req.ToUserId || ledgerReq.Amount != req.Amount {
					return nil, errors.New("unexpected credit request")
				}
			}
			return &pb.RecordTransactionResponse{
				TransactionId: "tx-multi",
				Status:        "COMPLETED",
			}, nil
		},
	}

	service := NewWalletService(mockLedger)
	res, err := service.Transfer(ctx, req)

	if err != nil {
		t.Fatalf("Transfer failed: %v", err)
	}
	if callCount != 2 {
		t.Errorf("expected 2 ledger calls, got %d", callCount)
	}
	if res.Status != "COMPLETED" {
		t.Errorf("expected status COMPLETED, got %s", res.Status)
	}
}
