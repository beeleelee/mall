package mcp

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/beeleelee/mall/domain/kernel"

	domainCart "github.com/beeleelee/mall/domain/cart"
	domainCatalog "github.com/beeleelee/mall/domain/catalog"
	domainCheckout "github.com/beeleelee/mall/domain/checkout"
	domainDiscount "github.com/beeleelee/mall/domain/discount"
	domainFulfillment "github.com/beeleelee/mall/domain/fulfillment"
	domainIdentity "github.com/beeleelee/mall/domain/identity"
	domainInventory "github.com/beeleelee/mall/domain/inventory"
	domainOAuth "github.com/beeleelee/mall/domain/oauth"
	domainOrder "github.com/beeleelee/mall/domain/order"
	domainPayment "github.com/beeleelee/mall/domain/payment"
)

// ---------------------------------------------------------------------------
// Shared helpers
// ---------------------------------------------------------------------------

func rpcCall(t *testing.T, router *MCPRouter, method, name string, args map[string]any) *httptest.ResponseRecorder {
	t.Helper()
	params := map[string]any{}
	if name != "" {
		params["name"] = name
		params["arguments"] = args
	}
	var body []byte
	if method == "tools/call" {
		body, _ = json.Marshal(map[string]any{
			"jsonrpc": "2.0",
			"method":  method,
			"params":  params,
			"id":      1,
		})
	} else {
		body, _ = json.Marshal(map[string]any{
			"jsonrpc": "2.0",
			"method":  method,
			"id":      1,
		})
	}
	w := httptest.NewRecorder()
	r := httptest.NewRequest(http.MethodPost, "/mcp", bytes.NewReader(body))
	r.Header.Set("Content-Type", "application/json")
	router.ServeHTTP(w, r)
	return w
}

func assertOK(t *testing.T, w *httptest.ResponseRecorder) {
	t.Helper()
	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", w.Code, w.Body.String())
	}
	var resp map[string]any
	if err := json.Unmarshal(w.Body.Bytes(), &resp); err != nil {
		t.Fatal(err)
	}
	if resp["error"] != nil {
		t.Fatalf("unexpected error: %v", resp["error"])
	}
}

// int64ID extracts an int64 from JSON text (e.g. Snowflake ID) without
// float64 precision loss by using json.NewDecoder with UseNumber().
func int64ID(t *testing.T, text string, key string) int64 {
	t.Helper()
	dec := json.NewDecoder(bytes.NewReader([]byte(text)))
	dec.UseNumber()
	var m map[string]any
	if err := dec.Decode(&m); err != nil {
		t.Fatalf("int64ID decode: %v", err)
	}
	n, ok := m[key].(json.Number)
	if !ok {
		t.Fatalf("int64ID: key %q not found or not a number", key)
	}
	v, err := n.Int64()
	if err != nil {
		t.Fatalf("int64ID: %v", err)
	}
	return v
}

// ---------------------------------------------------------------------------
// Cart fakes
// ---------------------------------------------------------------------------

type fakeCartRepo struct {
	carts map[kernel.ID]*domainCart.Cart
	byUID map[kernel.ID]kernel.ID
}

func newFakeCartRepo() *fakeCartRepo {
	return &fakeCartRepo{
		carts: make(map[kernel.ID]*domainCart.Cart),
		byUID: make(map[kernel.ID]kernel.ID),
	}
}

func (f *fakeCartRepo) Save(_ context.Context, c *domainCart.Cart) error {
	f.carts[c.ID] = c
	f.byUID[c.UserID] = c.ID
	return nil
}
func (f *fakeCartRepo) FindByID(_ context.Context, id kernel.ID) (*domainCart.Cart, error) {
	c, ok := f.carts[id]
	if !ok {
		return nil, kernel.NewDomainError(kernel.ErrNotFound, "not found")
	}
	return c, nil
}
func (f *fakeCartRepo) FindByUserID(_ context.Context, uid kernel.ID) (*domainCart.Cart, error) {
	cid, ok := f.byUID[uid]
	if !ok {
		return nil, kernel.NewDomainError(kernel.ErrNotFound, "not found")
	}
	return f.carts[cid], nil
}
func (f *fakeCartRepo) Delete(_ context.Context, id kernel.ID) error {
	delete(f.carts, id)
	return nil
}

type fakeCartPub struct{}

func (fakeCartPub) PublishCartUpdated(_ context.Context, _ *domainCart.Cart) error { return nil }

// ---------------------------------------------------------------------------
// Checkout fakes
// ---------------------------------------------------------------------------

type fakeCheckoutRepo struct {
	sessions map[kernel.ID]*domainCheckout.CheckoutSession
	byUID    map[kernel.ID]kernel.ID
}

func newFakeCheckoutRepo() *fakeCheckoutRepo {
	return &fakeCheckoutRepo{
		sessions: make(map[kernel.ID]*domainCheckout.CheckoutSession),
		byUID:    make(map[kernel.ID]kernel.ID),
	}
}
func (f *fakeCheckoutRepo) Save(_ context.Context, s *domainCheckout.CheckoutSession) error {
	f.sessions[s.ID] = s
	f.byUID[s.UserID] = s.ID
	return nil
}
func (f *fakeCheckoutRepo) FindByID(_ context.Context, id kernel.ID) (*domainCheckout.CheckoutSession, error) {
	s, ok := f.sessions[id]
	if !ok {
		return nil, kernel.NewDomainError(kernel.ErrNotFound, "not found")
	}
	return s, nil
}
func (f *fakeCheckoutRepo) FindByUserID(_ context.Context, uid kernel.ID) (*domainCheckout.CheckoutSession, error) {
	cid, ok := f.byUID[uid]
	if !ok {
		return nil, kernel.NewDomainError(kernel.ErrNotFound, "not found")
	}
	return f.sessions[cid], nil
}
func (f *fakeCheckoutRepo) Delete(_ context.Context, _ kernel.ID) error { return nil }

type fakeCheckoutPub struct{}

func (fakeCheckoutPub) PublishCheckoutUpdated(_ context.Context, _ *domainCheckout.CheckoutSession) error {
	return nil
}

type fakeTaxSvc struct{}

func (fakeTaxSvc) CalculateTax(_ context.Context, _ domainCheckout.TaxInput) (*domainCheckout.TaxResult, error) {
	return &domainCheckout.TaxResult{TaxAmount: 0, Provider: "test"}, nil
}

type fakePriceCalc struct{}

func (fakePriceCalc) Calculate(_ context.Context, in domainCheckout.PriceInput) (*domainCheckout.PriceResult, error) {
	return &domainCheckout.PriceResult{
		Subtotal:   in.ShippingCost + in.TaxAmount + 1000,
		Shipping:   in.ShippingCost,
		Tax:        in.TaxAmount,
		GrandTotal: in.ShippingCost + in.TaxAmount + 1000,
	}, nil
}

type fakeMandateVerifier struct{}

