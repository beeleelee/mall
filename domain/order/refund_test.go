package order

import (
	"context"
	"sync"
	"testing"

	"github.com/beeleelee/mall/domain/inventory"
	"github.com/beeleelee/mall/domain/kernel"
	"github.com/beeleelee/mall/domain/payment"
)

type testFakeOrderPub struct{}

func (testFakeOrderPub) PublishOrderEvent(_ context.Context, _ *Order) error { return nil }

type testFakeLog struct{}

func (testFakeLog) Debug(_ context.Context, _ string, _ ...kernel.LogField)          {}
func (testFakeLog) Info(_ context.Context, _ string, _ ...kernel.LogField)           {}
func (testFakeLog) Warn(_ context.Context, _ string, _ ...kernel.LogField)           {}
func (testFakeLog) Error(_ context.Context, _ string, _ error, _ ...kernel.LogField) {}

type testFakeInventoryRepo struct {
	mu        sync.Mutex
	items     map[kernel.ID]*inventory.InventoryItem
	byProduct map[kernel.ID]kernel.ID
}

func newTestFakeInventoryRepo() *testFakeInventoryRepo {
	return &testFakeInventoryRepo{
		items:     make(map[kernel.ID]*inventory.InventoryItem),
		byProduct: make(map[kernel.ID]kernel.ID),
	}
}

func (f *testFakeInventoryRepo) Save(_ context.Context, item *inventory.InventoryItem) error {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.items[item.ID] = item
	f.byProduct[item.ProductID] = item.ID
	return nil
}

func (f *testFakeInventoryRepo) FindByProductID(_ context.Context, productID kernel.ID) (*inventory.InventoryItem, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	id, ok := f.byProduct[productID]
	if !ok {
		return nil, kernel.NewDomainError(kernel.ErrNotFound, "inventory not found")
	}
	item, ok := f.items[id]
	if !ok {
		return nil, kernel.NewDomainError(kernel.ErrNotFound, "inventory not found")
	}
	return item, nil
}

func (f *testFakeInventoryRepo) FindAll(_ context.Context, offset, limit int) ([]*inventory.InventoryItem, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	result := make([]*inventory.InventoryItem, 0, len(f.items))
	for _, item := range f.items {
		result = append(result, item)
	}
	if offset >= len(result) {
		return []*inventory.InventoryItem{}, nil
	}
	end := offset + limit
	if end > len(result) {
		end = len(result)
	}
	return result[offset:end], nil
}

func (f *testFakeInventoryRepo) FindLowStock(_ context.Context, threshold int) ([]*inventory.InventoryItem, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	var result []*inventory.InventoryItem
	for _, item := range f.items {
		if item.QuantityAvailable <= threshold {
			result = append(result, item)
		}
	}
	return result, nil
}

func (f *testFakeInventoryRepo) Delete(_ context.Context, id kernel.ID) error {
	f.mu.Lock()
	defer f.mu.Unlock()
	item, ok := f.items[id]
	if !ok {
		return kernel.NewDomainError(kernel.ErrNotFound, "inventory not found")
	}
	delete(f.byProduct, item.ProductID)
	delete(f.items, id)
	return nil
}

type testFakeMandateRepo struct {
	mandates map[kernel.ID]*payment.Mandate
}

func newTestFakeMandateRepo() *testFakeMandateRepo {
	return &testFakeMandateRepo{mandates: make(map[kernel.ID]*payment.Mandate)}
}

func (r *testFakeMandateRepo) Save(_ context.Context, m *payment.Mandate) error {
	r.mandates[m.ID] = m
	return nil
}

func (r *testFakeMandateRepo) FindByID(_ context.Context, id kernel.ID) (*payment.Mandate, error) {
	m, ok := r.mandates[id]
	if !ok {
		return nil, kernel.NewDomainError(kernel.ErrNotFound, "mandate not found")
	}
	return m, nil
}

func (r *testFakeMandateRepo) FindByUserID(_ context.Context, userID kernel.ID) ([]*payment.Mandate, error) {
	var result []*payment.Mandate
	for _, m := range r.mandates {
		if m.UserID == userID {
			result = append(result, m)
		}
	}
	return result, nil
}

func (r *testFakeMandateRepo) FindActiveByUser(_ context.Context, userID kernel.ID) ([]*payment.Mandate, error) {
	var result []*payment.Mandate
	for _, m := range r.mandates {
		if m.UserID == userID && m.IsActive() {
			result = append(result, m)
		}
	}
	return result, nil
}

func (r *testFakeMandateRepo) Delete(_ context.Context, id kernel.ID) error {
	delete(r.mandates, id)
	return nil
}

type fakeRefundRepo struct {
	mu      sync.Mutex
	refunds map[kernel.ID]*Refund
}

func newFakeRefundRepo() *fakeRefundRepo {
	return &fakeRefundRepo{refunds: make(map[kernel.ID]*Refund)}
}

