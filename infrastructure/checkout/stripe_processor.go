package checkout

import (
	"context"

	domain "github.com/beeleelee/mall/domain/checkout"
	"github.com/beeleelee/mall/domain/kernel"
	infraPayment "github.com/beeleelee/mall/infrastructure/payment"
)

type StripeProcessor struct {
	checkoutHandler *infraPayment.StripeCheckoutHandler
	intentHandler   *infraPayment.StripePaymentIntentHandler
}

func NewStripeProcessor(checkoutHandler *infraPayment.StripeCheckoutHandler, intentHandler *infraPayment.StripePaymentIntentHandler) *StripeProcessor {
	return &StripeProcessor{
		checkoutHandler: checkoutHandler,
		intentHandler:   intentHandler,
	}
}

func (p *StripeProcessor) CreateCheckoutSession(ctx context.Context, checkout *domain.CheckoutSession) (string, string, error) {
	info, err := p.checkoutHandler.CreateCheckoutSession(ctx, checkout)
	if err != nil {
		return "", "", err
	}
	return info.URL, info.ID, nil
}

func (p *StripeProcessor) CreatePaymentIntent(ctx context.Context, checkoutID kernel.ID, amount int64) (string, string, error) {
	info, err := p.intentHandler.CreatePaymentIntent(ctx, checkoutID, amount)
	if err != nil {
		return "", "", err
	}
	return info.ClientSecret, info.ID, nil
}

func (p *StripeProcessor) GetPaymentIntentStatus(ctx context.Context, paymentIntentID string) (string, error) {
	info, err := p.intentHandler.GetPaymentIntent(ctx, paymentIntentID)
	if err != nil {
		return "", err
	}
	return info.Status, nil
}

var _ domain.StripePaymentProcessor = (*StripeProcessor)(nil)
