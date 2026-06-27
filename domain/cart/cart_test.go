package cart

import (
	"testing"

	"github.com/beeleelee/mall/domain/kernel"
)

func TestNewCart_Valid(t *testing.T) {
	c, err := NewCart(1, 42)
	if err != nil {
		t.Fatal(err)
	}
	if c.ID != 1 {
		t.Errorf("expected cart ID 1, got %d", c.ID)
	}
	if c.UserID != 42 {
		t.Errorf("expected user 42, got %d", c.UserID)
	}
	if c.Status != CartStatusActive {
		t.Errorf("expected active, got %s", c.Status)
	}
	if !c.IsEmpty() {
		t.Error("expected empty cart")
	}
}

func TestNewCart_InvalidUserID(t *testing.T) {
	_, err := NewCart(1, 0)
	if !kernel.IsInvalidArgument(err) {
		t.Errorf("expected invalid argument, got %v", err)
	}
}

func TestCart_AddItem(t *testing.T) {
	c, _ := NewCart(1, 42)
	item := CartItem{ProductID: 100, SKU: "SKU001", Name: "Product 1", Quantity: 2, UnitPrice: 1000}
	if err := c.AddItem(item); err != nil {
		t.Fatal(err)
	}
	if len(c.Items) != 1 {
		t.Errorf("expected 1 item, got %d", len(c.Items))
	}
	if c.Items[0].Quantity != 2 {
		t.Errorf("expected quantity 2, got %d", c.Items[0].Quantity)
	}
}

func TestCart_AddItem_DuplicateProduct(t *testing.T) {
	c, _ := NewCart(1, 42)
	_ = c.AddItem(CartItem{ProductID: 100, SKU: "SKU001", Name: "Product 1", Quantity: 2, UnitPrice: 1000})
	_ = c.AddItem(CartItem{ProductID: 100, SKU: "SKU001", Name: "Product 1", Quantity: 3, UnitPrice: 1000})

	if len(c.Items) != 1 {
		t.Errorf("expected 1 item, got %d", len(c.Items))
	}
	if c.Items[0].Quantity != 5 {
		t.Errorf("expected quantity 5, got %d", c.Items[0].Quantity)
	}
}

func TestCart_AddItem_InvalidProductID(t *testing.T) {
	c, _ := NewCart(1, 42)
	err := c.AddItem(CartItem{ProductID: 0, Quantity: 1, UnitPrice: 1000})
	if !kernel.IsInvalidArgument(err) {
		t.Errorf("expected invalid argument, got %v", err)
	}
}

func TestCart_AddItem_NegativeQuantity(t *testing.T) {
	c, _ := NewCart(1, 42)
	err := c.AddItem(CartItem{ProductID: 100, Quantity: 0, UnitPrice: 1000})
	if !kernel.IsInvalidArgument(err) {
		t.Errorf("expected invalid argument, got %v", err)
	}
}

func TestCart_UpdateQuantity(t *testing.T) {
	c, _ := NewCart(1, 42)
	_ = c.AddItem(CartItem{ProductID: 100, SKU: "SKU001", Name: "P", Quantity: 2, UnitPrice: 1000})

	if err := c.UpdateQuantity(100, 5); err != nil {
		t.Fatal(err)
	}
	if c.Items[0].Quantity != 5 {
		t.Errorf("expected quantity 5, got %d", c.Items[0].Quantity)
	}
}

func TestCart_UpdateQuantity_RemoveWhenZero(t *testing.T) {
	c, _ := NewCart(1, 42)
	_ = c.AddItem(CartItem{ProductID: 100, SKU: "SKU001", Name: "P", Quantity: 2, UnitPrice: 1000})

	if err := c.UpdateQuantity(100, 0); err != nil {
		t.Fatal(err)
	}
	if len(c.Items) != 0 {
		t.Errorf("expected 0 items, got %d", len(c.Items))
	}
}

func TestCart_UpdateQuantity_NotFound(t *testing.T) {
	c, _ := NewCart(1, 42)
	err := c.UpdateQuantity(999, 3)
	if !kernel.IsNotFound(err) {
		t.Errorf("expected not found, got %v", err)
	}
}

func TestCart_RemoveItem(t *testing.T) {
	c, _ := NewCart(1, 42)
	_ = c.AddItem(CartItem{ProductID: 100, SKU: "SKU001", Name: "P", Quantity: 2, UnitPrice: 1000})
	_ = c.AddItem(CartItem{ProductID: 101, SKU: "SKU002", Name: "Q", Quantity: 1, UnitPrice: 2000})

	if err := c.RemoveItem(100); err != nil {
		t.Fatal(err)
	}
	if len(c.Items) != 1 {
		t.Errorf("expected 1 item, got %d", len(c.Items))
	}
	if c.Items[0].ProductID != 101 {
		t.Errorf("expected product 101, got %d", c.Items[0].ProductID)
	}
}

func TestCart_RemoveItem_NotFound(t *testing.T) {
	c, _ := NewCart(1, 42)
	err := c.RemoveItem(999)
	if !kernel.IsNotFound(err) {
		t.Errorf("expected not found, got %v", err)
	}
}

func TestCart_Clear(t *testing.T) {
	c, _ := NewCart(1, 42)
	_ = c.AddItem(CartItem{ProductID: 100, Quantity: 2, UnitPrice: 1000})
	_ = c.AddItem(CartItem{ProductID: 101, Quantity: 1, UnitPrice: 2000})

	c.Clear()
	if !c.IsEmpty() {
		t.Error("expected empty cart after clear")
	}
}

func TestCart_GetTotal(t *testing.T) {
	c, _ := NewCart(1, 42)
	_ = c.AddItem(CartItem{ProductID: 100, SKU: "A", Name: "A", Quantity: 2, UnitPrice: 1000})
	_ = c.AddItem(CartItem{ProductID: 101, SKU: "B", Name: "B", Quantity: 3, UnitPrice: 500})

	total := c.GetTotal()
	if total.Subtotal != 3500 {
		t.Errorf("expected subtotal 3500, got %d", total.Subtotal)
	}
	if total.ItemCount != 2 {
		t.Errorf("expected 2 items, got %d", total.ItemCount)
	}
}

func TestCart_ItemTotalPrice(t *testing.T) {
	item := CartItem{Quantity: 3, UnitPrice: 1500}
	if item.TotalPrice() != 4500 {
		t.Errorf("expected 4500, got %d", item.TotalPrice())
	}
}

func TestCart_Merge(t *testing.T) {
	c1, _ := NewCart(1, 42)
	_ = c1.AddItem(CartItem{ProductID: 100, SKU: "A", Name: "A", Quantity: 2, UnitPrice: 1000})

	c2, _ := NewCart(2, 42)
	_ = c2.AddItem(CartItem{ProductID: 101, SKU: "B", Name: "B", Quantity: 1, UnitPrice: 2000})

	if err := c1.Merge(c2); err != nil {
		t.Fatal(err)
	}
	if len(c1.Items) != 2 {
		t.Errorf("expected 2 items, got %d", len(c1.Items))
	}
	if c2.Status != CartStatusMerged {
		t.Errorf("expected source cart status merged, got %s", c2.Status)
	}
}

func TestCart_Merge_DifferentUser(t *testing.T) {
	c1, _ := NewCart(1, 42)
	c2, _ := NewCart(2, 99)
	err := c1.Merge(c2)
	if !kernel.IsInvalidArgument(err) {
		t.Errorf("expected invalid argument, got %v", err)
	}
}