func (r *fakeRefundRepo) Save(_ context.Context, refund *Refund) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.refunds[refund.ID] = refund
	return nil
}

func (r *fakeRefundRepo) FindByID(_ context.Context, id kernel.ID) (*Refund, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	refund, ok := r.refunds[id]
	if !ok {
		return nil, kernel.NewDomainError(kernel.ErrNotFound, "refund not found")
	}
	return refund, nil
}

func (r *fakeRefundRepo) FindByOrderID(_ context.Context, orderID kernel.ID) ([]*Refund, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	var result []*Refund
	for _, refund := range r.refunds {
		if refund.OrderID == orderID {
			result = append(result, refund)
		}
	}
	return result, nil
}

type refundTestFixture struct {
	refundSvc    *RefundService
	refundRepo   *fakeRefundRepo
	orderRepo    *fakeOrderRepo
	inventorySvc *inventory.InventoryService
	paymentSvc   *payment.PaymentService
	sf           *kernel.Snowflake
}

func newRefundTestFixture(t *testing.T) *refundTestFixture {
	t.Helper()

	refundRepo := newFakeRefundRepo()
	orderRepo := newFakeOrderRepo()
	orderPub := testFakeOrderPub{}
	logger := testFakeLog{}
	orderSvc := NewOrderService(orderRepo, orderPub, logger)

	inventoryRepo := newTestFakeInventoryRepo()
	inventorySvc := inventory.NewInventoryService(inventoryRepo, logger)

	paymentRepo := newTestFakeMandateRepo()
	paymentSvc := payment.NewPaymentService(paymentRepo, logger)

	refundSvc := NewRefundService(refundRepo, paymentSvc, inventorySvc, orderSvc, logger)

	sf, err := kernel.NewSnowflake(1)
	if err != nil {
		t.Fatalf("NewSnowflake: %v", err)
	}

	return &refundTestFixture{
		refundSvc:    refundSvc,
		refundRepo:   refundRepo,
		orderRepo:    orderRepo,
		inventorySvc: inventorySvc,
		paymentSvc:   paymentSvc,
		sf:           sf,
	}
}

func TestNewRefund_Success(t *testing.T) {
	r, err := NewRefund(1, 10, 500, 2500, "buyer requested return")
	if err != nil {
		t.Fatalf("NewRefund: %v", err)
	}
	if r.Status != RefundStatusPending {
		t.Errorf("expected pending, got %s", r.Status)
	}
	if r.Amount != 2500 {
		t.Errorf("expected amount 2500, got %d", r.Amount)
	}
	if r.OrderID != 10 {
		t.Errorf("expected order id 10, got %d", r.OrderID)
	}
}

func TestNewRefund_InvalidArgs(t *testing.T) {
	_, err := NewRefund(1, 0, 500, 2500, "reason")
	if err == nil {
		t.Fatal("expected error for zero order id")
	}

	_, err = NewRefund(1, 10, 500, 0, "reason")
	if err == nil {
		t.Fatal("expected error for zero amount")
	}

	_, err = NewRefund(1, 10, 500, -1, "reason")
	if err == nil {
		t.Fatal("expected error for negative amount")
	}
}

func TestRefund_MarkProcessed(t *testing.T) {
	r, _ := NewRefund(1, 10, 500, 2500, "return")
	if err := r.MarkProcessed(); err != nil {
		t.Fatalf("MarkProcessed: %v", err)
	}
	if r.Status != RefundStatusProcessed {
		t.Errorf("expected processed, got %s", r.Status)
	}
	if r.ProcessedAt == nil {
		t.Error("expected ProcessedAt to be set")
	}
}

func TestRefund_MarkFailed(t *testing.T) {
	r, _ := NewRefund(1, 10, 500, 2500, "return")
	if err := r.MarkFailed("payment gateway error"); err != nil {
		t.Fatalf("MarkFailed: %v", err)
	}
	if r.Status != RefundStatusFailed {
		t.Errorf("expected failed, got %s", r.Status)
	}
	if r.FailedAt == nil {
		t.Error("expected FailedAt to be set")
	}
	if r.FailureReason != "payment gateway error" {
		t.Errorf("expected reason 'payment gateway error', got %s", r.FailureReason)
	}
}

func TestRefund_MarkFailed_NoReason(t *testing.T) {
	r, _ := NewRefund(1, 10, 500, 2500, "return")
	if err := r.MarkFailed(""); err == nil {
		t.Fatal("expected error for empty reason")
	}
}

func TestRefund_MarkProcessed_AlreadyProcessed(t *testing.T) {
	r, _ := NewRefund(1, 10, 500, 2500, "return")
	r.MarkProcessed()
	if err := r.MarkProcessed(); err == nil {
		t.Fatal("expected error for already processed")
	}
	if err := r.MarkFailed("error"); err == nil {
		t.Fatal("expected error for non-pending refund")
	}
}
