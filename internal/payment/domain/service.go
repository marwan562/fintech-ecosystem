package domain

import (
	"context"

	"github.com/sapliy/sapliy-core/pkg/validation"
)

type PaymentService struct {
	repo Repository
}

func NewPaymentService(repo Repository) *PaymentService {
	return &PaymentService{repo: repo}
}

func (s *PaymentService) CreatePaymentIntent(ctx context.Context, intent *PaymentIntent) error {
	if err := validation.Validate(
		validation.PositiveAmount(intent.Amount, "amount"),
		validation.NotEmpty(intent.Currency, "currency"),
		validation.NotEmpty(intent.ZoneID, "zone_id"),
	); err != nil {
		return err
	}
	return s.repo.CreatePaymentIntent(ctx, intent)
}

func (s *PaymentService) GetPaymentIntent(ctx context.Context, id string) (*PaymentIntent, error) {
	return s.repo.GetPaymentIntent(ctx, id)
}

func (s *PaymentService) UpdateStatus(ctx context.Context, id, status string) error {
	if err := validation.Validate(
		validation.NotEmpty(id, "id"),
		validation.InList(status, []string{"CREATED", "PROCESSING", "SUCCEEDED", "FAILED", "CANCELLED"}, "status"),
	); err != nil {
		return err
	}
	return s.repo.UpdateStatus(ctx, id, status)
}

func (s *PaymentService) GetIdempotencyKey(ctx context.Context, userID, key string) (*IdempotencyRecord, error) {
	return s.repo.GetIdempotencyKey(ctx, userID, key)
}

func (s *PaymentService) SaveIdempotencyKey(ctx context.Context, userID, key string, statusCode int, body string) error {
	return s.repo.SaveIdempotencyKey(ctx, userID, key, statusCode, body)
}

func (s *PaymentService) ListPaymentIntents(ctx context.Context, zoneID string, limit int) ([]PaymentIntent, error) {
	if limit <= 0 {
		limit = 50
	}
	return s.repo.ListPaymentIntents(ctx, zoneID, limit)
}
