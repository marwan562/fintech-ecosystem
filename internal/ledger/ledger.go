package ledger

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"log"
	"time"

	"github.com/prometheus/client_golang/prometheus"
)

type AccountType string
type TransactionType string

const (
	Asset     AccountType = "asset"
	Liability AccountType = "liability"
	Equity    AccountType = "equity"
	Revenue   AccountType = "revenue"
	Expense   AccountType = "expense"
)

const (
	Debit  TransactionType = "debit"
	Credit TransactionType = "credit"
)

type Account struct {
	ID        string      `json:"id"`
	Name      string      `json:"name"`
	Type      AccountType `json:"type"`
	Balance   int64       `json:"balance"`
	UserID    *string     `json:"user_id,omitempty"`
	CreatedAt time.Time   `json:"created_at"`
}

type Transaction struct {
	ID          string    `json:"id"`
	ReferenceID string    `json:"reference_id"`
	Description string    `json:"description"`
	CreatedAt   time.Time `json:"created_at"`
}

type Entry struct {
	ID            string          `json:"id"`
	TransactionID string          `json:"transaction_id"`
	AccountID     string          `json:"account_id"`
	Amount        int64           `json:"amount"`
	Direction     TransactionType `json:"direction"`
	CreatedAt     time.Time       `json:"created_at"`
}

type Repository struct {
	db *sql.DB
}

func NewRepository(db *sql.DB) *Repository {
	return &Repository{db: db}
}

func (r *Repository) CreateAccount(ctx context.Context, name string, accType AccountType, userID *string) (*Account, error) {
	acc := &Account{
		Name:   name,
		Type:   accType,
		UserID: userID,
	}
	err := r.db.QueryRowContext(ctx,
		`INSERT INTO accounts (name, type, user_id) VALUES ($1, $2, $3) RETURNING id, created_at`,
		name, accType, userID).Scan(&acc.ID, &acc.CreatedAt)
	if err != nil {
		return nil, fmt.Errorf("failed to create account: %w", err)
	}
	acc.Balance = 0
	return acc, nil
}

func (r *Repository) GetAccount(ctx context.Context, id string) (*Account, error) {
	acc := &Account{}
	// Calculate balance on the fly from immutable entries (Projection)
	err := r.db.QueryRowContext(ctx,
		`SELECT a.id, a.name, a.type, COALESCE(SUM(e.amount), 0) as balance, a.user_id, a.created_at 
		 FROM accounts a 
		 LEFT JOIN entries e ON a.id = e.account_id 
		 WHERE a.id = $1 
		 GROUP BY a.id`,
		id).Scan(&acc.ID, &acc.Name, &acc.Type, &acc.Balance, &acc.UserID, &acc.CreatedAt)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, nil
		}
		return nil, fmt.Errorf("failed to get account: %w", err)
	}
	return acc, nil
}

type TransactionRequest struct {
	ReferenceID string         `json:"reference_id"`
	Description string         `json:"description"`
	Entries     []EntryRequest `json:"entries"`
}

type EntryRequest struct {
	AccountID string `json:"account_id"`
	Amount    int64  `json:"amount"`    // Signed amount
	Direction string `json:"direction"` // Optional, helpful for validation
}

// RecordTransaction creates a transaction and its entries atomically.
// It is idempotent based on the ReferenceID.
func (r *Repository) RecordTransaction(ctx context.Context, req TransactionRequest) (err error) {
	timer := prometheus.NewTimer(TransactionLatency)
	defer func() {
		timer.ObserveDuration()
		if err != nil {
			TransactionsRecorded.WithLabelValues("error").Inc()
		} else {
			TransactionsRecorded.WithLabelValues("success").Inc()
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

	tx, err := r.db.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer func() {
		if err := tx.Rollback(); err != nil && err != sql.ErrTxDone {
			log.Printf("Failed to rollback transaction: %v", err)
		}
	}()

	// 2. Check for existing transaction (Idempotency)
	var existingID string
	err = tx.QueryRowContext(ctx, `SELECT id FROM transactions WHERE reference_id = $1`, req.ReferenceID).Scan(&existingID)
	if err == nil {
		// Already exists, return nil (success)
		return nil
	} else if !errors.Is(err, sql.ErrNoRows) {
		return fmt.Errorf("failed to check idempotency: %w", err)
	}

	// 3. Insert Transaction Record
	var transactionID string
	err = tx.QueryRowContext(ctx,
		`INSERT INTO transactions (reference_id, description) VALUES ($1, $2) RETURNING id`,
		req.ReferenceID, req.Description).Scan(&transactionID)
	if err != nil {
		return fmt.Errorf("failed to create transaction: %w", err)
	}

	// 4. Insert Entries (No UPDATE on accounts, balance is a projection)
	for _, e := range req.Entries {
		_, err := tx.ExecContext(ctx,
			`INSERT INTO entries (transaction_id, account_id, amount, direction) VALUES ($1, $2, $3, $4)`,
			transactionID, e.AccountID, e.Amount, e.Direction)
		if err != nil {
			return fmt.Errorf("failed to create entry for account %s: %w", e.AccountID, err)
		}
	}

	// 5. Insert Outbox Event
	eventData, _ := json.Marshal(map[string]interface{}{
		"id":           transactionID,
		"reference_id": req.ReferenceID,
		"description":  req.Description,
		"entries":      req.Entries,
	})
	_, err = tx.ExecContext(ctx,
		`INSERT INTO outbox (event_type, payload) VALUES ($1, $2)`,
		"transaction.recorded", eventData)
	if err != nil {
		return fmt.Errorf("failed to create outbox event: %w", err)
	}

	return tx.Commit()
}

type OutboxEvent struct {
	ID        string
	Type      string
	Payload   []byte
	CreatedAt time.Time
}

func (r *Repository) GetUnprocessedEvents(ctx context.Context, limit int) ([]OutboxEvent, error) {
	rows, err := r.db.QueryContext(ctx,
		`SELECT id, event_type, payload, created_at FROM outbox WHERE processed_at IS NULL ORDER BY created_at ASC LIMIT $1`,
		limit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var events []OutboxEvent
	for rows.Next() {
		var e OutboxEvent
		if err := rows.Scan(&e.ID, &e.Type, &e.Payload, &e.CreatedAt); err != nil {
			return nil, err
		}
		events = append(events, e)
	}
	return events, nil
}

func (r *Repository) MarkEventProcessed(ctx context.Context, id string) error {
	_, err := r.db.ExecContext(ctx,
		`UPDATE outbox SET processed_at = NOW() WHERE id = $1`,
		id)
	return err
}
