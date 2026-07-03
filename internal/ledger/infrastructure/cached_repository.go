package infrastructure

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	"github.com/redis/go-redis/v9"
	"github.com/sapliy/sapliy-core/internal/ledger/domain"
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
		ttl:   10 * time.Minute,
	}
}

func (r *CachedRepository) accKey(id string) string {
	return fmt.Sprintf("ledger:acc:%s", id)
}

func (r *CachedRepository) CreateAccount(ctx context.Context, acc *domain.Account) error {
	err := r.repo.CreateAccount(ctx, acc)
	if err != nil {
		return err
	}
	// Invalidate cache just in case
	r.redis.Del(ctx, r.accKey(acc.ID))
	return nil
}

func (r *CachedRepository) GetAccount(ctx context.Context, id string) (*domain.Account, error) {
	key := r.accKey(id)
	val, err := r.redis.Get(ctx, key).Result()
	if err == nil {
		var acc domain.Account
		if err := json.Unmarshal([]byte(val), &acc); err == nil {
			return &acc, nil
		}
	}

	acc, err := r.repo.GetAccount(ctx, id)
	if err != nil {
		return nil, err
	}

	if acc != nil {
		data, _ := json.Marshal(acc)
		r.redis.Set(ctx, key, data, r.ttl)
	}

	return acc, nil
}

func (r *CachedRepository) BeginTx(ctx context.Context) (domain.TransactionContext, error) {
	tx, err := r.repo.BeginTx(ctx)
	if err != nil {
		return nil, err
	}
	return &cachedTransactionContext{
		TransactionContext: tx,
		redis:              r.redis,
		accKey:             r.accKey,
	}, nil
}

func (r *CachedRepository) GetUnprocessedEvents(ctx context.Context, limit int) ([]domain.OutboxEvent, error) {
	return r.repo.GetUnprocessedEvents(ctx, limit)
}

func (r *CachedRepository) MarkEventProcessed(ctx context.Context, id string) error {
	return r.repo.MarkEventProcessed(ctx, id)
}

func (r *CachedRepository) ListTransactions(ctx context.Context, zoneID string, limit int) ([]domain.TransactionWithEntries, error) {
	return r.repo.ListTransactions(ctx, zoneID, limit)
}

func (r *CachedRepository) GetTransaction(ctx context.Context, id string) (*domain.TransactionWithEntries, error) {
	return r.repo.GetTransaction(ctx, id)
}

type cachedTransactionContext struct {
	domain.TransactionContext
	redis       *redis.Client
	accKey      func(string) string
	changedAccs []string
}

func (c *cachedTransactionContext) CreateEntry(ctx context.Context, entry *domain.Entry) error {
	err := c.TransactionContext.CreateEntry(ctx, entry)
	if err == nil {
		c.changedAccs = append(c.changedAccs, entry.AccountID)
	}
	return err
}

func (c *cachedTransactionContext) Commit() error {
	err := c.TransactionContext.Commit()
	if err == nil {
		// Invalidate all changed accounts on success
		for _, id := range c.changedAccs {
			c.redis.Del(context.Background(), c.accKey(id))
		}
	}
	return err
}
