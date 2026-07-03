package infrastructure

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	"github.com/redis/go-redis/v9"
	"github.com/sapliy/sapliy-core/internal/zone/domain"
)

type CachedRepository struct {
	repo  domain.Repository
	redis *redis.Client
	ttl   time.Duration
}

func NewCachedRepository(repo domain.Repository, redis *redis.Client) *CachedRepository {
	return &CachedRepository{
		repo:  repo,
		redis: redis,
		ttl:   30 * time.Minute,
	}
}

func (r *CachedRepository) zoneKey(id string) string {
	return fmt.Sprintf("zone:id:%s", id)
}

func (r *CachedRepository) orgKey(orgID string) string {
	return fmt.Sprintf("zone:org:%s", orgID)
}

func (r *CachedRepository) Create(ctx context.Context, zone *domain.Zone) error {
	err := r.repo.Create(ctx, zone)
	if err == nil {
		r.redis.Del(ctx, r.orgKey(zone.OrgID))
	}
	return err
}

func (r *CachedRepository) GetByID(ctx context.Context, id string) (*domain.Zone, error) {
	key := r.zoneKey(id)
	val, err := r.redis.Get(ctx, key).Result()
	if err == nil {
		var zone domain.Zone
		if err := json.Unmarshal([]byte(val), &zone); err == nil {
			return &zone, nil
		}
	}

	zone, err := r.repo.GetByID(ctx, id)
	if err != nil {
		return nil, err
	}

	if zone != nil {
		data, _ := json.Marshal(zone)
		r.redis.Set(ctx, key, data, r.ttl)
	}

	return zone, nil
}

func (r *CachedRepository) ListByOrgID(ctx context.Context, orgID string) ([]*domain.Zone, error) {
	key := r.orgKey(orgID)
	val, err := r.redis.Get(ctx, key).Result()
	if err == nil {
		var zones []*domain.Zone
		if err := json.Unmarshal([]byte(val), &zones); err == nil {
			return zones, nil
		}
	}

	zones, err := r.repo.ListByOrgID(ctx, orgID)
	if err != nil {
		return nil, err
	}

	if zones != nil {
		data, _ := json.Marshal(zones)
		r.redis.Set(ctx, key, data, r.ttl)
	}

	return zones, nil
}

func (r *CachedRepository) UpdateMetadata(ctx context.Context, id string, metadata map[string]string) error {
	err := r.repo.UpdateMetadata(ctx, id, metadata)
	if err == nil {
		r.redis.Del(ctx, r.zoneKey(id))
		zone, _ := r.repo.GetByID(ctx, id)
		if zone != nil {
			r.redis.Del(ctx, r.orgKey(zone.OrgID))
		}
	}
	return err
}

func (r *CachedRepository) Delete(ctx context.Context, id string) error {
	zone, _ := r.repo.GetByID(ctx, id)
	err := r.repo.Delete(ctx, id)
	if err == nil {
		r.redis.Del(ctx, r.zoneKey(id))
		if zone != nil {
			r.redis.Del(ctx, r.orgKey(zone.OrgID))
		}
	}
	return err
}
