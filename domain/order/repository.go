package order

import (
	"context"

	"github.com/beeleelee/mall/domain/kernel"
)

type OrderRepository interface {
	Save(ctx context.Context, order *Order) error
	FindByID(ctx context.Context, id kernel.ID) (*Order, error)
	FindByUserID(ctx context.Context, userID kernel.ID) ([]*Order, error)
	Delete(ctx context.Context, id kernel.ID) error
}

type OrderEventPublisher interface {
	PublishOrderEvent(ctx context.Context, order *Order) error
}
