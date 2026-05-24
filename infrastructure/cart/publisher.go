package cart

import (
	"context"
	"encoding/json"

	"github.com/nats-io/nats.go"

	domain "github.com/beeleelee/mall/domain/cart"
	"github.com/beeleelee/mall/domain/kernel"
)

type NATSCartEventPublisher struct {
	conn *nats.Conn
}

func NewNATSCartEventPublisher(conn *nats.Conn) *NATSCartEventPublisher {
	return &NATSCartEventPublisher{conn: conn}
}

func (p *NATSCartEventPublisher) PublishCartUpdated(ctx context.Context, cart *domain.Cart) error {
	total := cart.GetTotal()
	payload := map[string]any{
		"cart_id":   cart.ID.Int64(),
		"user_id":   cart.UserID.Int64(),
		"item_count": total.ItemCount,
		"subtotal":  total.Subtotal,
		"status":    string(cart.Status),
		"items":     cart.Items,
	}

	data, err := json.Marshal(payload)
	if err != nil {
		return kernel.NewDomainErrorWithCause(kernel.ErrInternal, "marshal cart event", err)
	}

	return p.conn.Publish("cart.updated", data)
}
