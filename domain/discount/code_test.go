package discount

import (
	"testing"
	"time"

	"github.com/beeleelee/mall/domain/kernel"
)

func TestNewDiscountCode(t *testing.T) {
	dc, err := NewDiscountCode(1, "SAVE10", DiscountTypeFlat, 1000, 5000, 100, time.Now().Add(24*time.Hour), false)
	if err != nil {
		t.Fatal(err)
	}
	if dc.Code != "SAVE10" {
		t.Fatalf("expected SAVE10, got %s", dc.Code)
	}
}

func TestDiscountValidation(t *testing.T) {
	dc, _ := NewDiscountCode(1, "SAVE10", DiscountTypeFlat, 1000, 5000, 100, time.Now().Add(24*time.Hour), false)

	if dc.IsValid(3000) {
		t.Fatal("should be invalid below min purchase")
	}

	if !dc.IsValid(5000) {
		t.Fatal("should be valid at min purchase")
	}

	if !dc.IsValid(10000) {
		t.Fatal("should be valid above min purchase")
	}
}

func TestApplyFlatDiscount(t *testing.T) {
	dc, _ := NewDiscountCode(1, "SAVE10", DiscountTypeFlat, 1000, 0, 100, time.Now().Add(24*time.Hour), false)

	result := dc.Apply(5000)
	if result != 4000 {
		t.Fatalf("expected 4000, got %d", result)
	}
}

func TestApplyPercentageDiscount(t *testing.T) {
	dc, _ := NewDiscountCode(1, "PCT20", DiscountTypePercentage, 20, 0, 100, time.Now().Add(24*time.Hour), false)

	result := dc.Apply(10000)
	if result != 8000 {
		t.Fatalf("expected 8000, got %d", result)
	}
}

func TestDiscountExhausted(t *testing.T) {
	dc, _ := NewDiscountCode(1, "LIMITED", DiscountTypeFlat, 1000, 0, 2, time.Now().Add(24*time.Hour), false)

	dc.Use()
	dc.Use()

	if dc.IsValid(5000) {
		t.Fatal("should be invalid after max usages")
	}
}

func TestDiscountExpired(t *testing.T) {
	dc, _ := NewDiscountCode(1, "EXPIRED", DiscountTypeFlat, 1000, 0, 100, time.Now().Add(-1*time.Hour), false)

	if dc.IsValid(5000) {
		t.Fatal("should be invalid after expiry")
	}
}

func TestDiscountInactive(t *testing.T) {
	dc, _ := NewDiscountCode(1, "OFF", DiscountTypeFlat, 1000, 0, 100, time.Now().Add(24*time.Hour), false)
	dc.Deactivate()

	if dc.IsValid(5000) {
		t.Fatal("should be invalid when inactive")
	}
}

func TestPercentageMaxValue(t *testing.T) {
	_, err := NewDiscountCode(1, "BAD", DiscountTypePercentage, 150, 0, 100, time.Now().Add(24*time.Hour), false)
	if err == nil {
		t.Fatal("expected error for value > 100")
	}
}

func TestEmptyCode(t *testing.T) {
	_, err := NewDiscountCode(1, "", DiscountTypeFlat, 1000, 0, 100, time.Now().Add(24*time.Hour), false)
	if !kernel.IsInvalidArgument(err) {
		t.Fatalf("expected invalid argument, got %v", err)
	}
}
