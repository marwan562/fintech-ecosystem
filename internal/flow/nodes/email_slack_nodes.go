package nodes

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/smtp"
	"strings"
	"time"
)

// EmailActionNode sends emails via SMTP
type EmailActionNode struct {
	NodeID   string            `json:"id"`
	To       string            `json:"to"`      // Template: {{payload.email}}
	Subject  string            `json:"subject"` // Template
	Body     string            `json:"body"`    // Template
	From     string            `json:"from"`
	SMTPHost string            `json:"smtp_host"`
	SMTPPort string            `json:"smtp_port"`
	Username string            `json:"username"`
	Password string            `json:"password"`
	NextNode string            `json:"next,omitempty"`
	Headers  map[string]string `json:"-"`
}

// EmailConfig for building email nodes
type EmailConfig struct {
	ID       string
	SMTPHost string
	SMTPPort string
	From     string
	Username string
	Password string
}

// NewEmailActionNode creates a new email action node
func NewEmailActionNode(config EmailConfig) *EmailActionNode {
	return &EmailActionNode{
		NodeID:   config.ID,
		SMTPHost: config.SMTPHost,
		SMTPPort: config.SMTPPort,
		From:     config.From,
		Username: config.Username,
		Password: config.Password,
	}
}

// ID returns the node ID
func (n *EmailActionNode) ID() string { return n.NodeID }

// Type returns the node type
func (n *EmailActionNode) Type() string { return "email" }

// Execute sends the email
func (n *EmailActionNode) Execute(ctx context.Context, input map[string]interface{}) (*NodeResult, error) {
	// Resolve templates
	to := resolveTemplate(n.To, input)
	subject := resolveTemplate(n.Subject, input)
	body := resolveTemplate(n.Body, input)

	// Build email message
	msg := fmt.Sprintf("From: %s\r\nTo: %s\r\nSubject: %s\r\nContent-Type: text/html; charset=UTF-8\r\n\r\n%s",
		n.From, to, subject, body)

	// Setup authentication
	auth := smtp.PlainAuth("", n.Username, n.Password, n.SMTPHost)

	// Send email
	addr := fmt.Sprintf("%s:%s", n.SMTPHost, n.SMTPPort)
	err := smtp.SendMail(addr, auth, n.From, strings.Split(to, ","), []byte(msg))
	if err != nil {
		return &NodeResult{
			Success: false,
			Error:   fmt.Sprintf("failed to send email: %v", err),
		}, err
	}

	return &NodeResult{
		Success: true,
		Output: map[string]interface{}{
			"to":      to,
			"subject": subject,
			"sent_at": time.Now().Format(time.RFC3339),
		},
		Next: n.NextNode,
	}, nil
}

// SlackActionNode sends messages to Slack via webhook
type SlackActionNode struct {
	NodeID     string          `json:"id"`
	WebhookURL string          `json:"webhook_url"`
	Channel    string          `json:"channel"` // Optional channel override
	Text       string          `json:"text"`    // Template for message text
	Blocks     json.RawMessage `json:"blocks"`  // Optional Block Kit blocks
	Username   string          `json:"username"`
	IconEmoji  string          `json:"icon_emoji"`
	NextNode   string          `json:"next,omitempty"`
	client     *http.Client    `json:"-"`
}

// SlackConfig for building Slack nodes
type SlackConfig struct {
	ID         string
	WebhookURL string
	Username   string
	IconEmoji  string
}

// NewSlackActionNode creates a new Slack action node
func NewSlackActionNode(config SlackConfig) *SlackActionNode {
	return &SlackActionNode{
		NodeID:     config.ID,
		WebhookURL: config.WebhookURL,
		Username:   config.Username,
		IconEmoji:  config.IconEmoji,
		client: &http.Client{
			Timeout: 10 * time.Second,
		},
	}
}

// ID returns the node ID
func (n *SlackActionNode) ID() string { return n.NodeID }

// Type returns the node type
func (n *SlackActionNode) Type() string { return "slack" }

// Execute sends the Slack message
func (n *SlackActionNode) Execute(ctx context.Context, input map[string]interface{}) (*NodeResult, error) {
	// Build payload
	payload := map[string]interface{}{}

	// Resolve text template
	if n.Text != "" {
		payload["text"] = resolveTemplate(n.Text, input)
	}

	// Add blocks if present
	if len(n.Blocks) > 0 {
		var blocks interface{}
		json.Unmarshal(n.Blocks, &blocks)
		payload["blocks"] = blocks
	}

	// Add optional fields
	if n.Channel != "" {
		payload["channel"] = n.Channel
	}
	if n.Username != "" {
		payload["username"] = n.Username
	}
	if n.IconEmoji != "" {
		payload["icon_emoji"] = n.IconEmoji
	}

	// Send to Slack
	payloadBytes, _ := json.Marshal(payload)
	req, err := http.NewRequestWithContext(ctx, "POST", n.WebhookURL, bytes.NewReader(payloadBytes))
	if err != nil {
		return &NodeResult{
			Success: false,
			Error:   fmt.Sprintf("failed to create request: %v", err),
		}, err
	}
	req.Header.Set("Content-Type", "application/json")

	resp, err := n.client.Do(req)
	if err != nil {
		return &NodeResult{
			Success: false,
			Error:   fmt.Sprintf("failed to send to Slack: %v", err),
		}, err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return &NodeResult{
			Success: false,
			Error:   fmt.Sprintf("Slack returned status %d", resp.StatusCode),
		}, nil
	}

	return &NodeResult{
		Success: true,
		Output: map[string]interface{}{
			"channel":     n.Channel,
			"sent_at":     time.Now().Format(time.RFC3339),
			"status_code": resp.StatusCode,
		},
		Next: n.NextNode,
	}, nil
}

// resolveTemplate replaces {{path}} with values from input
func resolveTemplate(template string, input map[string]interface{}) string {
	result := template
	// Simple template resolution - find {{...}} and replace
	for strings.Contains(result, "{{") {
		start := strings.Index(result, "{{")
		end := strings.Index(result, "}}")
		if start == -1 || end == -1 {
			break
		}

		path := strings.TrimSpace(result[start+2 : end])
		value, err := extractValue(input, path)
		if err == nil {
			result = result[:start] + toString(value) + result[end+2:]
		} else {
			result = result[:start] + "" + result[end+2:]
		}
	}
	return result
}
