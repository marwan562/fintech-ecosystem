package infrastructure

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	"github.com/redis/go-redis/v9"
	"github.com/sapliy/sapliy-core/internal/auth/domain"
)

type CachedRepository struct {
	domain.Repository
	rdb *redis.Client
}

func NewCachedRepository(repo domain.Repository, rdb *redis.Client) *CachedRepository {
	return &CachedRepository{
		Repository: repo,
		rdb:        rdb,
	}
}

func (r *CachedRepository) GetAPIKeyByHash(ctx context.Context, hash string) (*domain.APIKey, error) {
	cacheKey := fmt.Sprintf("auth:apikey:%s", hash)

	// Try cache
	val, err := r.rdb.Get(ctx, cacheKey).Result()
	if err == nil {
		var key domain.APIKey
		if err := json.Unmarshal([]byte(val), &key); err == nil {
			return &key, nil
		}
	}

	// Fallback to database
	key, err := r.Repository.GetAPIKeyByHash(ctx, hash)
	if err != nil {
		return nil, err
	}

	if key != nil {
		// Cache for 5 minutes
		data, _ := json.Marshal(key)
		r.rdb.Set(ctx, cacheKey, data, 5*time.Minute)
	}

	return key, nil
}

// Override other methods if needed, otherwise they delegate to the wrapped Repository
