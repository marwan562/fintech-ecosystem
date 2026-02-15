package main

import (
	"context"
	"fmt"
	"log"
	"os"

	"github.com/resend/resend-go/v2"
)

func main() {
	apiKey := os.Getenv("RESEND_API_KEY")
	if apiKey == "" {
		log.Fatal("RESEND_API_KEY is not set")
	}

	from := os.Getenv("FROM_EMAIL")
	if from == "" {
		from = "onboarding@resend.dev"
	}

	to := os.Getenv("CONTACT_EMAIL")
	if to == "" {
		log.Fatal("CONTACT_EMAIL is not set")
	}

	client := resend.NewClient(apiKey)

	params := &resend.SendEmailRequest{
		From:    from,
		To:      []string{to},
		Subject: "Test Email from Sapliy Fintech",
		Html:    "<p>This is a test email to verify Resend integration.</p>",
	}

	sent, err := client.Emails.SendWithContext(context.Background(), params)
	if err != nil {
		log.Fatalf("Failed to send email: %v", err)
	}

	fmt.Printf("Email sent successfully! ID: %s\n", sent.Id)
}
