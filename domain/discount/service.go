package discount

import (
	"context"
	"time"

	"github.com/beeleelee/mall/domain/kernel"
)

type DiscountRepository interface {
	Save(ctx context.Context, code *DiscountCode) error
	FindByCode(ctx context.Context, code string) (*DiscountCode, error)
	IncrementUsage(ctx context.Context, id kernel.ID) error
}

type DiscountService struct {
	repo   DiscountRepository
	logger kernel.Logger
}

func NewDiscountService(repo DiscountRepository, logger kernel.Logger) *DiscountService {
	return &DiscountService{repo: repo, logger: logger}
}

func (s *DiscountService) CreateCode(ctx context.Context, id kernel.ID, code string, discountType DiscountType, value, minPurchase int64, maxUsages int, expiry time.Time, stackable bool) (*DiscountCode, error) {
	dc, err := NewDiscountCode(id, code, discountType, value, minPurchase, maxUsages, expiry, stackable)
	if err != nil {
		return nil, err
	}
	if err := s.repo.Save(ctx, dc); err != nil {
		return nil, err
	}
	return dc, nil
}

func (s *DiscountService) ValidateCode(ctx context.Context, code string, subtotal int64) (*DiscountCode, bool) {
	dc, err := s.repo.FindByCode(ctx, code)
	if err != nil {
		return nil, false
	}
	return dc, dc.IsValid(subtotal)
}

func (s *DiscountService) ApplyCode(ctx context.Context, code string, subtotal int64) (int64, bool, error) {
	dc, err := s.repo.FindByCode(ctx, code)
	if err != nil {
		return subtotal, false, kernel.NewDomainError(kernel.ErrNotFound, "discount code not found")
	}

	if !dc.IsValid(subtotal) {
		return subtotal, false, nil
	}

	finalAmount := dc.Apply(subtotal)

	if err := s.repo.IncrementUsage(ctx, dc.ID); err != nil {
		return subtotal, false, err
	}
	dc.Use()

	s.logger.Info(ctx, "discount.applied", kernel.Field("code", code), kernel.Field("original", subtotal), kernel.Field("final", finalAmount))
	return finalAmount, true, nil
}

func (s *DiscountService) DeactivateCode(ctx context.Context, code string) error {
	dc, err := s.repo.FindByCode(ctx, code)
	if err != nil {
		return err
	}
	dc.Deactivate()
	return s.repo.Save(ctx, dc)
}
