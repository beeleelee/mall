package rest

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strconv"
	"testing"

	"github.com/zeromicro/go-zero/rest/pathvar"

	domain "github.com/beeleelee/mall/domain/checkout"
	"github.com/beeleelee/mall/domain/kernel"
	"github.com/beeleelee/mall/interfaces/middleware"
)

type fakeCheckoutRepo struct {
	sessions map[kernel.ID]*domain.CheckoutSession
}

func newFakeCheckoutRepo() *fakeCheckoutRepo {
	return &fakeCheckoutRepo{sessions: make(map[kernel.ID]*domain.CheckoutSession)}
}

func (f *fakeCheckoutRepo) Save(_ context.Context, s *domain.CheckoutSession) error {
	f.sessions[s.ID] = s
	return nil
}

func (f *fakeCheckoutRepo) FindByID(_ context.Context, id kernel.ID) (*domain.CheckoutSession, error) {
	s, ok := f.sessions[id]
	if !ok {
		return nil, kernel.NewDomainError(kernel.ErrNotFound, "checkout not found")
	}
	return s, nil
}

func (f *fakeCheckoutRepo) FindByUserID(_ context.Context, userID kernel.ID) (*domain.CheckoutSession, error) {
	for _, s := range f.sessions {
		if s.UserID == userID {
			return s, nil
		}
	}
	return nil, kernel.NewDomainError(kernel.ErrNotFound, "checkout not found")
}

func (f *fakeCheckoutRepo) FindByStripeSessionID(_ context.Context, stripeSessionID string) (*domain.CheckoutSession, error) {
	for _, s := range f.sessions {
		if s.StripeSessionID == stripeSessionID {
			return s, nil
		}
	}
	return nil, kernel.NewDomainError(kernel.ErrNotFound, "checkout not found")
}

func (f *fakeCheckoutRepo) Delete(_ context.Context, id kernel.ID) error {
	delete(f.sessions, id)
	return nil
}

type fakeCheckoutPub struct{}

func (fakeCheckoutPub) PublishCheckoutUpdated(_ context.Context, _ *domain.CheckoutSession) error {
	return nil
}

type fakeStripeProcessor struct {
	createCheckoutSessionFn  func(ctx context.Context, checkout *domain.CheckoutSession) (string, string, error)
	createPaymentIntentFn    func(ctx context.Context, checkoutID kernel.ID, amount int64) (string, string, error)
	getPaymentIntentStatusFn func(ctx context.Context, paymentIntentID string) (string, error)
}

func (f *fakeStripeProcessor) CreateCheckoutSession(ctx context.Context, checkout *domain.CheckoutSession) (string, string, error) {
	return f.createCheckoutSessionFn(ctx, checkout)
}

func (f *fakeStripeProcessor) CreatePaymentIntent(ctx context.Context, checkoutID kernel.ID, amount int64) (string, string, error) {
	return f.createPaymentIntentFn(ctx, checkoutID, amount)
}

func (f *fakeStripeProcessor) GetPaymentIntentStatus(ctx context.Context, paymentIntentID string) (string, error) {
	return f.getPaymentIntentStatusFn(ctx, paymentIntentID)
}

type fakeTaxService struct{}

func (fakeTaxService) CalculateTax(_ context.Context, _ domain.TaxInput) (*domain.TaxResult, error) {
	return &domain.TaxResult{TaxAmount: 0, Provider: "fake"}, nil
}

type fakePriceCalculator struct{}

func (fakePriceCalculator) Calculate(_ context.Context, input domain.PriceInput) (*domain.PriceResult, error) {
	itemsTotal := int64(0)
	for _, item := range input.Items {
		itemsTotal += item.UnitPrice * int64(item.Quantity)
	}
	return &domain.PriceResult{
		Subtotal:   itemsTotal,
		Shipping:   input.ShippingCost,
		Tax:        input.TaxAmount,
		GrandTotal: itemsTotal + input.ShippingCost + input.TaxAmount,
	}, nil
}

