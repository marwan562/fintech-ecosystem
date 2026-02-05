package zone

import (
	"context"
	"fmt"
	"time"
)

// TemplateType represents the type of zone template
type TemplateType string

const (
	TemplateEcommerce    TemplateType = "e-commerce"
	TemplateSaaSBilling  TemplateType = "saas-billing"
	TemplateMarketplace  TemplateType = "marketplace"
	TemplateFintechBasic TemplateType = "fintech-basic"
	TemplateAutomation   TemplateType = "automation-hub"
)

// Template represents a zone template configuration
type Template struct {
	Type        TemplateType     `json:"type"`
	Name        string           `json:"name"`
	Description string           `json:"description"`
	Flows       []FlowTemplate   `json:"flows"`
	Webhooks    []WebhookConfig  `json:"webhooks"`
	EventTypes  []string         `json:"eventTypes"`
	Policies    []PolicyTemplate `json:"policies,omitempty"`
}

// FlowTemplate represents a pre-configured flow
type FlowTemplate struct {
	Name    string         `json:"name"`
	Trigger TriggerConfig  `json:"trigger"`
	Actions []ActionConfig `json:"actions"`
	Logic   []LogicConfig  `json:"logic,omitempty"`
}

// TriggerConfig represents a flow trigger configuration
type TriggerConfig struct {
	Type      string `json:"type"` // event, schedule, webhook
	EventType string `json:"eventType,omitempty"`
	Cron      string `json:"cron,omitempty"`
	Path      string `json:"path,omitempty"`
}

// ActionConfig represents a flow action configuration
type ActionConfig struct {
	Type     string            `json:"type"` // webhook, notification, ledger
	URL      string            `json:"url,omitempty"`
	Method   string            `json:"method,omitempty"`
	Template string            `json:"template,omitempty"`
	Channel  string            `json:"channel,omitempty"`
	Headers  map[string]string `json:"headers,omitempty"`
}

// LogicConfig represents flow logic (conditions, delays, approvals)
type LogicConfig struct {
	Type       string `json:"type"` // condition, delay, approval
	Expression string `json:"expression,omitempty"`
	Duration   string `json:"duration,omitempty"`
}

// WebhookConfig represents a webhook endpoint configuration
type WebhookConfig struct {
	Name    string            `json:"name"`
	Path    string            `json:"path"`
	Events  []string          `json:"events"`
	Secret  string            `json:"secret,omitempty"`
	Headers map[string]string `json:"headers,omitempty"`
}

// PolicyTemplate represents a policy configuration
type PolicyTemplate struct {
	Name    string   `json:"name"`
	Actions []string `json:"actions"`
	Roles   []string `json:"roles"`
}

// TemplateApplyResult contains the result of applying a template
type TemplateApplyResult struct {
	ZoneID          string    `json:"zoneId"`
	TemplateName    string    `json:"templateName"`
	FlowsCreated    int       `json:"flowsCreated"`
	WebhooksCreated int       `json:"webhooksCreated"`
	PoliciesCreated int       `json:"policiesCreated"`
	AppliedAt       time.Time `json:"appliedAt"`
}

