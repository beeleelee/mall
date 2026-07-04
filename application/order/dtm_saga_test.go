package order

import (
	"context"
	"encoding/json"
	"errors"
	"testing"
	"time"

	checkout "github.com/beeleelee/mall/domain/checkout"
	"github.com/beeleelee/mall/domain/kernel"
	domain "github.com/beeleelee/mall/domain/order"
)

type fakeDTMSubmitter struct {
	submitted bool
	gid       string
	items     []sagaItem
	mandateID int64
	order     sagaOrderCreatePayload
	failErr   error
}

func (f *fakeDTMSubmitter) submit(dtmServer, gid, cbURL string, items []sagaItem, mandateID int64, order sagaOrderCreatePayload) error {
	if f.failErr != nil {
		return f.failErr
	}
	f.submitted = true
	f.gid = gid
	f.items = items
	f.mandateID = mandateID
	f.order = order
	return nil
}

type fakeIDGen struct {
	next kernel.ID
	err  error
}

func (g *fakeIDGen) nextID() (kernel.ID, error) {
	return g.next, g.err
}

func newDTMSaga() (*DTMCheckoutSaga, *domain.OrderService, *fakeDTMSubmitter) {
	repo := newFakeOrderRepo()
	pub := &fakeOrderPublisher{}
	logger := fakeLogger{}
	orderSvc := domain.NewOrderService(repo, pub, logger)

	submitter := &fakeDTMSubmitter{}
	idGen := &fakeIDGen{next: 500}

	saga := &DTMCheckoutSaga{
		orderSvc: orderSvc,
		submitFn: submitter.submit,
		dtmServer: "http://dtm:36789/api/dtmsvr",
		cbURL:    "http://localhost:8080",
		idGen:    idGen.nextID,
		logger:   logger,
	}
	return saga, orderSvc, submitter
}

func makeDTMCompletedPayload(checkoutID int64, mandateID int64) []byte {
	data, _ := json.Marshal(checkoutCompletedPayload{
		CheckoutID:     checkoutID,
		UserID:         42,
		CartID:         10,
		Status:         "completed",
		Subtotal:       2000,
		ShippingCost:   500,
		TaxAmount:      250,
		GrandTotal:     2750,
		PaymentHandler: "ap2_mandate",
		MandateID:      mandateID,
		Items: []checkout.CartSnapshotItem{
			{ProductID: 100, SKU: "SKU001", Name: "P1", Quantity: 2, UnitPrice: 1000},
		},
		ShippingAddress: &checkout.Address{Line1: "123 Main St", City: "Portland", State: "OR", PostalCode: "97201", Country: "US"},
		BillingAddress:  &checkout.Address{Line1: "123 Main St", City: "Portland", State: "OR", PostalCode: "97201", Country: "US"},
		ShippingOption:  &checkout.ShippingOption{ID: "std", Name: "Standard", Cost: 500},
	})
	return data
}

func TestDTMCheckoutSaga_HappyPath(t *testing.T) {
	saga, orderSvc, submitter := newDTMSaga()
	ctx := context.Background()

	err := saga.Handle(ctx, makeDTMCompletedPayload(100, 0))
	if err != nil {
		t.Fatal(err)
	}

	if !submitter.submitted {
		t.Fatal("expected saga to be submitted")
	}
	if submitter.order.CheckoutID != 100 {
		t.Errorf("expected checkout_id 100, got %d", submitter.order.CheckoutID)
	}
	if submitter.order.OrderID != 500 {
		t.Errorf("expected order_id 500, got %d", submitter.order.OrderID)
	}

	_, err = orderSvc.FindByCheckoutID(ctx, 100)
	if !kernel.IsNotFound(err) {
		t.Error("expected no order yet (created asynchronously via DTM)")
	}
}

func TestDTMCheckoutSaga_WithMandate(t *testing.T) {
	saga, _, submitter := newDTMSaga()

	err := saga.Handle(context.Background(), makeDTMCompletedPayload(200, 77))
	if err != nil {
		t.Fatal(err)
	}

	if submitter.mandateID != 77 {
		t.Errorf("expected mandate_id 77, got %d", submitter.mandateID)
	}
}

func TestDTMCheckoutSaga_DifferentOrderID(t *testing.T) {
	saga, _, submitter := newDTMSaga()
	ctx := context.Background()

	_ = saga.Handle(ctx, makeDTMCompletedPayload(300, 0))
	orderID1 := submitter.order.OrderID

	saga.idGen = (&fakeIDGen{next: 600}).nextID
	_ = saga.Handle(ctx, makeDTMCompletedPayload(301, 0))
	orderID2 := submitter.order.OrderID

	if orderID1 == orderID2 {
		t.Error("expected different order IDs for different checkouts")
	}
}

func TestDTMCheckoutSaga_IgnoresNonCompleted(t *testing.T) {
	saga, _, submitter := newDTMSaga()

	payload := makeDTMCompletedPayload(400, 0)
	var evt checkoutCompletedPayload
	_ = json.Unmarshal(payload, &evt)
	evt.Status = "cancelled"
	data, _ := json.Marshal(evt)

	err := saga.Handle(context.Background(), data)
	if err != nil {
		t.Fatal(err)
	}

	if submitter.submitted {
		t.Error("expected no saga submission for non-completed event")
	}
}

func TestDTMCheckoutSaga_InvalidJSON(t *testing.T) {
	saga := &DTMCheckoutSaga{}
	err := saga.Handle(context.Background(), []byte("not json"))
	if err == nil {
		t.Fatal("expected error for invalid JSON")
	}
}

func TestDTMCheckoutSaga_Idempotent(t *testing.T) {
	saga, orderSvc, submitter := newDTMSaga()
	ctx := context.Background()

	sf, _ := kernel.NewSnowflake(1)
	newID, _ := sf.NextID()
	session := rebuildSessionFromPayload(makeDTMCompletedPayload(500, 0), newID)
	orderSvc.CreateOrder(ctx, newID, session)

	err := saga.Handle(ctx, makeDTMCompletedPayload(500, 0))
	if err != nil {
		t.Fatal(err)
	}

	if submitter.submitted {
		t.Error("expected no saga submission for already processed checkout")
	}
}

func TestDTMCheckoutSaga_SubmitFailure(t *testing.T) {
	saga, _, submitter := newDTMSaga()
	submitter.failErr = errors.New("dtm unavailable")

	err := saga.Handle(context.Background(), makeDTMCompletedPayload(600, 0))
	if err == nil {
		t.Fatal("expected error on DTM submit failure")
	}
}

func TestDTMCheckoutSaga_ItemsPassed(t *testing.T) {
	saga, _, submitter := newDTMSaga()

	err := saga.Handle(context.Background(), makeDTMCompletedPayload(700, 0))
	if err != nil {
		t.Fatal(err)
	}

	if len(submitter.items) != 1 {
		t.Fatalf("expected 1 item, got %d", len(submitter.items))
	}
	if submitter.items[0].ProductID != 100 {
		t.Errorf("expected product_id 100, got %d", submitter.items[0].ProductID)
	}
	if submitter.items[0].Quantity != 2 {
		t.Errorf("expected quantity 2, got %d", submitter.items[0].Quantity)
	}
}

func rebuildSessionFromPayload(data []byte, orderID kernel.ID) *checkout.CheckoutSession {
	var evt checkoutCompletedPayload
	json.Unmarshal(data, &evt)
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
