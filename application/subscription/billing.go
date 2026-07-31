package subscription

import (
	"context"
	"time"

	"github.com/beeleelee/mall/domain/kernel"
	domain "github.com/beeleelee/mall/domain/subscription"
)

type SubscriptionBillingService struct {
	svc    *domain.SubscriptionService
	logger kernel.Logger
}

func NewSubscriptionBillingService(svc *domain.SubscriptionService, logger kernel.Logger) *SubscriptionBillingService {
	return &SubscriptionBillingService{svc: svc, logger: logger}
}

func (b *SubscriptionBillingService) ProcessDueBilling(ctx context.Context) error {
	now := time.Now()

	due, err := b.svc.ListDueForBilling(ctx, now)
	if err != nil {
		return err
	}
	for _, sub := range due {
		renewed, err := b.svc.HandleBillingCycle(ctx, sub.ID)
		if err != nil {
			b.logger.Error(ctx, "subscription.billing.failed", err, kernel.Field("subscription_id", sub.ID.String()))
			continue
		}
		b.logger.Info(ctx, "subscription.billing.processed",
			kernel.Field("subscription_id", renewed.ID.String()),
			kernel.Field("status", string(renewed.Status)),
		)
	}

	expiredTrials, err := b.svc.ExpireTrials(ctx, now)
	if err != nil {
		return err
	}
	for _, sub := range expiredTrials {
		b.logger.Info(ctx, "subscription.trial_expired",
			kernel.Field("subscription_id", sub.ID.String()),
		)
	}
	return nil
}
