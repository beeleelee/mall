package inventory

import (
	"context"
	"sync"
	"testing"

	"github.com/beeleelee/mall/domain/kernel"
)

type fakeInventoryRepo struct {
	mu        sync.Mutex
	items     map[kernel.ID]*InventoryItem
	byProduct map[kernel.ID]kernel.ID
}

func newFakeInventoryRepo() *fakeInventoryRepo {
	return &fakeInventoryRepo{
		items:     make(map[kernel.ID]*InventoryItem),
		byProduct: make(map[kernel.ID]kernel.ID),
	}
}

func (f *fakeInventoryRepo) Save(_ context.Context, item *InventoryItem) error {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.items[item.ID] = item
	f.byProduct[item.ProductID] = item.ID
	return nil
}

func (f *fakeInventoryRepo) FindByProductID(_ context.Context, productID kernel.ID) (*InventoryItem, error) {
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

func (f *fakeInventoryRepo) FindAll(_ context.Context, offset, limit int) ([]*InventoryItem, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	result := make([]*InventoryItem, 0, len(f.items))
	for _, item := range f.items {
		result = append(result, item)
	}
	if offset >= len(result) {
		return []*InventoryItem{}, nil
	}
	end := offset + limit
	if end > len(result) {
		end = len(result)
	}
	return result[offset:end], nil
}

func (f *fakeInventoryRepo) FindLowStock(_ context.Context, threshold int) ([]*InventoryItem, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	var result []*InventoryItem
	for _, item := range f.items {
		if item.QuantityAvailable <= threshold {
			result = append(result, item)
		}
	}
	return result, nil
}

func (f *fakeInventoryRepo) Delete(_ context.Context, id kernel.ID) error {
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

type fakeLogger struct{}

func (fakeLogger) Debug(_ context.Context, _ string, _ ...kernel.LogField)          {}
func (fakeLogger) Info(_ context.Context, _ string, _ ...kernel.LogField)           {}
func (fakeLogger) Warn(_ context.Context, _ string, _ ...kernel.LogField)           {}
func (fakeLogger) Error(_ context.Context, _ string, _ error, _ ...kernel.LogField) {}

func TestInventoryService_SetStock(t *testing.T) {
	repo := newFakeInventoryRepo()
	logger := fakeLogger{}
	svc := NewInventoryService(repo, logger)

	item, err := svc.SetStock(context.Background(), 1, 101, 100, 10)
	if err != nil {
		t.Fatalf("SetStock failed: %v", err)
	}
	if item.ProductID != 101 {
		t.Errorf("expected product_id 101, got %d", item.ProductID)
	}
	if item.QuantityAvailable != 100 {
		t.Errorf("expected quantity 100, got %d", item.QuantityAvailable)
	}
}

func TestInventoryService_UpdateStock(t *testing.T) {
	repo := newFakeInventoryRepo()
	logger := fakeLogger{}
	svc := NewInventoryService(repo, logger)

	_ = svc.SetStock(context.Background(), 1, 101, 100, 10)
	item, err := svc.UpdateStock(context.Background(), 101, 200)
	if err != nil {
		t.Fatalf("UpdateStock failed: %v", err)
	}
	if item.QuantityAvailable != 200 {
		t.Errorf("expected quantity 200, got %d", item.QuantityAvailable)
	}
}

func TestInventoryService_Reserve(t *testing.T) {
	repo := newFakeInventoryRepo()
	logger := fakeLogger{}
	svc := NewInventoryService(repo, logger)

	_ = svc.SetStock(context.Background(), 1, 101, 100, 10)
	item, err := svc.Reserve(context.Background(), 101, 30)
	if err != nil {
		t.Fatalf("Reserve failed: %v", err)
	}
	if item.ReservedQuantity != 30 {
		t.Errorf("expected reserved 30, got %d", item.ReservedQuantity)
	}
}

func TestInventoryService_Reserve_Insufficient(t *testing.T) {
	repo := newFakeInventoryRepo()
	logger := fakeLogger{}
	svc := NewInventoryService(repo, logger)

	_ = svc.SetStock(context.Background(), 1, 101, 10, 10)
	_, err := svc.Reserve(context.Background(), 101, 30)
	if err == nil {
		t.Fatal("expected error for insufficient stock")
	}
}

func TestInventoryService_GetStock(t *testing.T) {
	repo := newFakeInventoryRepo()
	logger := fakeLogger{}
	svc := NewInventoryService(repo, logger)

	_ = svc.SetStock(context.Background(), 1, 101, 50, 10)
	item, err := svc.GetStock(context.Background(), 101)
	if err != nil {
		t.Fatalf("GetStock failed: %v", err)
	}
	if item.QuantityAvailable != 50 {
		t.Errorf("expected 50, got %d", item.QuantityAvailable)
	}
}

func TestInventoryService_GetStock_NotFound(t *testing.T) {
	repo := newFakeInventoryRepo()
	logger := fakeLogger{}
	svc := NewInventoryService(repo, logger)

	_, err := svc.GetStock(context.Background(), 999)
	if err == nil {
		t.Fatal("expected error for not found")
	}
}

func TestInventoryService_ListLowStock(t *testing.T) {
	repo := newFakeInventoryRepo()
	logger := fakeLogger{}
	svc := NewInventoryService(repo, logger)

	_ = svc.SetStock(context.Background(), 1, 101, 5, 10)
	_ = svc.SetStock(context.Background(), 2, 102, 20, 10)
	_ = svc.SetStock(context.Background(), 3, 103, 50, 10)

	items, err := svc.ListLowStock(context.Background(), 10)
	if err != nil {
		t.Fatalf("ListLowStock failed: %v", err)
	}
	if len(items) != 1 {
		t.Errorf("expected 1 low stock item, got %d", len(items))
	}
	if items[0].ProductID != 101 {
		t.Errorf("expected product 101, got %d", items[0].ProductID)
	}
}

func TestInventoryService_ReleaseAndConfirm(t *testing.T) {
	repo := newFakeInventoryRepo()
	logger := fakeLogger{}
	svc := NewInventoryService(repo, logger)

	_ = svc.SetStock(context.Background(), 1, 101, 100, 10)
	_ = svc.Reserve(context.Background(), 101, 40)
	_ = svc.ReleaseReservation(context.Background(), 101, 10)

	item, _ := svc.GetStock(context.Background(), 101)
	if item.ReservedQuantity != 30 {
		t.Errorf("after partial release, expected reserved 30, got %d", item.ReservedQuantity)
	}

	_ = svc.ConfirmReservation(context.Background(), 101, 30)
	item, _ = svc.GetStock(context.Background(), 101)
	if item.QuantityAvailable != 70 {
		t.Errorf("after confirm, expected available 70, got %d", item.QuantityAvailable)
	}
	if item.ReservedQuantity != 0 {
		t.Errorf("after confirm, expected reserved 0, got %d", item.ReservedQuantity)
	}
}
