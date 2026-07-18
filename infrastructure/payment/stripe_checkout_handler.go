package payment

import (
	"context"
	"fmt"

	"github.com/stripe/stripe-go/v74"
	"github.com/stripe/stripe-go/v74/checkout/session"

	domain "github.com/beeleelee/mall/domain/checkout"
)

type StripeCheckoutHandler struct {
	client *StripeClient
}

func NewStripeCheckoutHandler(client *StripeClient) *StripeCheckoutHandler {
	return &StripeCheckoutHandler{client: client}
}

func (h *StripeCheckoutHandler) CreateCheckoutSession(ctx context.Context, checkout *domain.CheckoutSession) (*StripeCheckoutSessionInfo, error) {
	lineItems := make([]*stripe.CheckoutSessionLineItemParams, len(checkout.CartSnapshot.Items))
	for i, item := range checkout.CartSnapshot.Items {
		qty := int64(item.Quantity)
		lineItems[i] = &stripe.CheckoutSessionLineItemParams{
			PriceData: &stripe.CheckoutSessionLineItemPriceDataParams{
				Currency: stripe.String("usd"),
				ProductData: &stripe.CheckoutSessionLineItemPriceDataProductDataParams{
					Name: stripe.String(item.Name),
				},
				UnitAmount: stripe.Int64(item.UnitPrice),
			},
			Quantity: stripe.Int64(qty),
		}
	}

	baseURL := h.client.Config.BaseURL
	params := &stripe.CheckoutSessionParams{
		Mode:              stripe.String(string(stripe.CheckoutSessionModePayment)),
		SuccessURL:        stripe.String(fmt.Sprintf("%s/api/v1/checkouts/%d/success", baseURL, checkout.ID.Int64())),
		CancelURL:         stripe.String(fmt.Sprintf("%s/api/v1/checkouts/%d/cancel", baseURL, checkout.ID.Int64())),
		LineItems:         lineItems,
		ClientReferenceID: stripe.String(checkout.ID.String()),
	}
	params.PaymentIntentData = &stripe.CheckoutSessionPaymentIntentDataParams{
		Metadata: map[string]string{
			"checkout_id": checkout.ID.String(),
		},
	}

	if checkout.ShippingAddress != nil {
		params.ShippingAddressCollection = &stripe.CheckoutSessionShippingAddressCollectionParams{
			AllowedCountries: []*string{stripe.String(checkout.ShippingAddress.Country)},
		}
	}

	ses, err := session.New(params)
	if err != nil {
		return nil, fmt.Errorf("create stripe checkout session: %w", err)
	}

	piID := ""
	if ses.PaymentIntent != nil {
		piID = ses.PaymentIntent.ID
	}

	return &StripeCheckoutSessionInfo{
		ID:              ses.ID,
		URL:             ses.URL,
		Status:          string(ses.Status),
		PaymentIntentID: piID,
	}, nil
}

func (h *StripeCheckoutHandler) RetrieveCheckoutSession(ctx context.Context, sessionID string) (*StripeCheckoutSessionInfo, error) {
	ses, err := session.Get(sessionID, nil)
	if err != nil {
		return nil, fmt.Errorf("retrieve stripe checkout session: %w", err)
	}

	piID := ""
	if ses.PaymentIntent != nil {
		piID = ses.PaymentIntent.ID
	}

	return &StripeCheckoutSessionInfo{
		ID:              ses.ID,
		URL:             ses.URL,
		Status:          string(ses.Status),
		PaymentIntentID: piID,
	}, nil
}
