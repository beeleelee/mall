package wishlist

import (
	"context"

	"github.com/beeleelee/mall/domain/kernel"
)

type WishlistRepository interface {
	Save(ctx context.Context, wishlist *Wishlist) error
	FindByUserID(ctx context.Context, userID kernel.ID) (*Wishlist, error)
	Delete(ctx context.Context, id kernel.ID) error
}
