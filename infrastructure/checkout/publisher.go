package checkout

import (
	"context"
	"encoding/json"

	"github.com/nats-io/nats.go/jetstream"

	domain "github.com/beeleelee/mall/domain/checkout"
	"github.com/beeleelee/mall/domain/kernel"
)

type NATSCheckoutEventPublisher struct {
	js jetstream.JetStream
}

func NewNATSCheckoutEventPublisher(js jetstream.JetStream) *NATSCheckoutEventPublisher {
	return &NATSCheckoutEventPublisher{js: js}
}

func (p *NATSCheckoutEventPublisher) PublishCheckoutUpdated(ctx context.Context, session *domain.CheckoutSession) error {
	payload := map[string]any{
		"checkout_id":      session.ID.Int64(),
		"user_id":          session.UserID.Int64(),
		"cart_id":          session.CartID.Int64(),
		"status":           string(session.Status),
		"subtotal":         session.Subtotal,
		"shipping_cost":    session.ShippingCost,
		"tax_amount":       session.TaxAmount,
		"grand_total":      session.GrandTotal,
		"payment_handler":  session.PaymentHandler,
		"items":            session.CartSnapshot.Items,
		"shipping_address": session.ShippingAddress,
		"billing_address":  session.BillingAddress,
		"shipping_option":  session.ShippingOption,
	}

	data, err := json.Marshal(payload)
	if err != nil {
		return kernel.NewDomainErrorWithCause(kernel.ErrInternal, "marshal checkout event", err)
	}

	_, err = p.js.Publish(ctx, "checkout.updated", data)
	if err != nil {
		return kernel.NewDomainErrorWithCause(kernel.ErrInternal, "publish checkout event", err)
	}

	return nil
}
