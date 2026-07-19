package review

import (
	"context"

	"github.com/beeleelee/mall/domain/kernel"
)

type ReviewQueryOptions struct {
	Limit  int
	Offset int
	Status ReviewStatus
}

type ReviewListResult struct {
	Reviews []*Review
	Total   int
}

type ReviewRepository interface {
	Save(ctx context.Context, review *Review) error
	FindByID(ctx context.Context, id kernel.ID) (*Review, error)
	FindByProduct(ctx context.Context, productID kernel.ID, opts ReviewQueryOptions) (*ReviewListResult, error)
	FindByUser(ctx context.Context, userID kernel.ID, opts ReviewQueryOptions) (*ReviewListResult, error)
	FindByProductAndUser(ctx context.Context, productID, userID kernel.ID) (*Review, error)
	FindAll(ctx context.Context, opts ReviewQueryOptions) (*ReviewListResult, error)
	Delete(ctx context.Context, id kernel.ID) error
	GetAverageRating(ctx context.Context, productID kernel.ID) (float64, error)
}
