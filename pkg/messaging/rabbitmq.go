package messaging

import (
	"context"
	"crypto/tls"
	"fmt"
	"log"
	"strings"
	"sync"
	"time"

	amqp "github.com/rabbitmq/amqp091-go"
)

// Config holds configuration for the RabbitMQ client
type Config struct {
	// Connection
	URL       string
	TLSConfig *tls.Config // Optional TLS configuration

	// Resilience
	ReconnectDelay    time.Duration
	MaxReconnectDelay time.Duration
	MaxRetries        int // -1 for infinite
	HeartbeatTimeout  time.Duration

	// Circuit Breaker
	CircuitBreakerEnabled   bool
	CircuitBreakerThreshold int
	CircuitBreakerTimeout   time.Duration
}

// DefaultConfig returns a default configuration
func DefaultConfig() Config {
	return Config{
		ReconnectDelay:          1 * time.Second,
		MaxReconnectDelay:       60 * time.Second,
		MaxRetries:              -1,
		HeartbeatTimeout:        10 * time.Second,
		CircuitBreakerEnabled:   true,
		CircuitBreakerThreshold: 5,
		CircuitBreakerTimeout:   30 * time.Second,
	}
}

// CircuitBreakerState represents the state of the circuit breaker
type CircuitBreakerState int

const (
	StateClosed CircuitBreakerState = iota
	StateOpen
	StateHalfOpen
)

type CircuitBreaker struct {
	state          CircuitBreakerState
	mu             sync.RWMutex
	failures       int
	threshold      int
	timeout        time.Duration
	lastFailure    time.Time
	successCounter int
}

type RabbitMQClient struct {
	config Config
	conn   *amqp.Connection
	ch     *amqp.Channel
	mu     sync.RWMutex

	// Connection management
	notifyConnClose chan *amqp.Error
	notifyChanClose chan *amqp.Error
	isReconnecting  bool
	isClosed        bool

	// Circuit Breaker
	cb *CircuitBreaker
}

func NewRabbitMQClient(config Config) (*RabbitMQClient, error) {
	// Apply defaults for zero values
	if config.ReconnectDelay == 0 {
		config.ReconnectDelay = time.Second
	}
	if config.MaxReconnectDelay == 0 {
		config.MaxReconnectDelay = 60 * time.Second
	}
	if config.HeartbeatTimeout == 0 {
		config.HeartbeatTimeout = 10 * time.Second
	}

	client := &RabbitMQClient{
		config: config,
		cb: &CircuitBreaker{
			state:     StateClosed,
			threshold: config.CircuitBreakerThreshold,
			timeout:   config.CircuitBreakerTimeout,
		},
	}

	if err := client.connect(); err != nil {
		return nil, err
	}

	go client.handleReconnect()

	return client, nil
}

// NewRabbitMQClientWithTLS is a helper for secure connections
func NewRabbitMQClientWithTLS(url string, tlsConfig *tls.Config) (*RabbitMQClient, error) {
	config := DefaultConfig()
	config.URL = url
	config.TLSConfig = tlsConfig
	return NewRabbitMQClient(config)
}

func (r *RabbitMQClient) connect() error {
	r.mu.Lock()
	defer r.mu.Unlock()

	var conn *amqp.Connection
	var err error

	// Mask password in logs
	safeURL := r.maskURL(r.config.URL)
	log.Printf("Connecting to RabbitMQ at %s", safeURL)

	if r.config.TLSConfig != nil {
		conn, err = amqp.DialTLS(r.config.URL, r.config.TLSConfig)
	} else {
		conn, err = amqp.DialConfig(r.config.URL, amqp.Config{
			Heartbeat: r.config.HeartbeatTimeout,
		})
	}

	if err != nil {
		return fmt.Errorf("failed to connect to rabbitmq: %w", err)
	}

	ch, err := conn.Channel()
	if err != nil {
		conn.Close()
		return fmt.Errorf("failed to open a channel: %w", err)
	}

	r.conn = conn
	r.ch = ch
	r.notifyConnClose = make(chan *amqp.Error)
	r.notifyChanClose = make(chan *amqp.Error)
	r.conn.NotifyClose(r.notifyConnClose)
	r.ch.NotifyClose(r.notifyChanClose)
	r.isReconnecting = false

	log.Println("Successfully connected to RabbitMQ")
	return nil
}