func (fakeMandateVerifier) VerifyAndExecute(_ context.Context, _, _ kernel.ID, _ int64) error {
	return nil
}

// ---------------------------------------------------------------------------
// Order fakes
// ---------------------------------------------------------------------------

type fakeOrderRepo struct {
	orders map[kernel.ID]*domainOrder.Order
	byUID  map[kernel.ID][]*domainOrder.Order
}

func newFakeOrderRepo() *fakeOrderRepo {
	return &fakeOrderRepo{
		orders: make(map[kernel.ID]*domainOrder.Order),
		byUID:  make(map[kernel.ID][]*domainOrder.Order),
	}
}
func (f *fakeOrderRepo) Save(_ context.Context, o *domainOrder.Order) error {
	f.orders[o.ID] = o
	f.byUID[o.UserID] = append(f.byUID[o.UserID], o)
	return nil
}
func (f *fakeOrderRepo) FindByID(_ context.Context, id kernel.ID) (*domainOrder.Order, error) {
	o, ok := f.orders[id]
	if !ok {
		return nil, kernel.NewDomainError(kernel.ErrNotFound, "not found")
	}
	return o, nil
}
func (f *fakeOrderRepo) FindByUserID(_ context.Context, uid kernel.ID) ([]*domainOrder.Order, error) {
	return f.byUID[uid], nil
}
func (f *fakeOrderRepo) FindByCheckoutID(_ context.Context, _ kernel.ID) (*domainOrder.Order, error) {
	return nil, kernel.NewDomainError(kernel.ErrNotFound, "not found")
}
func (f *fakeOrderRepo) FindAll(_ context.Context, _, _ int) ([]*domainOrder.Order, error) {
	var all []*domainOrder.Order
	for _, o := range f.orders {
		all = append(all, o)
	}
	return all, nil
}
func (f *fakeOrderRepo) Delete(_ context.Context, _ kernel.ID) error { return nil }

type fakeOrderPub struct{}

func (fakeOrderPub) PublishOrderEvent(_ context.Context, _ *domainOrder.Order) error { return nil }

// ---------------------------------------------------------------------------
// Discount fakes
// ---------------------------------------------------------------------------

type fakeDiscountRepo struct {
	codes map[string]*domainDiscount.DiscountCode
}

func newFakeDiscountRepo() *fakeDiscountRepo {
	return &fakeDiscountRepo{codes: make(map[string]*domainDiscount.DiscountCode)}
}
func (f *fakeDiscountRepo) Save(_ context.Context, dc *domainDiscount.DiscountCode) error {
	f.codes[dc.Code] = dc
	return nil
}
func (f *fakeDiscountRepo) FindByCode(_ context.Context, code string) (*domainDiscount.DiscountCode, error) {
	dc, ok := f.codes[code]
	if !ok {
		return nil, kernel.NewDomainError(kernel.ErrNotFound, "not found")
	}
	return dc, nil
}
func (f *fakeDiscountRepo) IncrementUsage(_ context.Context, _ kernel.ID) error { return nil }

// ---------------------------------------------------------------------------
// Inventory fakes
// ---------------------------------------------------------------------------

type fakeInventoryRepo struct {
	items map[kernel.ID]*domainInventory.InventoryItem
	byPID map[kernel.ID]kernel.ID
}

func newFakeInventoryRepo() *fakeInventoryRepo {
	return &fakeInventoryRepo{
		items: make(map[kernel.ID]*domainInventory.InventoryItem),
		byPID: make(map[kernel.ID]kernel.ID),
	}
}
func (f *fakeInventoryRepo) Save(_ context.Context, item *domainInventory.InventoryItem) error {
	f.items[item.ID] = item
	f.byPID[item.ProductID] = item.ID
	return nil
}
func (f *fakeInventoryRepo) FindByProductID(_ context.Context, pid kernel.ID) (*domainInventory.InventoryItem, error) {
	id, ok := f.byPID[pid]
	if !ok {
		return nil, kernel.NewDomainError(kernel.ErrNotFound, "not found")
	}
	return f.items[id], nil
}
func (f *fakeInventoryRepo) FindAll(_ context.Context, _, _ int) ([]*domainInventory.InventoryItem, error) {
	var all []*domainInventory.InventoryItem
	for _, item := range f.items {
		all = append(all, item)
	}
	return all, nil
}
func (f *fakeInventoryRepo) FindLowStock(_ context.Context, threshold int) ([]*domainInventory.InventoryItem, error) {
	var low []*domainInventory.InventoryItem
	for _, item := range f.items {
		if item.QuantityAvailable <= threshold {
			low = append(low, item)
		}
	}
	return low, nil
}
func (f *fakeInventoryRepo) Delete(_ context.Context, _ kernel.ID) error { return nil }

// ---------------------------------------------------------------------------
// Payment fakes
// ---------------------------------------------------------------------------

type fakeMandateRepo struct {
	mandates map[kernel.ID]*domainPayment.Mandate
	byUID    map[kernel.ID][]*domainPayment.Mandate
}

func newFakeMandateRepo() *fakeMandateRepo {
	return &fakeMandateRepo{
		mandates: make(map[kernel.ID]*domainPayment.Mandate),
		byUID:    make(map[kernel.ID][]*domainPayment.Mandate),
	}
}
func (f *fakeMandateRepo) Save(_ context.Context, m *domainPayment.Mandate) error {
	f.mandates[m.ID] = m
	f.byUID[m.UserID] = append(f.byUID[m.UserID], m)
	return nil
}
func (f *fakeMandateRepo) FindByID(_ context.Context, id kernel.ID) (*domainPayment.Mandate, error) {
	m, ok := f.mandates[id]
	if !ok {
		return nil, kernel.NewDomainError(kernel.ErrNotFound, "not found")
	}
	return m, nil
}
func (f *fakeMandateRepo) FindByUserID(_ context.Context, uid kernel.ID) ([]*domainPayment.Mandate, error) {
	return f.byUID[uid], nil
}
func (f *fakeMandateRepo) FindActiveByUser(_ context.Context, _ kernel.ID) ([]*domainPayment.Mandate, error) {
	return nil, nil
}
func (f *fakeMandateRepo) Delete(_ context.Context, _ kernel.ID) error { return nil }

// ---------------------------------------------------------------------------
// Cart tests
// ---------------------------------------------------------------------------

