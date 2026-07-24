package rest

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	domain "github.com/beeleelee/mall/domain/fulfillment"
)

type fakeRateCalculator struct{}

func (fakeRateCalculator) CalculateRates(_ context.Context, input domain.RateInput) (*domain.RateResult, error) {
	return &domain.RateResult{
		Options: []domain.ShippingOption{
			{ID: "std", Name: "Standard", Cost: 500, Estimated: "3-5 days", Carrier: "UPS"},
			{ID: "exp", Name: "Express", Cost: 1500, Estimated: "1-2 days", Carrier: "UPS"},
		},
	}, nil
}

func TestFulfillmentHandler_CalculateRates_Success(t *testing.T) {
	h := NewFulfillmentHandler(fakeRateCalculator{})
	body, _ := json.Marshal(map[string]any{
		"destination_country": "US",
		"destination_state":   "CA",
		"destination_city":    "Los Angeles",
		"items": []map[string]any{
			{"weight": 1.5, "quantity": 2},
			{"weight": 0.5, "quantity": 1},
		},
	})
	req := httptest.NewRequest(http.MethodPost, "/api/v1/fulfillment/rates", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()
	h.CalculateRates(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", rec.Code, rec.Body.String())
	}
	var resp map[string]any
	if err := json.NewDecoder(rec.Body).Decode(&resp); err != nil {
		t.Fatalf("decode: %v", err)
	}
	options := resp["Options"].([]any)
	if len(options) != 2 {
		t.Errorf("expected 2 options, got %d", len(options))
	}
}

func TestFulfillmentHandler_CalculateRates_InvalidBody(t *testing.T) {
	h := NewFulfillmentHandler(fakeRateCalculator{})
	req := httptest.NewRequest(http.MethodPost, "/api/v1/fulfillment/rates", bytes.NewReader([]byte("not json")))
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()
	h.CalculateRates(rec, req)

	if rec.Code != http.StatusBadRequest {
		t.Errorf("expected 400, got %d", rec.Code)
	}
}

func TestFulfillmentHandler_CalculateRates_EmptyItems(t *testing.T) {
	h := NewFulfillmentHandler(fakeRateCalculator{})
	body, _ := json.Marshal(map[string]any{
		"destination_country": "US",
		"destination_state":   "NY",
		"destination_city":    "New York",
		"items":               []map[string]any{},
	})
	req := httptest.NewRequest(http.MethodPost, "/api/v1/fulfillment/rates", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()
	h.CalculateRates(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", rec.Code)
	}
}
