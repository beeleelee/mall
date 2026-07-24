package rest

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"sync"
	"testing"

	"github.com/zeromicro/go-zero/rest/pathvar"

	"github.com/beeleelee/mall/domain/kernel"
	domain "github.com/beeleelee/mall/domain/wishlist"
	"github.com/beeleelee/mall/interfaces/middleware"
)

type fakeWishlistRepo struct {
	mu        sync.Mutex
	wishlists map[kernel.ID]*domain.Wishlist
}

func newFakeWishlistRepo() *fakeWishlistRepo {
	return &fakeWishlistRepo{wishlists: make(map[kernel.ID]*domain.Wishlist)}
}

func (f *fakeWishlistRepo) Save(_ context.Context, w *domain.Wishlist) error {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.wishlists[w.ID] = w
	return nil
}

func (f *fakeWishlistRepo) FindByUserID(_ context.Context, userID kernel.ID) (*domain.Wishlist, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	for _, w := range f.wishlists {
		if w.UserID == userID {
			return w, nil
		}
	}
	return nil, kernel.NewDomainError(kernel.ErrNotFound, "wishlist not found")
}

func (f *fakeWishlistRepo) Delete(_ context.Context, id kernel.ID) error {
	f.mu.Lock()
	defer f.mu.Unlock()
	delete(f.wishlists, id)
	return nil
}

func authWishlistRequest(r *http.Request, userID int64) *http.Request {
	return r.WithContext(middleware.ContextWithUser(r.Context(), middleware.UserInfo{UserID: userID}))
}

func newTestWishlistHandler(t *testing.T) *WishlistHandler {
	t.Helper()
	repo := newFakeWishlistRepo()
	logger := fakeLog{}
	sf, err := kernel.NewSnowflake(1)
	if err != nil {
		t.Fatalf("NewSnowflake: %v", err)
	}
	svc := domain.NewWishlistService(repo, sf, logger)
	return NewWishlistHandler(svc)
}

func TestWishlistHandler_Get_Success(t *testing.T) {
	h := newTestWishlistHandler(t)

	body, _ := json.Marshal(map[string]int64{"product_id": 100})
	addReq := httptest.NewRequest(http.MethodPost, "/api/v1/wishlist/items", bytes.NewReader(body))
	addReq.Header.Set("Content-Type", "application/json")
	addReq = authWishlistRequest(addReq, 50)
	h.AddItem(httptest.NewRecorder(), addReq)

	getReq := httptest.NewRequest(http.MethodGet, "/api/v1/wishlist", nil)
	getReq = authWishlistRequest(getReq, 50)
	getRec := httptest.NewRecorder()
	h.Get(getRec, getReq)

	if getRec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", getRec.Code, getRec.Body.String())
	}
	var resp map[string]any
	if err := json.NewDecoder(getRec.Body).Decode(&resp); err != nil {
		t.Fatalf("decode: %v", err)
	}
	if resp["user_id"].(float64) != 50 {
		t.Errorf("expected user_id 50, got %v", resp["user_id"])
	}
	items := resp["items"].([]any)
	if len(items) != 1 {
		t.Errorf("expected 1 item, got %d", len(items))
	}
}

func TestWishlistHandler_Get_Empty(t *testing.T) {
	h := newTestWishlistHandler(t)
	req := httptest.NewRequest(http.MethodGet, "/api/v1/wishlist", nil)
	req = authWishlistRequest(req, 99)
	rec := httptest.NewRecorder()
	h.Get(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", rec.Code)
	}
	var resp map[string]any
	json.NewDecoder(rec.Body).Decode(&resp)
	if resp["count"].(float64) != 0 {
		t.Errorf("expected count 0, got %v", resp["count"])
	}
}

func TestWishlistHandler_Get_Unauthenticated(t *testing.T) {
	h := newTestWishlistHandler(t)
	req := httptest.NewRequest(http.MethodGet, "/api/v1/wishlist", nil)
	rec := httptest.NewRecorder()
	h.Get(rec, req)

	if rec.Code != http.StatusUnauthorized {
		t.Errorf("expected 401, got %d", rec.Code)
	}
}

func TestWishlistHandler_AddItem_Success(t *testing.T) {
	h := newTestWishlistHandler(t)
	body, _ := json.Marshal(map[string]int64{"product_id": 200})
	req := httptest.NewRequest(http.MethodPost, "/api/v1/wishlist/items", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	req = authWishlistRequest(req, 60)
	rec := httptest.NewRecorder()
	h.AddItem(rec, req)

	if rec.Code != http.StatusCreated {
		t.Fatalf("expected 201, got %d", rec.Code)
	}
}

func TestWishlistHandler_RemoveItem_Success(t *testing.T) {
	h := newTestWishlistHandler(t)

	body, _ := json.Marshal(map[string]int64{"product_id": 300})
	addReq := httptest.NewRequest(http.MethodPost, "/api/v1/wishlist/items", bytes.NewReader(body))
	addReq.Header.Set("Content-Type", "application/json")
	addReq = authWishlistRequest(addReq, 70)
	h.AddItem(httptest.NewRecorder(), addReq)

	delReq := httptest.NewRequest(http.MethodDelete, "/api/v1/wishlist/items/300", nil)
	delReq = pathvar.WithVars(delReq, map[string]string{"productId": "300"})
	delReq = authWishlistRequest(delReq, 70)
	delRec := httptest.NewRecorder()
	h.RemoveItem(delRec, delReq)

	if delRec.Code != http.StatusOK {
		t.Errorf("expected 200, got %d", delRec.Code)
	}
}

func TestWishlistHandler_RemoveItem_NotFound(t *testing.T) {
	h := newTestWishlistHandler(t)
	req := httptest.NewRequest(http.MethodDelete, "/api/v1/wishlist/items/999", nil)
	req = pathvar.WithVars(req, map[string]string{"productId": "999"})
	req = authWishlistRequest(req, 80)
	rec := httptest.NewRecorder()
	h.RemoveItem(rec, req)

	if rec.Code != http.StatusNotFound {
		t.Errorf("expected 404, got %d", rec.Code)
	}
}

func TestWishlistHandler_Clear_Success(t *testing.T) {
	h := newTestWishlistHandler(t)

	body, _ := json.Marshal(map[string]int64{"product_id": 400})
	addReq := httptest.NewRequest(http.MethodPost, "/api/v1/wishlist/items", bytes.NewReader(body))
	addReq.Header.Set("Content-Type", "application/json")
	addReq = authWishlistRequest(addReq, 90)
	h.AddItem(httptest.NewRecorder(), addReq)

	clearReq := httptest.NewRequest(http.MethodDelete, "/api/v1/wishlist", nil)
	clearReq = authWishlistRequest(clearReq, 90)
	clearRec := httptest.NewRecorder()
	h.Clear(clearRec, clearReq)

	if clearRec.Code != http.StatusOK {
		t.Errorf("expected 200, got %d", clearRec.Code)
	}
}