func TestMCP_CartTools(t *testing.T) {
	sf, _ := kernel.NewSnowflake(1)
	repo := newFakeCartRepo()
	svc := domainCart.NewCartService(repo, fakeCartPub{}, fakeLoggerMCP{})
	router := NewMCPRouter()
	router.Register(NewCartMCPHandler(svc, sf))

	// add_cart_item
	w := rpcCall(t, router, "tools/call", "add_cart_item", map[string]any{
		"user_id":    100,
		"product_id": 200,
		"sku":        "SKU-001",
		"name":       "Test Item",
		"quantity":   2,
		"unit_price": 1500,
	})
	assertOK(t, w)

	// get_cart
	w = rpcCall(t, router, "tools/call", "get_cart", map[string]any{
		"user_id": 100,
	})
	assertOK(t, w)
	var resp map[string]any
	json.Unmarshal(w.Body.Bytes(), &resp)
	content := resp["result"].(map[string]any)["content"].([]any)
	var cartData map[string]any
	json.Unmarshal([]byte(content[0].(map[string]any)["text"].(string)), &cartData)
	if cartData["item_count"].(float64) != 1 {
		t.Fatalf("expected 1 item, got %v", cartData["item_count"])
	}

	// update_cart_item
	w = rpcCall(t, router, "tools/call", "update_cart_item", map[string]any{
		"user_id":    100,
		"product_id": 200,
		"quantity":   5,
	})
	assertOK(t, w)

	// remove_cart_item
	w = rpcCall(t, router, "tools/call", "remove_cart_item", map[string]any{
		"user_id":    100,
		"product_id": 200,
	})
	assertOK(t, w)

	// clear_cart
	w = rpcCall(t, router, "tools/call", "clear_cart", map[string]any{
		"user_id": 100,
	})
	assertOK(t, w)
}

// ---------------------------------------------------------------------------
// Checkout tests
// ---------------------------------------------------------------------------

func TestMCP_CheckoutTools(t *testing.T) {
	sf, _ := kernel.NewSnowflake(1)
	repo := newFakeCheckoutRepo()
	svc := domainCheckout.NewCheckoutService(
		repo, fakeTaxSvc{}, fakePriceCalc{}, fakeCheckoutPub{}, fakeLoggerMCP{}, fakeMandateVerifier{},
	)
	router := NewMCPRouter()
	router.Register(NewCheckoutMCPHandler(svc, sf))

	// create_checkout
	itemsJSON, _ := json.Marshal([]map[string]any{
		{"product_id": 1, "sku": "SKU-001", "name": "Item 1", "quantity": 2, "unit_price": 1000},
	})
	w := rpcCall(t, router, "tools/call", "create_checkout", map[string]any{
		"user_id": 100,
		"cart_id": 99,
		"items":   string(itemsJSON),
	})
	assertOK(t, w)

	var resp map[string]any
	json.Unmarshal(w.Body.Bytes(), &resp)
	content := resp["result"].(map[string]any)["content"].([]any)
	checkoutID := int64ID(t, content[0].(map[string]any)["text"].(string), "id")

	// get_checkout
	w = rpcCall(t, router, "tools/call", "get_checkout", map[string]any{
		"id": checkoutID,
	})
	assertOK(t, w)

	// set_shipping_address
	w = rpcCall(t, router, "tools/call", "set_shipping_address", map[string]any{
		"id":          checkoutID,
		"line1":       "123 Main St",
		"city":        "Springfield",
		"state":       "IL",
		"postal_code": "62701",
		"country":     "US",
	})
	assertOK(t, w)

	// set_billing_address
	w = rpcCall(t, router, "tools/call", "set_billing_address", map[string]any{
		"id":          checkoutID,
		"line1":       "456 Oak Ave",
		"city":        "Springfield",
		"state":       "IL",
		"postal_code": "62702",
		"country":     "US",
	})
	assertOK(t, w)

	// select_shipping_option
	w = rpcCall(t, router, "tools/call", "select_shipping_option", map[string]any{
		"id":        checkoutID,
		"option_id": "standard",
		"name":      "Standard Shipping",
		"cost":      500,
	})
	assertOK(t, w)

	// select_payment_handler
	w = rpcCall(t, router, "tools/call", "select_payment_handler", map[string]any{
		"id":      checkoutID,
		"handler": "ap2_mandate",
	})
	assertOK(t, w)

	// cancel_checkout
	w = rpcCall(t, router, "tools/call", "cancel_checkout", map[string]any{
		"id": checkoutID,
	})
	assertOK(t, w)
}

// ---------------------------------------------------------------------------
// Order tests
// ---------------------------------------------------------------------------

func TestMCP_OrderTools(t *testing.T) {
	sf, _ := kernel.NewSnowflake(1)
	repo := newFakeOrderRepo()
	svc := domainOrder.NewOrderService(repo, fakeOrderPub{}, fakeLoggerMCP{})
	router := NewMCPRouter()
	router.Register(NewOrderMCPHandler(svc))

	// Seed an order via completed checkout
	checkoutSf, _ := kernel.NewSnowflake(2)
	oID, _ := sf.NextID()
	checkoutID, _ := checkoutSf.NextID()
	cartCheckoutID, _ := sf.NextID()
	session, err := domainCheckout.NewCheckoutSession(checkoutID, 100, cartCheckoutID,
		domainCheckout.NewCartSnapshot([]domainCheckout.CartSnapshotItem{
			{ProductID: 1, SKU: "SKU-001", Name: "Item 1", Quantity: 2, UnitPrice: 1000},
		}),
	)
	if err != nil {
		t.Fatal(err)
	}
	shipAddr := domainCheckout.Address{Line1: "123 Main", City: "Springfield", State: "IL", PostalCode: "62701", Country: "US"}
	billAddr := domainCheckout.Address{Line1: "456 Oak", City: "Springfield", State: "IL", PostalCode: "62702", Country: "US"}
	shipOpt := domainCheckout.ShippingOption{ID: "standard", Name: "Standard", Cost: 500}
	session.ShippingAddress = &shipAddr
	session.BillingAddress = &billAddr
	session.ShippingOption = &shipOpt
	session.PaymentHandler = "ap2_mandate"
	session.Subtotal = 2000
	session.ShippingCost = 500
	session.TaxAmount = 100
	session.GrandTotal = 2600
	session.Status = domainCheckout.CheckoutStatusCompleted
	order, err := domainOrder.NewOrderFromCheckout(oID, session)
	if err != nil {
		t.Fatal(err)
	}
	if err := repo.Save(context.Background(), order); err != nil {
		t.Fatal(err)
	}

	// list_orders
	w := rpcCall(t, router, "tools/call", "list_orders", map[string]any{
		"user_id": 100,
	})
	assertOK(t, w)

	// get_order
	w = rpcCall(t, router, "tools/call", "get_order", map[string]any{
		"id": oID.Int64(),
	})
	assertOK(t, w)
}

// ---------------------------------------------------------------------------
// Discount tests
// ---------------------------------------------------------------------------

