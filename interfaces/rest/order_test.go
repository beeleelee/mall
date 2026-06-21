package rest

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strconv"
	"testing"
	"time"

	"github.com/zeromicro/go-zero/rest/pathvar"

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

func (f *fakeOrderRepo) Save(_ context.Context, o *domain.Order) error {
	f.orders[o.ID] = o
	return nil
}

func (f *fakeOrderRepo) FindByID(_ context.Context, id kernel.ID) (*domain.Order, error) {
	o, ok := f.orders[id]
	if !ok {
		return nil, kernel.NewDomainError(kernel.ErrNotFound, "order not found")
	}
	return o, nil
}

func (f *fakeOrderRepo) FindByUserID(_ context.Context, userID kernel.ID) ([]*domain.Order, error) {
	var result []*domain.Order
	for _, o := range f.orders {
		if o.UserID == userID {
			result = append(result, o)
		}
	}
	return result, nil
}

func (f *fakeOrderRepo) FindByCheckoutID(_ context.Context, checkoutID kernel.ID) (*domain.Order, error) {
	for _, o := range f.orders {
		if o.CheckoutID == checkoutID {
			return o, nil
		}
	}
	return nil, kernel.NewDomainError(kernel.ErrNotFound, "order not found")
}

func (f *fakeOrderRepo) FindAll(_ context.Context, offset, limit int) ([]*domain.Order, error) {
	result := make([]*domain.Order, 0, len(f.orders))
	for _, o := range f.orders {
		result = append(result, o)
	}
	if offset >= len(result) {
		return []*domain.Order{}, nil
	}
	end := offset + limit
	if end > len(result) {
		end = len(result)
	}
	return result[offset:end], nil
}

func (f *fakeOrderRepo) Delete(_ context.Context, id kernel.ID) error {
	delete(f.orders, id)
	return nil
}

type fakeOrderPub struct{}

func (fakeOrderPub) PublishOrderEvent(_ context.Context, _ *domain.Order) error { return nil }

type orderTestFixture struct {
	handler *OrderHandler
	repo    *fakeOrderRepo
	sf      *kernel.Snowflake
}

func newOrderTestFixture(t *testing.T) *orderTestFixture {
	t.Helper()
	repo := newFakeOrderRepo()
	pub := fakeOrderPub{}
	logger := fakeLog{}
	svc := domain.NewOrderService(repo, pub, logger)
	sf, err := kernel.NewSnowflake(1)
	if err != nil {
		t.Fatalf("NewSnowflake: %v", err)
	}
	return &orderTestFixture{handler: NewOrderHandler(svc), repo: repo, sf: sf}
}

func (f *orderTestFixture) seedOrder(t *testing.T, userID int64) kernel.ID {
	t.Helper()
	id, err := f.sf.NextID()
	if err != nil {
		t.Fatalf("NextID: %v", err)
	}
	now := time.Now()
	o := domain.NewOrderFromSnapshot(
		id, kernel.ID(userID), 10, 20,
		[]domain.OrderLineItem{{ProductID: 100, SKU: "SKU001", Name: "Test", Quantity: 2, UnitPrice: 1000, TotalPrice: 2000}},
		checkout.Address{Line1: "123 Main St", City: "Springfield", State: "IL", PostalCode: "62701", Country: "US"},
		checkout.Address{Line1: "123 Main St", City: "Springfield", State: "IL", PostalCode: "62701", Country: "US"},
		checkout.ShippingOption{ID: "std", Name: "Standard", Cost: 500},
		"stripe", 2000, 500, 0, 2500,
		domain.OrderStatusConfirmed, "", "",
		now, nil, nil, nil, nil, nil, now, now,
	)
	if err := f.repo.Save(context.Background(), o); err != nil {
		t.Fatalf("seed order: %v", err)
	}
	return o.ID
}

