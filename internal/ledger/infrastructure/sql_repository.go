package infrastructure

import (
	"context"
	"database/sql"
	"errors"
	"fmt"

	"github.com/marwan562/fintech-ecosystem/internal/ledger/domain"
)

type SQLRepository struct {
	db *sql.DB
}

func NewSQLRepository(db *sql.DB) *SQLRepository {
	return &SQLRepository{db: db}
}

func (r *SQLRepository) CreateAccount(ctx context.Context, acc *domain.Account) error {
	err := r.db.QueryRowContext(ctx,
		`INSERT INTO accounts (name, type, currency, user_id) VALUES ($1, $2, $3, $4) RETURNING id, created_at`,
		acc.Name, acc.Type, acc.Currency, acc.UserID).Scan(&acc.ID, &acc.CreatedAt)
	if err != nil {
		return fmt.Errorf("failed to create account: %w", err)
	}
	return nil
}

func (r *SQLRepository) GetAccount(ctx context.Context, id string) (*domain.Account, error) {
	acc := &domain.Account{}
	err := r.db.QueryRowContext(ctx,
		`SELECT a.id, a.name, a.type, a.currency, COALESCE(SUM(e.amount), 0) as balance, a.user_id, a.created_at 
		 FROM accounts a 
		 LEFT JOIN entries e ON a.id = e.account_id 
		 WHERE a.id = $1 
		 GROUP BY a.id`,
		id).Scan(&acc.ID, &acc.Name, &acc.Type, &acc.Currency, &acc.Balance, &acc.UserID, &acc.CreatedAt)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, nil
		}
		return nil, fmt.Errorf("failed to get account: %w", err)
	}
	return acc, nil
}

func (r *SQLRepository) BeginTx(ctx context.Context) (domain.TransactionContext, error) {
	tx, err := r.db.BeginTx(ctx, nil)
	if err != nil {
		return nil, err
	}
	return &sqlTxContext{tx: tx}, nil
}

func (r *SQLRepository) GetUnprocessedEvents(ctx context.Context, limit int) ([]domain.OutboxEvent, error) {
	rows, err := r.db.QueryContext(ctx,
		`SELECT id, event_type, payload, created_at FROM outbox WHERE processed_at IS NULL ORDER BY created_at ASC LIMIT $1`,
		limit)
	if err != nil {
		return nil, err
	}
	defer func() { _ = rows.Close() }()

	var events []domain.OutboxEvent
	for rows.Next() {
		var e domain.OutboxEvent
		if err := rows.Scan(&e.ID, &e.Type, &e.Payload, &e.CreatedAt); err != nil {
			return nil, err
		}
		events = append(events, e)
	}
	return events, nil
}

func (r *SQLRepository) MarkEventProcessed(ctx context.Context, id string) error {
	_, err := r.db.ExecContext(ctx, `UPDATE outbox SET processed_at = NOW() WHERE id = $1`, id)
	return err
}

type sqlTxContext struct {
	tx *sql.Tx
}

func (c *sqlTxContext) CreateTransaction(ctx context.Context, tx *domain.Transaction) (string, error) {
	var id string
	err := c.tx.QueryRowContext(ctx,
		`INSERT INTO transactions (reference_id, description) VALUES ($1, $2) RETURNING id`,
		tx.ReferenceID, tx.Description).Scan(&id)
	return id, err
}

func (c *sqlTxContext) CreateEntry(ctx context.Context, entry *domain.Entry) error {
	_, err := c.tx.ExecContext(ctx,
		`INSERT INTO entries (transaction_id, account_id, amount, direction) VALUES ($1, $2, $3, $4)`,
		entry.TransactionID, entry.AccountID, entry.Amount, entry.Direction)
	return err
}

func (c *sqlTxContext) CheckIdempotency(ctx context.Context, referenceID string) (string, error) {
	var id string
	err := c.tx.QueryRowContext(ctx, `SELECT id FROM transactions WHERE reference_id = $1`, referenceID).Scan(&id)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return "", nil
		}
		return "", err
	}
	return id, nil
}

func (c *sqlTxContext) CreateOutboxEvent(ctx context.Context, eventType string, payload []byte) error {
	_, err := c.tx.ExecContext(ctx,
		`INSERT INTO outbox (event_type, payload) VALUES ($1, $2)`,
		eventType, payload)
	return err
}

func (c *sqlTxContext) Commit() error {
	return c.tx.Commit()
}

func (c *sqlTxContext) Rollback() error {
	return c.tx.Rollback()
}
