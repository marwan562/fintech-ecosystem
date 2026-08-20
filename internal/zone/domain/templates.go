package domain

import (
	"context"
)

// Template represents a pre-configured setup for a new Zone.
type Template struct {
	Name        string
	Description string
	Apply       func(ctx context.Context, z *Zone, providers TemplateProviders) error
}

// TemplateProviders defines the services needed to apply a template.
// This avoids circular dependencies by using specific interfaces.
type TemplateProviders struct {
	CreateLedgerAccount func(ctx context.Context, name, accType, currency string, zoneID, mode string) error
	CreateFlow          func(ctx context.Context, zoneID string, name string, nodes interface{}, edges interface{}) error
}

var registry = make(map[string]Template)

// RegisterTemplate adds a new template to the registry.
func RegisterTemplate(t Template) {
	registry[t.Name] = t
}

// GetTemplate retrieves a template by name.
func GetTemplate(name string) (Template, bool) {
	t, ok := registry[name]
	return t, ok
}

// ListTemplates returns all registered templates.
func ListTemplates() []Template {
	var list []Template
	for _, t := range registry {
		list = append(list, t)
	}
	return list
}

func init() {
	RegisterTemplate(Template{
		Name:        "default",
		Description: "Standard sapliy setup with Revenue and Platform accounts.",
		Apply: func(ctx context.Context, z *Zone, p TemplateProviders) error {
			// 1. Create Default Ledger Accounts
			err := p.CreateLedgerAccount(ctx, "Revenue", "revenue", "USD", z.ID, string(z.Mode))
			if err != nil {
				return err
			}
			err = p.CreateLedgerAccount(ctx, "Platform", "asset", "USD", z.ID, string(z.Mode))
			if err != nil {
				return err
			}

			// 2. Create an Onboarding Flow (Placeholder logic)
			return p.CreateFlow(ctx, z.ID, "Welcome Flow", nil, nil)
		},
	})

	RegisterTemplate(Template{
		Name:        "marketplace",
		Description: "Setup for marketplaces with Escrow and Fee accounts.",
		Apply: func(ctx context.Context, z *Zone, p TemplateProviders) error {
			_ = p.CreateLedgerAccount(ctx, "Escrow", "liability", "USD", z.ID, string(z.Mode))
			_ = p.CreateLedgerAccount(ctx, "Fees", "revenue", "USD", z.ID, string(z.Mode))
			return nil
		},
	})
}
