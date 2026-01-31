package infrastructure

import (
	"context"
	"database/sql"
	"time"

	"github.com/marwan562/fintech-ecosystem/internal/billing/domain"
)

type SQLRepository struct {
	db *sql.DB
}

func NewSQLRepository(db *sql.DB) *SQLRepository {
	return &SQLRepository{db: db}
}

func (r *SQLRepository) CreateSubscription(ctx context.Context, sub *domain.Subscription) error {
	query := `
		INSERT INTO subscriptions (id, user_id, org_id, plan_id, status, current_period_start, current_period_end, created_at, updated_at)
		VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9)`
	_, err := r.db.ExecContext(ctx, query, sub.ID, sub.UserID, sub.OrgID, sub.PlanID, sub.Status, sub.CurrentPeriodStart, sub.CurrentPeriodEnd, sub.CreatedAt, sub.UpdatedAt)
	return err
}

func (r *SQLRepository) GetSubscription(ctx context.Context, id string) (*domain.Subscription, error) {
	query := `SELECT id, user_id, org_id, plan_id, status, current_period_start, current_period_end, canceled_at, created_at, updated_at FROM subscriptions WHERE id = $1`
	row := r.db.QueryRowContext(ctx, query, id)

	var sub domain.Subscription
	err := row.Scan(&sub.ID, &sub.UserID, &sub.OrgID, &sub.PlanID, &sub.Status, &sub.CurrentPeriodStart, &sub.CurrentPeriodEnd, &sub.CanceledAt, &sub.CreatedAt, &sub.UpdatedAt)
	if err == sql.ErrNoRows {
		return nil, nil
	}
	return &sub, err
}

func (r *SQLRepository) UpdateSubscription(ctx context.Context, sub *domain.Subscription) error {
	query := `UPDATE subscriptions SET status = $1, current_period_start = $2, current_period_end = $3, canceled_at = $4, updated_at = $5 WHERE id = $6`
	_, err := r.db.ExecContext(ctx, query, sub.Status, sub.CurrentPeriodStart, sub.CurrentPeriodEnd, sub.CanceledAt, time.Now(), sub.ID)
	return err
}

func (r *SQLRepository) ListDueSubscriptions(ctx context.Context) ([]*domain.Subscription, error) {
	query := `SELECT id, user_id, org_id, plan_id, status, current_period_start, current_period_end, created_at, updated_at FROM subscriptions WHERE status = 'active' AND current_period_end <= $1`
	rows, err := r.db.QueryContext(ctx, query, time.Now())
	if err != nil {
		return nil, err
	}
	defer func() { _ = rows.Close() }()

	var subs []*domain.Subscription
	for rows.Next() {
		var sub domain.Subscription
		if err := rows.Scan(&sub.ID, &sub.UserID, &sub.OrgID, &sub.PlanID, &sub.Status, &sub.CurrentPeriodStart, &sub.CurrentPeriodEnd, &sub.CreatedAt, &sub.UpdatedAt); err != nil {
			return nil, err
		}
		subs = append(subs, &sub)
	}
	return subs, nil
}

func (r *SQLRepository) GetPlan(ctx context.Context, id string) (*domain.Plan, error) {
	query := `SELECT id, name, amount, currency, interval FROM plans WHERE id = $1`
	row := r.db.QueryRowContext(ctx, query, id)

	var plan domain.Plan
	err := row.Scan(&plan.ID, &plan.Name, &plan.Amount, &plan.Currency, &plan.Interval)
	if err == sql.ErrNoRows {
		return nil, nil
	}
	return &plan, err
}
