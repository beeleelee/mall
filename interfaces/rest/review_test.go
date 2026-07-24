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

	"github.com/beeleelee/mall/domain/kernel"
	domain "github.com/beeleelee/mall/domain/review"
	"github.com/beeleelee/mall/interfaces/middleware"
)

type fakeReviewRepo struct {
	mu      sync.Mutex
	reviews map[kernel.ID]*domain.Review
}

func newFakeReviewRepo() *fakeReviewRepo {
	return &fakeReviewRepo{reviews: make(map[kernel.ID]*domain.Review)}
}

func (f *fakeReviewRepo) Save(_ context.Context, r *domain.Review) error {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.reviews[r.ID] = r
	return nil
}

func (f *fakeReviewRepo) FindByID(_ context.Context, id kernel.ID) (*domain.Review, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	r, ok := f.reviews[id]
	if !ok {
		return nil, kernel.NewDomainError(kernel.ErrNotFound, "review not found")
	}
	return r, nil
}

func (f *fakeReviewRepo) FindByProduct(_ context.Context, productID kernel.ID, _ domain.ReviewQueryOptions) (*domain.ReviewListResult, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	var result []*domain.Review
	for _, r := range f.reviews {
		if r.ProductID == productID {
			result = append(result, r)
		}
	}
	return &domain.ReviewListResult{Reviews: result, Total: len(result)}, nil
}

func (f *fakeReviewRepo) FindByUser(_ context.Context, userID kernel.ID, _ domain.ReviewQueryOptions) (*domain.ReviewListResult, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	var result []*domain.Review
	for _, r := range f.reviews {
		if r.UserID == userID {
			result = append(result, r)
		}
	}
	return &domain.ReviewListResult{Reviews: result, Total: len(result)}, nil
}

func (f *fakeReviewRepo) FindByProductAndUser(_ context.Context, productID, userID kernel.ID) (*domain.Review, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	for _, r := range f.reviews {
		if r.ProductID == productID && r.UserID == userID {
			return r, nil
		}
	}
	return nil, kernel.NewDomainError(kernel.ErrNotFound, "not found")
}

func (f *fakeReviewRepo) FindAll(_ context.Context, _ domain.ReviewQueryOptions) (*domain.ReviewListResult, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	var result []*domain.Review
	for _, r := range f.reviews {
		result = append(result, r)
	}
	return &domain.ReviewListResult{Reviews: result, Total: len(result)}, nil
}

func (f *fakeReviewRepo) Delete(_ context.Context, id kernel.ID) error {
	f.mu.Lock()
	defer f.mu.Unlock()
	delete(f.reviews, id)
	return nil
}

func (f *fakeReviewRepo) GetAverageRating(_ context.Context, productID kernel.ID) (float64, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	var sum, count float64
	for _, r := range f.reviews {
		if r.ProductID == productID {
			sum += float64(r.Rating)
			count++
		}
	}
	if count == 0 {
		return 0, nil
	}
	return sum / count, nil
}

type reviewTestFixture struct {
	handler *ReviewHandler
	sf      *kernel.Snowflake
	svc     *domain.ReviewService
	repo    *fakeReviewRepo
}

func newReviewTestFixture(t *testing.T) *reviewTestFixture {
	t.Helper()
	repo := newFakeReviewRepo()
	logger := fakeLog{}
	svc := domain.NewReviewService(repo, logger)
	sf, err := kernel.NewSnowflake(1)
	if err != nil {
		t.Fatalf("NewSnowflake: %v", err)
	}
	return &reviewTestFixture{
		handler: NewReviewHandler(svc, sf),
		sf:      sf,
		svc:     svc,
		repo:    repo,
	}
}

func authReviewRequest(r *http.Request, userID int64) *http.Request {
	return r.WithContext(middleware.ContextWithUser(r.Context(), middleware.UserInfo{UserID: userID}))
}

func withProductVar(r *http.Request, productID string) *http.Request {
	return pathvar.WithVars(r, map[string]string{"id": productID})
}

