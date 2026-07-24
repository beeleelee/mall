package rest

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strconv"
	"sync"
	"testing"

	"github.com/zeromicro/go-zero/rest/pathvar"

	appidentity "github.com/beeleelee/mall/application/identity"
	catalogdomain "github.com/beeleelee/mall/domain/catalog"
	domainidentity "github.com/beeleelee/mall/domain/identity"
	"github.com/beeleelee/mall/domain/inventory"
	"github.com/beeleelee/mall/domain/kernel"
	orderdomain "github.com/beeleelee/mall/domain/order"
	reviewdomain "github.com/beeleelee/mall/domain/review"
)

type fakeInventoryRepo struct {
	mu  sync.Mutex
	byP map[kernel.ID]*inventory.InventoryItem
}

func newFakeInventoryRepo() *fakeInventoryRepo {
	return &fakeInventoryRepo{byP: make(map[kernel.ID]*inventory.InventoryItem)}
}

func (f *fakeInventoryRepo) Save(_ context.Context, item *inventory.InventoryItem) error {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.byP[item.ProductID] = item
	return nil
}

func (f *fakeInventoryRepo) FindByProductID(_ context.Context, productID kernel.ID) (*inventory.InventoryItem, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	item, ok := f.byP[productID]
	if !ok {
		return nil, kernel.NewDomainError(kernel.ErrNotFound, "inventory not found")
	}
	return item, nil
}

func (f *fakeInventoryRepo) FindAll(_ context.Context, offset, limit int) ([]*inventory.InventoryItem, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	var result []*inventory.InventoryItem
	for _, item := range f.byP {
		result = append(result, item)
	}
	return result, nil
}

func (f *fakeInventoryRepo) FindLowStock(_ context.Context, threshold int) ([]*inventory.InventoryItem, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	var result []*inventory.InventoryItem
	for _, item := range f.byP {
		if item.QuantityAvailable <= threshold {
			result = append(result, item)
		}
	}
	return result, nil
}

func (f *fakeInventoryRepo) Delete(_ context.Context, id kernel.ID) error {
	f.mu.Lock()
	defer f.mu.Unlock()
	for pid, item := range f.byP {
		if item.ID == id {
			delete(f.byP, pid)
			return nil
		}
	}
	return kernel.NewDomainError(kernel.ErrNotFound, "not found")
}

type fakeCategoryRepo struct {
	mu         sync.Mutex
	categories map[kernel.ID]*catalogdomain.Category
}

func newFakeCategoryRepo() *fakeCategoryRepo {
	return &fakeCategoryRepo{categories: make(map[kernel.ID]*catalogdomain.Category)}
}

func (f *fakeCategoryRepo) Save(_ context.Context, cat *catalogdomain.Category) error {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.categories[cat.ID] = cat
	return nil
}

func (f *fakeCategoryRepo) FindByID(_ context.Context, id kernel.ID) (*catalogdomain.Category, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	cat, ok := f.categories[id]
	if !ok {
		return nil, kernel.NewDomainError(kernel.ErrNotFound, "category not found")
	}
	return cat, nil
}

func (f *fakeCategoryRepo) FindBySlug(_ context.Context, slug string) (*catalogdomain.Category, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	for _, cat := range f.categories {
		if cat.Slug == slug {
			return cat, nil
		}
	}
	return nil, kernel.NewDomainError(kernel.ErrNotFound, "category not found")
}

func (f *fakeCategoryRepo) FindAll(_ context.Context) ([]*catalogdomain.Category, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	var result []*catalogdomain.Category
	for _, cat := range f.categories {
		result = append(result, cat)
	}
	return result, nil
}

func (f *fakeCategoryRepo) FindChildren(_ context.Context, parentID kernel.ID) ([]*catalogdomain.Category, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	var result []*catalogdomain.Category
	for _, cat := range f.categories {
		if cat.ParentID == parentID {
			result = append(result, cat)
		}
	}
	return result, nil
}

