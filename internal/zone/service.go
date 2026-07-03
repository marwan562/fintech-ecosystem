package zone

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/sapliy/sapliy-core/internal/zone/domain"
)

type Service struct {
	repo      domain.Repository
	providers domain.TemplateProviders
	publisher domain.EventPublisher
}

func NewService(repo domain.Repository, providers domain.TemplateProviders, publisher domain.EventPublisher) *Service {
	return &Service{
		repo:      repo,
		providers: providers,
		publisher: publisher,
	}
}

func (s *Service) CreateZone(ctx context.Context, params domain.CreateZoneParams) (*domain.Zone, error) {
	if params.OrgID == "" {
		return nil, fmt.Errorf("org_id is required")
	}
	if params.Name == "" {
		return nil, fmt.Errorf("name is required")
	}
	if params.Mode == "" {
		params.Mode = domain.ModeTest
	}

	id := fmt.Sprintf("zone_%s", strings.ReplaceAll(uuid.NewString(), "-", ""))

	zone := &domain.Zone{
		ID:        id,
		OrgID:     params.OrgID,
		Name:      params.Name,
		Mode:      params.Mode,
		Metadata:  params.Metadata,
		CreatedAt: time.Now(),
		UpdatedAt: time.Now(),
	}

	if err := s.repo.Create(ctx, zone); err != nil {
		return nil, fmt.Errorf("failed to create zone: %w", err)
	}

	// Apply template if specified
	if params.TemplateName != "" {
		if t, ok := domain.GetTemplate(params.TemplateName); ok {
			if err := t.Apply(ctx, zone, s.providers); err != nil {
				// We log the error but don't fail zone creation
				fmt.Printf("Warning: Failed to apply template %s: %v\n", params.TemplateName, err)
			}
		}
	}

	// Publish event
	if s.publisher != nil {
		event := domain.ZoneCreatedEvent{
			ZoneID:    zone.ID,
			OrgID:     zone.OrgID,
			Name:      zone.Name,
			Mode:      zone.Mode,
			Timestamp: zone.CreatedAt,
			Metadata:  zone.Metadata,
		}
		if err := s.publisher.PublishZoneCreated(ctx, event); err != nil {
			fmt.Printf("Warning: Failed to publish zone.created event: %v\n", err)
		}
	}

	return zone, nil
}

func (s *Service) GetZone(ctx context.Context, id string) (*domain.Zone, error) {
	return s.repo.GetByID(ctx, id)
}

func (s *Service) ListZones(ctx context.Context, orgID string) ([]*domain.Zone, error) {
	return s.repo.ListByOrgID(ctx, orgID)
}

func (s *Service) DeleteZone(ctx context.Context, id string) error {
	return s.repo.Delete(ctx, id)
}

func (s *Service) BulkUpdateMetadata(ctx context.Context, zoneIDs []string, metadata map[string]string) (int, error) {
	count := 0
	for _, id := range zoneIDs {
		if err := s.repo.UpdateMetadata(ctx, id, metadata); err == nil {
			count++
		}
	}
	return count, nil
}
