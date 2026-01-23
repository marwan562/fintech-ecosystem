package notification

import (
	"context"
	"log"
)

// Service handles the business logic for sending notifications.
type Service struct {
	repo     *Repository
	registry *DriverRegistry
}

func NewService(repo *Repository, registry *DriverRegistry) *Service {
	return &Service{
		repo:     repo,
		registry: registry,
	}
}

// Send processes a notification request: renders the template, persists, and sends via the appropriate driver.
func (s *Service) Send(ctx context.Context, req *NotificationRequest) (*Notification, error) {
	// Render the template
	content, err := RenderTemplate(req.TemplateID, req.Data)
	if err != nil {
		log.Printf("Failed to render template %s: %v", req.TemplateID, err)
		content = "Notification content unavailable"
	}

	// Determine title from template data or use a default
	title := req.Data["Title"]
	if title == "" {
		title = "Notification"
	}

	// Create notification record
	notif := &Notification{
		UserID:    req.UserID,
		Recipient: req.Recipient,
		Channel:   req.Channel,
		Title:     title,
		Content:   content,
	}

	// Persist to database
	if s.repo != nil {
		if err := s.repo.Create(ctx, notif); err != nil {
			log.Printf("Failed to persist notification: %v", err)
			// Continue to send even if persistence fails
		}
	}

	// Get the appropriate driver
	driver, err := s.registry.Get(req.Channel)
	if err != nil {
		log.Printf("No driver for channel %s: %v", req.Channel, err)
		if s.repo != nil {
			s.repo.UpdateStatus(ctx, notif.ID, StatusFailed)
		}
		return notif, err
	}

	// Send the notification
	if err := driver.Send(ctx, req.Recipient, title, content); err != nil {
		log.Printf("Failed to send notification via %s: %v", req.Channel, err)
		if s.repo != nil {
			s.repo.UpdateStatus(ctx, notif.ID, StatusFailed)
		}
		notif.Status = StatusFailed
		return notif, err
	}

	// Update status to sent
	if s.repo != nil {
		s.repo.UpdateStatus(ctx, notif.ID, StatusSent)
	}
	notif.Status = StatusSent

	log.Printf("Notification %s sent successfully via %s to %s", notif.ID, req.Channel, req.Recipient)
	return notif, nil
}

// SendSimple is a convenience method for sending a notification without templates.
func (s *Service) SendSimple(ctx context.Context, userID, recipient string, channel Channel, title, content string) (*Notification, error) {
	notif := &Notification{
		UserID:    userID,
		Recipient: recipient,
		Channel:   channel,
		Title:     title,
		Content:   content,
	}

	// Persist to database
	if s.repo != nil {
		if err := s.repo.Create(ctx, notif); err != nil {
			log.Printf("Failed to persist notification: %v", err)
		}
	}

	// Get the appropriate driver
	driver, err := s.registry.Get(channel)
	if err != nil {
		log.Printf("No driver for channel %s: %v", channel, err)
		if s.repo != nil {
			s.repo.UpdateStatus(ctx, notif.ID, StatusFailed)
		}
		return notif, err
	}

	// Send the notification
	if err := driver.Send(ctx, recipient, title, content); err != nil {
		log.Printf("Failed to send notification: %v", err)
		if s.repo != nil {
			s.repo.UpdateStatus(ctx, notif.ID, StatusFailed)
		}
		notif.Status = StatusFailed
		return notif, err
	}

	// Update status to sent
	if s.repo != nil {
		s.repo.UpdateStatus(ctx, notif.ID, StatusSent)
	}
	notif.Status = StatusSent

	log.Printf("Notification %s sent successfully via %s to %s", notif.ID, channel, recipient)
	return notif, nil
}
