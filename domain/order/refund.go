package order

import (
	"time"

	"github.com/beeleelee/mall/domain/kernel"
)

type RefundStatus string

const (
	RefundStatusPending   RefundStatus = "pending"
	RefundStatusProcessed RefundStatus = "processed"
	RefundStatusFailed    RefundStatus = "failed"
)

type Refund struct {
	kernel.AggregateRoot
	OrderID       kernel.ID
	MandateID     kernel.ID
	Amount        int64
	Reason        string
	Status        RefundStatus
	CreatedAt     time.Time
	ProcessedAt   *time.Time
	FailedAt      *time.Time
	FailureReason string
}

func NewRefund(id kernel.ID, orderID kernel.ID, mandateID kernel.ID, amount int64, reason string) (*Refund, error) {
	if orderID <= 0 {
		return nil, kernel.NewDomainError(kernel.ErrInvalidArgument, "order id must be positive")
	}
	if amount <= 0 {
		return nil, kernel.NewDomainError(kernel.ErrInvalidArgument, "refund amount must be positive")
	}

	r := &Refund{
		AggregateRoot: kernel.NewAggregateRoot(id),
		OrderID:       orderID,
		MandateID:     mandateID,
		Amount:        amount,
		Reason:        reason,
		Status:        RefundStatusPending,
		CreatedAt:     time.Now(),
	}

	r.AddEvent(RefundInitiatedEvent{
		RefundID: id,
		OrderID:  orderID,
		Amount:   amount,
	})

	return r, nil
}

func (r *Refund) MarkProcessed() error {
	if r.Status != RefundStatusPending {
		return kernel.NewDomainError(kernel.ErrInvalidArgument, "can only process pending refunds")
	}
	now := time.Now()
	r.Status = RefundStatusProcessed
	r.ProcessedAt = &now

	r.AddEvent(RefundProcessedEvent{
		RefundID: r.ID,
		OrderID:  r.OrderID,
		Amount:   r.Amount,
	})

	return nil
}

func (r *Refund) MarkFailed(reason string) error {
	if r.Status != RefundStatusPending {
		return kernel.NewDomainError(kernel.ErrInvalidArgument, "can only fail pending refunds")
	}
	if reason == "" {
		return kernel.NewDomainError(kernel.ErrInvalidArgument, "failure reason must not be empty")
	}
	now := time.Now()
	r.Status = RefundStatusFailed
	r.FailedAt = &now
	r.FailureReason = reason

	r.AddEvent(RefundFailedEvent{
		RefundID: r.ID,
		OrderID:  r.OrderID,
		Reason:   reason,
	})

	return nil
}