func (r *RabbitMQClient) handleReconnect() {
	for {
		r.mu.RLock()
		if r.isClosed {
			r.mu.RUnlock()
			return
		}
		notifyClose := r.notifyConnClose
		r.mu.RUnlock()

		err := <-notifyClose
		if err != nil {
			log.Printf("RabbitMQ connection closed: %v. Reconnecting...", err)
			r.reconnect()
		}
		return // reconnect creates a new handler loop
	}
}

func (r *RabbitMQClient) reconnect() {
	r.mu.Lock()
	r.isReconnecting = true
	r.mu.Unlock()

	backoff := r.config.ReconnectDelay
	retries := 0

	for {
		r.mu.RLock()
		if r.isClosed {
			r.mu.RUnlock()
			return
		}
		maxRetries := r.config.MaxRetries
		r.mu.RUnlock()

		if maxRetries != -1 && retries >= maxRetries {
			log.Printf("Max retries reached. Stopping reconnection attempts.")
			// In a real app we might want to panic or signal a fatal error here
			return
		}

		if err := r.connect(); err == nil {
			log.Println("RabbitMQ reconnected")
			go r.handleReconnect()
			return
		}

		log.Printf("Failed to reconnect: waiting %v", backoff)
		time.Sleep(backoff)

		// Exponential backoff
		backoff *= 2
		if backoff > r.config.MaxReconnectDelay {
			backoff = r.config.MaxReconnectDelay
		}
		retries++
	}
}

func (r *RabbitMQClient) DeclareQueue(name string) (amqp.Queue, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()

	if r.ch == nil {
		return amqp.Queue{}, fmt.Errorf("channel is not initialized")
	}

	return r.ch.QueueDeclare(
		name,  // name
		true,  // durable
		false, // delete when unused
		false, // exclusive
		false, // no-wait
		nil,   // arguments
	)
}

func (r *RabbitMQClient) DeclareQueueWithDLQ(name string) (amqp.Queue, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()

	if r.ch == nil {
		return amqp.Queue{}, fmt.Errorf("channel is not initialized")
	}

	dlqName := name + ".dlq"

	// Declare the DLQ first
	_, err := r.ch.QueueDeclare(
		dlqName,
		true,  // durable
		false, // delete when unused
		false, // exclusive
		false, // no-wait
		nil,   // arguments
	)
	if err != nil {
		return amqp.Queue{}, fmt.Errorf("failed to declare DLQ: %w", err)
	}

	// Declare the main queue with DLQ routing
	return r.ch.QueueDeclare(
		name,
		true,  // durable
		false, // delete when unused
		false, // exclusive
		false, // no-wait
		amqp.Table{
			"x-dead-letter-exchange":    "",
			"x-dead-letter-routing-key": dlqName,
		},
	)
}

func (r *RabbitMQClient) Publish(ctx context.Context, queueName string, body []byte) error {
	if r.config.CircuitBreakerEnabled && !r.cb.Allow() {
		return fmt.Errorf("circuit breaker is open")
	}

	r.mu.RLock()
	if r.isReconnecting || r.ch == nil {
		r.mu.RUnlock()
		return fmt.Errorf("connection is not available")
	}
	ch := r.ch
	r.mu.RUnlock()

	err := ch.PublishWithContext(ctx,
		"",        // exchange
		queueName, // routing key
		false,     // mandatory
		false,     // immediate
		amqp.Publishing{
			ContentType: "application/json",
			Body:        body,
		})

	if r.config.CircuitBreakerEnabled {
		if err != nil {
			r.cb.RecordFailure()
		} else {
			r.cb.RecordSuccess()
		}
	}

	return err
}