func TestMCP_DiscountTools(t *testing.T) {
	sf, _ := kernel.NewSnowflake(1)
	repo := newFakeDiscountRepo()
	svc := domainDiscount.NewDiscountService(repo, fakeLoggerMCP{})
	router := NewMCPRouter()
	router.Register(NewDiscountMCPHandler(svc, sf))

	// create_discount_code
	w := rpcCall(t, router, "tools/call", "create_discount_code", map[string]any{
		"code":          "SAVE10",
		"discount_type": "percentage",
		"value":         10,
		"max_usages":    100,
	})
	assertOK(t, w)

	// validate_discount_code
	w = rpcCall(t, router, "tools/call", "validate_discount_code", map[string]any{
		"code":     "SAVE10",
		"subtotal": 1000,
	})
	assertOK(t, w)

	// apply_discount_code
	w = rpcCall(t, router, "tools/call", "apply_discount_code", map[string]any{
		"code":     "SAVE10",
		"subtotal": 1000,
	})
	assertOK(t, w)

	var resp map[string]any
	json.Unmarshal(w.Body.Bytes(), &resp)
	content := resp["result"].(map[string]any)["content"].([]any)
	var applyResult map[string]any
	json.Unmarshal([]byte(content[0].(map[string]any)["text"].(string)), &applyResult)
	if applyResult["final_amount"].(float64) != 900 {
		t.Fatalf("expected 900, got %v", applyResult["final_amount"])
	}

	// deactivate_discount_code
	w = rpcCall(t, router, "tools/call", "deactivate_discount_code", map[string]any{
		"code": "SAVE10",
	})
	assertOK(t, w)
}

// ---------------------------------------------------------------------------
// Inventory tests
// ---------------------------------------------------------------------------

func TestMCP_InventoryTools(t *testing.T) {
	sf, _ := kernel.NewSnowflake(1)
	repo := newFakeInventoryRepo()
	svc := domainInventory.NewInventoryService(repo, fakeLoggerMCP{})
	router := NewMCPRouter()
	router.Register(NewInventoryMCPHandler(svc, sf))

	// set_stock
	w := rpcCall(t, router, "tools/call", "set_stock", map[string]any{
		"product_id": 42,
		"quantity":   100,
	})
	assertOK(t, w)

	// get_stock
	w = rpcCall(t, router, "tools/call", "get_stock", map[string]any{
		"product_id": 42,
	})
	assertOK(t, w)

	var resp map[string]any
	json.Unmarshal(w.Body.Bytes(), &resp)
	content := resp["result"].(map[string]any)["content"].([]any)
	var stock map[string]any
	json.Unmarshal([]byte(content[0].(map[string]any)["text"].(string)), &stock)
	if stock["quantity"].(float64) != 100 {
		t.Fatalf("expected 100, got %v", stock["quantity"])
	}

	// list_low_stock
	w = rpcCall(t, router, "tools/call", "list_low_stock", map[string]any{
		"threshold": 10,
	})
	assertOK(t, w)
}

// ---------------------------------------------------------------------------
// Payment tests
// ---------------------------------------------------------------------------

func TestMCP_PaymentTools(t *testing.T) {
	sf, _ := kernel.NewSnowflake(1)
	repo := newFakeMandateRepo()
	svc := domainPayment.NewPaymentService(repo, fakeLoggerMCP{})
	router := NewMCPRouter()
	router.Register(NewPaymentMCPHandler(svc, sf))

	// create_mandate
	w := rpcCall(t, router, "tools/call", "create_mandate", map[string]any{
		"user_id":     100,
		"max_amount":  50000,
		"merchant_id": 1,
	})
	assertOK(t, w)

	var resp map[string]any
	json.Unmarshal(w.Body.Bytes(), &resp)
	content := resp["result"].(map[string]any)["content"].([]any)
	mandateID := int64ID(t, content[0].(map[string]any)["text"].(string), "id")

	// approve_mandate
	w = rpcCall(t, router, "tools/call", "approve_mandate", map[string]any{
		"id":        mandateID,
		"signature": "test-sig",
	})
	assertOK(t, w)

	// execute_mandate
	w = rpcCall(t, router, "tools/call", "execute_mandate", map[string]any{
		"id":    mandateID,
		"token": "test-token",
	})
	assertOK(t, w)

	// list_mandates
	w = rpcCall(t, router, "tools/call", "list_mandates", map[string]any{
		"user_id": 100,
	})
	assertOK(t, w)
}

// ---------------------------------------------------------------------------
// Tools list includes all domains
// ---------------------------------------------------------------------------

