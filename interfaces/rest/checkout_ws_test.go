package rest

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	domain "github.com/beeleelee/mall/domain/checkout"
	"github.com/beeleelee/mall/domain/ecp"
	"github.com/beeleelee/mall/domain/kernel"
)

type fakeMandateVerifier struct{}

func (fakeMandateVerifier) VerifyAndExecute(_ context.Context, mandateID, userID kernel.ID, amount int64) error {
	return nil
}

func (fakeMandateVerifier) VerifyAndExecuteWithToken(_ context.Context, mandateID, userID kernel.ID, amount int64, token, provider string) error {
	return nil
}

type checkoutWSTestFixture struct {
	handler *CheckoutWSHandler
	repo    *fakeCheckoutRepo
	sf      *kernel.Snowflake
}

func newCheckoutWSTestFixture(t *testing.T) *checkoutWSTestFixture {
	t.Helper()
	repo := newFakeCheckoutRepo()
	taxSvc := fakeTaxService{}
	priceCalc := fakePriceCalculator{}
	pub := fakeCheckoutPub{}
	logger := fakeLog{}
	mandateVerifier := fakeMandateVerifier{}
	stripeProc := &fakeStripeProcessor{
		createCheckoutSessionFn: func(_ context.Context, _ *domain.CheckoutSession) (string, string, error) {
			return "https://stripe.example.com", "cs_test", nil
		},
		createPaymentIntentFn: func(_ context.Context, _ kernel.ID, _ int64) (string, string, error) {
			return "pi_secret", "pi_test", nil
		},
		getPaymentIntentStatusFn: func(_ context.Context, _ string) (string, error) {
			return "succeeded", nil
		},
	}
	svc := domain.NewCheckoutService(repo, taxSvc, priceCalc, pub, logger, mandateVerifier, stripeProc)
	sf, err := kernel.NewSnowflake(1)
	if err != nil {
		t.Fatalf("NewSnowflake: %v", err)
	}
	return &checkoutWSTestFixture{
		handler: NewCheckoutWSHandler(svc, logger),
		repo:    repo,
		sf:      sf,
	}
}

func (f *checkoutWSTestFixture) seedCheckout(userID int64) kernel.ID {
	id, _ := f.sf.NextID()
	cartID, _ := f.sf.NextID()
	items := []domain.CartSnapshotItem{
		{ProductID: 1, SKU: "TST-001", Name: "Test", UnitPrice: 1000, Quantity: 1},
	}
	snapshot := domain.NewCartSnapshot(items)
	session, _ := domain.NewCheckoutSession(id, kernel.ID(userID), cartID, snapshot)
	f.repo.Save(context.Background(), session)
	return id
}

func TestCheckoutWSHandler_StateUpdate_Success(t *testing.T) {
	fx := newCheckoutWSTestFixture(t)
	checkoutID := fx.seedCheckout(100)

	msg := ecp.ECPMessage{Method: ecp.MethodStateUpdate, ID: json.Number("1")}
	resp := fx.handler.handleMessage(nil, checkoutID, kernel.ID(100), msg)
	if resp == nil {
		t.Fatal("expected non-nil response")
	}
	if resp.Error != nil {
		t.Fatalf("unexpected error: %s", resp.Error.Message)
	}
	result, ok := resp.Result.(ecp.StateUpdateResult)
	if !ok {
		t.Fatalf("expected StateUpdateResult, got %T", resp.Result)
	}
	if result.CheckoutID != checkoutID.Int64() {
		t.Errorf("expected checkoutID %d, got %d", checkoutID.Int64(), result.CheckoutID)
	}
	if result.Status != "incomplete" {
		t.Errorf("expected status incomplete, got %s", result.Status)
	}
}

func TestCheckoutWSHandler_StateUpdate_PermissionDenied(t *testing.T) {
	fx := newCheckoutWSTestFixture(t)
	checkoutID := fx.seedCheckout(100)

	msg := ecp.ECPMessage{Method: ecp.MethodStateUpdate, ID: json.Number("1")}
	resp := fx.handler.handleMessage(nil, checkoutID, kernel.ID(999), msg)
	if resp == nil {
		t.Fatal("expected non-nil response")
	}
	if resp.Error == nil {
		t.Fatal("expected error")
	}
	if resp.Error.Code != -32001 {
		t.Errorf("expected error code -32001, got %d", resp.Error.Code)
	}
}

