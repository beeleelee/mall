package checkout

import (
	"context"
	"encoding/json"

	"github.com/nats-io/nats.go"

	domain "github.com/beeleelee/mall/domain/checkout"
	"github.com/beeleelee/mall/domain/kernel"
)

type NATSCheckoutEventPublisher struct {
	conn *nats.Conn
}

func NewNATSCheckoutEventPublisher(conn *nats.Conn) *NATSCheckoutEventPublisher {
	return &NATSCheckoutEventPublisher{conn: conn}
}

func (p *NATSCheckoutEventPublisher) PublishCheckoutUpdated(ctx context.Context, session *domain.CheckoutSession) error {
	payload := map[string]any{
		"checkout_id":  session.ID.Int64(),
		"user_id":      session.UserID.Int64(),
		"cart_id":      session.CartID.Int64(),
		"status":       string(session.Status),
		"subtotal":     session.Subtotal,
		"shipping_cost": session.ShippingCost,
		"tax_amount":   session.TaxAmount,
		"grand_total":  session.GrandTotal,
		"items":        session.CartSnapshot.Items,
	}

	data, err := json.Marshal(payload)
	if err != nil {
		return kernel.NewDomainErrorWithCause(kernel.ErrInternal, "marshal checkout event", err)
	}

	return p.conn.Publish("checkout.updated", data)
}