func TestMCP_ToolsList_AllDomains(t *testing.T) {
	sf, _ := kernel.NewSnowflake(1)

	router := NewMCPRouter()
	router.Register(NewCatalogMCPHandler(nil))

	// Register all providers
	cartRepo := newFakeCartRepo()
	cartSvc := domainCart.NewCartService(cartRepo, fakeCartPub{}, fakeLoggerMCP{})
	router.Register(NewCartMCPHandler(cartSvc, sf))

	checkoutRepo := newFakeCheckoutRepo()
	checkoutSvc := domainCheckout.NewCheckoutService(
		checkoutRepo, fakeTaxSvc{}, fakePriceCalc{}, fakeCheckoutPub{}, fakeLoggerMCP{}, fakeMandateVerifier{},
	)
	router.Register(NewCheckoutMCPHandler(checkoutSvc, sf))

	orderRepo := newFakeOrderRepo()
	orderSvc := domainOrder.NewOrderService(orderRepo, fakeOrderPub{}, fakeLoggerMCP{})
	router.Register(NewOrderMCPHandler(orderSvc))

	discountRepo := newFakeDiscountRepo()
	discountSvc := domainDiscount.NewDiscountService(discountRepo, fakeLoggerMCP{})
	router.Register(NewDiscountMCPHandler(discountSvc, sf))

	inventoryRepo := newFakeInventoryRepo()
	inventorySvc := domainInventory.NewInventoryService(inventoryRepo, fakeLoggerMCP{})
	router.Register(NewInventoryMCPHandler(inventorySvc, sf))

	paymentRepo := newFakeMandateRepo()
	paymentSvc := domainPayment.NewPaymentService(paymentRepo, fakeLoggerMCP{})
	router.Register(NewPaymentMCPHandler(paymentSvc, sf))

	userRepo := newFakeUserRepoForMCP()
	identitySvc := domainIdentity.NewIdentityService(userRepo, fakeLoggerMCP{})
	router.Register(NewIdentityMCPHandler(identitySvc, sf))

	webhookRepo := newFakeWebhookRepoForMCP()
	webhookSvc := domainOrder.NewWebhookService(webhookRepo, sf)
	router.Register(NewWebhookMCPHandler(webhookSvc))

	router.Register(NewFulfillmentMCPHandler(fakeRateCalculator{}))

	oauthClientRepo := newFakeOAuthClientRepoForMCP()
	oauthCodeRepo := newFakeOAuthCodeRepoForMCP()
	oauthTokenRepo := newFakeOAuthTokenRepoForMCP()
	oauthSvc := domainOAuth.NewOAuthService(oauthClientRepo, oauthCodeRepo, oauthTokenRepo, fakeLoggerMCP{}, []byte("test-secret"))
	router.Register(NewOAuthMCPHandler(oauthSvc))

	catalogSvc := domainCatalog.NewCatalogService(newFakeRepo(), fakeLoggerMCP{})
	orderSvc2 := domainOrder.NewOrderService(orderRepo, fakeOrderPub{}, fakeLoggerMCP{})
	inventorySvc2 := domainInventory.NewInventoryService(inventoryRepo, fakeLoggerMCP{})
	router.Register(NewAdminMCPHandler(catalogSvc, orderSvc2, identitySvc, userRepo, inventorySvc2, sf))

	// tools/list should return all tools from all providers
	w := rpcCall(t, router, "tools/list", "", nil)
	assertOK(t, w)

	var resp map[string]any
	json.Unmarshal(w.Body.Bytes(), &resp)
	tools := resp["result"].(map[string]any)["tools"].([]any)

	expectedTools := map[string]bool{
		"search_catalog": false, "lookup_catalog": false, "get_product": false,
		"get_cart": false, "add_cart_item": false, "update_cart_item": false, "remove_cart_item": false, "clear_cart": false,
		"create_checkout": false, "get_checkout": false,
		"set_shipping_address": false, "set_billing_address": false,
		"select_shipping_option": false, "select_payment_handler": false,
		"complete_checkout": false, "cancel_checkout": false, "select_mandate": false,
		"list_orders": false, "get_order": false,
		"process_order": false, "ship_order": false, "deliver_order": false, "return_order": false, "cancel_order": false,
		"create_discount_code": false, "validate_discount_code": false,
		"apply_discount_code": false, "deactivate_discount_code": false,
		"set_stock": false, "get_stock": false, "list_low_stock": false,
		"create_mandate": false, "list_mandates": false,
		"approve_mandate": false, "execute_mandate": false,
		"get_mandate": false, "settle_mandate": false, "cancel_mandate": false,
		"register_user": false, "login_user": false, "get_user": false, "suspend_user": false,
		"register_webhook": false, "list_webhooks": false, "delete_webhook": false,
		"calculate_rates": false,
		"authorize":       false, "token": false, "revoke": false,
		"create_product": false, "update_product": false, "delete_product": false,
		"list_all_orders": false, "list_users": false, "activate_user": false,
	}

	expectedCount := 3 + 5 + 9 + 7 + 4 + 3 + 7 + 4 + 3 + 1 + 3 + 9 // 58 with 3 duplicates

	for _, tDef := range tools {
		name := tDef.(map[string]any)["name"].(string)
		if _, ok := expectedTools[name]; ok {
			expectedTools[name] = true
		}
	}

	for name, found := range expectedTools {
		if !found {
			t.Errorf("expected tool %q not found in tools/list", name)
		}
	}

	// Verify total count (including duplicate entries)
	if len(tools) != expectedCount {
		t.Errorf("expected %d tool entries, got %d", expectedCount, len(tools))
	}
}

// ---------------------------------------------------------------------------
// Order lifecycle tests (process, ship, deliver, return, cancel)
// ---------------------------------------------------------------------------

func TestMCP_OrderLifecycleTools(t *testing.T) {
	sf, _ := kernel.NewSnowflake(1)
	repo := newFakeOrderRepo()
	svc := domainOrder.NewOrderService(repo, fakeOrderPub{}, fakeLoggerMCP{})
	router := NewMCPRouter()
	router.Register(NewOrderMCPHandler(svc))

	// Seed a confirmed order
	oID, _ := sf.NextID()
	checkoutID, _ := sf.NextID()
	order, err := domainOrder.NewOrderFromCheckout(oID, seededCheckoutSession(checkoutID))
	if err != nil {
		t.Fatal(err)
	}
	if err := repo.Save(context.Background(), order); err != nil {
		t.Fatal(err)
	}

	// process_order
	w := rpcCall(t, router, "tools/call", "process_order", map[string]any{
		"id": oID.Int64(),
	})
	assertOK(t, w)

	// ship_order
	w = rpcCall(t, router, "tools/call", "ship_order", map[string]any{
		"id":              oID.Int64(),
		"tracking_number": "TRACK-001",
		"carrier":         "UPS",
	})
	assertOK(t, w)

	// deliver_order
	w = rpcCall(t, router, "tools/call", "deliver_order", map[string]any{
		"id": oID.Int64(),
	})
	assertOK(t, w)

	// return_order
	w = rpcCall(t, router, "tools/call", "return_order", map[string]any{
		"id": oID.Int64(),
	})
	assertOK(t, w)

	// cancel from returned is invalid, so create a new one for cancel test
	oID2, _ := sf.NextID()
	checkoutID2, _ := sf.NextID()
	order2, err := domainOrder.NewOrderFromCheckout(oID2, seededCheckoutSession(checkoutID2))
	if err != nil {
		t.Fatal(err)
	}
	if err := repo.Save(context.Background(), order2); err != nil {
		t.Fatal(err)
	}

	w = rpcCall(t, router, "tools/call", "cancel_order", map[string]any{
		"id": oID2.Int64(),
	})
	assertOK(t, w)
}

func seededCheckoutSession(id kernel.ID) *domainCheckout.CheckoutSession {
	session, _ := domainCheckout.NewCheckoutSession(id, 100, 99,
		domainCheckout.NewCartSnapshot([]domainCheckout.CartSnapshotItem{
			{ProductID: 1, SKU: "SKU-001", Name: "Item 1", Quantity: 2, UnitPrice: 1000},
		}),
	)
	shipAddr := domainCheckout.Address{Line1: "123 Main", City: "Springfield", State: "IL", PostalCode: "62701", Country: "US"}
	billAddr := domainCheckout.Address{Line1: "456 Oak", City: "Springfield", State: "IL", PostalCode: "62702", Country: "US"}
	shipOpt := domainCheckout.ShippingOption{ID: "standard", Name: "Standard", Cost: 500}
	session.ShippingAddress = &shipAddr
	session.BillingAddress = &billAddr
	session.ShippingOption = &shipOpt
	session.PaymentHandler = "mock"
	session.Subtotal = 2000
	session.ShippingCost = 500
	session.TaxAmount = 100
	session.GrandTotal = 2600
	session.Status = domainCheckout.CheckoutStatusCompleted
	return session
}

// ---------------------------------------------------------------------------
// Payment new tools (get, settle, cancel mandate)
// ---------------------------------------------------------------------------