func TestCheckoutWSHandler_StateUpdate_NotFound(t *testing.T) {
	fx := newCheckoutWSTestFixture(t)
	sf, _ := kernel.NewSnowflake(2)
	missingID, _ := sf.NextID()

	msg := ecp.ECPMessage{Method: ecp.MethodStateUpdate, ID: json.Number("1")}
	resp := fx.handler.handleMessage(nil, missingID, kernel.ID(1), msg)
	if resp == nil {
		t.Fatal("expected non-nil response")
	}
	if resp.Error == nil {
		t.Fatal("expected error")
	}
	if resp.Error.Code != -32000 {
		t.Errorf("expected error code -32000, got %d", resp.Error.Code)
	}
}

func TestCheckoutWSHandler_CredentialsSubmit_Success(t *testing.T) {
	fx := newCheckoutWSTestFixture(t)
	checkoutID := fx.seedCheckout(100)

	params := ecp.CredentialsSubmitParams{
		ShippingLine1: "123 Main St", ShippingCity: "Springfield",
		ShippingState: "IL", ShippingPostal: "62701", ShippingCountry: "US",
		BillingLine1: "456 Oak Ave", BillingCity: "Chicago",
		BillingState: "IL", BillingPostal: "60601", BillingCountry: "US",
	}
	msg := ecp.ECPMessage{
		Method: ecp.MethodCredentialsSubmit,
		Params: params,
		ID:     json.Number("2"),
	}
	r := httptest.NewRequest(http.MethodPost, "/api/v1/checkout/ws/1", nil)
	resp := fx.handler.handleMessage(r, checkoutID, kernel.ID(100), msg)
	if resp == nil {
		t.Fatal("expected non-nil response")
	}
	if resp.Error != nil {
		t.Fatalf("unexpected error: %s", resp.Error.Message)
	}
	result, ok := resp.Result.(ecp.CredentialsSubmitResult)
	if !ok {
		t.Fatalf("expected CredentialsSubmitResult, got %T", resp.Result)
	}
	if result.CheckoutID != checkoutID.Int64() {
		t.Errorf("expected checkoutID %d, got %d", checkoutID.Int64(), result.CheckoutID)
	}
}

func TestCheckoutWSHandler_PaymentAuthorize_Success(t *testing.T) {
	fx := newCheckoutWSTestFixture(t)
	checkoutID := fx.seedCheckout(100)
	fx.handler.svc.SetShippingAddress(context.Background(), checkoutID, domain.Address{
		Line1: "123 Main St", City: "Springfield", State: "IL", PostalCode: "62701", Country: "US",
	})
	fx.handler.svc.SetBillingAddress(context.Background(), checkoutID, domain.Address{
		Line1: "456 Oak Ave", City: "Chicago", State: "IL", PostalCode: "60601", Country: "US",
	})
	fx.handler.svc.SelectShippingOption(context.Background(), checkoutID, domain.ShippingOption{ID: "std", Name: "Standard", Cost: 500})
	session, err := fx.handler.svc.SelectPaymentHandler(context.Background(), checkoutID, "stripe")
	if err != nil {
		t.Fatalf("SelectPaymentHandler: %v", err)
	}
	session.MarkReady()
	if err := fx.repo.Save(context.Background(), session); err != nil {
		t.Fatalf("Save after MarkReady: %v", err)
	}

	params := ecp.PaymentAuthorizeParams{}
	msg := ecp.ECPMessage{
		Method: ecp.MethodPaymentAuthorize,
		Params: params,
		ID:     json.Number("3"),
	}
	r := httptest.NewRequest(http.MethodPost, "/api/v1/checkout/ws/1", nil)
	resp := fx.handler.handleMessage(r, checkoutID, kernel.ID(100), msg)
	if resp == nil {
		t.Fatal("expected non-nil response")
	}
	if resp.Error != nil {
		t.Fatalf("unexpected error: %s", resp.Error.Message)
	}
	result, ok := resp.Result.(ecp.PaymentAuthorizeResult)
	if !ok {
		t.Fatalf("expected PaymentAuthorizeResult, got %T", resp.Result)
	}
	if result.CheckoutID != checkoutID.Int64() {
		t.Errorf("expected checkoutID %d, got %d", checkoutID.Int64(), result.CheckoutID)
	}
}

