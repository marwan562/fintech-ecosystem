package main

import (
	"context"
	"fmt"
	"log"
	"time"

	"github.com/sapliy/sapliy-core/internal/zone"
	"github.com/sapliy/sapliy-core/internal/zone/domain"
)

func main() {
	fmt.Println("=== Testing Zone Templates ===")

	// Create a mock repository for testing
	mockRepo := NewMockRepo()

	// Create template providers (mock)
	providers := domain.TemplateProviders{
		CreateLedgerAccount: func(ctx context.Context, name, accType, currency string, zoneID, mode string) error {
			fmt.Printf("✅ Created ledger account: %s (%s) for zone %s\n", name, accType, zoneID)
			return nil
		},
		CreateFlow: func(ctx context.Context, zoneID string, name string, nodes interface{}, edges interface{}) error {
			fmt.Printf("✅ Created flow: %s for zone %s\n", name, zoneID)
			return nil
		},
	}

	// Create zone service
	zoneService := zone.NewService(mockRepo, providers, &MockEventPublisher{})

	// Create template service
	templateService := zone.NewTemplateService(zoneService)

	fmt.Println("\n1. Listing all templates:")
	templates := templateService.List()
	for _, template := range templates {
		fmt.Printf("   - %s: %s\n", template.Type, template.Name)
	}

	fmt.Println("\n2. Getting e-commerce template:")
	ecommerceTemplate, err := templateService.Get(zone.TemplateEcommerce)
	if err != nil {
		log.Fatalf("❌ Failed to get e-commerce template: %v", err)
	}
	fmt.Printf("   ✅ Found: %s\n", ecommerceTemplate.Name)
	fmt.Printf("   📝 Description: %s\n", ecommerceTemplate.Description)
	fmt.Printf("   🔄 Flows: %d\n", len(ecommerceTemplate.Flows))
	fmt.Printf("   🪝 Webhooks: %d\n", len(ecommerceTemplate.Webhooks))
	fmt.Printf("   📋 Event types: %d\n", len(ecommerceTemplate.EventTypes))

	fmt.Println("\n3. Creating a test zone:")
	testZone, err := zoneService.CreateZone(context.Background(), domain.CreateZoneParams{
		OrgID:        "test-org-123",
		Name:         "Test E-commerce Zone",
		Mode:         domain.ModeTest,
		TemplateName: "e-commerce",
		Metadata:     map[string]string{"test": "true"},
	})
	if err != nil {
		log.Fatalf("❌ Failed to create zone: %v", err)
	}
	fmt.Printf("   ✅ Zone created: %s (%s)\n", testZone.ID, testZone.Name)

	fmt.Println("\n4. Applying e-commerce template to zone:")
	result, err := templateService.Apply(context.Background(), testZone.ID, zone.TemplateEcommerce)
	if err != nil {
		log.Fatalf("❌ Failed to apply template: %v", err)
	}
	fmt.Printf("   ✅ Template applied successfully!\n")
	fmt.Printf("   📊 Flows created: %d\n", result.FlowsCreated)
	fmt.Printf("   🪝 Webhooks created: %d\n", result.WebhooksCreated)
	fmt.Printf("   📋 Policies created: %d\n", result.PoliciesCreated)
	fmt.Printf("   ⏰ Applied at: %s\n", result.AppliedAt.Format("2006-01-02 15:04:05"))

	fmt.Println("\n5. Testing marketplace template:")
	marketplaceTemplate, err := templateService.Get(zone.TemplateMarketplace)
	if err != nil {
		log.Fatalf("❌ Failed to get marketplace template: %v", err)
	}
	fmt.Printf("   ✅ Found: %s\n", marketplaceTemplate.Name)
	fmt.Printf("   📝 Description: %s\n", marketplaceTemplate.Description)
	fmt.Printf("   🔄 Flows: %d\n", len(marketplaceTemplate.Flows))

	for i, flow := range marketplaceTemplate.Flows {
		fmt.Printf("     %d. %s (%s trigger)\n", i+1, flow.Name, flow.Trigger.EventType)
	}

	fmt.Println("\n6. Testing SaaS billing template:")
	saasTemplate, err := templateService.Get(zone.TemplateSaaSBilling)
	if err != nil {
		log.Fatalf("❌ Failed to get SaaS template: %v", err)
	}
	fmt.Printf("   ✅ Found: %s\n", saasTemplate.Name)
	fmt.Printf("   📝 Description: %s\n", saasTemplate.Description)
	fmt.Printf("   🔄 Flows: %d\n", len(saasTemplate.Flows))

	fmt.Println("\n=== ✅ All template tests passed! ===")
}

// MockRepo implements a simple in-memory repository for testing
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

func (m *MockRepo) Delete(ctx context.Context, id string) error {
	if _, exists := m.zones[id]; exists {
		delete(m.zones, id)
		return nil
	}
	return domain.ErrZoneNotFound
}

// MockEventPublisher implements domain.EventPublisher for testing
type MockEventPublisher struct{}

func (m *MockEventPublisher) PublishZoneCreated(ctx context.Context, event domain.ZoneCreatedEvent) error {
	fmt.Printf("📢 [Event] Zone Created: %s (Org: %s)\n", event.ZoneID, event.OrgID)
	return nil
}
