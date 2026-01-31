package domain

import (
	"time"
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
	Currency  string      `json:"currency"`
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

type OutboxEvent struct {
	ID        string    `json:"id"`
	Type      string    `json:"type"`
	Payload   []byte    `json:"payload"`
	CreatedAt time.Time `json:"created_at"`
}
