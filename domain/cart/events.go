package cart

import (
	"time"

	"github.com/beeleelee/mall/domain/kernel"
)

type CartCreatedEvent struct {
	CartID kernel.ID
	UserID kernel.ID
}

func (e CartCreatedEvent) EventName() string      { return "cart.created" }
func (e CartCreatedEvent) OccurredAt() time.Time  { return time.Now() }
func (e CartCreatedEvent) AggregateID() kernel.ID { return e.CartID }

type CartUpdatedEvent struct {
	CartID kernel.ID
	UserID kernel.ID
}

func (e CartUpdatedEvent) EventName() string      { return "cart.updated" }
func (e CartUpdatedEvent) OccurredAt() time.Time  { return time.Now() }
func (e CartUpdatedEvent) AggregateID() kernel.ID { return e.CartID }

type CartClearedEvent struct {
	CartID kernel.ID
	UserID kernel.ID
}

func (e CartClearedEvent) EventName() string      { return "cart.cleared" }
func (e CartClearedEvent) OccurredAt() time.Time  { return time.Now() }
func (e CartClearedEvent) AggregateID() kernel.ID { return e.CartID }

type CartMergedEvent struct {
	CartID kernel.ID
	UserID kernel.ID
}

func (e CartMergedEvent) EventName() string      { return "cart.merged" }
func (e CartMergedEvent) OccurredAt() time.Time  { return time.Now() }
func (e CartMergedEvent) AggregateID() kernel.ID { return e.CartID }
