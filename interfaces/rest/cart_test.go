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

	domain "github.com/beeleelee/mall/domain/cart"
	"github.com/beeleelee/mall/domain/kernel"
	"github.com/beeleelee/mall/interfaces/middleware"
)

type fakeCartRepo struct {
	carts map[kernel.ID]*domain.Cart
	byUID map[kernel.ID]kernel.ID
}

func newFakeCartRepo() *fakeCartRepo {
	return &fakeCartRepo{
		carts: make(map[kernel.ID]*domain.Cart),
		byUID: make(map[kernel.ID]kernel.ID),
	}
}

func (f *fakeCartRepo) Save(_ context.Context, cart *domain.Cart) error {
	f.carts[cart.ID] = cart
	f.byUID[cart.UserID] = cart.ID
	return nil
}

func (f *fakeCartRepo) FindByID(_ context.Context, id kernel.ID) (*domain.Cart, error) {
	c, ok := f.carts[id]
	if !ok {
		return nil, kernel.NewDomainError(kernel.ErrNotFound, "cart not found")
	}
	return c, nil
}

func (f *fakeCartRepo) FindByUserID(_ context.Context, userID kernel.ID) (*domain.Cart, error) {
	id, ok := f.byUID[userID]
	if !ok {
		return nil, kernel.NewDomainError(kernel.ErrNotFound, "cart not found")
	}
	return f.carts[id], nil
}

func (f *fakeCartRepo) Delete(_ context.Context, id kernel.ID) error {
	c, ok := f.carts[id]
	if !ok {
		return kernel.NewDomainError(kernel.ErrNotFound, "cart not found")
	}
	delete(f.byUID, c.UserID)
	delete(f.carts, id)
	return nil
}

type fakeCartPub struct{}

func (fakeCartPub) PublishCartUpdated(_ context.Context, _ *domain.Cart) error { return nil }

func newTestCartHandler(t *testing.T) *CartHandler {
	t.Helper()
	repo := newFakeCartRepo()
	pub := fakeCartPub{}
	logger := fakeLog{}
	svc := domain.NewCartService(repo, pub, logger)
	sf, err := kernel.NewSnowflake(1)
	if err != nil {
		t.Fatalf("NewSnowflake failed: %v", err)
	}
	return NewCartHandler(svc, sf)
}

