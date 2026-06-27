package cart

import (
	"context"
	"testing"

	"github.com/beeleelee/mall/domain/kernel"
)

func newTestService() *CartService {
	return NewCartService(newFakeCartRepo(), newFakeCartPublisher(), fakeLoggerCart{})
}

func TestCartService_GetOrCreateCart_New(t *testing.T) {
	svc := newTestService()
	ctx := context.Background()

	cart, err := svc.GetOrCreateCart(ctx, 1, 42)
	if err != nil {
		t.Fatal(err)
	}
	if cart.ID != 1 {
		t.Errorf("expected cart ID 1, got %d", cart.ID)
	}
	if cart.UserID != 42 {
		t.Errorf("expected user 42, got %d", cart.UserID)
	}
}

func TestCartService_GetOrCreateCart_Existing(t *testing.T) {
	svc := newTestService()
	ctx := context.Background()

	cart1, _ := svc.GetOrCreateCart(ctx, 1, 42)
	cart2, err := svc.GetOrCreateCart(ctx, 2, 42)
	if err != nil {
		t.Fatal(err)
	}
	if cart2.ID != cart1.ID {
		t.Errorf("expected same cart %d, got %d", cart1.ID, cart2.ID)
	}
}

func TestCartService_AddItem(t *testing.T) {
	svc := newTestService()
	ctx := context.Background()

	cart, err := svc.AddItem(ctx, AddItemInput{
		CartID:    1,
		UserID:    42,
		ProductID: 100,
		SKU:       "SKU001",
		Name:      "Product 1",
		Quantity:  2,
		UnitPrice: 1000,
	})
	if err != nil {
		t.Fatal(err)
	}
	if len(cart.Items) != 1 {
		t.Errorf("expected 1 item, got %d", len(cart.Items))
	}
}

func TestCartService_UpdateQuantity(t *testing.T) {
	svc := newTestService()
	ctx := context.Background()

	_, _ = svc.AddItem(ctx, AddItemInput{CartID: 1, UserID: 42, ProductID: 100, Quantity: 2, UnitPrice: 1000})

	cart, err := svc.UpdateQuantity(ctx, 42, 100, 5)
	if err != nil {
		t.Fatal(err)
	}
	if cart.Items[0].Quantity != 5 {
		t.Errorf("expected quantity 5, got %d", cart.Items[0].Quantity)
	}
}

func TestCartService_RemoveItem(t *testing.T) {
	svc := newTestService()
	ctx := context.Background()

	_, _ = svc.AddItem(ctx, AddItemInput{CartID: 1, UserID: 42, ProductID: 100, Quantity: 2, UnitPrice: 1000})

	cart, err := svc.RemoveItem(ctx, 42, 100)
	if err != nil {
		t.Fatal(err)
	}
	if !cart.IsEmpty() {
		t.Error("expected empty cart")
	}
}

func TestCartService_ClearCart(t *testing.T) {
	svc := newTestService()
	ctx := context.Background()

	_, _ = svc.AddItem(ctx, AddItemInput{CartID: 1, UserID: 42, ProductID: 100, Quantity: 2, UnitPrice: 1000})

	cart, err := svc.ClearCart(ctx, 42)
	if err != nil {
		t.Fatal(err)
	}
	if !cart.IsEmpty() {
		t.Error("expected empty cart")
	}
}

func TestCartService_GetCart_NotFound(t *testing.T) {
	svc := newTestService()
	_, err := svc.GetCart(context.Background(), 999)
	if !kernel.IsNotFound(err) {
		t.Errorf("expected not found, got %v", err)
	}
}

func TestCartService_MergeCarts(t *testing.T) {
	ctx := context.Background()

	repo := newFakeCartRepo()
	pub := newFakeCartPublisher()
	logger := fakeLoggerCart{}

	cart1, _ := NewCart(1, 42)
	_ = cart1.AddItem(CartItem{ProductID: 100, SKU: "A", Name: "A", Quantity: 2, UnitPrice: 1000})
	_ = repo.Save(ctx, cart1)

	cart2, _ := NewCart(2, 42)
	_ = cart2.AddItem(CartItem{ProductID: 101, SKU: "B", Name: "B", Quantity: 1, UnitPrice: 2000})
	_ = repo.Save(ctx, cart2)

	svc := NewCartService(repo, pub, logger)
	cart, err := svc.MergeCarts(ctx, 1, 2)
	if err != nil {
		t.Fatal(err)
	}
	if len(cart.Items) != 2 {
		t.Errorf("expected 2 items after merge, got %d", len(cart.Items))
	}
}

func TestCartService_EventsPublished(t *testing.T) {
	pub := newFakeCartPublisher()
	svc := NewCartService(newFakeCartRepo(), pub, fakeLoggerCart{})
	ctx := context.Background()

	_, _ = svc.AddItem(ctx, AddItemInput{CartID: 1, UserID: 42, ProductID: 100, Quantity: 2, UnitPrice: 1000})
	_, _ = svc.AddItem(ctx, AddItemInput{CartID: 1, UserID: 42, ProductID: 101, Quantity: 1, UnitPrice: 2000})
	_, _ = svc.RemoveItem(ctx, 42, 100)
	_, _ = svc.ClearCart(ctx, 42)

	pub.mu.Lock()
	count := len(pub.published)
	pub.mu.Unlock()

	if count != 4 {
		t.Errorf("expected 4 published events, got %d", count)
	}
}
