package order

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/dtm-labs/client/dtmcli"

	checkout "github.com/beeleelee/mall/domain/checkout"
	"github.com/beeleelee/mall/domain/kernel"
	domain "github.com/beeleelee/mall/domain/order"
)

type sagaItem struct {
	ProductID int64 `json:"product_id"`
	Quantity  int   `json:"quantity"`
}

type sagaOrderCreatePayload struct {
	OrderID    int64 `json:"order_id"`
	CheckoutID int64 `json:"checkout_id"`
	UserID     int64 `json:"user_id"`
	CartID     int64 `json:"cart_id"`
}

type sagaSubmitFn func(dtmServer, gid, cbURL string, items []sagaItem, mandateID int64, order sagaOrderCreatePayload) error

type DTMCheckoutSaga struct {
	orderSvc  *domain.OrderService
	submitFn  sagaSubmitFn
	dtmServer string
	cbURL     string
	idGen     func() (kernel.ID, error)
	logger    kernel.Logger
}

func NewDTMCheckoutSaga(
	orderSvc *domain.OrderService,
	dtmServer string,
	callbackURL string,
	sf *kernel.Snowflake,
	logger kernel.Logger,
) *DTMCheckoutSaga {
	return &DTMCheckoutSaga{
		orderSvc:  orderSvc,
		submitFn:  submitSaga,
		dtmServer: dtmServer,
		cbURL:     callbackURL,
		idGen:     sf.NextID,
		logger:    logger,
	}
}

func submitSaga(dtmServer, gid, cbURL string, items []sagaItem, mandateID int64, order sagaOrderCreatePayload) error {
	saga := dtmcli.NewSaga(dtmServer, gid)
	saga.Add(
		cbURL+"/api/v1/saga/inventory/reserve",
		cbURL+"/api/v1/saga/inventory/release",
		map[string]any{"items": items},
	)
	if mandateID > 0 {
		saga.Add(
			cbURL+"/api/v1/saga/payment/verify",
			cbURL+"/api/v1/saga/payment/cancel",
			map[string]any{"mandate_id": mandateID, "token": ""},
		)
	}
	saga.Add(
		cbURL+"/api/v1/saga/order/create",
		cbURL+"/api/v1/saga/order/cancel",
		order,
	)
	return saga.Submit()
}

func (s *DTMCheckoutSaga) Handle(ctx context.Context, data []byte) error {
	var evt checkoutCompletedPayload
	if err := json.Unmarshal(data, &evt); err != nil {
		return err
	}

	if evt.Status != string(checkout.CheckoutStatusCompleted) {
		return nil
	}

	existing, err := s.orderSvc.FindByCheckoutID(ctx, kernel.ID(evt.CheckoutID))
	if err == nil && existing != nil {
		s.logger.Info(ctx, "dtm-saga: order already exists (idempotent)",
			kernel.Field("checkout_id", evt.CheckoutID))
		return nil
	}

	newID, err := s.idGen()
	if err != nil {
		return err
	}

	gid := fmt.Sprintf("co-%d-%d", evt.CheckoutID, newID.Int64())

	items := make([]sagaItem, len(evt.Items))
	for i, item := range evt.Items {
		items[i] = sagaItem{
			ProductID: item.ProductID.Int64(),
			Quantity:  item.Quantity,
		}
	}

	orderPayload := sagaOrderCreatePayload{
		OrderID:    newID.Int64(),
		CheckoutID: evt.CheckoutID,
		UserID:     evt.UserID,
		CartID:     evt.CartID,
	}

	if err := s.submitFn(s.dtmServer, gid, s.cbURL, items, evt.MandateID, orderPayload); err != nil {
		s.logger.Error(ctx, "dtm-saga: submit failed", err,
			kernel.Field("gid", gid),
			kernel.Field("checkout_id", evt.CheckoutID))
		return err
	}

	s.logger.Info(ctx, "dtm-saga: order created",
		kernel.Field("order_id", newID.String()),
		kernel.Field("checkout_id", evt.CheckoutID),
		kernel.Field("gid", gid))
	return nil
}
