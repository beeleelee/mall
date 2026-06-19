package order

import (
	"context"
	"encoding/json"
	"testing"

	checkout "github.com/beeleelee/mall/domain/checkout"
	"github.com/beeleelee/mall/domain/kernel"
	domain "github.com/beeleelee/mall/domain/order"
)

type fakeOrderRepo struct {
	orders map[kernel.ID]*domain.Order
}

func newFakeOrderRepo() *fakeOrderRepo {
	return &fakeOrderRepo{orders: make(map[kernel.ID]*domain.Order)}
}

func (r *fakeOrderRepo) Save(_ context.Context, o *domain.Order) error {
	r.orders[o.ID] = o
	return nil
}

func (r *fakeOrderRepo) FindByID(_ context.Context, id kernel.ID) (*domain.Order, error) {
	o, ok := r.orders[id]
	if !ok {
		return nil, kernel.NewDomainError(kernel.ErrNotFound, "order not found")
	}
	return o, nil
}

func (r *fakeOrderRepo) FindByUserID(_ context.Context, userID kernel.ID) ([]*domain.Order, error) {
	var result []*domain.Order
	for _, o := range r.orders {
		if o.UserID == userID {
			result = append(result, o)
		}
	}
	if len(result) == 0 {
		return nil, kernel.NewDomainError(kernel.ErrNotFound, "no orders found for user")
	}
	return result, nil
}

func (r *fakeOrderRepo) FindByCheckoutID(_ context.Context, checkoutID kernel.ID) (*domain.Order, error) {
	for _, o := range r.orders {
		if o.CheckoutID == checkoutID {
			return o, nil
		}
	}
	return nil, kernel.NewDomainError(kernel.ErrNotFound, "order not found for checkout")
}

func (r *fakeOrderRepo) Delete(_ context.Context, id kernel.ID) error {
	delete(r.orders, id)
	return nil
}

type fakeOrderPublisher struct{}

func (p *fakeOrderPublisher) PublishOrderEvent(_ context.Context, _ *domain.Order) error {
	return nil
}

type fakeLogger struct{}

func (fakeLogger) Debug(_ context.Context, _ string, _ ...kernel.LogField)          {}
func (fakeLogger) Info(_ context.Context, _ string, _ ...kernel.LogField)           {}
func (fakeLogger) Warn(_ context.Context, _ string, _ ...kernel.LogField)           {}
func (fakeLogger) Error(_ context.Context, _ string, _ error, _ ...kernel.LogField) {}

func newSaga() *CheckoutCompletedSaga {
	repo := newFakeOrderRepo()
	pub := &fakeOrderPublisher{}
	logger := fakeLogger{}
	orderSvc := domain.NewOrderService(repo, pub, logger)
	sf, _ := kernel.NewSnowflake(1)
	return NewCheckoutCompletedSaga(orderSvc, sf, logger)
}

func makeCompletedPayload(checkoutID int64) []byte {
	data, _ := json.Marshal(checkoutCompletedPayload{
		CheckoutID:     checkoutID,
		UserID:         42,
		CartID:         10,
		Status:         "completed",
		Subtotal:       2000,
		ShippingCost:   500,
		TaxAmount:      250,
		GrandTotal:     2750,
		PaymentHandler: "stripe",
		Items: []checkout.CartSnapshotItem{
			{ProductID: 100, SKU: "SKU001", Name: "Product 1", Quantity: 2, UnitPrice: 1000},
		},
		ShippingAddress: &checkout.Address{Line1: "123 Main St", City: "Portland", State: "OR", PostalCode: "97201", Country: "US"},
		BillingAddress:  &checkout.Address{Line1: "123 Main St", City: "Portland", State: "OR", PostalCode: "97201", Country: "US"},
		ShippingOption:  &checkout.ShippingOption{ID: "std", Name: "Standard", Cost: 500},
	})
	return data
}

func TestCheckoutCompletedSaga_HappyPath(t *testing.T) {
	saga := newSaga()
	ctx := context.Background()
	data := makeCompletedPayload(100)

	err := saga.Handle(ctx, data)
	if err != nil {
		t.Fatal(err)
	}

	order, err := saga.orderSvc.FindByCheckoutID(ctx, 100)
	if err != nil {
		t.Fatal("order should have been created, got: ", err)
	}
	if order.CheckoutID != 100 {
		t.Errorf("expected checkout_id 100, got %d", order.CheckoutID)
	}
	if order.CheckoutID.Int64() == order.ID.Int64() {
		t.Error("order.ID should differ from checkout_id")
	}
}

func TestCheckoutCompletedSaga_IgnoresNonCompleted(t *testing.T) {
	saga := newSaga()
	ctx := context.Background()

	payload := makeCompletedPayload(200)
	var evt checkoutCompletedPayload
	json.Unmarshal(payload, &evt)
	evt.Status = "cancelled"
	data, _ := json.Marshal(evt)

	err := saga.Handle(ctx, data)
	if err != nil {
		t.Fatal(err)
	}

	_, err = saga.orderSvc.FindByCheckoutID(ctx, 200)
	if !kernel.IsNotFound(err) {
		t.Errorf("expected no order created for cancelled event, got: %v", err)
	}
}

func TestCheckoutCompletedSaga_InvalidJSON(t *testing.T) {
	saga := newSaga()
	err := saga.Handle(context.Background(), []byte("not json"))
	if err == nil {
		t.Fatal("expected error for invalid JSON")
	}
}

func TestCheckoutCompletedSaga_Idempotent(t *testing.T) {
	saga := newSaga()
	ctx := context.Background()
	data := makeCompletedPayload(300)

	if err := saga.Handle(ctx, data); err != nil {
		t.Fatal(err)
	}

	if err := saga.Handle(ctx, data); err != nil {
		t.Fatal("expected idempotent handling, got: ", err)
	}

	order, err := saga.orderSvc.FindByCheckoutID(ctx, 300)
	if err != nil {
		t.Fatal("order should exist, got: ", err)
	}
	if order == nil {
		t.Fatal("expected non-nil order")
	}
}

func TestCheckoutCompletedSaga_FieldsPopulatedCorrectly(t *testing.T) {
	saga := newSaga()
	ctx := context.Background()
	data := makeCompletedPayload(400)

	saga.Handle(ctx, data)

	order, _ := saga.orderSvc.FindByCheckoutID(ctx, 400)
	if order.UserID != 42 {
		t.Errorf("expected user 42, got %d", order.UserID)
	}
	if order.Subtotal != 2000 {
		t.Errorf("expected subtotal 2000, got %d", order.Subtotal)
	}
	if order.GrandTotal != 2750 {
		t.Errorf("expected grand_total 2750, got %d", order.GrandTotal)
	}
	if order.PaymentHandler != "stripe" {
		t.Errorf("expected payment_handler stripe, got %s", order.PaymentHandler)
	}
	if order.Status != domain.OrderStatusConfirmed {
		t.Errorf("expected confirmed, got %s", order.Status)
	}
}
