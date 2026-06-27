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
	domainCheckout "github.com/beeleelee/mall/domain/checkout"
	domainDiscount "github.com/beeleelee/mall/domain/discount"
	domainInventory "github.com/beeleelee/mall/domain/inventory"
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

func (fakeMandateVerifier) VerifyAndExecute(_ context.Context, _, _ kernel.ID, _ int64) error { return nil }

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
	var chk map[string]any
	json.Unmarshal([]byte(content[0].(map[string]any)["text"].(string)), &chk)
	checkoutID := int64(chk["id"].(float64))

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
	var mandate map[string]any
	json.Unmarshal([]byte(content[0].(map[string]any)["text"].(string)), &mandate)
	mandateID := int64(mandate["id"].(float64))

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
		"complete_checkout": false, "cancel_checkout": false,
		"list_orders": false, "get_order": false,
		"create_discount_code": false, "validate_discount_code": false,
		"apply_discount_code": false, "deactivate_discount_code": false,
		"set_stock": false, "get_stock": false, "list_low_stock": false,
		"create_mandate": false, "list_mandates": false,
		"approve_mandate": false, "execute_mandate": false,
	}

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

	// Verify total count
	toolCount := 0
	for _, v := range expectedTools {
		if v {
			toolCount++
		}
	}
	if len(tools) != toolCount {
		t.Errorf("expected %d tools, got %d", toolCount, len(tools))
	}
}