func TestCheckoutWSHandler_AddressSelect_Shipping(t *testing.T) {
	fx := newCheckoutWSTestFixture(t)
	checkoutID := fx.seedCheckout(100)

	params := ecp.AddressSelectParams{
		AddressType: "shipping",
		Line1: "789 Pine Rd", City: "Portland", State: "OR",
		PostalCode: "97201", Country: "US",
	}
	msg := ecp.ECPMessage{
		Method: ecp.MethodAddressSelect,
		Params: params,
		ID:     json.Number("4"),
	}
	r := httptest.NewRequest(http.MethodPost, "/api/v1/checkout/ws/1", nil)
	resp := fx.handler.handleMessage(r, checkoutID, kernel.ID(100), msg)
	if resp == nil {
		t.Fatal("expected non-nil response")
	}
	if resp.Error != nil {
		t.Fatalf("unexpected error: %s", resp.Error.Message)
	}
	result, ok := resp.Result.(ecp.AddressSelectResult)
	if !ok {
		t.Fatalf("expected AddressSelectResult, got %T", resp.Result)
	}
	if result.AddressType != "shipping" {
		t.Errorf("expected address_type shipping, got %s", result.AddressType)
	}
}

func TestCheckoutWSHandler_AddressSelect_Billing(t *testing.T) {
	fx := newCheckoutWSTestFixture(t)
	checkoutID := fx.seedCheckout(100)

	params := ecp.AddressSelectParams{
		AddressType: "billing",
		Line1: "321 Elm St", City: "Seattle", State: "WA",
		PostalCode: "98101", Country: "US",
	}
	msg := ecp.ECPMessage{
		Method: ecp.MethodAddressSelect,
		Params: params,
		ID:     json.Number("5"),
	}
	r := httptest.NewRequest(http.MethodPost, "/api/v1/checkout/ws/1", nil)
	resp := fx.handler.handleMessage(r, checkoutID, kernel.ID(100), msg)
	if resp == nil {
		t.Fatal("expected non-nil response")
	}
	if resp.Error != nil {
		t.Fatalf("unexpected error: %s", resp.Error.Message)
	}
	result, ok := resp.Result.(ecp.AddressSelectResult)
	if !ok {
		t.Fatalf("expected AddressSelectResult, got %T", resp.Result)
	}
	if result.AddressType != "billing" {
		t.Errorf("expected address_type billing, got %s", result.AddressType)
	}
}

func TestCheckoutWSHandler_AddressSelect_InvalidType(t *testing.T) {
	fx := newCheckoutWSTestFixture(t)
	checkoutID := fx.seedCheckout(100)

	params := ecp.AddressSelectParams{
		AddressType: "invalid",
		Line1: "Nowhere", City: "Null", State: "XX",
		PostalCode: "00000", Country: "US",
	}
	msg := ecp.ECPMessage{
		Method: ecp.MethodAddressSelect,
		Params: params,
		ID:     json.Number("6"),
	}
	r := httptest.NewRequest(http.MethodPost, "/api/v1/checkout/ws/1", nil)
	resp := fx.handler.handleMessage(r, checkoutID, kernel.ID(100), msg)
	if resp == nil {
		t.Fatal("expected non-nil response")
	}
	if resp.Error == nil {
		t.Fatal("expected error for invalid address type")
	}
	if resp.Error.Code != -32602 {
		t.Errorf("expected error code -32602, got %d", resp.Error.Code)
	}
}

func TestCheckoutWSHandler_UnknownMethod(t *testing.T) {
	fx := newCheckoutWSTestFixture(t)
	checkoutID := fx.seedCheckout(100)

	msg := ecp.ECPMessage{
		Method: "unknown_method",
		ID:     json.Number("7"),
	}
	resp := fx.handler.handleMessage(nil, checkoutID, kernel.ID(100), msg)
	if resp == nil {
		t.Fatal("expected non-nil response")
	}
	if resp.Error == nil {
		t.Fatal("expected error")
	}
	if resp.Error.Code != -32601 {
		t.Errorf("expected error code -32601, got %d", resp.Error.Code)
	}
}
