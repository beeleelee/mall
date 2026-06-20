package cart

import (
	"context"
	"encoding/json"

	"github.com/nats-io/nats.go"
	"github.com/nats-io/nats.go/jetstream"

	domain "github.com/beeleelee/mall/domain/cart"
	"github.com/beeleelee/mall/domain/kernel"
	"github.com/beeleelee/mall/infrastructure/tracing"
)

type NATSCartEventPublisher struct {
	js jetstream.JetStream
}

func NewNATSCartEventPublisher(js jetstream.JetStream) *NATSCartEventPublisher {
	return &NATSCartEventPublisher{js: js}
}

func (p *NATSCartEventPublisher) PublishCartUpdated(ctx context.Context, cart *domain.Cart) error {
	total := cart.GetTotal()
	payload := map[string]any{
		"cart_id":    cart.ID.Int64(),
		"user_id":    cart.UserID.Int64(),
		"item_count": total.ItemCount,
		"subtotal":   total.Subtotal,
		"status":     string(cart.Status),
		"items":      cart.Items,
	}

	data, err := json.Marshal(payload)
	if err != nil {
		return kernel.NewDomainErrorWithCause(kernel.ErrInternal, "marshal cart event", err)
	}

	msg := &nats.Msg{
		Subject: "cart.updated",
		Data:    data,
		Header:  nats.Header{},
	}
	tracing.InjectTrace(ctx, msg)

	_, err = p.js.PublishMsg(ctx, msg)
	if err != nil {
		return kernel.NewDomainErrorWithCause(kernel.ErrInternal, "publish cart event", err)
	}

	return nil
}
