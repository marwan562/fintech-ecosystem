package domain

import (
	"context"
)

type MockRepository struct {
	CreateAccountFunc        func(ctx context.Context, acc *Account) error
	GetAccountFunc           func(ctx context.Context, id string) (*Account, error)
	BeginTxFunc              func(ctx context.Context) (TransactionContext, error)
	GetUnprocessedEventsFunc func(ctx context.Context, limit int) ([]OutboxEvent, error)
	MarkEventProcessedFunc   func(ctx context.Context, id string) error
}

func (m *MockRepository) CreateAccount(ctx context.Context, acc *Account) error {
	return m.CreateAccountFunc(ctx, acc)
}

func (m *MockRepository) GetAccount(ctx context.Context, id string) (*Account, error) {
	return m.GetAccountFunc(ctx, id)
}

func (m *MockRepository) BeginTx(ctx context.Context) (TransactionContext, error) {
	return m.BeginTxFunc(ctx)
}

func (m *MockRepository) GetUnprocessedEvents(ctx context.Context, limit int) ([]OutboxEvent, error) {
	return m.GetUnprocessedEventsFunc(ctx, limit)
}

func (m *MockRepository) MarkEventProcessed(ctx context.Context, id string) error {
	return m.MarkEventProcessedFunc(ctx, id)
}

type MockTransactionContext struct {
	CreateTransactionFunc func(ctx context.Context, tx *Transaction) (string, error)
	CreateEntryFunc       func(ctx context.Context, entry *Entry) error
	CheckIdempotencyFunc  func(ctx context.Context, referenceID string) (string, error)
	CreateOutboxEventFunc func(ctx context.Context, eventType string, payload []byte) error
	CommitFunc            func() error
	RollbackFunc          func() error
}

func (m *MockTransactionContext) CreateTransaction(ctx context.Context, tx *Transaction) (string, error) {
	return m.CreateTransactionFunc(ctx, tx)
}

func (m *MockTransactionContext) CreateEntry(ctx context.Context, entry *Entry) error {
	return m.CreateEntryFunc(ctx, entry)
}

func (m *MockTransactionContext) CheckIdempotency(ctx context.Context, referenceID string) (string, error) {
	return m.CheckIdempotencyFunc(ctx, referenceID)
}

func (m *MockTransactionContext) CreateOutboxEvent(ctx context.Context, eventType string, payload []byte) error {
	return m.CreateOutboxEventFunc(ctx, eventType, payload)
}

func (m *MockTransactionContext) Commit() error {
	return m.CommitFunc()
}

func (m *MockTransactionContext) Rollback() error {
	return m.RollbackFunc()
}
