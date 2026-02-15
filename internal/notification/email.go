package notification

import (
	"context"
	"fmt"
	"os"

	"strings"

	"github.com/resend/resend-go/v2"
)

// EmailService handles sending emails via Resend
type EmailService struct {
	client    *resend.Client
	fromEmail string
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
	from = strings.Trim(from, "\"")

	return &EmailService{
		client:    resend.NewClient(apiKey),
		fromEmail: from,
	}
}

// SendEmail sends a transactional email
func (s *EmailService) SendEmail(ctx context.Context, to, subject, htmlBody string) error {
	params := &resend.SendEmailRequest{
		From:    s.fromEmail,
		To:      []string{to},
		Subject: subject,
		Html:    htmlBody,
	}

	_, err := s.client.Emails.SendWithContext(ctx, params)
	if err != nil {
		return fmt.Errorf("failed to send email via Resend: %w", err)
	}

	return nil
}
