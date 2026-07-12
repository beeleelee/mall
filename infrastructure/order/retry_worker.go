package order

import (
	"context"
	"log"
	"time"

	"github.com/beeleelee/mall/domain/kernel"
	domain "github.com/beeleelee/mall/domain/order"
)

type WebhookRetryWorker struct {
	logRepo   domain.DeliveryLogRepository
	whRepo    domain.WebhookRepository
	deliverer *WebhookDeliverer
	interval  time.Duration
	logger    kernel.Logger
}

func NewWebhookRetryWorker(logRepo domain.DeliveryLogRepository, whRepo domain.WebhookRepository, deliverer *WebhookDeliverer, interval time.Duration, logger kernel.Logger) *WebhookRetryWorker {
	if interval <= 0 {
		interval = 30 * time.Second
	}
	return &WebhookRetryWorker{
		logRepo:   logRepo,
		whRepo:    whRepo,
		deliverer: deliverer,
		interval:  interval,
		logger:    logger,
	}
}

func (w *WebhookRetryWorker) Start(ctx context.Context) {
	ticker := time.NewTicker(w.interval)
	defer ticker.Stop()

	if w.logger != nil {
		w.logger.Info(ctx, "webhook retry worker started", kernel.Field("interval", w.interval.String()))
	}

	for {
		select {
		case <-ctx.Done():
			if w.logger != nil {
				w.logger.Info(ctx, "webhook retry worker stopped")
			}
			return
		case <-ticker.C:
			w.runOnce(ctx)
		}
	}
}

func (w *WebhookRetryWorker) runOnce(ctx context.Context) {
	entries, err := w.logRepo.ListFailedDueForRetry(ctx, 50)
	if err != nil {
		log.Printf("webhook retry worker: list failed: %v", err)
		return
	}

	for _, entry := range entries {
		wh, err := w.whRepo.FindByID(ctx, kernel.ID(entry.WebhookID))
		if err != nil {
			log.Printf("webhook retry worker: find webhook %d: %v", entry.WebhookID, err)
			continue
		}
		if !wh.Active {
			if err := w.logRepo.MarkRetried(ctx, entry.ID); err != nil {
				log.Printf("webhook retry worker: mark retried %d: %v", entry.ID, err)
			}
			continue
		}

		if err := w.deliverer.Deliver(ctx, wh, entry.Event, entry.Payload); err != nil {
			log.Printf("webhook retry worker: deliver failed for log %d: %v", entry.ID, err)
		}
	}
}
