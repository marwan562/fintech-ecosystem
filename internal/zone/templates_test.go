package zone

import (
	"context"
	"testing"
	"time"

	"github.com/sapliy/sapliy-core/internal/zone/domain"
)

func TestTemplateService_List(t *testing.T) {
	// Setup
	mockRepo := NewMockRepo()
	providers := domain.TemplateProviders{}
	service := NewService(mockRepo, providers, &MockEventPublisher{})
	templateService := NewTemplateService(service)

	// Test
	templates := templateService.List()

	// Assert
	if len(templates) == 0 {
		t.Fatal("Templates list should not be empty")
	}
	if len(templates) < 4 {
		t.Errorf("Should have at least 4 default templates, got %d", len(templates))
	}

	// Check that all required template types are present
	templateTypes := make(map[string]bool)
	for _, template := range templates {
		templateTypes[string(template.Type)] = true
	}

	expectedTypes := []string{
		string(TemplateEcommerce),
		string(TemplateSaaSBilling),
		string(TemplateMarketplace),
		string(TemplateFintechBasic),
		string(TemplateAutomation),
	}

	for _, expectedType := range expectedTypes {
		if !templateTypes[expectedType] {
			t.Errorf("Missing template type: %s", expectedType)
		}
	}
}

func TestTemplateService_Get(t *testing.T) {
	// Setup
	mockRepo := NewMockRepo()
	providers := domain.TemplateProviders{}
	service := NewService(mockRepo, providers, &MockEventPublisher{})
	templateService := NewTemplateService(service)

	tests := []struct {
		name         string
		templateType TemplateType
		expectError  bool
		expectedName string
	}{
		{
			name:         "Valid e-commerce template",
			templateType: TemplateEcommerce,
			expectError:  false,
			expectedName: "E-Commerce",
		},
		{
			name:         "Valid SaaS billing template",
			templateType: TemplateSaaSBilling,
			expectError:  false,
			expectedName: "SaaS Billing",
		},
		{
			name:         "Invalid template type",
			templateType: TemplateType("invalid"),
			expectError:  true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			template, err := templateService.Get(tt.templateType)

			if tt.expectError {
				if err == nil {
					t.Error("Expected error but got none")
				}
				if template != nil {
					t.Error("Expected nil template on error")
				}
			} else {
				if err != nil {
					t.Errorf("Unexpected error: %v", err)
				}
				if template == nil {
					t.Error("Expected template but got nil")
				} else {
					if tt.expectedName != template.Name {
						t.Errorf("Expected name %s, got %s", tt.expectedName, template.Name)
					}
					if template.Description == "" {
						t.Error("Description should not be empty")
					}
					if len(template.Flows) == 0 {
						t.Error("Flows should not be empty")
					}
					if len(template.EventTypes) == 0 {
						t.Error("EventTypes should not be empty")
					}
				}
			}
		})
	}
}

func TestTemplateService_Apply(t *testing.T) {
	// Setup
	mockRepo := NewMockRepo()
	providers := domain.TemplateProviders{}
	service := NewService(mockRepo, providers, &MockEventPublisher{})
	templateService := NewTemplateService(service)

	zoneID := "test-zone-123"

	t.Run("Apply e-commerce template", func(t *testing.T) {
		result, err := templateService.Apply(context.Background(), zoneID, TemplateEcommerce)

		if err != nil {
			t.Errorf("Unexpected error: %v", err)
		}
		if result == nil {
			t.Fatal("Expected result but got nil")
		}
		if zoneID != result.ZoneID {
			t.Errorf("Expected zone ID %s, got %s", zoneID, result.ZoneID)
		}
		if "E-Commerce" != result.TemplateName {
			t.Errorf("Expected template name %s, got %s", "E-Commerce", result.TemplateName)
		}
		if result.FlowsCreated != 3 {
			t.Errorf("Expected 3 flows for e-commerce template, got %d", result.FlowsCreated)
		}
		if result.WebhooksCreated != 2 {
			t.Errorf("Expected 2 webhook endpoints, got %d", result.WebhooksCreated)
		}
		if time.Since(result.AppliedAt) > 5*time.Second {
			t.Error("AppliedAt time should be recent")
		}
	})

	t.Run("Apply invalid template", func(t *testing.T) {
		result, err := templateService.Apply(context.Background(), zoneID, TemplateType("invalid"))

		if err == nil {
			t.Error("Expected error but got none")
		}
		if result != nil {
			t.Error("Expected nil result on error")
		}
		if !contains(err.Error(), "template not found") {
			t.Errorf("Expected 'template not found' in error, got: %v", err)
		}
	})
}