func TestMCP_PaymentNewTools(t *testing.T) {
	sf, _ := kernel.NewSnowflake(1)
	repo := newFakeMandateRepo()
	svc := domainPayment.NewPaymentService(repo, fakeLoggerMCP{})
	router := NewMCPRouter()
	router.Register(NewPaymentMCPHandler(svc, sf))

	// create mandate
	w := rpcCall(t, router, "tools/call", "create_mandate", map[string]any{
		"user_id":     100,
		"max_amount":  50000,
		"merchant_id": 1,
	})
	assertOK(t, w)

	var resp map[string]any
	json.Unmarshal(w.Body.Bytes(), &resp)
	content := resp["result"].(map[string]any)["content"].([]any)
	mandateID := int64ID(t, content[0].(map[string]any)["text"].(string), "id")

	// get_mandate
	w = rpcCall(t, router, "tools/call", "get_mandate", map[string]any{
		"id": mandateID,
	})
	assertOK(t, w)

	// approve + execute so settle works
	w = rpcCall(t, router, "tools/call", "approve_mandate", map[string]any{
		"id":        mandateID,
		"signature": "test-sig",
	})
	assertOK(t, w)
	w = rpcCall(t, router, "tools/call", "execute_mandate", map[string]any{
		"id":    mandateID,
		"token": "test-token",
	})
	assertOK(t, w)

	// settle_mandate
	w = rpcCall(t, router, "tools/call", "settle_mandate", map[string]any{
		"id": mandateID,
	})
	assertOK(t, w)

	// cancel_mandate — create a fresh mandate and cancel before settling
	w = rpcCall(t, router, "tools/call", "create_mandate", map[string]any{
		"user_id":     100,
		"max_amount":  10000,
		"merchant_id": 1,
	})
	assertOK(t, w)
	var createResp map[string]any
	json.Unmarshal(w.Body.Bytes(), &createResp)
	createContent := createResp["result"].(map[string]any)["content"].([]any)
	cancelID := int64ID(t, createContent[0].(map[string]any)["text"].(string), "id")

	w = rpcCall(t, router, "tools/call", "cancel_mandate", map[string]any{
		"id": cancelID,
	})
	assertOK(t, w)
}

// ---------------------------------------------------------------------------
// Checkout select_mandate
// ---------------------------------------------------------------------------

func TestMCP_CheckoutSelectMandate(t *testing.T) {
	sf, _ := kernel.NewSnowflake(1)
	repo := newFakeCheckoutRepo()
	svc := domainCheckout.NewCheckoutService(
		repo, fakeTaxSvc{}, fakePriceCalc{}, fakeCheckoutPub{}, fakeLoggerMCP{}, fakeMandateVerifier{},
	)
	router := NewMCPRouter()
	router.Register(NewCheckoutMCPHandler(svc, sf))

	itemsJSON, _ := json.Marshal([]map[string]any{
		{"product_id": 1, "sku": "SKU-001", "name": "Item 1", "quantity": 2, "unit_price": 1000},
	})
	w := rpcCall(t, router, "tools/call", "create_checkout", map[string]any{
		"user_id": 100,
		"cart_id": 99,
		"items":   string(itemsJSON),
	})
	assertOK(t, w)

	var resp map[string]any
	json.Unmarshal(w.Body.Bytes(), &resp)
	content := resp["result"].(map[string]any)["content"].([]any)
	checkoutID := int64ID(t, content[0].(map[string]any)["text"].(string), "id")

	w = rpcCall(t, router, "tools/call", "select_mandate", map[string]any{
		"id":         checkoutID,
		"mandate_id": 42,
	})
	assertOK(t, w)
}

// ---------------------------------------------------------------------------
// Identity fakes
// ---------------------------------------------------------------------------

type fakeUserRepoForMCP struct {
	users   map[kernel.ID]*domainIdentity.User
	byEmail map[string]kernel.ID
}

func newFakeUserRepoForMCP() *fakeUserRepoForMCP {
	return &fakeUserRepoForMCP{
		users:   make(map[kernel.ID]*domainIdentity.User),
		byEmail: make(map[string]kernel.ID),
	}
}

func (f *fakeUserRepoForMCP) Save(_ context.Context, u *domainIdentity.User) error {
	f.users[u.ID] = u
	f.byEmail[u.Email] = u.ID
	return nil
}
func (f *fakeUserRepoForMCP) FindByID(_ context.Context, id kernel.ID) (*domainIdentity.User, error) {
	u, ok := f.users[id]
	if !ok {
		return nil, kernel.NewDomainError(kernel.ErrNotFound, "not found")
	}
	return u, nil
}
func (f *fakeUserRepoForMCP) FindByEmail(_ context.Context, email string) (*domainIdentity.User, error) {
	id, ok := f.byEmail[email]
	if !ok {
		return nil, kernel.NewDomainError(kernel.ErrNotFound, "not found")
	}
	return f.users[id], nil
}
func (f *fakeUserRepoForMCP) FindAll(_ context.Context, _, _ int) ([]*domainIdentity.User, error) {
	var all []*domainIdentity.User
	for _, u := range f.users {
		all = append(all, u)
	}
	return all, nil
}
func (f *fakeUserRepoForMCP) Delete(_ context.Context, _ kernel.ID) error { return nil }

// ---------------------------------------------------------------------------
// Identity MCP tests
// ---------------------------------------------------------------------------

func TestMCP_IdentityTools(t *testing.T) {
	sf, _ := kernel.NewSnowflake(1)
	userRepo := newFakeUserRepoForMCP()
	identitySvc := domainIdentity.NewIdentityService(userRepo, fakeLoggerMCP{})
	router := NewMCPRouter()
	router.Register(NewIdentityMCPHandler(identitySvc, sf))

	// register_user
	w := rpcCall(t, router, "tools/call", "register_user", map[string]any{
		"email":    "test@example.com",
		"password": "password123",
		"name":     "Test User",
	})
	assertOK(t, w)

	var resp map[string]any
	json.Unmarshal(w.Body.Bytes(), &resp)
	content := resp["result"].(map[string]any)["content"].([]any)
	var user map[string]any
	json.Unmarshal([]byte(content[0].(map[string]any)["text"].(string)), &user)
	userID := int64ID(t, content[0].(map[string]any)["text"].(string), "id")

	if user["email"] != "test@example.com" {
		t.Fatalf("expected test@example.com, got %v", user["email"])
	}

	// login_user
	w = rpcCall(t, router, "tools/call", "login_user", map[string]any{
		"email":    "test@example.com",
		"password": "password123",
	})
	assertOK(t, w)

	// get_user
	w = rpcCall(t, router, "tools/call", "get_user", map[string]any{
		"id": userID,
	})
	assertOK(t, w)

	// suspend_user
	w = rpcCall(t, router, "tools/call", "suspend_user", map[string]any{
		"id": userID,
	})
	assertOK(t, w)
}

// ---------------------------------------------------------------------------
// Webhook MCP tests
// ---------------------------------------------------------------------------

type fakeWebhookRepoForMCP struct {
	webhooks map[kernel.ID]*domainOrder.Webhook
	byUser   map[kernel.ID][]*domainOrder.Webhook
}

func newFakeWebhookRepoForMCP() *fakeWebhookRepoForMCP {
	return &fakeWebhookRepoForMCP{
		webhooks: make(map[kernel.ID]*domainOrder.Webhook),
		byUser:   make(map[kernel.ID][]*domainOrder.Webhook),
	}
}