func (f *fakeCategoryRepo) Delete(_ context.Context, id kernel.ID) error {
	f.mu.Lock()
	defer f.mu.Unlock()
	delete(f.categories, id)
	return nil
}

type fakeStorageSvc struct{}

func (fakeStorageSvc) Upload(_ context.Context, key string, _ *bytes.Reader, _ string) (string, error) {
	return "https://storage.example.com/" + key, nil
}
func (fakeStorageSvc) Delete(_ context.Context, key string) error { return nil }

type fakeRefundSvc struct{}

func (fakeRefundSvc) RequestRefund(ctx context.Context, orderID kernel.ID, reason string) (*orderdomain.Refund, error) {
	return &orderdomain.Refund{}, nil
}
func (fakeRefundSvc) ApproveRefund(ctx context.Context, refundID kernel.ID) (*orderdomain.Refund, error) {
	return &orderdomain.Refund{}, nil
}
func (fakeRefundSvc) ProcessRefund(ctx context.Context, refundID kernel.ID) (*orderdomain.Refund, error) {
	return &orderdomain.Refund{}, nil
}
func (fakeRefundSvc) RejectRefund(ctx context.Context, refundID kernel.ID, reason string) (*orderdomain.Refund, error) {
	return &orderdomain.Refund{}, nil
}
func (fakeRefundSvc) ListRefunds(ctx context.Context, userID kernel.ID) ([]*orderdomain.Refund, error) {
	return nil, nil
}
func (fakeRefundSvc) ListAllRefunds(ctx context.Context) ([]*orderdomain.Refund, error) {
	return nil, nil
}

func TestAdminHandler_ListOrders_Success(t *testing.T) {
	f := newAdminOrderTestFixture(t)
	f.seedOrder(t, 1)
	f.seedOrder(t, 2)

	req := httptest.NewRequest(http.MethodGet, "/api/v1/admin/orders?limit=10", nil)
	rec := httptest.NewRecorder()
	f.handler.ListOrders(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", rec.Code)
	}
	var resp []map[string]any
	json.NewDecoder(rec.Body).Decode(&resp)
	if len(resp) != 2 {
		t.Errorf("expected 2 orders, got %d", len(resp))
	}
}

func TestAdminHandler_ListOrders_Empty(t *testing.T) {
	f := newAdminOrderTestFixture(t)
	req := httptest.NewRequest(http.MethodGet, "/api/v1/admin/orders?limit=10", nil)
	rec := httptest.NewRecorder()
	f.handler.ListOrders(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", rec.Code)
	}
	var resp []map[string]any
	json.NewDecoder(rec.Body).Decode(&resp)
	if len(resp) != 0 {
		t.Errorf("expected 0 orders, got %d", len(resp))
	}
}

