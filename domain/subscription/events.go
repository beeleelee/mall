package subscription

import (
	"time"

	"github.com/beeleelee/mall/domain/kernel"
)

type SubscriptionCreatedEvent struct {
	SubscriptionID kernel.ID
	UserID         kernel.ID
	PlanID         kernel.ID
}

func (e SubscriptionCreatedEvent) EventName() string      { return "subscription.created" }
func (e SubscriptionCreatedEvent) OccurredAt() time.Time  { return time.Now() }
func (e SubscriptionCreatedEvent) AggregateID() kernel.ID { return e.SubscriptionID }

type SubscriptionActivatedEvent struct {
	SubscriptionID kernel.ID
	UserID         kernel.ID
	PlanID         kernel.ID
	PreviousStatus SubscriptionStatus
}

func (e SubscriptionActivatedEvent) EventName() string      { return "subscription.activated" }
func (e SubscriptionActivatedEvent) OccurredAt() time.Time  { return time.Now() }
func (e SubscriptionActivatedEvent) AggregateID() kernel.ID { return e.SubscriptionID }

type SubscriptionCancelledEvent struct {
	SubscriptionID kernel.ID
	UserID         kernel.ID
	CancelledAt    time.Time
}

func (e SubscriptionCancelledEvent) EventName() string      { return "subscription.cancelled" }
func (e SubscriptionCancelledEvent) OccurredAt() time.Time  { return time.Now() }
func (e SubscriptionCancelledEvent) AggregateID() kernel.ID { return e.SubscriptionID }

type SubscriptionExpiredEvent struct {
	SubscriptionID kernel.ID
	UserID         kernel.ID
}

func (e SubscriptionExpiredEvent) EventName() string      { return "subscription.expired" }
func (e SubscriptionExpiredEvent) OccurredAt() time.Time  { return time.Now() }
func (e SubscriptionExpiredEvent) AggregateID() kernel.ID { return e.SubscriptionID }

type SubscriptionPaymentFailedEvent struct {
	SubscriptionID kernel.ID
	UserID         kernel.ID
}

func (e SubscriptionPaymentFailedEvent) EventName() string      { return "subscription.payment_failed" }
func (e SubscriptionPaymentFailedEvent) OccurredAt() time.Time  { return time.Now() }
func (e SubscriptionPaymentFailedEvent) AggregateID() kernel.ID { return e.SubscriptionID }

type SubscriptionPlanChangedEvent struct {
	SubscriptionID kernel.ID
	UserID         kernel.ID
	OldPlanID      kernel.ID
	NewPlanID      kernel.ID
}

func (e SubscriptionPlanChangedEvent) EventName() string      { return "subscription.plan_changed" }
func (e SubscriptionPlanChangedEvent) OccurredAt() time.Time  { return time.Now() }
func (e SubscriptionPlanChangedEvent) AggregateID() kernel.ID { return e.SubscriptionID }
