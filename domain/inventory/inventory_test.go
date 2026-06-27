package inventory

import (
	"testing"

	"github.com/beeleelee/mall/domain/kernel"
)

func TestNewInventoryItem_Success(t *testing.T) {
	item, err := NewInventoryItem(1, 101, 100, 10)
	if err != nil {
		t.Fatalf("NewInventoryItem failed: %v", err)
	}
	if item.ProductID != 101 {
		t.Errorf("expected product_id 101, got %d", item.ProductID)
	}
	if item.QuantityAvailable != 100 {
		t.Errorf("expected quantity 100, got %d", item.QuantityAvailable)
	}
	if item.ReservedQuantity != 0 {
		t.Errorf("expected reserved 0, got %d", item.ReservedQuantity)
	}
	if item.AvailableQuantity() != 100 {
		t.Errorf("expected available 100, got %d", item.AvailableQuantity())
	}
}

func TestNewInventoryItem_NegativeQuantity(t *testing.T) {
	_, err := NewInventoryItem(1, 101, -1, 10)
	if err == nil {
		t.Fatal("expected error for negative quantity")
	}
	if !kernel.IsInvalidArgument(err) {
		t.Errorf("expected invalid argument error, got %v", err)
	}
}

func TestNewInventoryItem_InvalidProductID(t *testing.T) {
	_, err := NewInventoryItem(1, 0, 10, 10)
	if err == nil {
		t.Fatal("expected error for invalid product id")
	}
}

func TestSetStock(t *testing.T) {
	item, _ := NewInventoryItem(1, 101, 100, 10)
	if err := item.SetStock(200); err != nil {
		t.Fatalf("SetStock failed: %v", err)
	}
	if item.QuantityAvailable != 200 {
		t.Errorf("expected quantity 200, got %d", item.QuantityAvailable)
	}
}

func TestSetStock_Negative(t *testing.T) {
	item, _ := NewInventoryItem(1, 101, 100, 10)
	if err := item.SetStock(-1); err == nil {
		t.Fatal("expected error for negative quantity")
	}
}

func TestReserve_Success(t *testing.T) {
	item, _ := NewInventoryItem(1, 101, 100, 10)
	if err := item.Reserve(30); err != nil {
		t.Fatalf("Reserve failed: %v", err)
	}
	if item.ReservedQuantity != 30 {
		t.Errorf("expected reserved 30, got %d", item.ReservedQuantity)
	}
	if item.AvailableQuantity() != 70 {
		t.Errorf("expected available 70, got %d", item.AvailableQuantity())
	}
}

func TestReserve_InsufficientStock(t *testing.T) {
	item, _ := NewInventoryItem(1, 101, 100, 10)
	_ = item.Reserve(90)
	if err := item.Reserve(20); err == nil {
		t.Fatal("expected error for insufficient stock")
	}
}

func TestReserve_InvalidQuantity(t *testing.T) {
	item, _ := NewInventoryItem(1, 101, 100, 10)
	if err := item.Reserve(0); err == nil {
		t.Fatal("expected error for zero quantity")
	}
}

func TestReleaseReservation(t *testing.T) {
	item, _ := NewInventoryItem(1, 101, 100, 10)
	_ = item.Reserve(50)
	if err := item.ReleaseReservation(20); err != nil {
		t.Fatalf("ReleaseReservation failed: %v", err)
	}
	if item.ReservedQuantity != 30 {
		t.Errorf("expected reserved 30, got %d", item.ReservedQuantity)
	}
	if item.AvailableQuantity() != 70 {
		t.Errorf("expected available 70, got %d", item.AvailableQuantity())
	}
}

func TestReleaseReservation_ExceedsReserved(t *testing.T) {
	item, _ := NewInventoryItem(1, 101, 100, 10)
	_ = item.Reserve(30)
	if err := item.ReleaseReservation(50); err == nil {
		t.Fatal("expected error for releasing more than reserved")
	}
}

func TestConfirmReservation(t *testing.T) {
	item, _ := NewInventoryItem(1, 101, 100, 10)
	_ = item.Reserve(40)
	if err := item.ConfirmReservation(40); err != nil {
		t.Fatalf("ConfirmReservation failed: %v", err)
	}
	if item.QuantityAvailable != 60 {
		t.Errorf("expected available 60, got %d", item.QuantityAvailable)
	}
	if item.ReservedQuantity != 0 {
		t.Errorf("expected reserved 0, got %d", item.ReservedQuantity)
	}
}

func TestConfirmReservation_ExceedsReserved(t *testing.T) {
	item, _ := NewInventoryItem(1, 101, 100, 10)
	_ = item.Reserve(30)
	if err := item.ConfirmReservation(50); err == nil {
		t.Fatal("expected error for confirming more than reserved")
	}
}

func TestIsLowStock(t *testing.T) {
	item, _ := NewInventoryItem(1, 101, 5, 10)
	if !item.IsLowStock() {
		t.Error("expected low stock")
	}
}

func TestIsOutOfStock(t *testing.T) {
	item, _ := NewInventoryItem(1, 101, 0, 10)
	if !item.IsOutOfStock() {
		t.Error("expected out of stock")
	}
}

func TestFullFlow(t *testing.T) {
	item, _ := NewInventoryItem(1, 101, 50, 10)

	_ = item.Reserve(10)
	_ = item.ConfirmReservation(10)
	if item.QuantityAvailable != 40 {
		t.Errorf("after confirm, expected 40, got %d", item.QuantityAvailable)
	}

	_ = item.Reserve(5)
	_ = item.ReleaseReservation(5)
	if item.ReservedQuantity != 0 {
		t.Errorf("after release, expected 0, got %d", item.ReservedQuantity)
	}

	_ = item.SetStock(3)
	if !item.IsLowStock() {
		t.Error("expected low stock after reducing to 3")
	}
}
