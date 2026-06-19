package checkout

import (
	"time"

	"github.com/beeleelee/mall/domain/kernel"
)

type CheckoutCreatedEvent struct {
	CheckoutID kernel.ID
	UserID     kernel.ID
}

func (e CheckoutCreatedEvent) EventName() string      { return "checkout.created" }
func (e CheckoutCreatedEvent) OccurredAt() time.Time  { return time.Now() }
func (e CheckoutCreatedEvent) AggregateID() kernel.ID { return e.CheckoutID }

type CheckoutUpdatedEvent struct {
	CheckoutID kernel.ID
	UserID     kernel.ID
}

func (e CheckoutUpdatedEvent) EventName() string      { return "checkout.updated" }
func (e CheckoutUpdatedEvent) OccurredAt() time.Time  { return time.Now() }
func (e CheckoutUpdatedEvent) AggregateID() kernel.ID { return e.CheckoutID }

type CheckoutReadyForCompleteEvent struct {
	CheckoutID kernel.ID
	UserID     kernel.ID
}

func (e CheckoutReadyForCompleteEvent) EventName() string      { return "checkout.ready_for_complete" }
func (e CheckoutReadyForCompleteEvent) OccurredAt() time.Time  { return time.Now() }
func (e CheckoutReadyForCompleteEvent) AggregateID() kernel.ID { return e.CheckoutID }

type CheckoutCompletedEvent struct {
	CheckoutID kernel.ID
	UserID     kernel.ID
}

func (e CheckoutCompletedEvent) EventName() string      { return "checkout.completed" }
func (e CheckoutCompletedEvent) OccurredAt() time.Time  { return time.Now() }
func (e CheckoutCompletedEvent) AggregateID() kernel.ID { return e.CheckoutID }

type CheckoutCancelledEvent struct {
	CheckoutID kernel.ID
	UserID     kernel.ID
}

func (e CheckoutCancelledEvent) EventName() string      { return "checkout.cancelled" }
func (e CheckoutCancelledEvent) OccurredAt() time.Time  { return time.Now() }
func (e CheckoutCancelledEvent) AggregateID() kernel.ID { return e.CheckoutID }