func (r *RabbitMQClient) Consume(queueName string, handler func(body []byte) error) {
	// Basic consume wrapper - for complex cases use ConsumeWithContext
	go func() {
		err := r.ConsumeWithContext(context.Background(), queueName, handler)
		if err != nil {
			log.Printf("Consumer for %s stopped: %v", queueName, err)
		}
	}()
}

// ConsumeWithContext allows graceful shutdown of consumers
func (r *RabbitMQClient) ConsumeWithContext(ctx context.Context, queueName string, handler func(body []byte) error) error {
	for {
		// Calculate retry delay with jitter to prevent thundering herd
		select {
		case <-ctx.Done():
			return nil
		default:
		}

		r.mu.RLock()
		if r.isReconnecting || r.ch == nil {
			r.mu.RUnlock()
			time.Sleep(time.Second) // Wait for reconnection
			continue
		}
		ch := r.ch
		r.mu.RUnlock()

		msgs, err := ch.Consume(
			queueName, // queue
			"",        // consumer
			false,     // auto-ack
			false,     // exclusive
			false,     // no-local
			false,     // no-wait
			nil,       // args
		)

		if err != nil {
			log.Printf("failed to register a consumer: %v", err)
			time.Sleep(2 * time.Second)
			continue
		}

		// Handle messages
		for {
			select {
			case <-ctx.Done():
				return nil
			case d, ok := <-msgs:
				if !ok {
					// Channel closed (likely connection lost)
					goto Reconnect
				}
				if err := handler(d.Body); err != nil {
					log.Printf("error handling message: %v", err)
					d.Nack(false, true) // Requeue
				} else {
					d.Ack(false)
				}
			}
		}

	Reconnect:
		log.Printf("Consumer channel closed for %s, waiting for reconnection...", queueName)
		time.Sleep(r.config.ReconnectDelay)
	}
}

func (r *RabbitMQClient) Close() {
	r.mu.Lock()
	defer r.mu.Unlock()

	r.isClosed = true
	if r.ch != nil {
		r.ch.Close()
	}
	if r.conn != nil {
		r.conn.Close()
	}
}

func (r *RabbitMQClient) IsHealthy() bool {
	r.mu.RLock()
	defer r.mu.RUnlock()
	return r.conn != nil && !r.conn.IsClosed() && !r.isReconnecting
}

func (r *RabbitMQClient) maskURL(url string) string {
	if parts := strings.Split(url, "@"); len(parts) > 1 {
		// Assuming amqp://user:pass@host...
		prefixParts := strings.Split(parts[0], "://")
		if len(prefixParts) == 2 {
			return prefixParts[0] + "://***:***@" + parts[1]
		}
	}
	return url
}

// Circuit Breaker Methods

func (cb *CircuitBreaker) Allow() bool {
	cb.mu.RLock()
	defer cb.mu.RUnlock()

	switch cb.state {
	case StateOpen:
		if time.Since(cb.lastFailure) > cb.timeout {
			// Allow one request to test (Half-Open logic handled in RecordSuccess/Failure)
			return true
		}
		return false
	case StateHalfOpen:
		// Limited allowance could be implemented here
		return true
	default: // Closed
		return true
	}
}

func (cb *CircuitBreaker) RecordSuccess() {
	cb.mu.Lock()
	defer cb.mu.Unlock()

	if cb.state == StateHalfOpen {
		cb.successCounter++
		if cb.successCounter >= 3 { // Arbitrary success threshold to close
			cb.state = StateClosed
			cb.failures = 0
			cb.successCounter = 0
		}
	} else if cb.state == StateOpen {
		// Should have been half-open first, but if we succeeded, reset
		cb.state = StateClosed
		cb.failures = 0
	}
}

func (cb *CircuitBreaker) RecordFailure() {
	cb.mu.Lock()
	defer cb.mu.Unlock()

	cb.failures++
	cb.lastFailure = time.Now()

	if cb.state == StateClosed && cb.failures >= cb.threshold {
		cb.state = StateOpen
	} else if cb.state == StateHalfOpen {
		cb.state = StateOpen // Back to open if check fails
	}
}
