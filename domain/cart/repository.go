package cart

import (
	"context"

	"github.com/beeleelee/mall/domain/kernel"
)

type CartRepository interface {
	Save(ctx context.Context, cart *Cart) error
	FindByID(ctx context.Context, id kernel.ID) (*Cart, error)
	FindByUserID(ctx context.Context, userID kernel.ID) (*Cart, error)
	Delete(ctx context.Context, id kernel.ID) error
}

type CartEventPublisher interface {
	PublishCartUpdated(ctx context.Context, cart *Cart) error
}
