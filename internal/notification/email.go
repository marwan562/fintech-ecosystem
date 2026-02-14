package notification

import (
	"context"
	"fmt"
	"os"

	"github.com/resend/resend-go/v2"
)

// EmailService handles sending emails via Resend
type EmailService struct {
	client       *resend.Client
	fromEmail    string
	contactEmail string // Override for development if needed
}

// NewEmailService creates a new email service
func NewEmailService(apiKey string) *EmailService {
	if apiKey == "" {
		apiKey = os.Getenv("RESEND_API_KEY")
	}

	from := os.Getenv("FROM_EMAIL")
	if from == "" {
		from = "onboarding@resend.dev"
	}

	contact := os.Getenv("CONTACT_EMAIL")

	return &EmailService{
		client:       resend.NewClient(apiKey),
		fromEmail:    from,
		contactEmail: contact,
	}
}

// SendEmail sends a transactional email
func (s *EmailService) SendEmail(ctx context.Context, to, subject, htmlBody string) error {
	// For development/testing, optionally redirect to contact email
	recipient := to
	if s.contactEmail != "" {
		// Logic to override or log could go here.
		// For now, we'll respect the input 'to', but assume the caller might handle dev overrides
		// or we can force it here if strictly in dev mode.
		// Given the user request "CONTACT_EMAIL=... it just now for developement",
		// let's override the recipient for safety if in dev mode (which we can infer or simpler: just use it if set).
		// However, purely replacing might block real testing.
		// Let's assume for this specific user request, they want to see the emails.
		recipient = s.contactEmail
		subject = fmt.Sprintf("[DEV-REDIRECT] %s (Original: %s)", subject, to)
	}

	params := &resend.SendEmailRequest{
		From:    s.fromEmail,
		To:      []string{recipient},
		Subject: subject,
		Html:    htmlBody,
	}

	_, err := s.client.Emails.SendWithContext(ctx, params)
	if err != nil {
		return fmt.Errorf("failed to send email via Resend: %w", err)
	}

	return nil
}
