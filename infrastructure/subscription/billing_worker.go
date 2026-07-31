package subscription

import (
	"context"
	"time"

	"github.com/beeleelee/mall/domain/kernel"
)

type BillingProcessor interface {
	ProcessDueBilling(ctx context.Context) error
}

type SubscriptionBillingWorker struct {
	processor BillingProcessor
	interval  time.Duration
	logger    kernel.Logger
}

func NewSubscriptionBillingWorker(processor BillingProcessor, interval time.Duration, logger kernel.Logger) *SubscriptionBillingWorker {
	if interval <= 0 {
		interval = 60 * time.Second
	}
	return &SubscriptionBillingWorker{
		processor: processor,
		interval:  interval,
		logger:    logger,
	}
}

func (w *SubscriptionBillingWorker) Start(ctx context.Context) {
	ticker := time.NewTicker(w.interval)
	defer ticker.Stop()

	if w.logger != nil {
		w.logger.Info(ctx, "subscription billing worker started", kernel.Field("interval", w.interval.String()))
	}

	for {
		select {
		case <-ctx.Done():
			if w.logger != nil {
				w.logger.Info(ctx, "subscription billing worker stopped")
			}
			return
		case <-ticker.C:
			if err := w.processor.ProcessDueBilling(ctx); err != nil {
				if w.logger != nil {
					w.logger.Error(ctx, "subscription billing worker: run failed", err)
				}
			}
		}
	}
}
