package inventory

import (
	"context"

	"github.com/beeleelee/mall/domain/kernel"
)

type InventoryRepository interface {
	Save(ctx context.Context, item *InventoryItem) error
	FindByProductID(ctx context.Context, productID kernel.ID) (*InventoryItem, error)
	FindAll(ctx context.Context, offset, limit int) ([]*InventoryItem, error)
	FindLowStock(ctx context.Context, threshold int) ([]*InventoryItem, error)
	Delete(ctx context.Context, id kernel.ID) error
}
