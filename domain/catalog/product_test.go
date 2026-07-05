package catalog

import (
	"testing"

	"github.com/beeleelee/mall/domain/kernel"
)

func validPrice() Money {
	return Money{Amount: 2999, Currency: "USD"}
}

func TestNewProduct_Success(t *testing.T) {
	p, err := NewProduct(1, "SKU-001", "Test Product", "A test product", "electronics", 0, validPrice(), nil)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if p.ID != 1 {
		t.Errorf("expected ID 1, got %d", p.ID)
	}
	if p.SKU != "SKU-001" {
		t.Errorf("expected SKU SKU-001, got %s", p.SKU)
	}
	if p.Name != "Test Product" {
		t.Errorf("expected name Test Product, got %s", p.Name)
	}
	if p.Status != ProductStatusActive {
		t.Errorf("expected status active, got %s", p.Status)
	}
	if p.Attributes == nil {
		t.Error("attributes should not be nil")
	}
}

func TestNewProduct_EmptySKU(t *testing.T) {
	_, err := NewProduct(1, "", "Product", "desc", "cat", 0, validPrice(), nil)
	if !kernel.IsInvalidArgument(err) {
		t.Errorf("expected invalid argument error, got %v", err)
	}
}

func TestNewProduct_EmptyName(t *testing.T) {
	_, err := NewProduct(1, "SKU-002", "", "desc", "cat", 0, validPrice(), nil)
	if !kernel.IsInvalidArgument(err) {
		t.Errorf("expected invalid argument error, got %v", err)
	}
}

func TestNewProduct_NegativePrice(t *testing.T) {
	_, err := NewProduct(1, "SKU-003", "Product", "desc", "cat", 0, Money{Amount: -100, Currency: "USD"}, nil)
	if !kernel.IsInvalidArgument(err) {
		t.Errorf("expected invalid argument error, got %v", err)
	}
}

func TestNewProduct_EmptyCurrency(t *testing.T) {
	_, err := NewProduct(1, "SKU-004", "Product", "desc", "cat", 0, Money{Amount: 1000, Currency: ""}, nil)
	if !kernel.IsInvalidArgument(err) {
		t.Errorf("expected invalid argument error, got %v", err)
	}
}

func TestNewProduct_EmitsCreatedEvent(t *testing.T) {
	p, err := NewProduct(1, "SKU-005", "Product", "desc", "cat", 0, validPrice(), nil)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	events := p.Events()
	if len(events) != 1 {
		t.Fatalf("expected 1 event, got %d", len(events))
	}
	ce, ok := events[0].(*ProductCreated)
	if !ok {
		t.Fatalf("expected ProductCreated event, got %T", events[0])
	}
	if ce.SKU != "SKU-005" {
		t.Errorf("expected SKU SKU-005, got %s", ce.SKU)
	}
}

func TestNewProduct_InitializesAttributes(t *testing.T) {
	attrs := map[string]any{"color": "red", "weight": "1kg"}
	p, err := NewProduct(1, "SKU-006", "Product", "desc", "cat", 0, validPrice(), attrs)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if p.Attributes["color"] != "red" {
		t.Errorf("expected color red, got %v", p.Attributes["color"])
	}
	if p.Attributes["weight"] != "1kg" {
		t.Errorf("expected weight 1kg, got %v", p.Attributes["weight"])
	}
}

func TestNewProduct_DefaultsAttributes(t *testing.T) {
	p, err := NewProduct(1, "SKU-007", "Product", "desc", "cat", 0, validPrice(), nil)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if p.Attributes == nil {
		t.Error("attributes should be initialized to empty map, not nil")
	}
}