func TestOrderHandler_ListByUser_Success(t *testing.T) {
	f := newOrderTestFixture(t)
	f.seedOrder(t, 1)
	f.seedOrder(t, 1)
	f.seedOrder(t, 2) // different user

	req := userRequest(t, http.MethodGet, "/api/v1/orders", nil, 1)
	rec := httptest.NewRecorder()
	f.handler.ListByUser(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", rec.Code, rec.Body.String())
	}

	var resp []orderResponse
	if err := json.NewDecoder(rec.Body).Decode(&resp); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	if len(resp) != 2 {
		t.Fatalf("expected 2 orders for user 1, got %d", len(resp))
	}
}

func TestOrderHandler_GetOrder_Success(t *testing.T) {
	f := newOrderTestFixture(t)
	oid := f.seedOrder(t, 1)

	idStr := strconv.FormatInt(oid.Int64(), 10)
	req := userRequest(t, http.MethodGet, "/api/v1/orders/"+idStr, nil, 1)
	req = pathvar.WithVars(req, map[string]string{"id": idStr})
	rec := httptest.NewRecorder()
	f.handler.GetOrder(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", rec.Code, rec.Body.String())
	}

	var resp orderResponse
	if err := json.NewDecoder(rec.Body).Decode(&resp); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	if resp.ID != oid.Int64() {
		t.Errorf("expected ID %d, got %d", oid.Int64(), resp.ID)
	}
}

func TestOrderHandler_GetOrder_Ownership(t *testing.T) {
	f := newOrderTestFixture(t)
	oid := f.seedOrder(t, 1)

	idStr := strconv.FormatInt(oid.Int64(), 10)
	req := userRequest(t, http.MethodGet, "/api/v1/orders/"+idStr, nil, 2)
	req = pathvar.WithVars(req, map[string]string{"id": idStr})
	rec := httptest.NewRecorder()
	f.handler.GetOrder(rec, req)

	if rec.Code != http.StatusForbidden {
		t.Errorf("expected 403 for wrong user, got %d", rec.Code)
	}
}

func TestOrderHandler_GetOrder_NotFound(t *testing.T) {
	f := newOrderTestFixture(t)
	req := userRequest(t, http.MethodGet, "/api/v1/orders/999", nil, 1)
	req = pathvar.WithVars(req, map[string]string{"id": "999"})
	rec := httptest.NewRecorder()
	f.handler.GetOrder(rec, req)

	if rec.Code != http.StatusNotFound {
		t.Errorf("expected 404, got %d", rec.Code)
	}
}

func TestOrderHandler_StartProcessing_Success(t *testing.T) {
	f := newOrderTestFixture(t)
	oid := f.seedOrder(t, 1)

	idStr := strconv.FormatInt(oid.Int64(), 10)
	req := userRequest(t, http.MethodPost, "/api/v1/orders/"+idStr+"/process", nil, 1)
	req = pathvar.WithVars(req, map[string]string{"id": idStr})
	rec := httptest.NewRecorder()
	f.handler.StartProcessing(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", rec.Code, rec.Body.String())
	}

	var resp orderResponse
	json.NewDecoder(rec.Body).Decode(&resp)
	if resp.Status != string(domain.OrderStatusProcessing) {
		t.Errorf("expected processing, got %s", resp.Status)
	}
}

func TestOrderHandler_Ship_Success(t *testing.T) {
	f := newOrderTestFixture(t)
	oid := f.seedOrder(t, 1)

	idStr := strconv.FormatInt(oid.Int64(), 10)

	processReq := userRequest(t, http.MethodPost, "/api/v1/orders/"+idStr+"/process", nil, 1)
	processReq = pathvar.WithVars(processReq, map[string]string{"id": idStr})
	f.handler.StartProcessing(httptest.NewRecorder(), processReq)

	shipBody := map[string]string{"tracking_number": "TRACK123", "carrier": "UPS"}
	data, _ := json.Marshal(shipBody)
	shipReq := userRequest(t, http.MethodPost, "/api/v1/orders/"+idStr+"/ship", data, 1)
	shipReq = pathvar.WithVars(shipReq, map[string]string{"id": idStr})
	rec := httptest.NewRecorder()
	f.handler.Ship(rec, shipReq)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", rec.Code, rec.Body.String())
	}

	var resp orderResponse
	json.NewDecoder(rec.Body).Decode(&resp)
	if resp.Status != string(domain.OrderStatusShipped) {
		t.Errorf("expected shipped, got %s", resp.Status)
	}
	if resp.TrackingNumber != "TRACK123" {
		t.Errorf("expected TRACK123, got %s", resp.TrackingNumber)
	}
}

