package domain

import (
	"context"
	"fmt"

	"github.com/sapliy/sapliy-core/pkg/validation"
	pb "github.com/sapliy/sapliy-core/proto/ledger"
	walletpb "github.com/sapliy/sapliy-core/proto/wallet"
)

type LedgerClient interface {
	GetAccount(ctx context.Context, accountID string) (*pb.GetAccountResponse, error)
	RecordTransaction(ctx context.Context, req *pb.RecordTransactionRequest) (*pb.RecordTransactionResponse, error)
}

type WalletService struct {
	ledgerClient LedgerClient
}

func NewWalletService(ledger LedgerClient) *WalletService {
	return &WalletService{ledgerClient: ledger}
}

func (s *WalletService) GetWallet(ctx context.Context, userID string) (*walletpb.Wallet, error) {
	// For now, we assume user ID is the account ID in the ledger
	// In a real system, there would be a mapping or a specific wallet account naming convention
	res, err := s.ledgerClient.GetAccount(ctx, userID)
	if err != nil {
		return nil, fmt.Errorf("ledger error: %w", err)
	}

	return &walletpb.Wallet{
		Id:       res.AccountId,
		UserId:   userID,
		Balance:  res.Balance,
		Currency: res.Currency,
	}, nil
}

func (s *WalletService) TopUp(ctx context.Context, req *walletpb.TopUpRequest) (*walletpb.TransactionResponse, error) {
	if err := validation.Validate(
		validation.NotEmpty(req.UserId, "user_id"),
		validation.PositiveAmount(req.Amount, "amount"),
		validation.NotEmpty(req.Currency, "currency"),
	); err != nil {
		return nil, err
	}

	// Top up involves recording a transaction in the ledger
	// From an internal "float" or "system" account to the user's liability account
	res, err := s.ledgerClient.RecordTransaction(ctx, &pb.RecordTransactionRequest{
		AccountId:   req.UserId,
		Amount:      req.Amount,
		Currency:    req.Currency,
		Description: "Wallet Top-Up",
		ReferenceId: req.ReferenceId,
	})
	if err != nil {
		return nil, fmt.Errorf("ledger error: %w", err)
	}

	return &walletpb.TransactionResponse{
		TransactionId: res.TransactionId,
		Status:        res.Status,
	}, nil
}

func (s *WalletService) Transfer(ctx context.Context, req *walletpb.TransferRequest) (*walletpb.TransactionResponse, error) {
	if err := validation.Validate(
		validation.NotEmpty(req.FromUserId, "from_user_id"),
		validation.NotEmpty(req.ToUserId, "to_user_id"),
		validation.PositiveAmount(req.Amount, "amount"),
		validation.NotEmpty(req.Currency, "currency"),
	); err != nil {
		return nil, err
	}

	// Transfer between two wallets
	// FIXME: This implementation is NOT atomic. If the debit succeeds but the credit fails,
	// money will be lost. This should use a multi-leg transaction supported by the ledger.
	// For now, we perform two sequential updates.

	// Debit from_user (amount is negative)
	_, err := s.ledgerClient.RecordTransaction(ctx, &pb.RecordTransactionRequest{
		AccountId:   req.FromUserId,
		Amount:      -req.Amount,
		Currency:    req.Currency,
		Description: fmt.Sprintf("Transfer to %s", req.ToUserId),
		ReferenceId: req.ReferenceId + "_debit",
	})
	if err != nil {
		return nil, fmt.Errorf("debit failed: %w", err)
	}

	// Credit to_user (amount is positive)
	res, err := s.ledgerClient.RecordTransaction(ctx, &pb.RecordTransactionRequest{
		AccountId:   req.ToUserId,
		Amount:      req.Amount,
		Currency:    req.Currency,
		Description: fmt.Sprintf("Transfer from %s", req.FromUserId),
		ReferenceId: req.ReferenceId + "_credit",
	})
	if err != nil {
		return nil, fmt.Errorf("credit failed: %w", err)
	}

	return &walletpb.TransactionResponse{
		TransactionId: res.TransactionId,
		Status:        res.Status,
	}, nil
}
