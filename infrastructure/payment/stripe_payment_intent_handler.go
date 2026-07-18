package payment

import (
	"context"
	"fmt"

	"github.com/stripe/stripe-go/v74"
	"github.com/stripe/stripe-go/v74/paymentintent"

	"github.com/beeleelee/mall/domain/kernel"
)

type StripePaymentIntentHandler struct {
	client *StripeClient
}

func NewStripePaymentIntentHandler(client *StripeClient) *StripePaymentIntentHandler {
	return &StripePaymentIntentHandler{client: client}
}

func (h *StripePaymentIntentHandler) CreatePaymentIntent(ctx context.Context, checkoutID kernel.ID, amount int64) (*StripePaymentIntentInfo, error) {
	params := &stripe.PaymentIntentParams{
		Amount:   stripe.Int64(amount),
		Currency: stripe.String("usd"),
		AutomaticPaymentMethods: &stripe.PaymentIntentAutomaticPaymentMethodsParams{
			Enabled: stripe.Bool(true),
		},
		Metadata: map[string]string{
			"checkout_id": checkoutID.String(),
		},
	}

	pi, err := paymentintent.New(params)
	if err != nil {
		return nil, fmt.Errorf("create stripe payment intent: %w", err)
	}

	return &StripePaymentIntentInfo{
		ID:           pi.ID,
		ClientSecret: pi.ClientSecret,
		Amount:       pi.Amount,
		Currency:     string(pi.Currency),
		Status:       string(pi.Status),
	}, nil
}

func (h *StripePaymentIntentHandler) GetPaymentIntent(ctx context.Context, paymentIntentID string) (*StripePaymentIntentInfo, error) {
	pi, err := paymentintent.Get(paymentIntentID, nil)
	if err != nil {
		return nil, fmt.Errorf("get stripe payment intent: %w", err)
	}

	return &StripePaymentIntentInfo{
		ID:           pi.ID,
		ClientSecret: pi.ClientSecret,
		Amount:       pi.Amount,
		Currency:     string(pi.Currency),
		Status:       string(pi.Status),
	}, nil
}

func (h *StripePaymentIntentHandler) ConfirmPaymentIntent(ctx context.Context, paymentIntentID string) (*StripePaymentIntentInfo, error) {
	params := &stripe.PaymentIntentConfirmParams{}
	pi, err := paymentintent.Confirm(paymentIntentID, params)
	if err != nil {
		return nil, fmt.Errorf("confirm stripe payment intent: %w", err)
	}

	return &StripePaymentIntentInfo{
		ID:           pi.ID,
		ClientSecret: pi.ClientSecret,
		Amount:       pi.Amount,
		Currency:     string(pi.Currency),
		Status:       string(pi.Status),
	}, nil
}
