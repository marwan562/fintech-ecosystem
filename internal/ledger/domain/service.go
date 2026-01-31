package domain

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
)

type Metrics interface {
	RecordTransaction(status string)
}

type LedgerService struct {
	repo    Repository
	metrics Metrics
}

func NewLedgerService(repo Repository, metrics Metrics) *LedgerService {
	return &LedgerService{
		repo:    repo,
		metrics: metrics,
	}
}

func (s *LedgerService) CreateAccount(ctx context.Context, name string, accType AccountType, currency string, userID *string) (*Account, error) {
	acc := &Account{
		Name:     name,
		Type:     accType,
		Currency: currency,
		UserID:   userID,
	}
	err := s.repo.CreateAccount(ctx, acc)
	if err != nil {
		return nil, err
	}
	acc.Balance = 0
	return acc, nil
}

func (s *LedgerService) GetAccount(ctx context.Context, id string) (*Account, error) {
	return s.repo.GetAccount(ctx, id)
}

func (s *LedgerService) RecordTransaction(ctx context.Context, req TransactionRequest) (err error) {
	defer func() {
		if s.metrics != nil {
			if err != nil {
				s.metrics.RecordTransaction("error")
			} else {
				s.metrics.RecordTransaction("success")
			}
		}
	}()

	// 1. Validate Balance (Sum of amounts must be 0)
	var sum int64
	for _, e := range req.Entries {
		sum += e.Amount
	}
	if sum != 0 {
		return errors.New("transaction is not balanced (sum != 0)")
	}

	// 2. Validate Currency Consistency
	var commonCurrency string
	for _, e := range req.Entries {
		acc, err := s.repo.GetAccount(ctx, e.AccountID)
		if err != nil {
			return fmt.Errorf("failed to get account %s for currency check: %w", e.AccountID, err)
		}
		if acc == nil {
			return fmt.Errorf("account %s not found", e.AccountID)
		}

		if commonCurrency == "" {
			commonCurrency = acc.Currency
		} else if commonCurrency != acc.Currency {
			return fmt.Errorf("multi-currency transactions not supported: account %s has currency %s, expected %s", e.AccountID, acc.Currency, commonCurrency)
		}
	}

	txCtx, err := s.repo.BeginTx(ctx)
	if err != nil {
		return err
	}
	defer func() { _ = txCtx.Rollback() }()

	// 3. Check for existing transaction (Idempotency)
	existingID, err := txCtx.CheckIdempotency(ctx, req.ReferenceID)
	if err != nil {
		return fmt.Errorf("failed to check idempotency: %w", err)
	}
	if existingID != "" {
		return nil // Already exists
	}

	// 4. Insert Transaction Record
	transactionID, err := txCtx.CreateTransaction(ctx, &Transaction{
		ReferenceID: req.ReferenceID,
		Description: req.Description,
	})
	if err != nil {
		return fmt.Errorf("failed to create transaction: %w", err)
	}

	// 5. Insert Entries
	for _, e := range req.Entries {
		err := txCtx.CreateEntry(ctx, &Entry{
			TransactionID: transactionID,
			AccountID:     e.AccountID,
			Amount:        e.Amount,
			Direction:     TransactionType(e.Direction),
		})
		if err != nil {
			return fmt.Errorf("failed to create entry for account %s: %w", e.AccountID, err)
		}
	}

	// 6. Insert Outbox Event
	eventData, _ := json.Marshal(map[string]interface{}{
		"id":           transactionID,
		"reference_id": req.ReferenceID,
		"description":  req.Description,
		"entries":      req.Entries,
	})
	err = txCtx.CreateOutboxEvent(ctx, "transaction.recorded", eventData)
	if err != nil {
		return fmt.Errorf("failed to create outbox event: %w", err)
	}

	return txCtx.Commit()
}