func TestOrderHandler_MarkDelivered_Success(t *testing.T) {
	f := newOrderTestFixture(t)
	oid := f.seedOrder(t, 1)

	idStr := strconv.FormatInt(oid.Int64(), 10)

	processReq := userRequest(t, http.MethodPost, "/api/v1/orders/"+idStr+"/process", nil, 1)
	processReq = pathvar.WithVars(processReq, map[string]string{"id": idStr})
	f.handler.StartProcessing(httptest.NewRecorder(), processReq)

	shipBody := map[string]string{"tracking_number": "TRACK123", "carrier": "UPS"}
	data, _ := json.Marshal(shipBody)
	shipReq := userRequest(t, http.MethodPost, "/api/v1/orders/"+idStr+"/ship", data, 1)
	shipReq = pathvar.WithVars(shipReq, map[string]string{"id": idStr})
	f.handler.Ship(httptest.NewRecorder(), shipReq)

	deliverReq := userRequest(t, http.MethodPost, "/api/v1/orders/"+idStr+"/deliver", nil, 1)
	deliverReq = pathvar.WithVars(deliverReq, map[string]string{"id": idStr})
	rec := httptest.NewRecorder()
	f.handler.MarkDelivered(rec, deliverReq)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", rec.Code, rec.Body.String())
	}

	var resp orderResponse
	json.NewDecoder(rec.Body).Decode(&resp)
	if resp.Status != string(domain.OrderStatusDelivered) {
		t.Errorf("expected delivered, got %s", resp.Status)
	}
}

func TestOrderHandler_CancelOrder_Success(t *testing.T) {
	f := newOrderTestFixture(t)
	oid := f.seedOrder(t, 1)

	idStr := strconv.FormatInt(oid.Int64(), 10)
	req := userRequest(t, http.MethodPost, "/api/v1/orders/"+idStr+"/cancel", nil, 1)
	req = pathvar.WithVars(req, map[string]string{"id": idStr})
	rec := httptest.NewRecorder()
	f.handler.CancelOrder(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", rec.Code, rec.Body.String())
	}

	var resp orderResponse
	json.NewDecoder(rec.Body).Decode(&resp)
	if resp.Status != string(domain.OrderStatusCancelled) {
		t.Errorf("expected cancelled, got %s", resp.Status)
	}
}

func TestOrderHandler_Lifecycle_WrongUser(t *testing.T) {
	f := newOrderTestFixture(t)
	oid := f.seedOrder(t, 1)

	idStr := strconv.FormatInt(oid.Int64(), 10)
	req := userRequest(t, http.MethodPost, "/api/v1/orders/"+idStr+"/process", nil, 2)
	req = pathvar.WithVars(req, map[string]string{"id": idStr})
	rec := httptest.NewRecorder()
	f.handler.StartProcessing(rec, req)

	if rec.Code != http.StatusForbidden {
		t.Errorf("expected 403 for wrong user, got %d", rec.Code)
	}
}

func TestOrderHandler_Unauthenticated(t *testing.T) {
	f := newOrderTestFixture(t)
	req := httptest.NewRequest(http.MethodGet, "/api/v1/orders", nil)
	rec := httptest.NewRecorder()
	f.handler.ListByUser(rec, req)

	if rec.Code != http.StatusUnauthorized {
		t.Errorf("expected 401, got %d", rec.Code)
	}
}