func TestAdminHandler_CreateProduct_Success(t *testing.T) {
	sf, _ := kernel.NewSnowflake(1)
	logger := fakeLog{}
	catalogSvc := catalogdomain.NewCatalogService(newFakeCatalogRepo(), logger)
	h := &AdminHandler{catalogSvc: catalogSvc, sf: sf}

	body, _ := json.Marshal(map[string]any{
		"sku": "SKU001", "name": "Test Product", "description": "A test",
		"category": "electronics", "price_amount": 2999, "currency": "USD",
	})
	req := httptest.NewRequest(http.MethodPost, "/api/v1/admin/products", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()
	h.CreateProduct(rec, req)

	if rec.Code != http.StatusCreated {
		t.Fatalf("expected 201, got %d: %s", rec.Code, rec.Body.String())
	}
	var resp map[string]any
	json.NewDecoder(rec.Body).Decode(&resp)
	if resp["sku"] != "SKU001" {
		t.Errorf("expected SKU001, got %v", resp["sku"])
	}
}

func TestAdminHandler_CreateProduct_InvalidBody(t *testing.T) {
	sf, _ := kernel.NewSnowflake(1)
	logger := fakeLog{}
	catalogSvc := catalogdomain.NewCatalogService(newFakeCatalogRepo(), logger)
	h := &AdminHandler{catalogSvc: catalogSvc, sf: sf}

	req := httptest.NewRequest(http.MethodPost, "/api/v1/admin/products", bytes.NewReader([]byte("not json")))
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()
	h.CreateProduct(rec, req)

	if rec.Code != http.StatusBadRequest {
		t.Errorf("expected 400, got %d", rec.Code)
	}
}

func TestAdminHandler_DeleteProduct_Success(t *testing.T) {
	sf, _ := kernel.NewSnowflake(1)
	logger := fakeLog{}
	catalogSvc := catalogdomain.NewCatalogService(newFakeCatalogRepo(), logger)
	h := &AdminHandler{catalogSvc: catalogSvc, sf: sf}

	createBody, _ := json.Marshal(map[string]any{
		"sku": "DELETE001", "name": "Delete Me", "description": "Will delete",
		"category": "test", "price_amount": 999, "currency": "USD",
	})
	createReq := httptest.NewRequest(http.MethodPost, "/api/v1/admin/products", bytes.NewReader(createBody))
	createReq.Header.Set("Content-Type", "application/json")
	createRec := httptest.NewRecorder()
	h.CreateProduct(createRec, createReq)

	var created map[string]any
	json.NewDecoder(createRec.Body).Decode(&created)
	pid := strconv.FormatInt(int64(created["id"].(float64)), 10)

	delReq := httptest.NewRequest(http.MethodDelete, "/api/v1/admin/products/"+pid, nil)
	delReq = pathvar.WithVars(delReq, map[string]string{"id": pid})
	delRec := httptest.NewRecorder()
	h.DeleteProduct(delRec, delReq)

	if delRec.Code != http.StatusNoContent {
		t.Errorf("expected 204, got %d", delRec.Code)
	}
}

func TestAdminHandler_ListUsers_Success(t *testing.T) {
	userRepo := newFakeUserRepo()
	logger := fakeLog{}
	domainSvc := domainidentity.NewIdentityService(userRepo, logger)
	sf, _ := kernel.NewSnowflake(1)
	identitySvc := appidentity.NewIdentityAppService(domainSvc, userRepo, newFakeTokenRepo(), logger, sf)

	userBody := map[string]string{"email": "admin-list@example.com", "password": "password123", "name": "List User"}
	userData, _ := json.Marshal(userBody)
	regReq := httptest.NewRequest(http.MethodPost, "/api/v1/auth/register", bytes.NewReader(userData))
	regReq.Header.Set("Content-Type", "application/json")
	regHandler := NewIdentityHandler(identitySvc)
	regHandler.Register(httptest.NewRecorder(), regReq)

	h := &AdminHandler{identitySvc: identitySvc}
	req := httptest.NewRequest(http.MethodGet, "/api/v1/admin/users?limit=10", nil)
	rec := httptest.NewRecorder()
	h.ListUsers(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", rec.Code)
	}
	var users []map[string]any
	json.NewDecoder(rec.Body).Decode(&users)
	if len(users) != 1 {
		t.Errorf("expected 1 user, got %d", len(users))
	}
}

func TestAdminHandler_ActivateUser_Success(t *testing.T) {
	userRepo := newFakeUserRepo()
	logger := fakeLog{}
	domainSvc := domainidentity.NewIdentityService(userRepo, logger)
	sf, _ := kernel.NewSnowflake(1)
	identitySvc := appidentity.NewIdentityAppService(domainSvc, userRepo, newFakeTokenRepo(), logger, sf)

	userBody := map[string]string{"email": "activate-test@example.com", "password": "password123", "name": "Activate"}
	userData, _ := json.Marshal(userBody)
	regReq := httptest.NewRequest(http.MethodPost, "/api/v1/auth/register", bytes.NewReader(userData))
	regReq.Header.Set("Content-Type", "application/json")
	regHandler := NewIdentityHandler(identitySvc)
	regRec := httptest.NewRecorder()
	regHandler.Register(regRec, regReq)
	var regResp map[string]any
	json.NewDecoder(regRec.Body).Decode(&regResp)
	userID := int64(regResp["UserID"].(float64))

	_, _ = identitySvc.SuspendUser(context.Background(), userID)

	h := &AdminHandler{identitySvc: identitySvc}
	uidStr := strconv.FormatInt(userID, 10)
	activateReq := httptest.NewRequest(http.MethodPost, "/api/v1/admin/users/"+uidStr+"/activate", nil)
	activateReq = pathvar.WithVars(activateReq, map[string]string{"id": uidStr})
	activateRec := httptest.NewRecorder()
	h.ActivateUser(activateRec, activateReq)

	if activateRec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", activateRec.Code)
	}
}

