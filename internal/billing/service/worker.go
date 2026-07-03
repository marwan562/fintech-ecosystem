package service

import (
	"context"
	"log"
	"time"

	"github.com/sapliy/sapliy-core/internal/billing/domain"
)

type PaymentClient interface {
	CreatePayment(ctx context.Context, userID, orgID string, amount int64, currency string) (string, error)
}

type SubscriptionWorker struct {
	repo          domain.Repository
	paymentClient PaymentClient
	interval      time.Duration
}

func NewSubscriptionWorker(repo domain.Repository, paymentClient PaymentClient, interval time.Duration) *SubscriptionWorker {
	return &SubscriptionWorker{
		repo:          repo,
		paymentClient: paymentClient,
		interval:      interval,
	}
}

func (w *SubscriptionWorker) Start(ctx context.Context) {
	ticker := time.NewTicker(w.interval)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			w.runProcess(ctx)
		}
	}
}

func (w *SubscriptionWorker) runProcess(ctx context.Context) {
	subs, err := w.repo.ListDueSubscriptions(ctx)
	if err != nil {
		log.Printf("Worker: failed to list due subscriptions: %v", err)
		return
	}

	for _, sub := range subs {
		if err := w.processSubscription(ctx, sub); err != nil {
			log.Printf("Worker: failed to process sub %s: %v", sub.ID, err)
		}
	}
}

func (w *SubscriptionWorker) processSubscription(ctx context.Context, sub *domain.Subscription) error {
	plan, err := w.repo.GetPlan(ctx, sub.PlanID)
	if err != nil {
		return err
	}

	// Create Payment
	log.Printf("Worker: Processing renewal for sub %s, user %s", sub.ID, sub.UserID)
	paymentID, err := w.paymentClient.CreatePayment(ctx, sub.UserID, sub.OrgID, plan.Amount, plan.Currency)
	if err != nil {
		// In a real scenario, we'd mark the sub as past_due after retries
		sub.Status = domain.SubscriptionStatusPastDue
		_ = w.repo.UpdateSubscription(ctx, sub)
		return err
	}

	// Update Period
	sub.CurrentPeriodStart = sub.CurrentPeriodEnd
	sub.CurrentPeriodEnd = domain.CalculateNextPeriod(sub.CurrentPeriodStart, plan.Interval)
	sub.Status = domain.SubscriptionStatusActive
	sub.UpdatedAt = time.Now()

	log.Printf("Worker: Sub %s renewed successfully, payment ID: %s", sub.ID, paymentID)
	return w.repo.UpdateSubscription(ctx, sub)
}
