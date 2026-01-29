package notification

import (
	"bytes"
	"context"
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"testing"
	"time"

	"github.com/redis/go-redis/v9"
)

// MockTransport mocks the http.RoundTripper interface
type MockTransport struct {
	RoundTripFunc func(*http.Request) (*http.Response, error)
}

func (m *MockTransport) RoundTrip(req *http.Request) (*http.Response, error) {
	return m.RoundTripFunc(req)
}

func TestWebhookWorker_ProcessWebhook(t *testing.T) {
	// Mock Redis (nil is fine for this test)
	var redisClient *redis.Client = nil
	secret := "test_secret"

	t.Run("Success", func(t *testing.T) {
		worker := NewWebhookWorker(redisClient)

		// Inject Mock Transport
		worker.httpClient = &http.Client{
			Transport: &MockTransport{
				RoundTripFunc: func(r *http.Request) (*http.Response, error) {
					// Verify Method
					if r.Method != "POST" {
						return nil, fmt.Errorf("Expected POST, got %s", r.Method)
					}

					// Verify Headers
					if r.Header.Get("Content-Type") != "application/json" {
						return nil, fmt.Errorf("Expected Content-Type application/json")
					}
					signature := r.Header.Get("X-Webhook-Signature")
					if signature == "" {
						return nil, fmt.Errorf("Missing signature header")
					}

					// Verify Signature
					body, _ := io.ReadAll(r.Body)
					// Need to reset body if we were a server, but here we can just read it.
					// Note: The RoundTrip receives the request before it goes out.

					h := hmac.New(sha256.New, []byte(secret))
					h.Write(body)
					expectedSig := hex.EncodeToString(h.Sum(nil))
					if signature != expectedSig {
						return nil, fmt.Errorf("Signature mismatch. Got %s, want %s", signature, expectedSig)
					}

					return &http.Response{
						StatusCode: http.StatusOK,
						Body:       io.NopCloser(bytes.NewBufferString("{}")),
						Header:     make(http.Header),
					}, nil
				},
			},
		}

		task := WebhookTask{
			ID:        "wh_123",
			URL:       "http://example.com/webhook", // URL doesn't matter now
			Payload:   json.RawMessage(`{"foo":"bar"}`),
			Secret:    secret,
			EventType: "test.event",
		}

		taskBytes, _ := json.Marshal(task)
		err := worker.ProcessWebhook(context.Background(), taskBytes)
		if err != nil {
			t.Errorf("Unexpected error: %v", err)
		}
	})

	t.Run("RetrySuccess", func(t *testing.T) {
		attempts := 0
		worker := NewWebhookWorker(redisClient)
		worker.maxRetry = 3

		worker.httpClient = &http.Client{
			Transport: &MockTransport{
				RoundTripFunc: func(r *http.Request) (*http.Response, error) {
					attempts++
					if attempts < 2 {
						// Fail first attempt
						return &http.Response{
							StatusCode: http.StatusInternalServerError,
							Body:       io.NopCloser(bytes.NewBufferString("Error")),
						}, nil
					}
					// Succeed second attempt
					return &http.Response{
						StatusCode: http.StatusOK,
						Body:       io.NopCloser(bytes.NewBufferString("OK")),
					}, nil
				},
			},
		}

		task := WebhookTask{
			ID:      "wh_retry",
			URL:     "http://example.com/retry",
			Payload: json.RawMessage(`{}`),
			Secret:  secret,
		}

		taskBytes, _ := json.Marshal(task)
		// To speed up test, could override retry logic or mock time, but default backoff is small enough (1s) for one retry.
		// Start timer
		start := time.Now()

		err := worker.ProcessWebhook(context.Background(), taskBytes)
		if err != nil {
			t.Errorf("Unexpected error: %v", err)
		}

		if attempts != 2 {
			t.Errorf("Expected 2 attempts, got %d", attempts)
		}

		// In mock transport, the sleep happens in ProcessWebhook
		duration := time.Since(start)
		if duration < 1*time.Second {
			t.Log("Warning: Expected delay >= 1s, got", duration)
		}
	})

	t.Run("MaxRetriesExceeded", func(t *testing.T) {
		attempts := 0
		worker := NewWebhookWorker(redisClient)
		worker.maxRetry = 2

		worker.httpClient = &http.Client{
			Transport: &MockTransport{
				RoundTripFunc: func(r *http.Request) (*http.Response, error) {
					attempts++
					return &http.Response{
						StatusCode: http.StatusInternalServerError,
						Body:       io.NopCloser(bytes.NewBufferString("Error")),
					}, nil
				},
			},
		}

		task := WebhookTask{
			ID:      "wh_fail",
			URL:     "http://example.com/fail",
			Payload: json.RawMessage(`{}`),
			Secret:  secret,
		}

		taskBytes, _ := json.Marshal(task)
		err := worker.ProcessWebhook(context.Background(), taskBytes)
		if err == nil {
			t.Error("Expected error, got nil")
		}

		if attempts != 3 {
			t.Errorf("Expected 3 attempts, got %d", attempts)
		}
	})

	t.Run("ClientErrorNoRetry", func(t *testing.T) {
		attempts := 0
		worker := NewWebhookWorker(redisClient)
		// High maxRetry to prove we stop early
		worker.maxRetry = 5

		worker.httpClient = &http.Client{
			Transport: &MockTransport{
				RoundTripFunc: func(r *http.Request) (*http.Response, error) {
					attempts++
					return &http.Response{
						StatusCode: http.StatusBadRequest,
						Body:       io.NopCloser(bytes.NewBufferString("Bad Request")),
					}, nil
				},
			},
		}

		task := WebhookTask{
			ID:      "wh_400",
			URL:     "http://example.com/400",
			Payload: json.RawMessage(`{}`),
			Secret:  secret,
		}

		taskBytes, _ := json.Marshal(task)
		err := worker.ProcessWebhook(context.Background(), taskBytes)
		if err == nil {
			t.Error("Expected error for 400")
		}

		if attempts != 1 {
			t.Errorf("Expected 1 attempt (no retry for 400), got %d", attempts)
		}
	})
}
