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

type DTMCheckoutSaga struct {
	orderSvc    *domain.OrderService
	dtmServer   string
	callbackURL string
	idGen       func() (kernel.ID, error)
	logger      kernel.Logger
}

func NewDTMCheckoutSaga(
	orderSvc *domain.OrderService,
	dtmServer string,
	callbackURL string,
	sf *kernel.Snowflake,
	logger kernel.Logger,
) *DTMCheckoutSaga {
	return &DTMCheckoutSaga{
		orderSvc:    orderSvc,
		dtmServer:   dtmServer,
		callbackURL: callbackURL,
		idGen:       sf.NextID,
		logger:      logger,
	}
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
	saga := dtmcli.NewSaga(s.dtmServer, gid)

	items := make([]sagaItem, len(evt.Items))
	for i, item := range evt.Items {
		items[i] = sagaItem{
			ProductID: item.ProductID.Int64(),
			Quantity:  item.Quantity,
		}
	}

	cb := s.callbackURL

	saga.Add(
		cb+"/api/v1/saga/inventory/reserve",
		cb+"/api/v1/saga/inventory/release",
		map[string]any{"items": items},
	)

	if evt.MandateID > 0 {
		saga.Add(
			cb+"/api/v1/saga/payment/verify",
			cb+"/api/v1/saga/payment/cancel",
			map[string]any{"mandate_id": evt.MandateID, "token": ""},
		)
	}

	saga.Add(
		cb+"/api/v1/saga/order/create",
		cb+"/api/v1/saga/order/cancel",
		sagaOrderCreatePayload{
			OrderID:    newID.Int64(),
			CheckoutID: evt.CheckoutID,
			UserID:     evt.UserID,
			CartID:     evt.CartID,
		},
	)

	if err := saga.Submit(); err != nil {
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