func TestTemplateEcommerce_Flows(t *testing.T) {
	template := TemplateRegistry[TemplateEcommerce]

	t.Run("Payment Success Handler flow", func(t *testing.T) {
		if len(template.Flows) == 0 {
			t.Fatal("Template should have flows")
		}
		paymentFlow := template.Flows[0]
		if "Payment Success Handler" != paymentFlow.Name {
			t.Errorf("Expected flow name 'Payment Success Handler', got %s", paymentFlow.Name)
		}
		if "event" != paymentFlow.Trigger.Type {
			t.Errorf("Expected trigger type 'event', got %s", paymentFlow.Trigger.Type)
		}
		if "payment.succeeded" != paymentFlow.Trigger.EventType {
			t.Errorf("Expected event type 'payment.succeeded', got %s", paymentFlow.Trigger.EventType)
		}
		if len(paymentFlow.Actions) != 2 {
			t.Errorf("Expected 2 actions, got %d", len(paymentFlow.Actions))
		} else {
			if "notification" != paymentFlow.Actions[0].Type {
				t.Errorf("Expected first action type 'notification', got %s", paymentFlow.Actions[0].Type)
			}
			if "email" != paymentFlow.Actions[0].Channel {
				t.Errorf("Expected email channel, got %s", paymentFlow.Actions[0].Channel)
			}
			if "ledger" != paymentFlow.Actions[1].Type {
				t.Errorf("Expected second action type 'ledger', got %s", paymentFlow.Actions[1].Type)
			}
		}
	})

	t.Run("Order Shipped Notification flow", func(t *testing.T) {
		if len(template.Flows) < 2 {
			t.Fatal("Template should have at least 2 flows")
		}
		shippingFlow := template.Flows[1]
		if "Order Shipped Notification" != shippingFlow.Name {
			t.Errorf("Expected flow name 'Order Shipped Notification', got %s", shippingFlow.Name)
		}
		if "event" != shippingFlow.Trigger.Type {
			t.Errorf("Expected trigger type 'event', got %s", shippingFlow.Trigger.Type)
		}
		if "order.shipped" != shippingFlow.Trigger.EventType {
			t.Errorf("Expected event type 'order.shipped', got %s", shippingFlow.Trigger.EventType)
		}
		if len(shippingFlow.Actions) != 1 {
			t.Errorf("Expected 1 action, got %d", len(shippingFlow.Actions))
		} else {
			if "notification" != shippingFlow.Actions[0].Type {
				t.Errorf("Expected action type 'notification', got %s", shippingFlow.Actions[0].Type)
			}
		}
	})

	t.Run("Checkout Abandoned Recovery flow", func(t *testing.T) {
		if len(template.Flows) < 3 {
			t.Fatal("Template should have at least 3 flows")
		}
		abandonedFlow := template.Flows[2]
		if "Checkout Abandoned Recovery" != abandonedFlow.Name {
			t.Errorf("Expected flow name 'Checkout Abandoned Recovery', got %s", abandonedFlow.Name)
		}
		if "event" != abandonedFlow.Trigger.Type {
			t.Errorf("Expected trigger type 'event', got %s", abandonedFlow.Trigger.Type)
		}
		if "checkout.abandoned" != abandonedFlow.Trigger.EventType {
			t.Errorf("Expected event type 'checkout.abandoned', got %s", abandonedFlow.Trigger.EventType)
		}
		if len(abandonedFlow.Logic) != 1 {
			t.Errorf("Expected 1 logic node, got %d", len(abandonedFlow.Logic))
		} else {
			if "delay" != abandonedFlow.Logic[0].Type {
				t.Errorf("Expected logic type 'delay', got %s", abandonedFlow.Logic[0].Type)
			}
			if "1h" != abandonedFlow.Logic[0].Duration {
				t.Errorf("Expected duration '1h', got %s", abandonedFlow.Logic[0].Duration)
			}
		}
	})
}