func (f *fakeWebhookRepoForMCP) Save(_ context.Context, w *domainOrder.Webhook) error {
	f.webhooks[w.ID] = w
	f.byUser[w.UserID] = append(f.byUser[w.UserID], w)
	return nil
}
func (f *fakeWebhookRepoForMCP) FindByID(_ context.Context, id kernel.ID) (*domainOrder.Webhook, error) {
	w, ok := f.webhooks[id]
	if !ok {
		return nil, kernel.NewDomainError(kernel.ErrNotFound, "not found")
	}
	return w, nil
}
func (f *fakeWebhookRepoForMCP) FindByUserID(_ context.Context, uid kernel.ID) ([]*domainOrder.Webhook, error) {
	return f.byUser[uid], nil
}
func (f *fakeWebhookRepoForMCP) FindByEvent(_ context.Context, _ string) ([]*domainOrder.Webhook, error) {
	return nil, nil
}
func (f *fakeWebhookRepoForMCP) Delete(_ context.Context, id kernel.ID) error {
	delete(f.webhooks, id)
	return nil
}

func TestMCP_WebhookTools(t *testing.T) {
	sf, _ := kernel.NewSnowflake(1)
	repo := newFakeWebhookRepoForMCP()
	svc := domainOrder.NewWebhookService(repo, sf)
	router := NewMCPRouter()
	router.Register(NewWebhookMCPHandler(svc))

	// register_webhook
	w := rpcCall(t, router, "tools/call", "register_webhook", map[string]any{
		"user_id": 100,
		"url":     "https://example.com/webhook",
		"secret":  "my-secret",
		"events":  `["order.confirmed","order.shipped"]`,
	})
	assertOK(t, w)

	var resp map[string]any
	json.Unmarshal(w.Body.Bytes(), &resp)
	content := resp["result"].(map[string]any)["content"].([]any)
	webhookID := int64ID(t, content[0].(map[string]any)["text"].(string), "id")

	// list_webhooks
	w = rpcCall(t, router, "tools/call", "list_webhooks", map[string]any{
		"user_id": 100,
	})
	assertOK(t, w)

	// delete_webhook
	w = rpcCall(t, router, "tools/call", "delete_webhook", map[string]any{
		"user_id": 100,
		"id":      webhookID,
	})
	assertOK(t, w)
}

// ---------------------------------------------------------------------------
// Fulfillment MCP tests
// ---------------------------------------------------------------------------

type fakeRateCalculator struct{}

func (fakeRateCalculator) CalculateRates(_ context.Context, _ domainFulfillment.RateInput) (*domainFulfillment.RateResult, error) {
	return &domainFulfillment.RateResult{
		Options: []domainFulfillment.ShippingOption{
			{ID: "standard", Name: "Standard", Cost: 500, Estimated: "5-8 days", Carrier: "Test"},
			{ID: "express", Name: "Express", Cost: 1500, Estimated: "2-3 days", Carrier: "Test"},
		},
	}, nil
}

func TestMCP_FulfillmentTools(t *testing.T) {
	router := NewMCPRouter()
	router.Register(NewFulfillmentMCPHandler(fakeRateCalculator{}))

	w := rpcCall(t, router, "tools/call", "calculate_rates", map[string]any{
		"country": "US",
	})
	assertOK(t, w)
}

// ---------------------------------------------------------------------------
// OAuth fakes
// ---------------------------------------------------------------------------

type fakeOAuthClientRepoForMCP struct {
	clients map[string]*domainOAuth.OAuthClient
}

func newFakeOAuthClientRepoForMCP() *fakeOAuthClientRepoForMCP {
	return &fakeOAuthClientRepoForMCP{clients: make(map[string]*domainOAuth.OAuthClient)}
}
func (f *fakeOAuthClientRepoForMCP) Save(_ context.Context, c *domainOAuth.OAuthClient) error {
	f.clients[c.ClientID] = c
	return nil
}
func (f *fakeOAuthClientRepoForMCP) FindByClientID(_ context.Context, id string) (*domainOAuth.OAuthClient, error) {
	c, ok := f.clients[id]
	if !ok {
		return nil, kernel.NewDomainError(kernel.ErrNotFound, "not found")
	}
	return c, nil
}
func (f *fakeOAuthClientRepoForMCP) FindByID(_ context.Context, id kernel.ID) (*domainOAuth.OAuthClient, error) {
	for _, c := range f.clients {
		if c.ID == id {
			return c, nil
		}
	}
	return nil, kernel.NewDomainError(kernel.ErrNotFound, "not found")
}

type fakeOAuthCodeRepoForMCP struct {
	codes map[string]*domainOAuth.AuthorizationCode
}

func newFakeOAuthCodeRepoForMCP() *fakeOAuthCodeRepoForMCP {
	return &fakeOAuthCodeRepoForMCP{codes: make(map[string]*domainOAuth.AuthorizationCode)}
}
func (f *fakeOAuthCodeRepoForMCP) Save(_ context.Context, c *domainOAuth.AuthorizationCode) error {
	f.codes[c.Code] = c
	return nil
}
func (f *fakeOAuthCodeRepoForMCP) FindByCode(_ context.Context, code string) (*domainOAuth.AuthorizationCode, error) {
	c, ok := f.codes[code]
	if !ok {
		return nil, kernel.NewDomainError(kernel.ErrNotFound, "not found")
	}
	return c, nil
}
func (f *fakeOAuthCodeRepoForMCP) Delete(_ context.Context, code string) error {
	delete(f.codes, code)
	return nil
}

type fakeOAuthTokenRepoForMCP struct {
	tokens map[string]*domainOAuth.RefreshToken
}

func newFakeOAuthTokenRepoForMCP() *fakeOAuthTokenRepoForMCP {
	return &fakeOAuthTokenRepoForMCP{tokens: make(map[string]*domainOAuth.RefreshToken)}
}
func (f *fakeOAuthTokenRepoForMCP) Save(_ context.Context, t *domainOAuth.RefreshToken) error {
	f.tokens[t.Hash] = t
	return nil
}
func (f *fakeOAuthTokenRepoForMCP) FindByID(_ context.Context, id string) (*domainOAuth.RefreshToken, error) {
	t, ok := f.tokens[id]
	if !ok {
		return nil, kernel.NewDomainError(kernel.ErrNotFound, "not found")
	}
	return t, nil
}
func (f *fakeOAuthTokenRepoForMCP) Revoke(_ context.Context, id string) error {
	t, ok := f.tokens[id]
	if ok {
		t.Revoke()
	}
	return nil
}
func (f *fakeOAuthTokenRepoForMCP) Delete(_ context.Context, _ string) error { return nil }

// ---------------------------------------------------------------------------
// OAuth MCP tests
// ---------------------------------------------------------------------------

