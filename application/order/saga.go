package order

import (
	"context"
	"encoding/json"
	"time"

	checkout "github.com/beeleelee/mall/domain/checkout"
	"github.com/beeleelee/mall/domain/kernel"
	domain "github.com/beeleelee/mall/domain/order"
)

type checkoutCompletedPayload struct {
	CheckoutID      int64                       `json:"checkout_id"`
	UserID          int64                       `json:"user_id"`
	CartID          int64                       `json:"cart_id"`
	Status          string                      `json:"status"`
	Subtotal        int64                       `json:"subtotal"`
	ShippingCost    int64                       `json:"shipping_cost"`
	TaxAmount       int64                       `json:"tax_amount"`
	GrandTotal      int64                       `json:"grand_total"`
	PaymentHandler  string                      `json:"payment_handler"`
	MandateID       int64                       `json:"mandate_id"`
	Items           []checkout.CartSnapshotItem `json:"items"`
	ShippingAddress *checkout.Address           `json:"shipping_address"`
	BillingAddress  *checkout.Address           `json:"billing_address"`
	ShippingOption  *checkout.ShippingOption    `json:"shipping_option"`
}

type CheckoutCompletedSaga struct {
	orderSvc *domain.OrderService
	idGen    func() (kernel.ID, error)
	logger   kernel.Logger
}

func NewCheckoutCompletedSaga(orderSvc *domain.OrderService, sf *kernel.Snowflake, logger kernel.Logger) *CheckoutCompletedSaga {
	return &CheckoutCompletedSaga{
		orderSvc: orderSvc,
		idGen:    sf.NextID,
		logger:   logger,
	}
}

func (s *CheckoutCompletedSaga) Handle(ctx context.Context, data []byte) error {
	var evt checkoutCompletedPayload
	if err := json.Unmarshal(data, &evt); err != nil {
		return err
	}

	if evt.Status != string(checkout.CheckoutStatusCompleted) {
		return nil
	}

	existing, err := s.orderSvc.FindByCheckoutID(ctx, kernel.ID(evt.CheckoutID))
	if err == nil && existing != nil {
		s.logger.Info(ctx, "saga: order already exists (idempotent)", kernel.Field("order_id", existing.ID.String()), kernel.Field("checkout_id", evt.CheckoutID))
		return nil
	}

	newID, err := s.idGen()
	if err != nil {
		return err
	}

	session := rebuildSession(evt)
	if _, err := s.orderSvc.CreateOrder(ctx, newID, session); err != nil {
		return err
	}

	s.logger.Info(ctx, "saga: order created", kernel.Field("order_id", newID.String()), kernel.Field("checkout_id", evt.CheckoutID))
	return nil
}

func rebuildSession(evt checkoutCompletedPayload) *checkout.CheckoutSession {
	now := time.Now()
	snapshot := checkout.NewCartSnapshot(evt.Items)
	return &checkout.CheckoutSession{
		AggregateRoot:   kernel.NewAggregateRoot(kernel.ID(evt.CheckoutID)),
		UserID:          kernel.ID(evt.UserID),
		CartID:          kernel.ID(evt.CartID),
		CartSnapshot:    snapshot,
		ShippingAddress: evt.ShippingAddress,
		BillingAddress:  evt.BillingAddress,
		ShippingOption:  evt.ShippingOption,
		PaymentHandler:  evt.PaymentHandler,
		Subtotal:        evt.Subtotal,
		ShippingCost:    evt.ShippingCost,
		TaxAmount:       evt.TaxAmount,
		GrandTotal:      evt.GrandTotal,
		Status:          checkout.CheckoutStatusCompleted,
		CompletedAt:     &now,
	}
}