func TestAdminHandler_ActivateUser_NotFound(t *testing.T) {
	userRepo := newFakeUserRepo()
	logger := fakeLog{}
	domainSvc := domainidentity.NewIdentityService(userRepo, logger)
	sf, _ := kernel.NewSnowflake(1)
	identitySvc := appidentity.NewIdentityAppService(domainSvc, userRepo, newFakeTokenRepo(), logger, sf)

	h := &AdminHandler{identitySvc: identitySvc}
	req := httptest.NewRequest(http.MethodPost, "/api/v1/admin/users/999/activate", nil)
	req = pathvar.WithVars(req, map[string]string{"id": "999"})
	rec := httptest.NewRecorder()
	h.ActivateUser(rec, req)

	if rec.Code != http.StatusNotFound {
		t.Errorf("expected 404, got %d", rec.Code)
	}
}

func TestAdminHandler_SetStock_Create(t *testing.T) {
	sf, _ := kernel.NewSnowflake(1)
	invRepo := newFakeInventoryRepo()
	logger := fakeLog{}
	invSvc := inventory.NewInventoryService(invRepo, logger)
	h := &AdminHandler{inventorySvc: invSvc, sf: sf}

	body, _ := json.Marshal(map[string]any{"product_id": 100, "quantity": 50})
	req := httptest.NewRequest(http.MethodPost, "/api/v1/admin/inventory", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()
	h.SetStock(rec, req)

	if rec.Code != http.StatusCreated {
		t.Fatalf("expected 201, got %d: %s", rec.Code, rec.Body.String())
	}
}

func TestAdminHandler_SetStock_Update(t *testing.T) {
	sf, _ := kernel.NewSnowflake(1)
	invRepo := newFakeInventoryRepo()
	logger := fakeLog{}
	invSvc := inventory.NewInventoryService(invRepo, logger)
	h := &AdminHandler{inventorySvc: invSvc, sf: sf}

	createBody, _ := json.Marshal(map[string]any{"product_id": 200, "quantity": 30})
	createReq := httptest.NewRequest(http.MethodPost, "/api/v1/admin/inventory", bytes.NewReader(createBody))
	createReq.Header.Set("Content-Type", "application/json")
	h.SetStock(httptest.NewRecorder(), createReq)

	updateBody, _ := json.Marshal(map[string]any{"product_id": 200, "quantity": 60})
	updateReq := httptest.NewRequest(http.MethodPost, "/api/v1/admin/inventory", bytes.NewReader(updateBody))
	updateReq.Header.Set("Content-Type", "application/json")
	updateRec := httptest.NewRecorder()
	h.SetStock(updateRec, updateReq)

	if updateRec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", updateRec.Code)
	}
}

func TestAdminHandler_GetStock_Success(t *testing.T) {
	sf, _ := kernel.NewSnowflake(1)
	invRepo := newFakeInventoryRepo()
	logger := fakeLog{}
	invSvc := inventory.NewInventoryService(invRepo, logger)
	h := &AdminHandler{inventorySvc: invSvc, sf: sf}

	body, _ := json.Marshal(map[string]any{"product_id": 300, "quantity": 25})
	req := httptest.NewRequest(http.MethodPost, "/api/v1/admin/inventory", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	h.SetStock(httptest.NewRecorder(), req)

	getReq := httptest.NewRequest(http.MethodGet, "/api/v1/admin/inventory/300", nil)
	getReq = pathvar.WithVars(getReq, map[string]string{"productId": "300"})
	getRec := httptest.NewRecorder()
	h.GetStock(getRec, getReq)

	if getRec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", getRec.Code)
	}
}

