package notification

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"math"
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
	redis    *redis.Client
	maxRetry int
}

// NewWebhookWorker creates a new webhook worker
func NewWebhookWorker(redisClient *redis.Client) *WebhookWorker {
	return &WebhookWorker{
		redis:    redisClient,
		maxRetry: 5,
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

	// Deliver webhook (mock for now)
	log.Printf("[WEBHOOK] Delivering to %s", task.URL)
	log.Printf("[WEBHOOK] Event: %s", task.EventType)
	log.Printf("[WEBHOOK] Signature: %s", signature)
	log.Printf("[WEBHOOK] Payload: %s", string(task.Payload))

	// In production, this would be an HTTP POST with:
	// - X-Webhook-Signature header
	// - Event type header
	// - Timestamp header
	// - JSON payload

	// Mark as delivered
	if w.redis != nil {
		w.redis.Set(ctx, fmt.Sprintf("webhook:sent:%s", task.ID), "1", 7*24*time.Hour)
	}

	log.Printf("[WEBHOOK] Successfully delivered %s", task.ID)
	return nil
}

// createHMAC creates an HMAC signature for webhook verification
func createHMAC(payload []byte, secret string) string {
	// In production, use crypto/hmac with SHA256
	// For now, just return a placeholder
	if secret == "" {
		return "unsigned"
	}
	return fmt.Sprintf("sha256=%x", "mock_signature")
}
