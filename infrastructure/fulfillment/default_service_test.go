package fulfillment

import (
	"context"
	"testing"

	domain "github.com/beeleelee/mall/domain/fulfillment"
)

func TestDefaultFulfillmentService_DomesticRates(t *testing.T) {
	svc := NewDefaultFulfillmentService()
	input := domain.RateInput{
		DestinationCountry: "US",
		Items:              []domain.RateItem{{Weight: 1, Quantity: 1}},
	}

	result, err := svc.CalculateRates(context.Background(), input)
	if err != nil {
		t.Fatalf("CalculateRates: %v", err)
	}
	if len(result.Options) != 3 {
		t.Fatalf("expected 3 options for domestic, got %d", len(result.Options))
	}

	expected := map[string]int64{"standard": 500, "express": 1500, "overnight": 3500}
	for _, opt := range result.Options {
		if expected[opt.ID] != opt.Cost {
			t.Errorf("option %s: expected cost %d, got %d", opt.ID, expected[opt.ID], opt.Cost)
		}
	}
}

func TestDefaultFulfillmentService_InternationalRates(t *testing.T) {
	svc := NewDefaultFulfillmentService()
	input := domain.RateInput{
		DestinationCountry: "GB",
		Items:              []domain.RateItem{{Weight: 1, Quantity: 1}},
	}

	result, err := svc.CalculateRates(context.Background(), input)
	if err != nil {
		t.Fatalf("CalculateRates: %v", err)
	}
	if len(result.Options) != 2 {
		t.Fatalf("expected 2 options for international, got %d", len(result.Options))
	}

	for _, opt := range result.Options {
		if opt.Cost != 1500 && opt.Cost != 4500 {
			t.Errorf("option %s: expected 3x cost, got %d", opt.ID, opt.Cost)
		}
	}
}

func TestDefaultFulfillmentService_OptionFields(t *testing.T) {
	svc := NewDefaultFulfillmentService()
	input := domain.RateInput{
		DestinationCountry: "US",
	}

	result, err := svc.CalculateRates(context.Background(), input)
	if err != nil {
		t.Fatalf("CalculateRates: %v", err)
	}

	for _, opt := range result.Options {
		if opt.Name == "" {
			t.Errorf("option %s: missing name", opt.ID)
		}
		if opt.Carrier == "" {
			t.Errorf("option %s: missing carrier", opt.ID)
		}
		if opt.Estimated == "" {
			t.Errorf("option %s: missing estimated delivery", opt.ID)
		}
	}
}
