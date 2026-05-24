package order

import (
	"context"
	"encoding/json"

	"github.com/nats-io/nats.go/jetstream"

	domain "github.com/beeleelee/mall/domain/order"
	"github.com/beeleelee/mall/domain/kernel"
)

type NATSOrderEventPublisher struct {
	js jetstream.JetStream
}

func NewNATSOrderEventPublisher(js jetstream.JetStream) *NATSOrderEventPublisher {
	return &NATSOrderEventPublisher{js: js}
}

func (p *NATSOrderEventPublisher) PublishOrderEvent(ctx context.Context, order *domain.Order) error {
	payload := map[string]any{
		"order_id":      order.ID.Int64(),
		"user_id":       order.UserID.Int64(),
		"checkout_id":   order.CheckoutID.Int64(),
		"cart_id":       order.CartID.Int64(),
		"status":        string(order.Status),
		"subtotal":      order.Subtotal,
		"shipping_cost": order.ShippingCost,
		"tax_amount":    order.TaxAmount,
		"grand_total":   order.GrandTotal,
		"items":         order.Items,
		"tracking":      order.TrackingNumber,
		"carrier":       order.Carrier,
	}

	data, err := json.Marshal(payload)
	if err != nil {
		return kernel.NewDomainErrorWithCause(kernel.ErrInternal, "marshal order event", err)
	}

	_, err = p.js.Publish(ctx, "order."+string(order.Status), data)
	if err != nil {
		return kernel.NewDomainErrorWithCause(kernel.ErrInternal, "publish order event", err)
	}

	return nil
}
