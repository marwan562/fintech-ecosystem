package nodes

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"regexp"
	"strings"
	"time"
)

// WebhookActionNode sends HTTP requests to external services
type WebhookActionNode struct {
	NodeID      string            `json:"id"`
	URL         string            `json:"url"`
	Method      string            `json:"method"`
	Headers     map[string]string `json:"headers,omitempty"`
	Body        string            `json:"body,omitempty"`
	Timeout     time.Duration     `json:"timeout,omitempty"`
	RetryCount  int               `json:"retryCount,omitempty"`
	RetryDelay  time.Duration     `json:"retryDelay,omitempty"`
	NextNode    string            `json:"next,omitempty"`
	OnErrorNode string            `json:"onError,omitempty"`
	client      *http.Client      `json:"-"`
}

// WebhookActionConfig is used to create a new webhook action node
type WebhookActionConfig struct {
	ID          string
	URL         string
	Method      string
	Headers     map[string]string
	Body        string
	Timeout     time.Duration
	RetryCount  int
	RetryDelay  time.Duration
	NextNode    string
	OnErrorNode string
}

// NewWebhookActionNode creates a new webhook action node
func NewWebhookActionNode(config WebhookActionConfig) *WebhookActionNode {
	timeout := config.Timeout
	if timeout == 0 {
		timeout = 30 * time.Second
	}

	method := config.Method
	if method == "" {
		method = "POST"
	}

	return &WebhookActionNode{
		NodeID:      config.ID,
		URL:         config.URL,
		Method:      method,
		Headers:     config.Headers,
		Body:        config.Body,
		Timeout:     timeout,
		RetryCount:  config.RetryCount,
		RetryDelay:  config.RetryDelay,
		NextNode:    config.NextNode,
		OnErrorNode: config.OnErrorNode,
		client: &http.Client{
			Timeout: timeout,
		},
	}
}

// ID returns the node ID
func (n *WebhookActionNode) ID() string {
	return n.NodeID
}

// Type returns the node type
func (n *WebhookActionNode) Type() string {
	return "webhook_action"
}

// Execute sends the webhook request
func (n *WebhookActionNode) Execute(ctx context.Context, input map[string]interface{}) (*NodeResult, error) {
	// Resolve template variables in URL and body
	resolvedURL := n.resolveTemplate(n.URL, input)
	resolvedBody := n.resolveTemplate(n.Body, input)

	var lastErr error
	attempts := n.RetryCount + 1
	if attempts < 1 {
		attempts = 1
	}

	for attempt := 1; attempt <= attempts; attempt++ {
		result, err := n.sendRequest(ctx, resolvedURL, resolvedBody, input)
		if err == nil && result.Success {
			result.Next = n.NextNode
			return result, nil
		}

		lastErr = err
		if attempt < attempts {
			time.Sleep(n.RetryDelay)
		}
	}

	// All retries failed
	errorMsg := "webhook request failed"
	if lastErr != nil {
		errorMsg = lastErr.Error()
	}

	return &NodeResult{
		Success: false,
		Error:   errorMsg,
		Next:    n.OnErrorNode,
	}, nil
}

// sendRequest performs the actual HTTP request
func (n *WebhookActionNode) sendRequest(ctx context.Context, url, body string, input map[string]interface{}) (*NodeResult, error) {
	var bodyReader io.Reader
	if body != "" {
		bodyReader = bytes.NewBufferString(body)
	}

	req, err := http.NewRequestWithContext(ctx, n.Method, url, bodyReader)
	if err != nil {
		return nil, fmt.Errorf("failed to create request: %w", err)
	}

	// Set default content type for POST/PUT/PATCH
	if n.Method == "POST" || n.Method == "PUT" || n.Method == "PATCH" {
		req.Header.Set("Content-Type", "application/json")
	}

	// Apply custom headers
	for key, value := range n.Headers {
		resolvedValue := n.resolveTemplate(value, input)
		req.Header.Set(key, resolvedValue)
	}

	// Send request
	resp, err := n.client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("request failed: %w", err)
	}
	defer resp.Body.Close()

	// Read response body
	respBody, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("failed to read response: %w", err)
	}

	// Parse response as JSON if possible
	var respData interface{}
	if err := json.Unmarshal(respBody, &respData); err != nil {
		respData = string(respBody)
	}

	// Check for success (2xx status codes)
	success := resp.StatusCode >= 200 && resp.StatusCode < 300

	return &NodeResult{
		Success: success,
		Output: map[string]interface{}{
			"statusCode":   resp.StatusCode,
			"responseBody": respData,
			"headers":      headerToMap(resp.Header),
		},
		Error: func() string {
			if !success {
				return fmt.Sprintf("HTTP %d: %s", resp.StatusCode, string(respBody))
			}
			return ""
		}(),
	}, nil
}