func newTestCheckoutHandlerWithStripe(t *testing.T) *CheckoutHandler {
	t.Helper()
	repo := newFakeCheckoutRepo()
	pub := fakeCheckoutPub{}
	logger := fakeLog{}
	taxSvc := fakeTaxService{}
	priceCalc := fakePriceCalculator{}
	svc := domain.NewCheckoutService(repo, taxSvc, priceCalc, pub, logger, nil, &fakeStripeProcessor{
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
	sf, err := kernel.NewSnowflake(1)
	if err != nil {
		t.Fatalf("NewSnowflake failed: %v", err)
	}
	return NewCheckoutHandler(svc, sf)
}

func newTestCheckoutHandler(t *testing.T) *CheckoutHandler {
	t.Helper()
	repo := newFakeCheckoutRepo()
	pub := fakeCheckoutPub{}
	logger := fakeLog{}
	taxSvc := fakeTaxService{}
	priceCalc := fakePriceCalculator{}
	svc := domain.NewCheckoutService(repo, taxSvc, priceCalc, pub, logger, nil, nil)
	sf, err := kernel.NewSnowflake(1)
	if err != nil {
		t.Fatalf("NewSnowflake failed: %v", err)
	}
	return NewCheckoutHandler(svc, sf)
}

func userRequest(t *testing.T, method, path string, body []byte, userID int64) *http.Request {
	t.Helper()
	req := httptest.NewRequest(method, path, bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	ctx := middleware.ContextWithUser(req.Context(), middleware.UserInfo{UserID: userID})
	return req.WithContext(ctx)
}

func createCheckout(t *testing.T, h *CheckoutHandler, userID int64, items []map[string]any) checkoutResponse {
	t.Helper()
	body := map[string]any{"cart_id": 10, "items": items}
	data, _ := json.Marshal(body)
	req := userRequest(t, http.MethodPost, "/api/v1/checkouts", data, userID)
	rec := httptest.NewRecorder()
	h.Create(rec, req)
	if rec.Code != http.StatusCreated {
		t.Fatalf("create checkout: expected 201, got %d: %s", rec.Code, rec.Body.String())
	}
	var resp checkoutResponse
	if err := json.NewDecoder(rec.Body).Decode(&resp); err != nil {
		t.Fatalf("create checkout: decode: %v", err)
	}
	return resp
}

func TestCheckoutHandler_Create_Success(t *testing.T) {
	h := newTestCheckoutHandler(t)
	resp := createCheckout(t, h, 1, []map[string]any{
		{"product_id": 100, "sku": "SKU001", "name": "Test", "quantity": 2, "unit_price": 1000},
	})

	if resp.UserID != 1 {
		t.Errorf("expected UserID 1, got %d", resp.UserID)
	}
	if resp.Status != string(domain.CheckoutStatusIncomplete) {
		t.Errorf("expected incomplete, got %s", resp.Status)
	}
}

func TestCheckoutHandler_GetCheckout_Ownership(t *testing.T) {
	h := newTestCheckoutHandler(t)
	created := createCheckout(t, h, 1, []map[string]any{
		{"product_id": 100, "sku": "SKU001", "name": "Test", "quantity": 1, "unit_price": 500},
	})

	req := userRequest(t, http.MethodGet, "/api/v1/checkouts/"+strconv.FormatInt(created.ID, 10), nil, 2)
	req = pathvar.WithVars(req, map[string]string{"id": strconv.FormatInt(created.ID, 10)})
	rec := httptest.NewRecorder()
	h.GetCheckout(rec, req)

	if rec.Code != http.StatusForbidden {
		t.Errorf("expected 403 for wrong user, got %d: %s", rec.Code, rec.Body.String())
	}
}

func TestCheckoutHandler_GetCheckout_Success(t *testing.T) {
	h := newTestCheckoutHandler(t)
	created := createCheckout(t, h, 1, []map[string]any{
		{"product_id": 100, "sku": "SKU001", "name": "Test", "quantity": 1, "unit_price": 500},
	})

	req := userRequest(t, http.MethodGet, "/api/v1/checkouts/"+strconv.FormatInt(created.ID, 10), nil, 1)
	req = pathvar.WithVars(req, map[string]string{"id": strconv.FormatInt(created.ID, 10)})
	rec := httptest.NewRecorder()
	h.GetCheckout(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", rec.Code, rec.Body.String())
	}
}

func TestCheckoutHandler_SetShippingAddress_Success(t *testing.T) {
	h := newTestCheckoutHandler(t)
	created := createCheckout(t, h, 1, []map[string]any{
		{"product_id": 100, "sku": "SKU001", "name": "Test", "quantity": 1, "unit_price": 500},
	})
	idStr := strconv.FormatInt(created.ID, 10)

	addrBody := map[string]string{
		"line1": "123 Main St", "city": "Springfield",
		"state": "IL", "postal_code": "62701", "country": "US",
	}
	data, _ := json.Marshal(addrBody)
	req := userRequest(t, http.MethodPost, "/api/v1/checkouts/"+idStr+"/shipping-address", data, 1)
	req = pathvar.WithVars(req, map[string]string{"id": idStr})
	rec := httptest.NewRecorder()
	h.SetShippingAddress(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", rec.Code, rec.Body.String())
	}

	var resp checkoutResponse
	json.NewDecoder(rec.Body).Decode(&resp)
	if resp.ShippingAddress == nil || resp.ShippingAddress.City != "Springfield" {
		t.Errorf("expected shipping city Springfield, got %v", resp.ShippingAddress)
	}
}

func TestCheckoutHandler_SetShippingAddress_WrongUser(t *testing.T) {
	h := newTestCheckoutHandler(t)
	created := createCheckout(t, h, 1, []map[string]any{
		{"product_id": 100, "sku": "SKU001", "name": "Test", "quantity": 1, "unit_price": 500},
	})
	idStr := strconv.FormatInt(created.ID, 10)

	addrBody := map[string]string{
		"line1": "123 Main St", "city": "Springfield",
		"state": "IL", "postal_code": "62701", "country": "US",
	}
	data, _ := json.Marshal(addrBody)
	req := userRequest(t, http.MethodPost, "/api/v1/checkouts/"+idStr+"/shipping-address", data, 2)
	req = pathvar.WithVars(req, map[string]string{"id": idStr})
	rec := httptest.NewRecorder()
	h.SetShippingAddress(rec, req)

	if rec.Code != http.StatusForbidden {
		t.Errorf("expected 403 for wrong user, got %d: %s", rec.Code, rec.Body.String())
	}
}

func TestCheckoutHandler_Complete_Success(t *testing.T) {
	h := newTestCheckoutHandler(t)
	created := createCheckout(t, h, 1, []map[string]any{
		{"product_id": 100, "sku": "SKU001", "name": "Test", "quantity": 2, "unit_price": 1000},
	})
	idStr := strconv.FormatInt(created.ID, 10)

	addrData := map[string]string{
		"line1": "123 Main St", "city": "Springfield",
		"state": "IL", "postal_code": "62701", "country": "US",
	}
	data, _ := json.Marshal(addrData)
	saReq := userRequest(t, http.MethodPost, "/api/v1/checkouts/"+idStr+"/shipping-address", data, 1)
	saReq = pathvar.WithVars(saReq, map[string]string{"id": idStr})
	h.SetShippingAddress(httptest.NewRecorder(), saReq)

	baReq := userRequest(t, http.MethodPost, "/api/v1/checkouts/"+idStr+"/billing-address", data, 1)
	baReq = pathvar.WithVars(baReq, map[string]string{"id": idStr})
	h.SetBillingAddress(httptest.NewRecorder(), baReq)

	soBody := map[string]any{"id": "std", "name": "Standard", "cost": 500}
	data, _ = json.Marshal(soBody)
	soReq := userRequest(t, http.MethodPost, "/api/v1/checkouts/"+idStr+"/shipping-option", data, 1)
	soReq = pathvar.WithVars(soReq, map[string]string{"id": idStr})
	h.SelectShippingOption(httptest.NewRecorder(), soReq)

	phBody := map[string]string{"handler": "stripe"}
	data, _ = json.Marshal(phBody)
	phReq := userRequest(t, http.MethodPost, "/api/v1/checkouts/"+idStr+"/payment-handler", data, 1)
	phReq = pathvar.WithVars(phReq, map[string]string{"id": idStr})
	h.SelectPaymentHandler(httptest.NewRecorder(), phReq)

	completeReq := userRequest(t, http.MethodPost, "/api/v1/checkouts/"+idStr+"/complete", nil, 1)
	completeReq = pathvar.WithVars(completeReq, map[string]string{"id": idStr})
	completeRec := httptest.NewRecorder()
	h.Complete(completeRec, completeReq)

	if completeRec.Code != http.StatusOK {
		t.Fatalf("expected 200 on complete, got %d: %s", completeRec.Code, completeRec.Body.String())
	}

	var resp checkoutResponse
	json.NewDecoder(completeRec.Body).Decode(&resp)
	if resp.Status != string(domain.CheckoutStatusCompleted) {
		t.Errorf("expected completed, got %s", resp.Status)
	}
}

func TestCheckoutHandler_Cancel_Success(t *testing.T) {
	h := newTestCheckoutHandler(t)
	created := createCheckout(t, h, 1, []map[string]any{
		{"product_id": 100, "sku": "SKU001", "name": "Test", "quantity": 1, "unit_price": 500},
	})
	idStr := strconv.FormatInt(created.ID, 10)

	cancelReq := userRequest(t, http.MethodPost, "/api/v1/checkouts/"+idStr+"/cancel", nil, 1)
	cancelReq = pathvar.WithVars(cancelReq, map[string]string{"id": idStr})
	cancelRec := httptest.NewRecorder()
	h.Cancel(cancelRec, cancelReq)

	if cancelRec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", cancelRec.Code, cancelRec.Body.String())
	}

	var resp checkoutResponse
	json.NewDecoder(cancelRec.Body).Decode(&resp)
	if resp.Status != string(domain.CheckoutStatusCancelled) {
		t.Errorf("expected cancelled, got %s", resp.Status)
	}
}

func TestCheckoutHandler_Unauthenticated(t *testing.T) {
	h := newTestCheckoutHandler(t)
	body := map[string]any{
		"cart_id": 10,
		"items": []map[string]any{
			{"product_id": 100, "sku": "SKU001", "name": "Test", "quantity": 1, "unit_price": 500},
		},
	}
	data, _ := json.Marshal(body)
	req := httptest.NewRequest(http.MethodPost, "/api/v1/checkouts", bytes.NewReader(data))
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()
	h.Create(rec, req)

	if rec.Code != http.StatusUnauthorized {
		t.Errorf("expected 401, got %d", rec.Code)
	}
}

func TestCheckoutHandler_SubmitPaymentToken_Success(t *testing.T) {
	h := newTestCheckoutHandler(t)

	body, _ := json.Marshal(map[string]any{
		"cart_id": 10,
		"items": []map[string]any{
			{"product_id": 100, "sku": "SKU001", "name": "Test", "quantity": 1, "unit_price": 500},
		},
	})
	req := userRequest(t, http.MethodPost, "/api/v1/checkouts", body, 42)
	rec := httptest.NewRecorder()
	h.Create(rec, req)

	var createResp map[string]any
	json.Unmarshal(rec.Body.Bytes(), &createResp)
	checkoutID := int64(createResp["id"].(float64))

	tokenBody, _ := json.Marshal(map[string]any{
		"wallet_provider": "google_pay",
		"token":           "encrypted-payment-data",
	})
	req2 := httptest.NewRequest(http.MethodPost, "/api/v1/checkouts/"+strconv.FormatInt(checkoutID, 10)+"/payment-token", bytes.NewReader(tokenBody))
	req2 = req2.WithContext(middleware.ContextWithUser(req2.Context(), middleware.UserInfo{UserID: 42}))
	req2 = pathvar.WithVars(req2, map[string]string{"id": strconv.FormatInt(checkoutID, 10)})
	rec2 := httptest.NewRecorder()
	h.SubmitPaymentToken(rec2, req2)

	if rec2.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", rec2.Code, rec2.Body.String())
	}

	var resp map[string]any
	json.Unmarshal(rec2.Body.Bytes(), &resp)
	if resp["wallet_provider"] != "google_pay" {
		t.Errorf("expected google_pay, got %v", resp["wallet_provider"])
	}
}

func TestCheckoutHandler_SubmitPaymentToken_WrongUser(t *testing.T) {
	h := newTestCheckoutHandler(t)

	body, _ := json.Marshal(map[string]any{
		"cart_id": 10,
		"items": []map[string]any{
			{"product_id": 100, "sku": "SKU001", "name": "Test", "quantity": 1, "unit_price": 500},
		},
	})
	req := userRequest(t, http.MethodPost, "/api/v1/checkouts", body, 42)
	rec := httptest.NewRecorder()
	h.Create(rec, req)

	var createResp map[string]any
	json.Unmarshal(rec.Body.Bytes(), &createResp)
	checkoutID := int64(createResp["id"].(float64))

	tokenBody, _ := json.Marshal(map[string]any{
		"wallet_provider": "google_pay",
		"token":           "token",
	})
	req2 := httptest.NewRequest(http.MethodPost, "/api/v1/checkouts/"+strconv.FormatInt(checkoutID, 10)+"/payment-token", bytes.NewReader(tokenBody))
	req2 = req2.WithContext(middleware.ContextWithUser(req2.Context(), middleware.UserInfo{UserID: 99}))
	req2 = pathvar.WithVars(req2, map[string]string{"id": strconv.FormatInt(checkoutID, 10)})
	rec2 := httptest.NewRecorder()
	h.SubmitPaymentToken(rec2, req2)

	if rec2.Code != http.StatusForbidden {
		t.Errorf("expected 403, got %d", rec2.Code)
	}
}

func setupCheckoutForTest(t *testing.T, h *CheckoutHandler) int64 {
	t.Helper()

	body := map[string]any{
		"cart_id": 10,
		"items": []map[string]any{
			{"product_id": 1, "sku": "SKU001", "name": "Test", "quantity": 1, "unit_price": 1000},
		},
	}
	bodyBytes, _ := json.Marshal(body)
	req := httptest.NewRequest(http.MethodPost, "/api/v1/checkouts", bytes.NewReader(bodyBytes))
	req = req.WithContext(middleware.ContextWithUser(req.Context(), middleware.UserInfo{UserID: 99}))
	rec := httptest.NewRecorder()
	h.Create(rec, req)

	var resp checkoutResponse
	json.NewDecoder(rec.Body).Decode(&resp)

	addr, _ := json.Marshal(map[string]string{"line1": "123 Main", "city": "Portland", "state": "OR", "postal_code": "97201", "country": "US"})
	req2 := httptest.NewRequest(http.MethodPost, "/api/v1/checkouts/"+strconv.FormatInt(resp.ID, 10)+"/shipping-address", bytes.NewReader(addr))
	req2 = req2.WithContext(middleware.ContextWithUser(req2.Context(), middleware.UserInfo{UserID: 99}))
	req2 = pathvar.WithVars(req2, map[string]string{"id": strconv.FormatInt(resp.ID, 10)})
	httptest.NewRecorder()
	h.SetShippingAddress(httptest.NewRecorder(), req2)

	req3 := httptest.NewRequest(http.MethodPost, "/api/v1/checkouts/"+strconv.FormatInt(resp.ID, 10)+"/billing-address", bytes.NewReader(addr))
	req3 = req3.WithContext(middleware.ContextWithUser(req3.Context(), middleware.UserInfo{UserID: 99}))
	req3 = pathvar.WithVars(req3, map[string]string{"id": strconv.FormatInt(resp.ID, 10)})
	h.SetBillingAddress(httptest.NewRecorder(), req3)

	shipOpt, _ := json.Marshal(map[string]any{"id": "std", "name": "Standard", "cost": 500})
	req4 := httptest.NewRequest(http.MethodPost, "/api/v1/checkouts/"+strconv.FormatInt(resp.ID, 10)+"/shipping-option", bytes.NewReader(shipOpt))
	req4 = req4.WithContext(middleware.ContextWithUser(req4.Context(), middleware.UserInfo{UserID: 99}))
	req4 = pathvar.WithVars(req4, map[string]string{"id": strconv.FormatInt(resp.ID, 10)})
	h.SelectShippingOption(httptest.NewRecorder(), req4)

	handlerReq, _ := json.Marshal(map[string]string{"handler": "stripe"})
	req5 := httptest.NewRequest(http.MethodPost, "/api/v1/checkouts/"+strconv.FormatInt(resp.ID, 10)+"/payment-handler", bytes.NewReader(handlerReq))
	req5 = req5.WithContext(middleware.ContextWithUser(req5.Context(), middleware.UserInfo{UserID: 99}))
	req5 = pathvar.WithVars(req5, map[string]string{"id": strconv.FormatInt(resp.ID, 10)})
	h.SelectPaymentHandler(httptest.NewRecorder(), req5)

	return resp.ID
}

func TestCheckoutHandler_CreateStripePaymentIntent_Success(t *testing.T) {
	h := newTestCheckoutHandlerWithStripe(t)
	checkoutID := setupCheckoutForTest(t, h)

	req := httptest.NewRequest(http.MethodPost, "/api/v1/checkouts/"+strconv.FormatInt(checkoutID, 10)+"/stripe/payment-intent", nil)
	req = req.WithContext(middleware.ContextWithUser(req.Context(), middleware.UserInfo{UserID: 99}))
	req = pathvar.WithVars(req, map[string]string{"id": strconv.FormatInt(checkoutID, 10)})
	rec := httptest.NewRecorder()
	h.CreateStripePaymentIntent(rec, req)

	if rec.Code != http.StatusCreated {
		t.Errorf("expected 201, got %d", rec.Code)
	}

	var resp createPaymentIntentResponse
	if err := json.NewDecoder(rec.Body).Decode(&resp); err != nil {
		t.Fatal(err)
	}
	if resp.ClientSecret != "secret_pi_1" {
		t.Errorf("expected secret_pi_1, got %s", resp.ClientSecret)
	}
	if resp.IntentID != "pi_1" {
		t.Errorf("expected pi_1, got %s", resp.IntentID)
	}
	if resp.Amount == 0 {
		t.Errorf("expected non-zero amount")
	}
}

func TestCheckoutHandler_CreateStripePaymentIntent_WrongUser(t *testing.T) {
	h := newTestCheckoutHandlerWithStripe(t)
	checkoutID := setupCheckoutForTest(t, h)

	req := httptest.NewRequest(http.MethodPost, "/api/v1/checkouts/"+strconv.FormatInt(checkoutID, 10)+"/stripe/payment-intent", nil)
	req = req.WithContext(middleware.ContextWithUser(req.Context(), middleware.UserInfo{UserID: 1}))
	req = pathvar.WithVars(req, map[string]string{"id": strconv.FormatInt(checkoutID, 10)})
	rec := httptest.NewRecorder()
	h.CreateStripePaymentIntent(rec, req)

	if rec.Code != http.StatusForbidden {
		t.Errorf("expected 403, got %d", rec.Code)
	}
}
