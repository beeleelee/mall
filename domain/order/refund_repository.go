package order

import (
	"context"

	"github.com/beeleelee/mall/domain/kernel"
)

type RefundRepository interface {
	Save(ctx context.Context, refund *Refund) error
	FindByID(ctx context.Context, id kernel.ID) (*Refund, error)
	FindByOrderID(ctx context.Context, orderID kernel.ID) ([]*Refund, error)
}