func TestAdminHandler_GetStock_NotFound(t *testing.T) {
	sf, _ := kernel.NewSnowflake(1)
	invRepo := newFakeInventoryRepo()
	logger := fakeLog{}
	invSvc := inventory.NewInventoryService(invRepo, logger)
	h := &AdminHandler{inventorySvc: invSvc, sf: sf}

	req := httptest.NewRequest(http.MethodGet, "/api/v1/admin/inventory/999", nil)
	req = pathvar.WithVars(req, map[string]string{"productId": "999"})
	rec := httptest.NewRecorder()
	h.GetStock(rec, req)

	if rec.Code != http.StatusNotFound {
		t.Errorf("expected 404, got %d", rec.Code)
	}
}

func TestAdminHandler_ListLowStock_Success(t *testing.T) {
	sf, _ := kernel.NewSnowflake(1)
	invRepo := newFakeInventoryRepo()
	logger := fakeLog{}
	invSvc := inventory.NewInventoryService(invRepo, logger)
	h := &AdminHandler{inventorySvc: invSvc, sf: sf}

	body1, _ := json.Marshal(map[string]any{"product_id": 1, "quantity": 5, "low_stock_threshold": 10})
	req1 := httptest.NewRequest(http.MethodPost, "/api/v1/admin/inventory", bytes.NewReader(body1))
	req1.Header.Set("Content-Type", "application/json")
	h.SetStock(httptest.NewRecorder(), req1)

	body2, _ := json.Marshal(map[string]any{"product_id": 2, "quantity": 100})
	req2 := httptest.NewRequest(http.MethodPost, "/api/v1/admin/inventory", bytes.NewReader(body2))
	req2.Header.Set("Content-Type", "application/json")
	h.SetStock(httptest.NewRecorder(), req2)

	req := httptest.NewRequest(http.MethodGet, "/api/v1/admin/inventory/low-stock", nil)
	rec := httptest.NewRecorder()
	h.ListLowStock(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", rec.Code)
	}
	var items []map[string]any
	json.NewDecoder(rec.Body).Decode(&items)
	if len(items) != 1 {
		t.Errorf("expected 1 low stock item, got %d", len(items))
	}
}