func authRequestWithVars(t *testing.T, method, path string, body []byte, vars map[string]string) *http.Request {
	t.Helper()
	req := httptest.NewRequest(method, path, bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	req = pathvar.WithVars(req, vars)
	ctx := middleware.ContextWithUser(req.Context(), middleware.UserInfo{UserID: 1})
	return req.WithContext(ctx)
}

func authRequest(t *testing.T, method, path string, body []byte) *http.Request {
	t.Helper()
	req := httptest.NewRequest(method, path, bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	ctx := middleware.ContextWithUser(req.Context(), middleware.UserInfo{UserID: 1})
	return req.WithContext(ctx)
}

func TestCartHandler_CreateOrGet_Success(t *testing.T) {
	h := newTestCartHandler(t)
	req := authRequest(t, http.MethodPost, "/api/v1/carts", []byte("{}"))
	rec := httptest.NewRecorder()
	h.CreateOrGet(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", rec.Code, rec.Body.String())
	}

	var resp cartResponse
	if err := json.NewDecoder(rec.Body).Decode(&resp); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	if resp.UserID != 1 {
		t.Errorf("expected UserID 1, got %d", resp.UserID)
	}
	if resp.Status != string(domain.CartStatusActive) {
		t.Errorf("expected active, got %s", resp.Status)
	}
}

func TestCartHandler_CreateOrGet_Existing(t *testing.T) {
	h := newTestCartHandler(t)
	req := authRequest(t, http.MethodPost, "/api/v1/carts", []byte("{}"))
	h.CreateOrGet(httptest.NewRecorder(), req)

	req2 := authRequest(t, http.MethodPost, "/api/v1/carts", []byte("{}"))
	rec2 := httptest.NewRecorder()
	h.CreateOrGet(rec2, req2)

	if rec2.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", rec2.Code)
	}
}

func TestCartHandler_GetCart_Success(t *testing.T) {
	h := newTestCartHandler(t)
	createReq := authRequest(t, http.MethodPost, "/api/v1/carts", []byte("{}"))
	createRec := httptest.NewRecorder()
	h.CreateOrGet(createRec, createReq)

	var created cartResponse
	json.NewDecoder(createRec.Body).Decode(&created)

	req := authRequestWithVars(t, http.MethodGet, "/api/v1/carts/"+strconv.FormatInt(created.ID, 10), nil, map[string]string{"id": strconv.FormatInt(created.ID, 10)})
	rec := httptest.NewRecorder()
	h.GetCart(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", rec.Code)
	}

	var resp cartResponse
	json.NewDecoder(rec.Body).Decode(&resp)
	if resp.ID != created.ID {
		t.Errorf("expected cart ID %d, got %d", created.ID, resp.ID)
	}
}

func TestCartHandler_AddItem_Success(t *testing.T) {
	h := newTestCartHandler(t)
	createReq := authRequest(t, http.MethodPost, "/api/v1/carts", []byte("{}"))
	createRec := httptest.NewRecorder()
	h.CreateOrGet(createRec, createReq)

	var created cartResponse
	json.NewDecoder(createRec.Body).Decode(&created)

	body := map[string]any{
		"product_id": 100,
		"sku":        "SKU001",
		"name":       "Test Product",
		"quantity":   2,
		"unit_price": 1000,
	}
	data, _ := json.Marshal(body)
	req := authRequestWithVars(t, http.MethodPost, "/api/v1/carts/"+strconv.FormatInt(created.ID, 10)+"/items", data, map[string]string{"id": strconv.FormatInt(created.ID, 10)})
	rec := httptest.NewRecorder()
	h.AddItem(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", rec.Code, rec.Body.String())
	}

	var resp cartResponse
	json.NewDecoder(rec.Body).Decode(&resp)
	if len(resp.Items) != 1 {
		t.Fatalf("expected 1 item, got %d", len(resp.Items))
	}
	if resp.Items[0].SKU != "SKU001" {
		t.Errorf("expected SKU001, got %s", resp.Items[0].SKU)
	}
}

func TestCartHandler_UpdateQuantity_Success(t *testing.T) {
	h := newTestCartHandler(t)
	createReq := authRequest(t, http.MethodPost, "/api/v1/carts", []byte("{}"))
	createRec := httptest.NewRecorder()
	h.CreateOrGet(createRec, createReq)

	var created cartResponse
	json.NewDecoder(createRec.Body).Decode(&created)

	addBody := map[string]any{
		"product_id": 100,
		"sku":        "SKU001",
		"name":       "Test",
		"quantity":   2,
		"unit_price": 1000,
	}
	data, _ := json.Marshal(addBody)
	addReq := authRequestWithVars(t, http.MethodPost, "/api/v1/carts/"+strconv.FormatInt(created.ID, 10)+"/items", data, map[string]string{"id": strconv.FormatInt(created.ID, 10)})
	h.AddItem(httptest.NewRecorder(), addReq)

	updateBody := map[string]any{"quantity": 5}
	data, _ = json.Marshal(updateBody)
	req := authRequestWithVars(t, http.MethodPut, "/api/v1/carts/"+strconv.FormatInt(created.ID, 10)+"/items/100", data, map[string]string{"id": strconv.FormatInt(created.ID, 10), "productId": "100"})
	rec := httptest.NewRecorder()
	h.UpdateQuantity(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", rec.Code, rec.Body.String())
	}

	var resp cartResponse
	json.NewDecoder(rec.Body).Decode(&resp)
	if len(resp.Items) != 1 {
		t.Fatalf("expected 1 item, got %d", len(resp.Items))
	}
	if resp.Items[0].Quantity != 5 {
		t.Errorf("expected quantity 5, got %d", resp.Items[0].Quantity)
	}
}

func TestCartHandler_RemoveItem_Success(t *testing.T) {
	h := newTestCartHandler(t)
	createReq := authRequest(t, http.MethodPost, "/api/v1/carts", []byte("{}"))
	createRec := httptest.NewRecorder()
	h.CreateOrGet(createRec, createReq)

	var created cartResponse
	json.NewDecoder(createRec.Body).Decode(&created)

	addBody := map[string]any{
		"product_id": 100,
		"sku":        "SKU001",
		"name":       "Test",
		"quantity":   2,
		"unit_price": 1000,
	}
	data, _ := json.Marshal(addBody)
	addReq := authRequestWithVars(t, http.MethodPost, "/api/v1/carts/"+strconv.FormatInt(created.ID, 10)+"/items", data, map[string]string{"id": strconv.FormatInt(created.ID, 10)})
	h.AddItem(httptest.NewRecorder(), addReq)

	req := authRequestWithVars(t, http.MethodDelete, "/api/v1/carts/"+strconv.FormatInt(created.ID, 10)+"/items/100", nil, map[string]string{"id": strconv.FormatInt(created.ID, 10), "productId": "100"})
	rec := httptest.NewRecorder()
	h.RemoveItem(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", rec.Code, rec.Body.String())
	}

	var resp cartResponse
	json.NewDecoder(rec.Body).Decode(&resp)
	if len(resp.Items) != 0 {
		t.Errorf("expected 0 items, got %d", len(resp.Items))
	}
}

func TestCartHandler_ClearCart_Success(t *testing.T) {
	h := newTestCartHandler(t)
	createReq := authRequest(t, http.MethodPost, "/api/v1/carts", []byte("{}"))
	createRec := httptest.NewRecorder()
	h.CreateOrGet(createRec, createReq)

	var created cartResponse
	json.NewDecoder(createRec.Body).Decode(&created)

	addBody := map[string]any{
		"product_id": 100,
		"sku":        "SKU001",
		"name":       "Test",
		"quantity":   2,
		"unit_price": 1000,
	}
	data, _ := json.Marshal(addBody)
	addReq := authRequestWithVars(t, http.MethodPost, "/api/v1/carts/"+strconv.FormatInt(created.ID, 10)+"/items", data, map[string]string{"id": strconv.FormatInt(created.ID, 10)})
	h.AddItem(httptest.NewRecorder(), addReq)

	req := authRequestWithVars(t, http.MethodDelete, "/api/v1/carts/"+strconv.FormatInt(created.ID, 10), nil, map[string]string{"id": strconv.FormatInt(created.ID, 10)})
	rec := httptest.NewRecorder()
	h.ClearCart(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", rec.Code, rec.Body.String())
	}
}

func TestCartHandler_Unauthenticated(t *testing.T) {
	h := newTestCartHandler(t)
	req := httptest.NewRequest(http.MethodPost, "/api/v1/carts", nil)
	rec := httptest.NewRecorder()
	h.CreateOrGet(rec, req)

	if rec.Code != http.StatusUnauthorized {
		t.Errorf("expected 401, got %d", rec.Code)
	}
}