// resolveTemplate replaces {{variable}} placeholders with values from input
func (n *WebhookActionNode) resolveTemplate(template string, input map[string]interface{}) string {
	if template == "" {
		return ""
	}

	re := regexp.MustCompile(`\{\{([^}]+)\}\}`)
	return re.ReplaceAllStringFunc(template, func(match string) string {
		path := strings.TrimSpace(match[2 : len(match)-2])

		// Handle special cases
		if path == "." || path == "input" {
			b, _ := json.Marshal(input)
			return string(b)
		}

		// Handle JSON filter
		if strings.HasSuffix(path, " | json") {
			path = strings.TrimSuffix(path, " | json")
			val, err := extractValue(input, path)
			if err != nil {
				return match
			}
			b, _ := json.Marshal(val)
			return string(b)
		}

		// Extract nested value
		val, err := extractValue(input, path)
		if err != nil {
			return match
		}

		return toString(val)
	})
}

// headerToMap converts http.Header to a simple map
func headerToMap(h http.Header) map[string]string {
	result := make(map[string]string)
	for key, values := range h {
		if len(values) > 0 {
			result[key] = values[0]
		}
	}
	return result
}

// WebhookActionBuilder provides a fluent interface for building webhook nodes
type WebhookActionBuilder struct {
	config WebhookActionConfig
}

// NewWebhookAction starts building a webhook action node
func NewWebhookAction(id string) *WebhookActionBuilder {
	return &WebhookActionBuilder{
		config: WebhookActionConfig{
			ID:      id,
			Method:  "POST",
			Headers: make(map[string]string),
		},
	}
}

// URL sets the webhook URL
func (b *WebhookActionBuilder) URL(url string) *WebhookActionBuilder {
	b.config.URL = url
	return b
}

// Method sets the HTTP method
func (b *WebhookActionBuilder) Method(method string) *WebhookActionBuilder {
	b.config.Method = method
	return b
}

// Header adds a header
func (b *WebhookActionBuilder) Header(key, value string) *WebhookActionBuilder {
	b.config.Headers[key] = value
	return b
}

// Body sets the request body template
func (b *WebhookActionBuilder) Body(body string) *WebhookActionBuilder {
	b.config.Body = body
	return b
}

// Timeout sets the request timeout
func (b *WebhookActionBuilder) Timeout(d time.Duration) *WebhookActionBuilder {
	b.config.Timeout = d
	return b
}

// Retry configures retry behavior
func (b *WebhookActionBuilder) Retry(count int, delay time.Duration) *WebhookActionBuilder {
	b.config.RetryCount = count
	b.config.RetryDelay = delay
	return b
}

// Then sets the next node on success
func (b *WebhookActionBuilder) Then(nodeID string) *WebhookActionBuilder {
	b.config.NextNode = nodeID
	return b
}

// OnError sets the node to execute on error
func (b *WebhookActionBuilder) OnError(nodeID string) *WebhookActionBuilder {
	b.config.OnErrorNode = nodeID
	return b
}

// Build creates the webhook action node
func (b *WebhookActionBuilder) Build() *WebhookActionNode {
	return NewWebhookActionNode(b.config)
}
