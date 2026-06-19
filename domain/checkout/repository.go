package checkout

import (
	"context"

	"github.com/beeleelee/mall/domain/kernel"
)

type CheckoutRepository interface {
	Save(ctx context.Context, session *CheckoutSession) error
	FindByID(ctx context.Context, id kernel.ID) (*CheckoutSession, error)
	FindByUserID(ctx context.Context, userID kernel.ID) (*CheckoutSession, error)
	Delete(ctx context.Context, id kernel.ID) error
}

type TaxInput struct {
	Items    []CartSnapshotItem
	Subtotal int64
	Cost     int64
	Address  *Address
}

type TaxResult struct {
	TaxAmount int64
	Provider  string
}

type TaxService interface {
	CalculateTax(ctx context.Context, input TaxInput) (*TaxResult, error)
}

type PriceInput struct {
	Items        []CartSnapshotItem
	ShippingCost int64
	TaxAmount    int64
	DiscountCode string
}

type PriceResult struct {
	Subtotal   int64
	Shipping   int64
	Tax        int64
	GrandTotal int64
}

type PriceCalculator interface {
	Calculate(ctx context.Context, input PriceInput) (*PriceResult, error)
}

type CheckoutEventPublisher interface {
	PublishCheckoutUpdated(ctx context.Context, session *CheckoutSession) error
}
