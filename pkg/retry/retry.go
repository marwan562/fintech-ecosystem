package retry

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"math"
	"time"

	"github.com/redis/go-redis/v9"
)

// Config defines retry behavior for actions
type Config struct {
	MaxAttempts     int           // Maximum number of attempts (including first)
	InitialDelay    time.Duration // Initial delay before first retry
	MaxDelay        time.Duration // Maximum delay between retries
	Multiplier      float64       // Multiplier for exponential backoff
	DLQEnabled      bool          // Whether to send to DLQ on final failure
	DLQStreamPrefix string        // DLQ stream prefix (default: "dlq")
}

// DefaultConfig returns sensible defaults for sapliy operations
func DefaultConfig() Config {
	return Config{
		MaxAttempts:     5,
		InitialDelay:    1 * time.Second,
		MaxDelay:        30 * time.Second,
		Multiplier:      2.0,
		DLQEnabled:      true,
		DLQStreamPrefix: "dlq",
	}
}

// DLQEntry represents a failed event in the Dead Letter Queue
type DLQEntry struct {
	OriginalStream string                 `json:"original_stream"`
	EventID        string                 `json:"event_id"`
	ZoneID         string                 `json:"zone_id"`
	Type           string                 `json:"type"`
	Payload        map[string]interface{} `json:"payload"`
	Error          string                 `json:"error"`
	Attempts       int                    `json:"attempts"`
	FailedAt       string                 `json:"failed_at"`
	LastAttemptAt  string                 `json:"last_attempt_at"`
}

// Retrier handles retry logic with exponential backoff
type Retrier struct {
	config Config
	rdb    *redis.Client
}

// NewRetrier creates a new retrier with the given config
func NewRetrier(rdb *redis.Client, config Config) *Retrier {
	if config.MaxAttempts < 1 {
		config.MaxAttempts = 1
	}
	if config.InitialDelay == 0 {
		config.InitialDelay = 1 * time.Second
	}
	if config.MaxDelay == 0 {
		config.MaxDelay = 30 * time.Second
	}
	if config.Multiplier == 0 {
		config.Multiplier = 2.0
	}
	if config.DLQStreamPrefix == "" {
		config.DLQStreamPrefix = "dlq"
	}
	return &Retrier{config: config, rdb: rdb}
}

// Execute runs the operation with exponential backoff retry
func (r *Retrier) Execute(ctx context.Context, op func() error) error {
	var lastErr error

	for attempt := 1; attempt <= r.config.MaxAttempts; attempt++ {
		lastErr = op()
		if lastErr == nil {
			return nil // Success
		}

		if attempt < r.config.MaxAttempts {
			delay := r.calculateDelay(attempt)
			log.Printf("Attempt %d/%d failed: %v. Retrying in %v", attempt, r.config.MaxAttempts, lastErr, delay)

			select {
			case <-ctx.Done():
				return ctx.Err()
			case <-time.After(delay):
				// Continue to next attempt
			}
		}
	}

	return lastErr
}

// ExecuteWithDLQ runs the operation with retry and sends to DLQ on final failure
func (r *Retrier) ExecuteWithDLQ(ctx context.Context, entry DLQEntry, op func() error) error {
	err := r.Execute(ctx, op)
	if err != nil && r.config.DLQEnabled && r.rdb != nil {
		entry.Error = err.Error()
		entry.Attempts = r.config.MaxAttempts
		entry.FailedAt = time.Now().UTC().Format(time.RFC3339)
		entry.LastAttemptAt = time.Now().UTC().Format(time.RFC3339)

		dlqErr := r.sendToDLQ(ctx, entry)
		if dlqErr != nil {
			log.Printf("Failed to send to DLQ: %v", dlqErr)
		}
	}
	return err
}

// calculateDelay computes the delay for the given attempt using exponential backoff
func (r *Retrier) calculateDelay(attempt int) time.Duration {
	delay := float64(r.config.InitialDelay) * math.Pow(r.config.Multiplier, float64(attempt-1))
	if delay > float64(r.config.MaxDelay) {
		delay = float64(r.config.MaxDelay)
	}
	return time.Duration(delay)
}

// sendToDLQ sends the failed entry to the Dead Letter Queue
func (r *Retrier) sendToDLQ(ctx context.Context, entry DLQEntry) error {
	streamName := fmt.Sprintf("%s.%s", r.config.DLQStreamPrefix, entry.ZoneID)
	entryBytes, _ := json.Marshal(entry)

	return r.rdb.XAdd(ctx, &redis.XAddArgs{
		Stream: streamName,
		Values: map[string]interface{}{
			"entry": entryBytes,
			"ts":    time.Now().Unix(),
		},
	}).Err()
}

// GetDLQEntries retrieves entries from the DLQ for a zone
func (r *Retrier) GetDLQEntries(ctx context.Context, zoneID string, count int64) ([]DLQEntry, error) {
	streamName := fmt.Sprintf("%s.%s", r.config.DLQStreamPrefix, zoneID)

	result, err := r.rdb.XRange(ctx, streamName, "-", "+").Result()
	if err != nil {
		return nil, err
	}

	entries := make([]DLQEntry, 0, len(result))
	for _, msg := range result {
		if entryStr, ok := msg.Values["entry"].(string); ok {
			var entry DLQEntry
			if json.Unmarshal([]byte(entryStr), &entry) == nil {
				entries = append(entries, entry)
			}
		}
	}

	return entries, nil
}

// ReplayDLQEntry replays a DLQ entry by publishing it back to the original stream
func (r *Retrier) ReplayDLQEntry(ctx context.Context, entry DLQEntry) error {
	payload, _ := json.Marshal(entry.Payload)

	return r.rdb.XAdd(ctx, &redis.XAddArgs{
		Stream: entry.OriginalStream,
		Values: map[string]interface{}{
			"data":     payload,
			"replayed": true,
			"ts":       time.Now().Unix(),
		},
	}).Err()
}

// PurgeDLQEntry removes an entry from the DLQ
func (r *Retrier) PurgeDLQEntry(ctx context.Context, zoneID, messageID string) error {
	streamName := fmt.Sprintf("%s.%s", r.config.DLQStreamPrefix, zoneID)
	return r.rdb.XDel(ctx, streamName, messageID).Err()
}