func TestAdminHandler_CreateCategory_Success(t *testing.T) {
	sf, _ := kernel.NewSnowflake(1)
	catRepo := newFakeCategoryRepo()
	h := &AdminHandler{categoryRepo: catRepo, sf: sf}

	body, _ := json.Marshal(map[string]any{"name": "Electronics", "slug": "electronics"})
	req := httptest.NewRequest(http.MethodPost, "/api/v1/admin/categories", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()
	h.CreateCategory(rec, req)

	if rec.Code != http.StatusCreated {
		t.Fatalf("expected 201, got %d: %s", rec.Code, rec.Body.String())
	}
}

func TestAdminHandler_CreateCategory_InvalidBody(t *testing.T) {
	sf, _ := kernel.NewSnowflake(1)
	catRepo := newFakeCategoryRepo()
	h := &AdminHandler{categoryRepo: catRepo, sf: sf}

	req := httptest.NewRequest(http.MethodPost, "/api/v1/admin/categories", bytes.NewReader([]byte("not json")))
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()
	h.CreateCategory(rec, req)

	if rec.Code != http.StatusBadRequest {
		t.Errorf("expected 400, got %d", rec.Code)
	}
}

func TestAdminHandler_ListCategories_Success(t *testing.T) {
	sf, _ := kernel.NewSnowflake(1)
	catRepo := newFakeCategoryRepo()

	cat1, _ := catalogdomain.NewCategory(kernel.ID(1), "Books", "books", 0, 0)
	catRepo.Save(context.Background(), cat1)
	cat2, _ := catalogdomain.NewCategory(kernel.ID(2), "Music", "music", 0, 0)
	catRepo.Save(context.Background(), cat2)

	h := &AdminHandler{categoryRepo: catRepo, sf: sf}
	req := httptest.NewRequest(http.MethodGet, "/api/v1/admin/categories", nil)
	rec := httptest.NewRecorder()
	h.ListCategories(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", rec.Code)
	}
	var cats []map[string]any
	json.NewDecoder(rec.Body).Decode(&cats)
	if len(cats) != 2 {
		t.Errorf("expected 2 categories, got %d", len(cats))
	}
}

func TestAdminHandler_GetCategory_Success(t *testing.T) {
	sf, _ := kernel.NewSnowflake(1)
	catRepo := newFakeCategoryRepo()

	cat, _ := catalogdomain.NewCategory(kernel.ID(5), "Clothing", "clothing", 0, 0)
	catRepo.Save(context.Background(), cat)

	h := &AdminHandler{categoryRepo: catRepo, sf: sf}
	req := httptest.NewRequest(http.MethodGet, "/api/v1/admin/categories/5", nil)
	req = pathvar.WithVars(req, map[string]string{"id": "5"})
	rec := httptest.NewRecorder()
	h.GetCategory(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", rec.Code)
	}
}

func TestAdminHandler_DeleteCategory_Success(t *testing.T) {
	sf, _ := kernel.NewSnowflake(1)
	catRepo := newFakeCategoryRepo()

	cat, _ := catalogdomain.NewCategory(kernel.ID(10), "Toys", "toys", 0, 0)
	catRepo.Save(context.Background(), cat)

	h := &AdminHandler{categoryRepo: catRepo, sf: sf}
	req := httptest.NewRequest(http.MethodDelete, "/api/v1/admin/categories/10", nil)
	req = pathvar.WithVars(req, map[string]string{"id": "10"})
	rec := httptest.NewRecorder()
	h.DeleteCategory(rec, req)

	if rec.Code != http.StatusNoContent {
		t.Errorf("expected 204, got %d", rec.Code)
	}
}

func TestAdminHandler_UpdateCategory_Success(t *testing.T) {
	sf, _ := kernel.NewSnowflake(1)
	catRepo := newFakeCategoryRepo()

	cat, _ := catalogdomain.NewCategory(kernel.ID(20), "Old Name", "old-name", 0, 0)
	catRepo.Save(context.Background(), cat)

	h := &AdminHandler{categoryRepo: catRepo, sf: sf}
	body, _ := json.Marshal(map[string]any{"name": "New Name", "slug": "new-name"})
	req := httptest.NewRequest(http.MethodPut, "/api/v1/admin/categories/20", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	req = pathvar.WithVars(req, map[string]string{"id": "20"})
	rec := httptest.NewRecorder()
	h.UpdateCategory(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", rec.Code)
	}
}

func TestAdminHandler_ListAllReviews_Success(t *testing.T) {
	sf, _ := kernel.NewSnowflake(1)
	reviewRepo := newFakeReviewRepo()
	logger := fakeLog{}
	reviewSvc := reviewdomain.NewReviewService(reviewRepo, logger)
	h := &AdminHandler{reviewSvc: reviewSvc, sf: sf}

	id1, _ := sf.NextID()
	r1, _ := reviewdomain.NewReview(id1, kernel.ID(10), kernel.ID(1), 5, "Great", "Love it")
	reviewRepo.Save(context.Background(), r1)

	id2, _ := sf.NextID()
	r2, _ := reviewdomain.NewReview(id2, kernel.ID(10), kernel.ID(2), 4, "Good", "Nice")
	reviewRepo.Save(context.Background(), r2)

	req := httptest.NewRequest(http.MethodGet, "/api/v1/admin/reviews?limit=10", nil)
	rec := httptest.NewRecorder()
	h.ListAllReviews(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", rec.Code)
	}
	var resp map[string]any
	json.NewDecoder(rec.Body).Decode(&resp)
	reviews := resp["reviews"].([]any)
	if len(reviews) != 2 {
		t.Errorf("expected 2 reviews, got %d", len(reviews))
	}
}

func TestAdminHandler_ListAllReviews_Empty(t *testing.T) {
	sf, _ := kernel.NewSnowflake(1)
	reviewRepo := newFakeReviewRepo()
	logger := fakeLog{}
	reviewSvc := reviewdomain.NewReviewService(reviewRepo, logger)
	h := &AdminHandler{reviewSvc: reviewSvc, sf: sf}

	req := httptest.NewRequest(http.MethodGet, "/api/v1/admin/reviews?limit=10", nil)
	rec := httptest.NewRecorder()
	h.ListAllReviews(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", rec.Code)
	}
	var resp map[string]any
	json.NewDecoder(rec.Body).Decode(&resp)
	reviews := resp["reviews"].([]any)
	if len(reviews) != 0 {
		t.Errorf("expected 0 reviews, got %d", len(reviews))
	}
}

func TestAdminHandler_ApproveReview_Success(t *testing.T) {
	sf, _ := kernel.NewSnowflake(1)
	reviewRepo := newFakeReviewRepo()
	logger := fakeLog{}
	reviewSvc := reviewdomain.NewReviewService(reviewRepo, logger)
	h := &AdminHandler{reviewSvc: reviewSvc, sf: sf}

	id, _ := sf.NextID()
	r, _ := reviewdomain.NewReview(id, kernel.ID(10), kernel.ID(1), 5, "Pending", "Approve me")
	reviewRepo.Save(context.Background(), r)

	idStr := strconv.FormatInt(id.Int64(), 10)
	req := httptest.NewRequest(http.MethodPost, "/api/v1/admin/reviews/"+idStr+"/approve", nil)
	req = pathvar.WithVars(req, map[string]string{"id": idStr})
	rec := httptest.NewRecorder()
	h.ApproveReview(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", rec.Code)
	}
	var resp map[string]any
	json.NewDecoder(rec.Body).Decode(&resp)
	if resp["status"] != "approved" {
		t.Errorf("expected status approved, got %v", resp["status"])
	}
}

func TestAdminHandler_RejectReview_Success(t *testing.T) {
	sf, _ := kernel.NewSnowflake(1)
	reviewRepo := newFakeReviewRepo()
	logger := fakeLog{}
	reviewSvc := reviewdomain.NewReviewService(reviewRepo, logger)
	h := &AdminHandler{reviewSvc: reviewSvc, sf: sf}

	id, _ := sf.NextID()
	r, _ := reviewdomain.NewReview(id, kernel.ID(10), kernel.ID(1), 3, "Flagged", "Reject me")
	reviewRepo.Save(context.Background(), r)

	idStr := strconv.FormatInt(id.Int64(), 10)
	req := httptest.NewRequest(http.MethodPost, "/api/v1/admin/reviews/"+idStr+"/reject", nil)
	req = pathvar.WithVars(req, map[string]string{"id": idStr})
	rec := httptest.NewRecorder()
	h.RejectReview(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", rec.Code)
	}
	var resp map[string]any
	json.NewDecoder(rec.Body).Decode(&resp)
	if resp["status"] != "rejected" {
		t.Errorf("expected status rejected, got %v", resp["status"])
	}
}

func TestAdminHandler_ApproveReview_NotFound(t *testing.T) {
	sf, _ := kernel.NewSnowflake(1)
	reviewRepo := newFakeReviewRepo()
	logger := fakeLog{}
	reviewSvc := reviewdomain.NewReviewService(reviewRepo, logger)
	h := &AdminHandler{reviewSvc: reviewSvc, sf: sf}

	req := httptest.NewRequest(http.MethodPost, "/api/v1/admin/reviews/999/approve", nil)
	req = pathvar.WithVars(req, map[string]string{"id": "999"})
	rec := httptest.NewRecorder()
	h.ApproveReview(rec, req)

	if rec.Code != http.StatusNotFound {
		t.Errorf("expected 404, got %d", rec.Code)
	}
}
