package notification

import (
	"bytes"
	"text/template"
)

// Templates is a map of template ID to template content.
var Templates = map[string]string{
	"payment_success": `
		Hello {{.UserName}},

		Your payment of {{.Amount}} {{.Currency}} was successful.

		Transaction ID: {{.TransactionID}}

		Thank you for your business!
	`,
	"payment_failed": `
		Hello {{.UserName}},

		Unfortunately, your payment of {{.Amount}} {{.Currency}} has failed.

		Please try again or contact support.
	`,
	"payment_refund": `
		Hello {{.UserName}},

		Your payment of {{.Amount}} {{.Currency}} has been refunded.

		Refund ID: {{.RefundID}}

		The funds should appear in your account within 5-10 business days.
	`,
	"welcome": `
		Welcome to our platform, {{.UserName}}!

		We're excited to have you on board. Start exploring our services today.

		Best regards,
		The Fintech Team
	`,
	"otp": `
		Your verification code is: {{.OTPCode}}

		This code will expire in {{.ExpiryMinutes}} minutes.

		If you did not request this code, please ignore this message.
	`,
}

// RenderTemplate renders a template by ID with the given data.
func RenderTemplate(templateID string, data map[string]string) (string, error) {
	content, ok := Templates[templateID]
	if !ok {
		// If template not found, return a generic message
		return "Notification: " + templateID, nil
	}

	tmpl, err := template.New(templateID).Parse(content)
	if err != nil {
		return "", err
	}

	var buf bytes.Buffer
	if err := tmpl.Execute(&buf, data); err != nil {
		return "", err
	}

	return buf.String(), nil
}
