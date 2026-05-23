package catalog

import (
	"context"

	"github.com/beeleelee/mall/domain/kernel"
)

type Cursor string

type SearchResult struct {
	Products   []*Product
	NextCursor Cursor
	HasMore    bool
}

type SearchOptions struct {
	Category string
	MinPrice int64
	MaxPrice int64
	Status   ProductStatus
	Cursor   Cursor
	Limit    int
}

type ProductRepository interface {
	Save(ctx context.Context, product *Product) error
	FindByID(ctx context.Context, id kernel.ID) (*Product, error)
	FindBySKU(ctx context.Context, sku SKU) (*Product, error)
	Search(ctx context.Context, query string, opts SearchOptions) (*SearchResult, error)
	Delete(ctx context.Context, id kernel.ID) error
}