func TestMCP_OAuthTools(t *testing.T) {
	clientRepo := newFakeOAuthClientRepoForMCP()
	codeRepo := newFakeOAuthCodeRepoForMCP()
	tokenRepo := newFakeOAuthTokenRepoForMCP()

	// Seed a client
	client, err := domainOAuth.NewClient(1, "test-client", "test-secret", []string{"/callback"}, []string{"read", "write"})
	if err != nil {
		t.Fatal(err)
	}
	if err := clientRepo.Save(context.Background(), client); err != nil {
		t.Fatal(err)
	}

	svc := domainOAuth.NewOAuthService(clientRepo, codeRepo, tokenRepo, fakeLoggerMCP{}, []byte("test-secret"))
	router := NewMCPRouter()
	router.Register(NewOAuthMCPHandler(svc))

	// authorize
	w := rpcCall(t, router, "tools/call", "authorize", map[string]any{
		"client_id":    "test-client",
		"redirect_uri": "/callback",
		"scope":        "read",
		"user_id":      100,
	})
	assertOK(t, w)

	var resp map[string]any
	json.Unmarshal(w.Body.Bytes(), &resp)
	content := resp["result"].(map[string]any)["content"].([]any)
	var authResult map[string]any
	json.Unmarshal([]byte(content[0].(map[string]any)["text"].(string)), &authResult)
	code := authResult["code"].(string)

	// token (authorization_code grant)
	w = rpcCall(t, router, "tools/call", "token", map[string]any{
		"grant_type":    "authorization_code",
		"code":          code,
		"client_id":     "test-client",
		"client_secret": "test-secret",
		"redirect_uri":  "/callback",
	})
	assertOK(t, w)

	json.Unmarshal(w.Body.Bytes(), &resp)
	content = resp["result"].(map[string]any)["content"].([]any)
	var tokenData map[string]any
	json.Unmarshal([]byte(content[0].(map[string]any)["text"].(string)), &tokenData)
	refreshToken := tokenData["refresh_token"].(string)

	// token (refresh_token grant)
	w = rpcCall(t, router, "tools/call", "token", map[string]any{
		"grant_type":    "refresh_token",
		"refresh_token": refreshToken,
		"client_id":     "test-client",
		"client_secret": "test-secret",
	})
	assertOK(t, w)

	// revoke
	w = rpcCall(t, router, "tools/call", "revoke", map[string]any{
		"token":         refreshToken,
		"client_id":     "test-client",
		"client_secret": "test-secret",
	})
	assertOK(t, w)
}

// ---------------------------------------------------------------------------
// Admin MCP tests
// ---------------------------------------------------------------------------

func TestMCP_AdminTools(t *testing.T) {
	sf, _ := kernel.NewSnowflake(1)
	userRepo := newFakeUserRepoForMCP()
	orderRepo := newFakeOrderRepo()
	catalogRepo := newFakeRepo()
	inventoryRepo := newFakeInventoryRepo()

	// Seed an admin user
	adminPw, _ := domainIdentity.NewPassword("adminpass123")
	adminUser, err := domainIdentity.NewUser(100, "admin@example.com", "Admin", adminPw, []domainIdentity.UserRole{domainIdentity.UserRoleAdmin})
	if err != nil {
		t.Fatal(err)
	}
	if err := userRepo.Save(context.Background(), adminUser); err != nil {
		t.Fatal(err)
	}

	// Create services
	catalogSvc := domainCatalog.NewCatalogService(catalogRepo, fakeLoggerMCP{})
	orderSvc := domainOrder.NewOrderService(orderRepo, fakeOrderPub{}, fakeLoggerMCP{})
	identitySvc := domainIdentity.NewIdentityService(userRepo, fakeLoggerMCP{})
	inventorySvc := domainInventory.NewInventoryService(inventoryRepo, fakeLoggerMCP{})

	router := NewMCPRouter()
	router.Register(NewAdminMCPHandler(catalogSvc, orderSvc, identitySvc, userRepo, inventorySvc, sf))

	// create_product
	w := rpcCall(t, router, "tools/call", "create_product", map[string]any{
		"admin_user_id": 100,
		"sku":           "ADM-PROD-001",
		"name":          "Admin Product",
		"description":   "Created via admin MCP",
		"category":      "test",
		"price_amount":  2999,
		"currency":      "USD",
	})
	assertOK(t, w)

	var resp map[string]any
	json.Unmarshal(w.Body.Bytes(), &resp)
	content := resp["result"].(map[string]any)["content"].([]any)
	productID := int64ID(t, content[0].(map[string]any)["text"].(string), "id")

	// update_product
	w = rpcCall(t, router, "tools/call", "update_product", map[string]any{
		"admin_user_id": 100,
		"id":            productID,
		"name":          "Updated Product",
		"description":   "Updated description",
		"category":      "test",
		"price_amount":  3999,
		"currency":      "USD",
	})
	assertOK(t, w)

	// Seed an order
	oID, _ := sf.NextID()
	checkoutID, _ := sf.NextID()
	order, err := domainOrder.NewOrderFromCheckout(oID, seededCheckoutSession(checkoutID))
	if err != nil {
		t.Fatal(err)
	}
	if err := orderRepo.Save(context.Background(), order); err != nil {
		t.Fatal(err)
	}

	// list_all_orders
	w = rpcCall(t, router, "tools/call", "list_all_orders", map[string]any{
		"admin_user_id": 100,
	})
	assertOK(t, w)

	// Seed a regular (non-admin) user
	userPw, _ := domainIdentity.NewPassword("userpass123")
	regularUser, err := domainIdentity.NewUser(200, "user@example.com", "User", userPw, nil)
	if err != nil {
		t.Fatal(err)
	}
	if err := userRepo.Save(context.Background(), regularUser); err != nil {
		t.Fatal(err)
	}

	// list_users
	w = rpcCall(t, router, "tools/call", "list_users", map[string]any{
		"admin_user_id": 100,
	})
	assertOK(t, w)

	// activate_user (user 200 is already active, so suspend first then activate)
	_, _ = identitySvc.SuspendUser(context.Background(), 200)
	w = rpcCall(t, router, "tools/call", "activate_user", map[string]any{
		"admin_user_id": 100,
		"user_id":       200,
	})
	assertOK(t, w)

	// set_stock
	w = rpcCall(t, router, "tools/call", "set_stock", map[string]any{
		"admin_user_id": 100,
		"product_id":    42,
		"quantity":      100,
	})
	assertOK(t, w)

	// get_stock
	w = rpcCall(t, router, "tools/call", "get_stock", map[string]any{
		"admin_user_id": 100,
		"product_id":    42,
	})
	assertOK(t, w)

	// list_low_stock
	w = rpcCall(t, router, "tools/call", "list_low_stock", map[string]any{
		"admin_user_id": 100,
	})
	assertOK(t, w)

	// delete_product
	w = rpcCall(t, router, "tools/call", "delete_product", map[string]any{
		"admin_user_id": 100,
		"id":            productID,
	})
	assertOK(t, w)
}