func TestTemplateWebhookConfigurations(t *testing.T) {
	tests := []struct {
		name             string
		templateType     TemplateType
		expectedWebhooks int
	}{
		{
			name:             "E-commerce template webhooks",
			templateType:     TemplateEcommerce,
			expectedWebhooks: 2,
		},
		{
			name:             "SaaS billing template webhooks",
			templateType:     TemplateSaaSBilling,
			expectedWebhooks: 1,
		},
		{
			name:             "Fintech basic template webhooks",
			templateType:     TemplateFintechBasic,
			expectedWebhooks: 2,
		},
		{
			name:             "Automation hub template webhooks",
			templateType:     TemplateAutomation,
			expectedWebhooks: 1,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			template := TemplateRegistry[tt.templateType]
			if len(template.Webhooks) != tt.expectedWebhooks {
				t.Errorf("Expected %d webhooks, got %d", tt.expectedWebhooks, len(template.Webhooks))
			}

			for _, webhook := range template.Webhooks {
				if webhook.Name == "" {
					t.Error("Webhook name should not be empty")
				}
				if webhook.Path == "" {
					t.Error("Webhook path should not be empty")
				}
				if len(webhook.Events) == 0 {
					t.Error("Webhook events should not be empty")
				}
			}
		})
	}
}

// Helper function to check if a string contains a substring
func contains(s, substr string) bool {
	return len(s) >= len(substr) && (s == substr || len(substr) == 0 ||
		(len(s) > len(substr) &&
			(s[:len(substr)] == substr ||
				s[len(s)-len(substr):] == substr ||
				indexOf(s, substr) >= 0)))
}

func indexOf(s, substr string) int {
	for i := 0; i <= len(s)-len(substr); i++ {
		if s[i:i+len(substr)] == substr {
			return i
		}
	}
	return -1
}

// MockRepo is a simple mock repository for testing
type MockRepo struct {
	zones map[string]*domain.Zone
}

func NewMockRepo() *MockRepo {
	return &MockRepo{
		zones: make(map[string]*domain.Zone),
	}
}

func (m *MockRepo) Create(ctx context.Context, zone *domain.Zone) error {
	m.zones[zone.ID] = zone
	return nil
}

func (m *MockRepo) GetByID(ctx context.Context, id string) (*domain.Zone, error) {
	if zone, exists := m.zones[id]; exists {
		return zone, nil
	}
	return nil, domain.ErrZoneNotFound
}

func (m *MockRepo) ListByOrgID(ctx context.Context, orgID string) ([]*domain.Zone, error) {
	var zones []*domain.Zone
	for _, zone := range m.zones {
		if zone.OrgID == orgID {
			zones = append(zones, zone)
		}
	}
	return zones, nil
}

func (m *MockRepo) UpdateMetadata(ctx context.Context, id string, metadata map[string]string) error {
	if zone, exists := m.zones[id]; exists {
		zone.Metadata = metadata
		zone.UpdatedAt = time.Now()
		return nil
	}
	return domain.ErrZoneNotFound
}
// MockEventPublisher implements domain.EventPublisher for testing
type MockEventPublisher struct{}

func (m *MockEventPublisher) PublishZoneCreated(ctx context.Context, event domain.ZoneCreatedEvent) error {
	return nil
}

func (m *MockRepo) Delete(ctx context.Context, id string) error {
	if _, exists := m.zones[id]; exists {
		delete(m.zones, id)
		return nil
	}
	return domain.ErrZoneNotFound
}