func TestReviewHandler_Create_Success(t *testing.T) {
	fx := newReviewTestFixture(t)
	body, _ := json.Marshal(map[string]any{"rating": 5, "title": "Great!", "content": "Loved it"})
	req := httptest.NewRequest(http.MethodPost, "/api/v1/products/1/reviews", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	req = authReviewRequest(req, 100)
	req = withProductVar(req, "1")
	rec := httptest.NewRecorder()
	fx.handler.Create(rec, req)

	if rec.Code != http.StatusCreated {
		t.Fatalf("expected 201, got %d: %s", rec.Code, rec.Body.String())
	}
	var resp map[string]any
	json.NewDecoder(rec.Body).Decode(&resp)
	if resp["rating"].(float64) != 5 {
		t.Errorf("expected rating 5, got %v", resp["rating"])
	}
	if resp["user_id"].(float64) != 100 {
		t.Errorf("expected user_id 100, got %v", resp["user_id"])
	}
}

func TestReviewHandler_Create_Unauthenticated(t *testing.T) {
	fx := newReviewTestFixture(t)
	body, _ := json.Marshal(map[string]any{"rating": 4, "title": "OK", "content": "Decent"})
	req := httptest.NewRequest(http.MethodPost, "/api/v1/products/1/reviews", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()
	fx.handler.Create(rec, req)

	if rec.Code != http.StatusUnauthorized {
		t.Errorf("expected 401, got %d", rec.Code)
	}
}

func TestReviewHandler_Create_Duplicate(t *testing.T) {
	fx := newReviewTestFixture(t)
	body, _ := json.Marshal(map[string]any{"rating": 3, "title": "Meh", "content": "Okay"})
	req := httptest.NewRequest(http.MethodPost, "/api/v1/products/1/reviews", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	req = authReviewRequest(req, 100)
	req = withProductVar(req, "1")
	fx.handler.Create(httptest.NewRecorder(), req)

	req2 := httptest.NewRequest(http.MethodPost, "/api/v1/products/1/reviews", bytes.NewReader(body))
	req2.Header.Set("Content-Type", "application/json")
	req2 = authReviewRequest(req2, 100)
	req2 = withProductVar(req2, "1")
	rec2 := httptest.NewRecorder()
	fx.handler.Create(rec2, req2)

	if rec2.Code != http.StatusConflict {
		t.Errorf("expected 409, got %d", rec2.Code)
	}
}

func TestReviewHandler_Create_InvalidBody(t *testing.T) {
	fx := newReviewTestFixture(t)
	req := httptest.NewRequest(http.MethodPost, "/api/v1/products/1/reviews", bytes.NewReader([]byte("not json")))
	req.Header.Set("Content-Type", "application/json")
	req = authReviewRequest(req, 100)
	rec := httptest.NewRecorder()
	fx.handler.Create(rec, req)

	if rec.Code != http.StatusBadRequest {
		t.Errorf("expected 400, got %d", rec.Code)
	}
}

func TestReviewHandler_Get_Success(t *testing.T) {
	fx := newReviewTestFixture(t)
	body, _ := json.Marshal(map[string]any{"rating": 4, "title": "Nice", "content": "Good product"})
	req := httptest.NewRequest(http.MethodPost, "/api/v1/products/5/reviews", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	req = authReviewRequest(req, 200)
	req = withProductVar(req, "5")
	crec := httptest.NewRecorder()
	fx.handler.Create(crec, req)
	var created map[string]any
	json.NewDecoder(crec.Body).Decode(&created)

	reviewID := int64(created["id"].(float64))
	getReq := httptest.NewRequest(http.MethodGet, "/api/v1/reviews/"+strconv.FormatInt(reviewID, 10), nil)
	getReq = pathvar.WithVars(getReq, map[string]string{"id": strconv.FormatInt(reviewID, 10)})
	getRec := httptest.NewRecorder()
	fx.handler.Get(getRec, getReq)

	if getRec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", getRec.Code)
	}
	var resp map[string]any
	json.NewDecoder(getRec.Body).Decode(&resp)
	if resp["id"].(float64) != float64(reviewID) {
		t.Errorf("expected id %d, got %v", reviewID, resp["id"])
	}
}

func TestReviewHandler_Get_NotFound(t *testing.T) {
	fx := newReviewTestFixture(t)
	req := httptest.NewRequest(http.MethodGet, "/api/v1/reviews/99999", nil)
	req = pathvar.WithVars(req, map[string]string{"id": "99999"})
	rec := httptest.NewRecorder()
	fx.handler.Get(rec, req)

	if rec.Code != http.StatusNotFound {
		t.Errorf("expected 404, got %d", rec.Code)
	}
}

func TestReviewHandler_ListByProduct_Success(t *testing.T) {
	fx := newReviewTestFixture(t)
	sf := fx.sf

	id1, _ := sf.NextID()
	r1, _ := domain.NewReview(id1, kernel.ID(10), kernel.ID(1), 5, "Great", "Love it")
	fx.repo.Save(context.Background(), r1)
	id2, _ := sf.NextID()
	r2, _ := domain.NewReview(id2, kernel.ID(10), kernel.ID(2), 4, "Good", "Nice")
	fx.repo.Save(context.Background(), r2)

	req := httptest.NewRequest(http.MethodGet, "/api/v1/products/10/reviews", nil)
	req = withProductVar(req, "10")
	rec := httptest.NewRecorder()
	fx.handler.ListByProduct(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", rec.Code)
	}
	var resp map[string]any
	json.NewDecoder(rec.Body).Decode(&resp)
	reviews := resp["reviews"].([]any)
	if len(reviews) != 2 {
		t.Errorf("expected 2 reviews, got %d", len(reviews))
	}
	if resp["average_rating"].(float64) != 4.5 {
		t.Errorf("expected avg 4.5, got %v", resp["average_rating"])
	}
}

func TestReviewHandler_ListByProduct_Empty(t *testing.T) {
	fx := newReviewTestFixture(t)
	req := httptest.NewRequest(http.MethodGet, "/api/v1/products/99/reviews", nil)
	req = withProductVar(req, "99")
	rec := httptest.NewRecorder()
	fx.handler.ListByProduct(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", rec.Code)
	}
	var resp map[string]any
	json.NewDecoder(rec.Body).Decode(&resp)
	reviews := resp["reviews"].([]any)
	if len(reviews) != 0 {
		t.Errorf("expected empty, got %d", len(reviews))
	}
}

func TestReviewHandler_Delete_Success(t *testing.T) {
	fx := newReviewTestFixture(t)
	body, _ := json.Marshal(map[string]any{"rating": 5, "title": "To Delete", "content": "Delete me"})
	req := httptest.NewRequest(http.MethodPost, "/api/v1/products/1/reviews", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	req = authReviewRequest(req, 50)
	req = withProductVar(req, "1")
	crec := httptest.NewRecorder()
	fx.handler.Create(crec, req)

	var created map[string]any
	json.NewDecoder(crec.Body).Decode(&created)
	reviewID := int64(created["id"].(float64))

	delReq := httptest.NewRequest(http.MethodDelete, "/api/v1/reviews/"+strconv.FormatInt(reviewID, 10), nil)
	delReq = pathvar.WithVars(delReq, map[string]string{"id": strconv.FormatInt(reviewID, 10)})
	delReq = authReviewRequest(delReq, 50)
	delRec := httptest.NewRecorder()
	fx.handler.Delete(delRec, delReq)

	if delRec.Code != http.StatusOK {
		t.Errorf("expected 200, got %d", delRec.Code)
	}
}

func TestReviewHandler_Delete_NotOwnReview(t *testing.T) {
	fx := newReviewTestFixture(t)
	body, _ := json.Marshal(map[string]any{"rating": 3, "title": "Mine", "content": "Mine only"})
	req := httptest.NewRequest(http.MethodPost, "/api/v1/products/1/reviews", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	req = authReviewRequest(req, 10)
	req = withProductVar(req, "1")
	crec := httptest.NewRecorder()
	fx.handler.Create(crec, req)

	var created map[string]any
	json.NewDecoder(crec.Body).Decode(&created)
	reviewID := int64(created["id"].(float64))

	delReq := httptest.NewRequest(http.MethodDelete, "/api/v1/reviews/"+strconv.FormatInt(reviewID, 10), nil)
	delReq = pathvar.WithVars(delReq, map[string]string{"id": strconv.FormatInt(reviewID, 10)})
	delReq = authReviewRequest(delReq, 99)
	delRec := httptest.NewRecorder()
	fx.handler.Delete(delRec, delReq)

	if delRec.Code != http.StatusForbidden {
		t.Errorf("expected 403, got %d", delRec.Code)
	}
}

func TestReviewHandler_Delete_Unauthenticated(t *testing.T) {
	fx := newReviewTestFixture(t)
	req := httptest.NewRequest(http.MethodDelete, "/api/v1/reviews/1", nil)
	rec := httptest.NewRecorder()
	fx.handler.Delete(rec, req)

	if rec.Code != http.StatusUnauthorized {
		t.Errorf("expected 401, got %d", rec.Code)
	}
}
