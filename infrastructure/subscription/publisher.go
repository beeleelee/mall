package subscription

import (
	"context"
	"encoding/json"

	"github.com/nats-io/nats.go"
	"github.com/nats-io/nats.go/jetstream"

	"github.com/beeleelee/mall/domain/kernel"
	domain "github.com/beeleelee/mall/domain/subscription"
	"github.com/beeleelee/mall/infrastructure/tracing"
)

type NATSSubscriptionEventPublisher struct {
	js jetstream.JetStream
}

func NewNATSSubscriptionEventPublisher(js jetstream.JetStream) *NATSSubscriptionEventPublisher {
	return &NATSSubscriptionEventPublisher{js: js}
}

func (p *NATSSubscriptionEventPublisher) PublishSubscriptionEvent(ctx context.Context, sub *domain.Subscription) error {
	events := sub.Events()
	if len(events) == 0 {
		return nil
	}

	payload := map[string]any{
		"subscription_id":     sub.ID.Int64(),
		"user_id":             sub.UserID.Int64(),
		"plan_id":             sub.PlanID.Int64(),
		"status":              string(sub.Status),
		"current_period_start": sub.CurrentPeriodStart,
		"current_period_end":   sub.CurrentPeriodEnd,
	}

	if sub.TrialEndsAt != nil {
		payload["trial_ends_at"] = sub.TrialEndsAt
	}
	if sub.CancelledAt != nil {
		payload["cancelled_at"] = sub.CancelledAt
	}

	data, err := json.Marshal(payload)
	if err != nil {
		return kernel.NewDomainErrorWithCause(kernel.ErrInternal, "marshal subscription event", err)
	}

	subject := "subscription." + string(sub.Status)
	msg := &nats.Msg{
		Subject: subject,
		Data:    data,
		Header:  nats.Header{},
	}
	tracing.InjectTrace(ctx, msg)

	if _, err := p.js.PublishMsg(ctx, msg); err != nil {
		return kernel.NewDomainErrorWithCause(kernel.ErrInternal, "publish subscription event", err)
	}

	return nil
}
