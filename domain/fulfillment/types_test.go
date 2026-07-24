package fulfillment

import (
	"testing"
)

func TestShippingOptionConstruction(t *testing.T) {
	opt := ShippingOption{
		ID:        "standard",
		Name:      "Standard Shipping",
		Cost:      500,
		Estimated: "3-5 business days",
		Carrier:   "USPS",
	}
	if opt.ID != "standard" {
		t.Errorf("ID = %q", opt.ID)
	}
	if opt.Cost != 500 {
		t.Errorf("Cost = %d", opt.Cost)
	}
	if opt.Estimated != "3-5 business days" {
		t.Errorf("Estimated = %q", opt.Estimated)
	}
	if opt.Carrier != "USPS" {
		t.Errorf("Carrier = %q", opt.Carrier)
	}
}

func TestShippingOption_ZeroCost(t *testing.T) {
	opt := ShippingOption{ID: "free", Name: "Free Shipping", Cost: 0}
	if opt.Cost != 0 {
		t.Errorf("expected zero cost")
	}
}

func TestShippingOption_OptionalFields(t *testing.T) {
	opt := ShippingOption{ID: "pickup", Name: "Store Pickup", Cost: 0}
	if opt.Estimated != "" {
		t.Errorf("expected empty Estimated, got %q", opt.Estimated)
	}
	if opt.Carrier != "" {
		t.Errorf("expected empty Carrier, got %q", opt.Carrier)
	}
}

func TestRateInputConstruction(t *testing.T) {
	input := RateInput{
		DestinationCountry: "US",
		DestinationState:   "OR",
		DestinationCity:    "Portland",
		Items: []RateItem{
			{Weight: 1.5, Quantity: 2},
			{Weight: 0.5, Quantity: 1},
		},
	}
	if input.DestinationCountry != "US" {
		t.Errorf("DestinationCountry = %q", input.DestinationCountry)
	}
	if len(input.Items) != 2 {
		t.Errorf("expected 2 items, got %d", len(input.Items))
	}
}

func TestRateInput_EmptyItems(t *testing.T) {
	input := RateInput{
		DestinationCountry: "US",
		Items:              []RateItem{},
	}
	if len(input.Items) != 0 {
		t.Errorf("expected 0 items, got %d", len(input.Items))
	}
}

func TestRateInput_NilItems(t *testing.T) {
	input := RateInput{
		DestinationCountry: "US",
	}
	if input.Items != nil {
		t.Errorf("expected nil items")
	}
}

func TestRateItemConstruction(t *testing.T) {
	item := RateItem{Weight: 2.0, Quantity: 3}
	if item.Weight != 2.0 {
		t.Errorf("Weight = %f", item.Weight)
	}
	if item.Quantity != 3 {
		t.Errorf("Quantity = %d", item.Quantity)
	}
}

func TestRateItem_ZeroValues(t *testing.T) {
	item := RateItem{}
	if item.Weight != 0 {
		t.Errorf("expected zero weight")
	}
	if item.Quantity != 0 {
		t.Errorf("expected zero quantity")
	}
}

func TestRateResultConstruction(t *testing.T) {
	result := RateResult{
		Options: []ShippingOption{
			{ID: "standard", Name: "Standard", Cost: 500},
			{ID: "express", Name: "Express", Cost: 1500},
		},
	}
	if len(result.Options) != 2 {
		t.Errorf("expected 2 options, got %d", len(result.Options))
	}
}

func TestRateResult_EmptyOptions(t *testing.T) {
	result := RateResult{Options: []ShippingOption{}}
	if len(result.Options) != 0 {
		t.Errorf("expected 0 options")
	}
}

func TestRateResult_NilOptions(t *testing.T) {
	result := RateResult{}
	if result.Options != nil {
		t.Errorf("expected nil options")
	}
}