// TemplateRegistry holds all available templates
var TemplateRegistry = map[TemplateType]Template{
	TemplateEcommerce: {
		Type:        TemplateEcommerce,
		Name:        "E-Commerce",
		Description: "Complete e-commerce solution with checkout, payments, and order tracking",
		Flows: []FlowTemplate{
			{
				Name:    "Payment Success Handler",
				Trigger: TriggerConfig{Type: "event", EventType: "payment.succeeded"},
				Actions: []ActionConfig{
					{Type: "notification", Channel: "email", Template: "payment-receipt"},
					{Type: "ledger", Template: "credit-entry"},
				},
			},
			{
				Name:    "Order Shipped Notification",
				Trigger: TriggerConfig{Type: "event", EventType: "order.shipped"},
				Actions: []ActionConfig{
					{Type: "notification", Channel: "email", Template: "shipping-confirmation"},
				},
			},
			{
				Name:    "Checkout Abandoned Recovery",
				Trigger: TriggerConfig{Type: "event", EventType: "checkout.abandoned"},
				Logic: []LogicConfig{
					{Type: "delay", Duration: "1h"},
				},
				Actions: []ActionConfig{
					{Type: "notification", Channel: "email", Template: "abandoned-cart"},
				},
			},
		},
		Webhooks: []WebhookConfig{
			{Name: "Payment Gateway", Path: "/webhooks/payment", Events: []string{"payment.*"}},
			{Name: "Shipping Provider", Path: "/webhooks/shipping", Events: []string{"shipping.*"}},
		},
		EventTypes: []string{
			"checkout.started", "checkout.completed", "checkout.abandoned",
			"payment.created", "payment.succeeded", "payment.failed",
			"order.created", "order.shipped", "order.delivered",
			"refund.requested", "refund.completed",
		},
	},
	TemplateSaaSBilling: {
		Type:        TemplateSaaSBilling,
		Name:        "SaaS Billing",
		Description: "Subscription and usage-based billing for SaaS products",
		Flows: []FlowTemplate{
			{
				Name:    "New Subscription Welcome",
				Trigger: TriggerConfig{Type: "event", EventType: "subscription.created"},
				Actions: []ActionConfig{
					{Type: "webhook", URL: "/api/provision", Method: "POST"},
					{Type: "notification", Channel: "email", Template: "welcome"},
				},
			},
			{
				Name:    "Invoice Payment Processing",
				Trigger: TriggerConfig{Type: "event", EventType: "invoice.created"},
				Actions: []ActionConfig{
					{Type: "webhook", URL: "/api/process-payment", Method: "POST"},
				},
			},
		},
		Webhooks: []WebhookConfig{
			{Name: "Billing", Path: "/webhooks/billing", Events: []string{"invoice.*", "subscription.*"}},
		},
		EventTypes: []string{
			"subscription.created", "subscription.updated", "subscription.cancelled",
			"invoice.created", "invoice.paid", "invoice.failed",
			"usage.recorded", "usage.threshold",
		},
	},
	TemplateFintechBasic: {
		Type:        TemplateFintechBasic,
		Name:        "Fintech Basic",
		Description: "Basic payment processing with fraud checks",
		Flows: []FlowTemplate{
			{
				Name:    "Fraud Check on Payment",
				Trigger: TriggerConfig{Type: "event", EventType: "payment.created"},
				Logic: []LogicConfig{
					{Type: "condition", Expression: "event.data.amount > 100000"},
				},
				Actions: []ActionConfig{
					{Type: "webhook", URL: "/api/fraud-check", Method: "POST"},
				},
			},
			{
				Name:    "High Value Payment Approval",
				Trigger: TriggerConfig{Type: "event", EventType: "payment.created"},
				Logic: []LogicConfig{
					{Type: "condition", Expression: "event.data.amount > 500000"},
					{Type: "approval", Expression: "finance@company.com"},
				},
				Actions: []ActionConfig{
					{Type: "ledger", Template: "pending-approval"},
				},
			},
		},
		Webhooks: []WebhookConfig{
			{Name: "Payments", Path: "/webhooks/payments", Events: []string{"payment.*"}},
			{Name: "Fraud", Path: "/webhooks/fraud", Events: []string{"fraud.*"}},
		},
		EventTypes: []string{
			"payment.created", "payment.succeeded", "payment.failed", "payment.disputed",
			"fraud.detected", "fraud.cleared",
		},
	},
	TemplateAutomation: {
		Type:        TemplateAutomation,
		Name:        "Automation Hub",
		Description: "Event-driven automation without payment processing",
		Flows: []FlowTemplate{
			{
				Name:    "Daily Report Generator",
				Trigger: TriggerConfig{Type: "schedule", Cron: "0 9 * * *"},
				Actions: []ActionConfig{
					{Type: "webhook", URL: "/api/generate-report", Method: "POST"},
					{Type: "notification", Channel: "email", Template: "daily-report"},
				},
			},
		},
		Webhooks: []WebhookConfig{
			{Name: "External Events", Path: "/webhooks/external", Events: []string{"*"}},
		},
		EventTypes: []string{
			"automation.triggered", "automation.completed", "automation.failed",
		},
	},
}

// TemplateService handles zone template operations
type TemplateService struct {
	zoneService *Service
}

// NewTemplateService creates a new template service
func NewTemplateService(zoneService *Service) *TemplateService {
	return &TemplateService{zoneService: zoneService}
}

// List returns all available templates
func (s *TemplateService) List() []Template {
	templates := make([]Template, 0, len(TemplateRegistry))
	for _, t := range TemplateRegistry {
		templates = append(templates, t)
	}
	return templates
}

// Get returns a specific template
func (s *TemplateService) Get(templateType TemplateType) (*Template, error) {
	t, ok := TemplateRegistry[templateType]
	if !ok {
		return nil, fmt.Errorf("template not found: %s", templateType)
	}
	return &t, nil
}

// Apply applies a template to a zone
func (s *TemplateService) Apply(ctx context.Context, zoneID string, templateType TemplateType) (*TemplateApplyResult, error) {
	template, err := s.Get(templateType)
	if err != nil {
		return nil, err
	}

	result := &TemplateApplyResult{
		ZoneID:       zoneID,
		TemplateName: template.Name,
		AppliedAt:    time.Now(),
	}

	// Create flows
	for range template.Flows {
		// TODO: Implement flow creation via flow service
		result.FlowsCreated++
	}

	// Create webhook endpoints
	for range template.Webhooks {
		// TODO: Implement webhook endpoint creation
		result.WebhooksCreated++
	}

	// Create policies
	for range template.Policies {
		// TODO: Implement policy creation
		result.PoliciesCreated++
	}

	return result, nil
}
