package rest

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	domain "github.com/beeleelee/mall/domain/checkout"
	"github.com/beeleelee/mall/domain/kernel"
)

func TestStripeWebhookHandler_UnknownEvent(t *testing.T) {
	h := newTestStripeWebhookHandler(t)

	body := map[string]any{
		"id":   "evt_unknown",
		"type": "charge.succeeded",
		"data": map[string]any{
			"object": map[string]any{"id": "ch_123"},
		},
	}
	raw, _ := json.Marshal(body)
	req := httptest.NewRequest(http.MethodPost, "/api/v1/stripe/webhook", bytes.NewReader(raw))
	rec := httptest.NewRecorder()
	h.HandleWebhook(rec, req)

	if rec.Code != http.StatusOK {
		t.Errorf("expected 200 for unknown event, got %d: %s", rec.Code, rec.Body.String())
	}
}

func TestStripeWebhookHandler_CheckoutSessionCompleted(t *testing.T) {
	svc := newTestCheckoutSvcWithStripe(t)
	h := NewStripeWebhookHandler(svc, "whsec_test", WithSkipVerification())

	checkoutID := setupCheckoutForSvcTest(t, svc)

	_, escalated, err := svc.StartComplete(context.Background(), checkoutID, "")
	if err != nil {
		t.Fatal(err)
	}
	if !escalated {
		t.Fatal("expected escalation")
	}

	body := map[string]any{
		"id":   "evt_test",
		"type": "checkout.session.completed",
		"data": map[string]any{
			"object": map[string]any{
				"id":                  "cs_test_123",
				"status":              "complete",
				"client_reference_id": checkoutID.String(),
				"payment_intent":      "pi_test",
				"mode":                "payment",
			},
		},
	}
	raw, _ := json.Marshal(body)
	req := httptest.NewRequest(http.MethodPost, "/api/v1/stripe/webhook", bytes.NewReader(raw))
	rec := httptest.NewRecorder()
	h.HandleWebhook(rec, req)

	if rec.Code != http.StatusOK {
		t.Errorf("expected 200, got %d: %s", rec.Code, rec.Body.String())
	}

	var resp map[string]string
	json.NewDecoder(rec.Body).Decode(&resp)
	if resp["status"] != "checkout_completed" {
		t.Errorf("expected checkout_completed, got %s", resp["status"])
	}
}

func newTestStripeWebhookHandler(t *testing.T) *StripeWebhookHandler {
	t.Helper()
	repo := newFakeCheckoutRepo()
	pub := fakeCheckoutPub{}
	logger := fakeLog{}
	taxSvc := fakeTaxService{}
	priceCalc := fakePriceCalculator{}
	svc := domain.NewCheckoutService(repo, taxSvc, priceCalc, pub, logger, nil, nil)
	return NewStripeWebhookHandler(svc, "whsec_test", WithSkipVerification())
}

func newTestCheckoutSvcWithStripe(t *testing.T) *domain.CheckoutService {
	t.Helper()
	repo := newFakeCheckoutRepo()
	pub := fakeCheckoutPub{}
	logger := fakeLog{}
	taxSvc := fakeTaxService{}
	priceCalc := fakePriceCalculator{}
	return domain.NewCheckoutService(repo, taxSvc, priceCalc, pub, logger, nil, &fakeStripeProcessor{
		createCheckoutSessionFn: func(_ context.Context, _ *domain.CheckoutSession) (string, string, error) {
			return "https://checkout.stripe.com/cs_test_123", "cs_test_123", nil
		},
		createPaymentIntentFn: func(_ context.Context, _ kernel.ID, _ int64) (string, string, error) {
			return "secret_pi_1", "pi_1", nil
		},
		getPaymentIntentStatusFn: func(_ context.Context, _ string) (string, error) {
			return "succeeded", nil
		},
	})
}

func setupCheckoutForSvcTest(t *testing.T, svc *domain.CheckoutService) kernel.ID {
	t.Helper()

	session, err := svc.CreateCheckout(context.Background(), domain.CreateCheckoutInput{
		CheckoutID: 99,
		UserID:     42,
		CartID:     10,
		CartItems: []domain.CartSnapshotItem{
			{ProductID: 1, SKU: "SKU001", Name: "Test", Quantity: 1, UnitPrice: 1000},
		},
	})
	if err != nil {
		t.Fatal(err)
	}

	addr := domain.Address{Line1: "123 Main", City: "Portland", State: "OR", PostalCode: "97201", Country: "US"}
	svc.SetShippingAddress(context.Background(), session.ID, addr)
	svc.SetBillingAddress(context.Background(), session.ID, addr)
	svc.SelectShippingOption(context.Background(), session.ID, domain.ShippingOption{ID: "std", Name: "Standard", Cost: 500})
	svc.SelectPaymentHandler(context.Background(), session.ID, "stripe")

	return session.ID
}
