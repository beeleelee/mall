package order

import (
	"time"

	"github.com/beeleelee/mall/domain/kernel"
)

type OrderConfirmedEvent struct {
	OrderID kernel.ID
	UserID  kernel.ID
}

func (e OrderConfirmedEvent) EventName() string      { return "order.confirmed" }
func (e OrderConfirmedEvent) OccurredAt() time.Time  { return time.Now() }
func (e OrderConfirmedEvent) AggregateID() kernel.ID { return e.OrderID }

type OrderProcessingEvent struct {
	OrderID kernel.ID
	UserID  kernel.ID
}

func (e OrderProcessingEvent) EventName() string      { return "order.processing" }
func (e OrderProcessingEvent) OccurredAt() time.Time  { return time.Now() }
func (e OrderProcessingEvent) AggregateID() kernel.ID { return e.OrderID }

type OrderShippedEvent struct {
	OrderID kernel.ID
	UserID  kernel.ID
}

func (e OrderShippedEvent) EventName() string      { return "order.shipped" }
func (e OrderShippedEvent) OccurredAt() time.Time  { return time.Now() }
func (e OrderShippedEvent) AggregateID() kernel.ID { return e.OrderID }

type OrderDeliveredEvent struct {
	OrderID kernel.ID
	UserID  kernel.ID
}

func (e OrderDeliveredEvent) EventName() string      { return "order.delivered" }
func (e OrderDeliveredEvent) OccurredAt() time.Time  { return time.Now() }
func (e OrderDeliveredEvent) AggregateID() kernel.ID { return e.OrderID }

type OrderReturnedEvent struct {
	OrderID kernel.ID
	UserID  kernel.ID
}

func (e OrderReturnedEvent) EventName() string      { return "order.returned" }
func (e OrderReturnedEvent) OccurredAt() time.Time  { return time.Now() }
func (e OrderReturnedEvent) AggregateID() kernel.ID { return e.OrderID }

type OrderCancelledEvent struct {
	OrderID kernel.ID
	UserID  kernel.ID
}

func (e OrderCancelledEvent) EventName() string      { return "order.cancelled" }
func (e OrderCancelledEvent) OccurredAt() time.Time  { return time.Now() }
func (e OrderCancelledEvent) AggregateID() kernel.ID { return e.OrderID }
