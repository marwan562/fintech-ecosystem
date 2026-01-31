package domain

import (
	"context"
)

type Repository interface {
	CreateAccount(ctx context.Context, acc *Account) error
	GetAccount(ctx context.Context, id string) (*Account, error)
	BeginTx(ctx context.Context) (TransactionContext, error)
	GetUnprocessedEvents(ctx context.Context, limit int) ([]OutboxEvent, error)
	MarkEventProcessed(ctx context.Context, id string) error
}

type TransactionContext interface {
	CreateTransaction(ctx context.Context, tx *Transaction) (string, error)
	CreateEntry(ctx context.Context, entry *Entry) error
	CheckIdempotency(ctx context.Context, referenceID string) (string, error)
	CreateOutboxEvent(ctx context.Context, eventType string, payload []byte) error
	Commit() error
	Rollback() error
}
