package notification

import (
	"context"
	"fmt"
	"log"
)

// Driver defines the interface for sending notifications via different channels.
type Driver interface {
	Send(ctx context.Context, recipient, title, content string) error
	Channel() Channel
}

// EmailDriver is a mock email driver (e.g., SendGrid).
type EmailDriver struct{}

func NewEmailDriver() *EmailDriver {
	return &EmailDriver{}
}

func (d *EmailDriver) Channel() Channel {
	return Email
}

func (d *EmailDriver) Send(ctx context.Context, recipient, title, content string) error {
	// In production, this would call SendGrid, AWS SES, etc.
	log.Printf("[EMAIL DRIVER] Sending email to %s", recipient)
	log.Printf("[EMAIL DRIVER] Subject: %s", title)
	log.Printf("[EMAIL DRIVER] Body:\n%s", content)
	log.Printf("[EMAIL DRIVER] Email sent successfully to %s", recipient)
	return nil
}

// SMSDriver is a mock SMS driver (e.g., Twilio).
type SMSDriver struct{}

func NewSMSDriver() *SMSDriver {
	return &SMSDriver{}
}

func (d *SMSDriver) Channel() Channel {
	return SMS
}

func (d *SMSDriver) Send(ctx context.Context, recipient, title, content string) error {
	// In production, this would call Twilio, AWS SNS, etc.
	log.Printf("[SMS DRIVER] Sending SMS to %s", recipient)
	log.Printf("[SMS DRIVER] Message: %s", content)
	log.Printf("[SMS DRIVER] SMS sent successfully to %s", recipient)
	return nil
}

// WebDriver is a mock web push notification driver.
type WebDriver struct{}

func NewWebDriver() *WebDriver {
	return &WebDriver{}
}

func (d *WebDriver) Channel() Channel {
	return Web
}

func (d *WebDriver) Send(ctx context.Context, recipient, title, content string) error {
	// In production, this would use Firebase Cloud Messaging, OneSignal, etc.
	log.Printf("[WEB PUSH DRIVER] Sending push notification to user %s", recipient)
	log.Printf("[WEB PUSH DRIVER] Title: %s", title)
	log.Printf("[WEB PUSH DRIVER] Body: %s", content)
	log.Printf("[WEB PUSH DRIVER] Push notification sent successfully")
	return nil
}

// DriverRegistry holds all available notification drivers.
type DriverRegistry struct {
	drivers map[Channel]Driver
}

func NewDriverRegistry() *DriverRegistry {
	return &DriverRegistry{
		drivers: make(map[Channel]Driver),
	}
}

func (r *DriverRegistry) Register(driver Driver) {
	r.drivers[driver.Channel()] = driver
}

func (r *DriverRegistry) Get(channel Channel) (Driver, error) {
	driver, ok := r.drivers[channel]
	if !ok {
		return nil, fmt.Errorf("no driver registered for channel: %s", channel)
	}
	return driver, nil
}
