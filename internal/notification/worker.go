package notification

import (
	"bytes"
	"context"
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"log"
	"math"
	"net/http"
	"time"

	"github.com/redis/go-redis/v9"
)

// Worker processes notification tasks from RabbitMQ
type Worker struct {
	channel  Channel
	driver   Driver
	redis    *redis.Client
	maxRetry int
}

// NewWorker creates a new notification worker
func NewWorker(channel Channel, driver Driver, redisClient *redis.Client) *Worker {
	return &Worker{
		channel:  channel,
		driver:   driver,
		redis:    redisClient,
		maxRetry: 3,
	}
}

// ProcessTask processes a notification task with idempotency and retry logic
func (w *Worker) ProcessTask(ctx context.Context, body []byte) error {
	var task NotificationTask
	if err := json.Unmarshal(body, &task); err != nil {
		return fmt.Errorf("failed to unmarshal task: %w", err)
	}

	// Idempotency check
	if w.redis != nil {
		idempotencyKey := fmt.Sprintf("notif:sent:%s", task.ID)
		exists, err := w.redis.Exists(ctx, idempotencyKey).Result()
		if err != nil {
			log.Printf("Redis error checking idempotency: %v", err)
		} else if exists > 0 {
			log.Printf("Task %s already processed (idempotent skip)", task.ID)
			return nil
		}
	}

	// Render template
	content, err := RenderTemplate(task.TemplateID, task.Data)
	if err != nil {
		log.Printf("Failed to render template: %v", err)
		content = "Notification content unavailable"
	}

	title := task.Data["Title"]
	if title == "" {
		title = "Notification"
	}

	// Send via driver
	if err := w.driver.Send(ctx, task.Recipient, title, content); err != nil {
		log.Printf("Failed to send notification: %v", err)
		return w.handleRetry(ctx, &task, err)
	}

	// Mark as sent (idempotency)
	if w.redis != nil {
		w.redis.Set(ctx, fmt.Sprintf("notif:sent:%s", task.ID), "1", 24*time.Hour)
	}

	log.Printf("Successfully processed task %s via %s", task.ID, w.channel)
	return nil
}

func (w *Worker) handleRetry(ctx context.Context, task *NotificationTask, originalErr error) error {
	task.RetryCount++
	if task.RetryCount >= task.MaxRetries {
		log.Printf("Task %s exceeded max retries, sending to DLQ", task.ID)
		return fmt.Errorf("max retries exceeded: %w", originalErr)
	}

	// Calculate exponential backoff delay
	delay := time.Duration(math.Pow(2, float64(task.RetryCount))) * time.Second
	log.Printf("Task %s will retry in %v (attempt %d/%d)", task.ID, delay, task.RetryCount, task.MaxRetries)

	// In a real system, you'd republish to a delayed queue or use RabbitMQ's delayed message plugin
	return originalErr
}

// WebhookWorker processes webhook delivery tasks
type WebhookWorker struct {
	redis      *redis.Client
	maxRetry   int
	httpClient *http.Client
}

// NewWebhookWorker creates a new webhook worker
func NewWebhookWorker(redisClient *redis.Client) *WebhookWorker {
	return &WebhookWorker{
		redis:      redisClient,
		maxRetry:   5,
		httpClient: &http.Client{Timeout: 10 * time.Second},
	}
}

// ProcessWebhook processes a webhook delivery task
func (w *WebhookWorker) ProcessWebhook(ctx context.Context, body []byte) error {
	var task WebhookTask
	if err := json.Unmarshal(body, &task); err != nil {
		return fmt.Errorf("failed to unmarshal webhook task: %w", err)
	}

	// Idempotency check
	if w.redis != nil {
		idempotencyKey := fmt.Sprintf("webhook:sent:%s", task.ID)
		exists, err := w.redis.Exists(ctx, idempotencyKey).Result()
		if err != nil {
			log.Printf("Redis error checking idempotency: %v", err)
		} else if exists > 0 {
			log.Printf("Webhook %s already delivered (idempotent skip)", task.ID)
			return nil
		}
	}

	// Skip if no URL configured
	if task.URL == "" {
		log.Printf("Webhook %s has no URL configured, skipping", task.ID)
		return nil
	}

	// Create HMAC signature
	signature := createHMAC(task.Payload, task.Secret)

	// Prepare request
	req, err := http.NewRequestWithContext(ctx, "POST", task.URL, bytes.NewBuffer(task.Payload))
	if err != nil {
		return fmt.Errorf("failed to create request: %w", err)
	}

	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("X-Webhook-Signature", signature)
	req.Header.Set("X-Webhook-Event", string(task.EventType))
	req.Header.Set("X-Webhook-ID", task.ID)
	req.Header.Set("X-Webhook-Timestamp", time.Now().UTC().Format(time.RFC3339))

	client := w.httpClient
	if client == nil {
		client = &http.Client{Timeout: 10 * time.Second}
	}

	// Retry loop
	var lastErr error
	for i := 0; i <= w.maxRetry; i++ {
		if i > 0 {
			// Exponential backoff: 1s, 2s, 4s, 8s, 16s
			sleepDuration := time.Duration(math.Pow(2, float64(i-1))) * time.Second
			log.Printf("Webhook %s retry %d/%d in %v...", task.ID, i, w.maxRetry, sleepDuration)
			select {
			case <-ctx.Done():
				return ctx.Err()
			case <-time.After(sleepDuration):
			}
		}

		resp, err := client.Do(req)
		if err != nil {
			log.Printf("Webhook %s attempt %d failed: %v", task.ID, i+1, err)
			lastErr = err
			continue // Retry on network error
		}
		defer resp.Body.Close()

		if resp.StatusCode >= 200 && resp.StatusCode < 300 {
			// Success
			log.Printf("[WEBHOOK] Successfully delivered %s to %s (Status: %d)", task.ID, task.URL, resp.StatusCode)

			// Mark as delivered
			if w.redis != nil {
				w.redis.Set(ctx, fmt.Sprintf("webhook:sent:%s", task.ID), "1", 7*24*time.Hour)
			}
			return nil
		}

		// Handle HTTP errors
		log.Printf("Webhook %s attempt %d returned status: %d", task.ID, i+1, resp.StatusCode)
		lastErr = fmt.Errorf("server returned status: %d", resp.StatusCode)

		// Don't retry on client errors (4xx) except maybe 429 (Too Many Requests) or 408 (Request Timeout)
		// For simplicity/safety in this robust reliable delivery, we might only retry 5xx and network errors.
		// However, standard practice often retries everything or specific codes.
		// Use a simple heuristic: if 4xx (except 429), break.
		if resp.StatusCode >= 400 && resp.StatusCode < 500 && resp.StatusCode != 429 {
			log.Printf("Webhook %s failed with client error %d, not retrying", task.ID, resp.StatusCode)
			return lastErr
		}
	}

	return fmt.Errorf("failed to deliver webhook %s after %d retries: %w", task.ID, w.maxRetry, lastErr)
}

// createHMAC creates an HMAC signature for webhook verification
func createHMAC(payload []byte, secret string) string {
	if secret == "" {
		return ""
	}
	h := hmac.New(sha256.New, []byte(secret))
	h.Write(payload)
	return hex.EncodeToString(h.Sum(nil))
}
