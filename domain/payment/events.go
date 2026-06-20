package payment

import (
	"time"

	"github.com/beeleelee/mall/domain/kernel"
)

type MandateRequestedEvent struct {
	MandateID  kernel.ID
	UserID     kernel.ID
	MaxAmount  int64
	MerchantID kernel.ID
}

func (e MandateRequestedEvent) EventName() string      { return "payment.mandate.requested" }
func (e MandateRequestedEvent) OccurredAt() time.Time  { return time.Now() }
func (e MandateRequestedEvent) AggregateID() kernel.ID { return e.MandateID }

type MandateApprovedEvent struct {
	MandateID kernel.ID
	UserID    kernel.ID
}

func (e MandateApprovedEvent) EventName() string      { return "payment.mandate.approved" }
func (e MandateApprovedEvent) OccurredAt() time.Time  { return time.Now() }
func (e MandateApprovedEvent) AggregateID() kernel.ID { return e.MandateID }

type MandateExecutedEvent struct {
	MandateID kernel.ID
	UserID    kernel.ID
	Token     string
}

func (e MandateExecutedEvent) EventName() string      { return "payment.mandate.executed" }
func (e MandateExecutedEvent) OccurredAt() time.Time  { return time.Now() }
func (e MandateExecutedEvent) AggregateID() kernel.ID { return e.MandateID }

type MandateSettledEvent struct {
	MandateID kernel.ID
	UserID    kernel.ID
}

func (e MandateSettledEvent) EventName() string      { return "payment.mandate.settled" }
func (e MandateSettledEvent) OccurredAt() time.Time  { return time.Now() }
func (e MandateSettledEvent) AggregateID() kernel.ID { return e.MandateID }

type MandateCancelledEvent struct {
	MandateID kernel.ID
	UserID    kernel.ID
}

func (e MandateCancelledEvent) EventName() string      { return "payment.mandate.cancelled" }
func (e MandateCancelledEvent) OccurredAt() time.Time  { return time.Now() }
func (e MandateCancelledEvent) AggregateID() kernel.ID { return e.MandateID }
