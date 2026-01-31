package fintech

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"time"
)

const (
	DefaultBaseURL = "http://localhost:8080"
)

// Client is the main entry point for the Fintech SDK.
type Client struct {
	baseURL    string
	apiKey     string
	httpClient *http.Client

	Ledger *LedgerService
	Auth   *AuthService
}

// ClientOption is a function that configures a Client.
type ClientOption func(*Client)

// NewClient creates a new Fintech SDK client.
func NewClient(apiKey string, opts ...ClientOption) *Client {
	c := &Client{
		baseURL:    DefaultBaseURL,
		apiKey:     apiKey,
		httpClient: &http.Client{Timeout: 30 * time.Second},
	}

	for _, opt := range opts {
		opt(c)
	}

	c.Ledger = &LedgerService{client: c}
	c.Auth = &AuthService{client: c}

	return c
}

// WithBaseURL sets the base URL for the client.
func WithBaseURL(url string) ClientOption {
	return func(c *Client) {
		c.baseURL = url
	}
}

// WithHTTPClient sets a custom HTTP client.
func WithHTTPClient(hc *http.Client) ClientOption {
	return func(c *Client) {
		c.httpClient = hc
	}
}

func (c *Client) do(ctx context.Context, method, path string, body interface{}, out interface{}) error {
	var bodyReader io.Reader
	if body != nil {
		data, err := json.Marshal(body)
		if err != nil {
			return fmt.Errorf("marshal body: %w", err)
		}
		bodyReader = bytes.NewReader(data)
	}

	req, err := http.NewRequestWithContext(ctx, method, c.baseURL+path, bodyReader)
	if err != nil {
		return fmt.Errorf("create request: %w", err)
	}

	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("X-API-Key", c.apiKey)

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return fmt.Errorf("do request: %w", err)
	}
	defer func() { _ = resp.Body.Close() }()

	if resp.StatusCode >= 400 {
		return fmt.Errorf("api error: status=%d", resp.StatusCode)
	}

	if out != nil {
		if err := json.NewDecoder(resp.Body).Decode(out); err != nil {
			return fmt.Errorf("decode response: %w", err)
		}
	}

	return nil
}

// LedgerService handles ledger-related operations.
type LedgerService struct {
	client *Client
}

type RecordTransactionRequest struct {
	AccountID   string `json:"accountId"`
	Amount      int64  `json:"amount"` // In cents
	Currency    string `json:"currency"`
	Description string `json:"description"`
	ReferenceID string `json:"referenceId"`
}

type RecordTransactionResponse struct {
	TransactionID string `json:"transactionId"`
	Status        string `json:"status"`
}

func (s *LedgerService) RecordTransaction(ctx context.Context, req *RecordTransactionRequest) (*RecordTransactionResponse, error) {
	var res RecordTransactionResponse
	err := s.client.do(ctx, http.MethodPost, "/v1/ledger/transactions", req, &res)
	return &res, err
}

type GetAccountResponse struct {
	AccountID string    `json:"accountId"`
	Balance   int64     `json:"balance"`
	Currency  string    `json:"currency"`
	CreatedAt time.Time `json:"createdAt"`
}

func (s *LedgerService) GetAccount(ctx context.Context, accountID string) (*GetAccountResponse, error) {
	var res GetAccountResponse
	err := s.client.do(ctx, http.MethodGet, fmt.Sprintf("/v1/ledger/accounts/%s", accountID), nil, &res)
	return &res, err
}

// AuthService handles authentication and authorization operations.
type AuthService struct {
	client *Client
}

type ValidateKeyResponse struct {
	Valid       bool   `json:"valid"`
	UserID      string `json:"userId"`
	OrgID       string `json:"orgId"`
	Environment string `json:"environment"`
	Scopes      string `json:"scopes"`
}

func (s *AuthService) ValidateKey(ctx context.Context, keyHash string) (*ValidateKeyResponse, error) {
	var res ValidateKeyResponse
	req := map[string]string{"keyHash": keyHash}
	err := s.client.do(ctx, http.MethodPost, "/v1/auth/validate", req, &res)
	return &res, err
}