func TestChangePrice_Success(t *testing.T) {
	p, _ := NewProduct(1, "SKU-008", "Product", "desc", "cat", 0, validPrice(), nil)
	p.ClearEvents()

	newPrice := Money{Amount: 1999, Currency: "USD"}
	err := p.ChangePrice(newPrice)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if p.Price.Amount != 1999 {
		t.Errorf("expected price 1999, got %d", p.Price.Amount)
	}

	events := p.Events()
	if len(events) != 1 {
		t.Fatalf("expected 1 event, got %d", len(events))
	}
	if _, ok := events[0].(*ProductPriceChanged); !ok {
		t.Fatalf("expected ProductPriceChanged event, got %T", events[0])
	}
}

func TestChangePrice_Negative(t *testing.T) {
	p, _ := NewProduct(1, "SKU-009", "Product", "desc", "cat", 0, validPrice(), nil)
	p.ClearEvents()

	err := p.ChangePrice(Money{Amount: -1, Currency: "USD"})
	if !kernel.IsInvalidArgument(err) {
		t.Errorf("expected invalid argument error, got %v", err)
	}
}

func TestChangePrice_EmptyCurrency(t *testing.T) {
	p, _ := NewProduct(1, "SKU-010", "Product", "desc", "cat", 0, validPrice(), nil)
	p.ClearEvents()

	err := p.ChangePrice(Money{Amount: 100, Currency: ""})
	if !kernel.IsInvalidArgument(err) {
		t.Errorf("expected invalid argument error, got %v", err)
	}
}

func TestChangeStatus_Success(t *testing.T) {
	p, _ := NewProduct(1, "SKU-011", "Product", "desc", "cat", 0, validPrice(), nil)
	p.ClearEvents()

	err := p.ChangeStatus(ProductStatusInactive)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if p.Status != ProductStatusInactive {
		t.Errorf("expected status inactive, got %s", p.Status)
	}

	events := p.Events()
	if len(events) != 1 {
		t.Fatalf("expected 1 event, got %d", len(events))
	}
	if _, ok := events[0].(*ProductStatusChanged); !ok {
		t.Fatalf("expected ProductStatusChanged event, got %T", events[0])
	}
}

func TestChangeStatus_Empty(t *testing.T) {
	p, _ := NewProduct(1, "SKU-012", "Product", "desc", "cat", 0, validPrice(), nil)
	p.ClearEvents()

	err := p.ChangeStatus("")
	if !kernel.IsInvalidArgument(err) {
		t.Errorf("expected invalid argument error, got %v", err)
	}
}

func TestUpdateDetails_Success(t *testing.T) {
	p, _ := NewProduct(1, "SKU-013", "Old Name", "Old desc", "old cat", 0, validPrice(), nil)
	p.ClearEvents()

	err := p.UpdateDetails("New Name", "New desc", "new cat", 0)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if p.Name != "New Name" {
		t.Errorf("expected name New Name, got %s", p.Name)
	}
	if p.Description != "New desc" {
		t.Errorf("expected description New desc, got %s", p.Description)
	}
	if p.Category != "new cat" {
		t.Errorf("expected category new cat, got %s", p.Category)
	}

	events := p.Events()
	if len(events) != 1 {
		t.Fatalf("expected 1 event, got %d", len(events))
	}
	if _, ok := events[0].(*ProductUpdated); !ok {
		t.Fatalf("expected ProductUpdated event, got %T", events[0])
	}
}

func TestUpdateDetails_EmptyName(t *testing.T) {
	p, _ := NewProduct(1, "SKU-014", "Name", "desc", "cat", 0, validPrice(), nil)
	p.ClearEvents()

	err := p.UpdateDetails("", "desc", "cat", 0)
	if !kernel.IsInvalidArgument(err) {
		t.Errorf("expected invalid argument error, got %v", err)
	}
}

func TestProduct_Equals(t *testing.T) {
	p1, _ := NewProduct(1, "SKU-015", "A", "desc", "cat", 0, validPrice(), nil)
	p2, _ := NewProduct(1, "SKU-016", "B", "desc", "cat", 0, validPrice(), nil)
	if !p1.Equals(p2.Entity) {
		t.Error("products with same ID should be equal")
	}
}
