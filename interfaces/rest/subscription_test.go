package rest

import (
	"bytes"
	"context"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"strconv"
	"sync"
	"testing"
	"time"

	"github.com/zeromicro/go-zero/rest/pathvar"

	"github.com/beeleelee/mall/application/subscription"
	"github.com/beeleelee/mall/domain/kernel"
	domain "github.com/beeleelee/mall/domain/subscription"
	"github.com/beeleelee/mall/interfaces/middleware"
)

type fakePlanRepo struct {
	mu   sync.Mutex
	byID map[kernel.ID]*domain.Plan
}

func newFakePlanRepo() *fakePlanRepo {
	return &fakePlanRepo{byID: make(map[kernel.ID]*domain.Plan)}
}

func (f *fakePlanRepo) Save(_ context.Context, p *domain.Plan) error {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.byID[p.ID] = p
	return nil
}

func (f *fakePlanRepo) FindByID(_ context.Context, id kernel.ID) (*domain.Plan, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	p, ok := f.byID[id]
	if !ok {
		return nil, kernel.NewDomainError(kernel.ErrNotFound, "plan not found")
	}
	return p, nil
}

func (f *fakePlanRepo) FindAll(_ context.Context) ([]*domain.Plan, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	plans := make([]*domain.Plan, 0, len(f.byID))
	for _, p := range f.byID {
		plans = append(plans, p)
	}
	return plans, nil
}

func (f *fakePlanRepo) FindActive(_ context.Context) ([]*domain.Plan, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	var plans []*domain.Plan
	for _, p := range f.byID {
		if p.Status == domain.PlanStatusActive {
			plans = append(plans, p)
		}
	}
	return plans, nil
}

type fakeSubRepo struct {
	mu   sync.Mutex
	byID map[kernel.ID]*domain.Subscription
}

func newFakeSubRepo() *fakeSubRepo {
	return &fakeSubRepo{byID: make(map[kernel.ID]*domain.Subscription)}
}

func (f *fakeSubRepo) Save(_ context.Context, s *domain.Subscription) error {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.byID[s.ID] = s
	return nil
}

func (f *fakeSubRepo) FindByID(_ context.Context, id kernel.ID) (*domain.Subscription, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	s, ok := f.byID[id]
	if !ok {
		return nil, kernel.NewDomainError(kernel.ErrNotFound, "subscription not found")
	}
	return s, nil
}

func (f *fakeSubRepo) FindByUserID(_ context.Context, userID kernel.ID) ([]*domain.Subscription, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	var subs []*domain.Subscription
	for _, s := range f.byID {
		if s.UserID == userID {
			subs = append(subs, s)
		}
	}
	return subs, nil
}

func (f *fakeSubRepo) FindActiveByUserID(_ context.Context, userID kernel.ID) (*domain.Subscription, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	for _, s := range f.byID {
		if s.UserID == userID && (s.Status == domain.SubscriptionStatusActive || s.Status == domain.SubscriptionStatusTrialing) {
			return s, nil
		}
	}
	return nil, kernel.NewDomainError(kernel.ErrNotFound, "no active subscription")
}

func (f *fakeSubRepo) FindDueForBilling(_ context.Context, now time.Time) ([]*domain.Subscription, error) {
	return nil, nil
}

type fakeSubPublisher struct{}

func (f *fakeSubPublisher) PublishSubscriptionEvent(_ context.Context, _ *domain.Subscription) error {
	return nil
}

func newTestSubscriptionHandler(t *testing.T) *SubscriptionHandler {
	t.Helper()
	planRepo := newFakePlanRepo()
	subRepo := newFakeSubRepo()
	pub := &fakeSubPublisher{}
	logger := fakeLog{}
	domainSvc := domain.NewSubscriptionService(planRepo, subRepo, pub, logger)
	sf, err := kernel.NewSnowflake(1)
	if err != nil {
		t.Fatalf("NewSnowflake: %v", err)
	}
	appSvc := subscription.NewSubscriptionAppService(domainSvc, sf)
	return NewSubscriptionHandler(appSvc)
}

func decodeMap(r io.Reader) (map[string]any, error) {
	dec := json.NewDecoder(r)
	dec.UseNumber()
	var m map[string]any
	if err := dec.Decode(&m); err != nil {
		return nil, err
	}
	return m, nil
}

func getInt64(m map[string]any, key string) int64 {
	n, _ := strconv.ParseInt(string(m[key].(json.Number)), 10, 64)
	return n
}

func TestSubscriptionHandler_CreatePlan_Success(t *testing.T) {
	h := newTestSubscriptionHandler(t)
	body, _ := json.Marshal(map[string]any{
		"name": "Basic", "amount": 999, "interval": "month",
		"interval_count": 1, "trial_days": 7,
		"features": []string{"feature1"},
	})
	req := httptest.NewRequest(http.MethodPost, "/api/v1/subscriptions/plans", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()
	h.CreatePlan(rec, req)

	if rec.Code != http.StatusCreated {
		t.Fatalf("expected 201, got %d: %s", rec.Code, rec.Body.String())
	}
	resp, err := decodeMap(rec.Body)
	if err != nil {
		t.Fatalf("decode: %v", err)
	}
	if resp["name"] != "Basic" {
		t.Errorf("expected name=Basic, got %v", resp["name"])
	}
	if resp["amount"] != json.Number("999") {
		t.Errorf("expected amount=999, got %v", resp["amount"])
	}
}

func TestSubscriptionHandler_CreatePlan_InvalidBody(t *testing.T) {
	h := newTestSubscriptionHandler(t)
	req := httptest.NewRequest(http.MethodPost, "/api/v1/subscriptions/plans", bytes.NewReader([]byte("not json")))
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()
	h.CreatePlan(rec, req)

	if rec.Code != http.StatusBadRequest {
		t.Errorf("expected 400, got %d", rec.Code)
	}
}

func TestSubscriptionHandler_CreatePlan_MissingName(t *testing.T) {
	h := newTestSubscriptionHandler(t)
	body, _ := json.Marshal(map[string]any{
		"amount": 999, "interval": "month",
	})
	req := httptest.NewRequest(http.MethodPost, "/api/v1/subscriptions/plans", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()
	h.CreatePlan(rec, req)

	if rec.Code != http.StatusBadRequest {
		t.Errorf("expected 400, got %d", rec.Code)
	}
}

func TestSubscriptionHandler_ListPlans_Empty(t *testing.T) {
	h := newTestSubscriptionHandler(t)
	req := httptest.NewRequest(http.MethodGet, "/api/v1/subscriptions/plans", nil)
	rec := httptest.NewRecorder()
	h.ListPlans(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", rec.Code)
	}
	var resp []any
	if err := json.NewDecoder(rec.Body).Decode(&resp); err != nil {
		t.Fatalf("decode: %v", err)
	}
	if len(resp) != 0 {
		t.Errorf("expected empty list, got %d items", len(resp))
	}
}

func TestSubscriptionHandler_ListPlans_WithPlans(t *testing.T) {
	h := newTestSubscriptionHandler(t)
	body, _ := json.Marshal(map[string]any{
		"name": "Pro", "amount": 1999, "interval": "month",
		"interval_count": 1, "trial_days": 0,
	})
	req := httptest.NewRequest(http.MethodPost, "/api/v1/subscriptions/plans", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	h.CreatePlan(httptest.NewRecorder(), req)

	req2 := httptest.NewRequest(http.MethodGet, "/api/v1/subscriptions/plans", nil)
	rec := httptest.NewRecorder()
	h.ListPlans(rec, req2)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", rec.Code)
	}
	var resp []any
	if err := json.NewDecoder(rec.Body).Decode(&resp); err != nil {
		t.Fatalf("decode: %v", err)
	}
	if len(resp) != 1 {
		t.Errorf("expected 1 plan, got %d", len(resp))
	}
}

func TestSubscriptionHandler_GetPlan_Success(t *testing.T) {
	h := newTestSubscriptionHandler(t)
	body, _ := json.Marshal(map[string]any{
		"name": "Basic", "amount": 999, "interval": "month",
		"interval_count": 1,
	})
	req := httptest.NewRequest(http.MethodPost, "/api/v1/subscriptions/plans", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	crec := httptest.NewRecorder()
	h.CreatePlan(crec, req)

	created, err := decodeMap(crec.Body)
	if err != nil {
		t.Fatalf("decode: %v", err)
	}
	planID := getInt64(created, "id")

	getReq := httptest.NewRequest(http.MethodGet, "/api/v1/subscriptions/plans/"+strconv.FormatInt(planID, 10), nil)
	getReq = pathvar.WithVars(getReq, map[string]string{"id": strconv.FormatInt(planID, 10)})
	rec := httptest.NewRecorder()
	h.GetPlan(rec, getReq)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", rec.Code, rec.Body.String())
	}
	resp, err := decodeMap(rec.Body)
	if err != nil {
		t.Fatalf("decode: %v", err)
	}
	if getInt64(resp, "id") != planID {
		t.Errorf("expected id=%d, got %v", planID, resp["id"])
	}
}

func TestSubscriptionHandler_GetPlan_NotFound(t *testing.T) {
	h := newTestSubscriptionHandler(t)
	getReq := httptest.NewRequest(http.MethodGet, "/api/v1/subscriptions/plans/999", nil)
	getReq = pathvar.WithVars(getReq, map[string]string{"id": "999"})
	rec := httptest.NewRecorder()
	h.GetPlan(rec, getReq)

	if rec.Code != http.StatusNotFound {
		t.Errorf("expected 404, got %d", rec.Code)
	}
}

func TestSubscriptionHandler_UpdatePlan_Success(t *testing.T) {
	h := newTestSubscriptionHandler(t)
	body, _ := json.Marshal(map[string]any{
		"name": "Basic", "amount": 999, "interval": "month",
		"interval_count": 1,
	})
	req := httptest.NewRequest(http.MethodPost, "/api/v1/subscriptions/plans", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	crec := httptest.NewRecorder()
	h.CreatePlan(crec, req)

	created, err := decodeMap(crec.Body)
	if err != nil {
		t.Fatalf("decode: %v", err)
	}
	planID := getInt64(created, "id")

	updateBody, _ := json.Marshal(map[string]any{
		"name": "Basic Plus", "amount": 1499, "interval": "month",
		"interval_count": 1,
	})
	updateReq := httptest.NewRequest(http.MethodPut, "/api/v1/subscriptions/plans/"+strconv.FormatInt(planID, 10), bytes.NewReader(updateBody))
	updateReq = pathvar.WithVars(updateReq, map[string]string{"id": strconv.FormatInt(planID, 10)})
	updateReq.Header.Set("Content-Type", "application/json")
	urec := httptest.NewRecorder()
	h.UpdatePlan(urec, updateReq)

	if urec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", urec.Code, urec.Body.String())
	}
	resp, err := decodeMap(urec.Body)
	if err != nil {
		t.Fatalf("decode: %v", err)
	}
	if resp["name"] != "Basic Plus" {
		t.Errorf("expected name='Basic Plus', got %v", resp["name"])
	}
	if resp["amount"] != json.Number("1499") {
		t.Errorf("expected amount=1499, got %v", resp["amount"])
	}
}

func TestSubscriptionHandler_UpdatePlan_NotFound(t *testing.T) {
	h := newTestSubscriptionHandler(t)
	body, _ := json.Marshal(map[string]any{
		"name": "Nope", "amount": 999, "interval": "month",
		"interval_count": 1,
	})
	req := httptest.NewRequest(http.MethodPut, "/api/v1/subscriptions/plans/999", bytes.NewReader(body))
	req = pathvar.WithVars(req, map[string]string{"id": "999"})
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()
	h.UpdatePlan(rec, req)

	if rec.Code != http.StatusNotFound {
		t.Errorf("expected 404, got %d", rec.Code)
	}
}

func TestSubscriptionHandler_Subscribe_Success(t *testing.T) {
	h := newTestSubscriptionHandler(t)
	body, _ := json.Marshal(map[string]any{
		"name": "Basic", "amount": 999, "interval": "month",
		"interval_count": 1,
	})
	req := httptest.NewRequest(http.MethodPost, "/api/v1/subscriptions/plans", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	crec := httptest.NewRecorder()
	h.CreatePlan(crec, req)

	plan, err := decodeMap(crec.Body)
	if err != nil {
		t.Fatalf("decode: %v", err)
	}
	planID := getInt64(plan, "id")

	subBody, _ := json.Marshal(map[string]any{"plan_id": planID})
	subReq := httptest.NewRequest(http.MethodPost, "/api/v1/subscriptions", bytes.NewReader(subBody))
	subReq.Header.Set("Content-Type", "application/json")
	subReq = subReq.WithContext(middleware.ContextWithUser(subReq.Context(), middleware.UserInfo{UserID: 1}))
	rec := httptest.NewRecorder()
	h.Subscribe(rec, subReq)

	if rec.Code != http.StatusCreated {
		t.Fatalf("expected 201, got %d: %s", rec.Code, rec.Body.String())
	}
	resp, err := decodeMap(rec.Body)
	if err != nil {
		t.Fatalf("decode: %v", err)
	}
	if getInt64(resp, "plan_id") != planID {
		t.Errorf("expected plan_id=%d, got %v", planID, resp["plan_id"])
	}
}

func TestSubscriptionHandler_Subscribe_NoPlan(t *testing.T) {
	h := newTestSubscriptionHandler(t)
	subBody, _ := json.Marshal(map[string]any{"plan_id": 999999})
	subReq := httptest.NewRequest(http.MethodPost, "/api/v1/subscriptions", bytes.NewReader(subBody))
	subReq.Header.Set("Content-Type", "application/json")
	subReq = subReq.WithContext(middleware.ContextWithUser(subReq.Context(), middleware.UserInfo{UserID: 1}))
	rec := httptest.NewRecorder()
	h.Subscribe(rec, subReq)

	if rec.Code != http.StatusNotFound {
		t.Errorf("expected 404, got %d", rec.Code)
	}
}

func TestSubscriptionHandler_ListUserSubscriptions_Empty(t *testing.T) {
	h := newTestSubscriptionHandler(t)
	req := httptest.NewRequest(http.MethodGet, "/api/v1/subscriptions", nil)
	req = req.WithContext(middleware.ContextWithUser(req.Context(), middleware.UserInfo{UserID: 1}))
	rec := httptest.NewRecorder()
	h.ListUserSubscriptions(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", rec.Code)
	}
	var resp []any
	json.NewDecoder(rec.Body).Decode(&resp)
	if len(resp) != 0 {
		t.Errorf("expected empty list, got %d items", len(resp))
	}
}

func TestSubscriptionHandler_GetSubscription_Success(t *testing.T) {
	h := newTestSubscriptionHandler(t)
	body, _ := json.Marshal(map[string]any{
		"name": "Basic", "amount": 999, "interval": "month",
		"interval_count": 1,
	})
	req := httptest.NewRequest(http.MethodPost, "/api/v1/subscriptions/plans", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	crec := httptest.NewRecorder()
	h.CreatePlan(crec, req)

	plan, err := decodeMap(crec.Body)
	if err != nil {
		t.Fatalf("decode: %v", err)
	}
	planID := getInt64(plan, "id")

	subBody, _ := json.Marshal(map[string]any{"plan_id": planID})
	subReq := httptest.NewRequest(http.MethodPost, "/api/v1/subscriptions", bytes.NewReader(subBody))
	subReq.Header.Set("Content-Type", "application/json")
	subReq = subReq.WithContext(middleware.ContextWithUser(subReq.Context(), middleware.UserInfo{UserID: 1}))
	crec2 := httptest.NewRecorder()
	h.Subscribe(crec2, subReq)

	created, err := decodeMap(crec2.Body)
	if err != nil {
		t.Fatalf("decode: %v", err)
	}
	subID := getInt64(created, "id")

	getReq := httptest.NewRequest(http.MethodGet, "/api/v1/subscriptions/"+strconv.FormatInt(subID, 10), nil)
	getReq = pathvar.WithVars(getReq, map[string]string{"id": strconv.FormatInt(subID, 10)})
	getReq = getReq.WithContext(middleware.ContextWithUser(getReq.Context(), middleware.UserInfo{UserID: 1}))
	rec := httptest.NewRecorder()
	h.GetSubscription(rec, getReq)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", rec.Code, rec.Body.String())
	}
	resp, err := decodeMap(rec.Body)
	if err != nil {
		t.Fatalf("decode: %v", err)
	}
	if getInt64(resp, "id") != subID {
		t.Errorf("expected id=%d, got %v", subID, resp["id"])
	}
}

func TestSubscriptionHandler_GetSubscription_NotFound(t *testing.T) {
	h := newTestSubscriptionHandler(t)
	getReq := httptest.NewRequest(http.MethodGet, "/api/v1/subscriptions/999", nil)
	getReq = pathvar.WithVars(getReq, map[string]string{"id": "999"})
	getReq = getReq.WithContext(middleware.ContextWithUser(getReq.Context(), middleware.UserInfo{UserID: 1}))
	rec := httptest.NewRecorder()
	h.GetSubscription(rec, getReq)

	if rec.Code != http.StatusNotFound {
		t.Errorf("expected 404, got %d", rec.Code)
	}
}

func TestSubscriptionHandler_GetSubscription_WrongUser(t *testing.T) {
	h := newTestSubscriptionHandler(t)
	body, _ := json.Marshal(map[string]any{
		"name": "Basic", "amount": 999, "interval": "month",
		"interval_count": 1,
	})
	req := httptest.NewRequest(http.MethodPost, "/api/v1/subscriptions/plans", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	crec := httptest.NewRecorder()
	h.CreatePlan(crec, req)

	plan, err := decodeMap(crec.Body)
	if err != nil {
		t.Fatalf("decode: %v", err)
	}
	planID := getInt64(plan, "id")

	subBody, _ := json.Marshal(map[string]any{"plan_id": planID})
	subReq := httptest.NewRequest(http.MethodPost, "/api/v1/subscriptions", bytes.NewReader(subBody))
	subReq.Header.Set("Content-Type", "application/json")
	subReq = subReq.WithContext(middleware.ContextWithUser(subReq.Context(), middleware.UserInfo{UserID: 1}))
	srec := httptest.NewRecorder()
	h.Subscribe(srec, subReq)

	created, err := decodeMap(srec.Body)
	if err != nil {
		t.Fatalf("decode: %v", err)
	}
	subID := getInt64(created, "id")

	getReq := httptest.NewRequest(http.MethodGet, "/api/v1/subscriptions/"+strconv.FormatInt(subID, 10), nil)
	getReq = pathvar.WithVars(getReq, map[string]string{"id": strconv.FormatInt(subID, 10)})
	getReq = getReq.WithContext(middleware.ContextWithUser(getReq.Context(), middleware.UserInfo{UserID: 99}))
	rec := httptest.NewRecorder()
	h.GetSubscription(rec, getReq)

	if rec.Code != http.StatusForbidden {
		t.Errorf("expected 403, got %d", rec.Code)
	}
}

func TestSubscriptionHandler_CancelSubscription_Success(t *testing.T) {
	h := newTestSubscriptionHandler(t)
	body, _ := json.Marshal(map[string]any{
		"name": "Basic", "amount": 999, "interval": "month",
		"interval_count": 1,
	})
	req := httptest.NewRequest(http.MethodPost, "/api/v1/subscriptions/plans", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	crec := httptest.NewRecorder()
	h.CreatePlan(crec, req)

	plan, err := decodeMap(crec.Body)
	if err != nil {
		t.Fatalf("decode: %v", err)
	}
	planID := getInt64(plan, "id")

	subBody, _ := json.Marshal(map[string]any{"plan_id": planID})
	subReq := httptest.NewRequest(http.MethodPost, "/api/v1/subscriptions", bytes.NewReader(subBody))
	subReq.Header.Set("Content-Type", "application/json")
	subReq = subReq.WithContext(middleware.ContextWithUser(subReq.Context(), middleware.UserInfo{UserID: 1}))
	srec := httptest.NewRecorder()
	h.Subscribe(srec, subReq)

	created, err := decodeMap(srec.Body)
	if err != nil {
		t.Fatalf("decode: %v", err)
	}
	subID := getInt64(created, "id")

	cancelReq := httptest.NewRequest(http.MethodPost, "/api/v1/subscriptions/"+strconv.FormatInt(subID, 10)+"/cancel", nil)
	cancelReq = pathvar.WithVars(cancelReq, map[string]string{"id": strconv.FormatInt(subID, 10)})
	cancelReq = cancelReq.WithContext(middleware.ContextWithUser(cancelReq.Context(), middleware.UserInfo{UserID: 1}))
	rec := httptest.NewRecorder()
	h.CancelSubscription(rec, cancelReq)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", rec.Code, rec.Body.String())
	}
	resp, err := decodeMap(rec.Body)
	if err != nil {
		t.Fatalf("decode: %v", err)
	}
	if resp["status"] != "cancelled" {
		t.Errorf("expected status=cancelled, got %v", resp["status"])
	}
}

func TestSubscriptionHandler_ChangePlan_PendingSubscription(t *testing.T) {
	h := newTestSubscriptionHandler(t)
	body, _ := json.Marshal(map[string]any{
		"name": "Basic", "amount": 999, "interval": "month",
		"interval_count": 1,
	})
	req := httptest.NewRequest(http.MethodPost, "/api/v1/subscriptions/plans", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	crec := httptest.NewRecorder()
	h.CreatePlan(crec, req)

	plan1, err := decodeMap(crec.Body)
	if err != nil {
		t.Fatalf("decode: %v", err)
	}
	plan1ID := getInt64(plan1, "id")

	body2, _ := json.Marshal(map[string]any{
		"name": "Pro", "amount": 1999, "interval": "month",
		"interval_count": 1,
	})
	req2 := httptest.NewRequest(http.MethodPost, "/api/v1/subscriptions/plans", bytes.NewReader(body2))
	req2.Header.Set("Content-Type", "application/json")
	crec2 := httptest.NewRecorder()
	h.CreatePlan(crec2, req2)

	plan2, err := decodeMap(crec2.Body)
	if err != nil {
		t.Fatalf("decode: %v", err)
	}
	plan2ID := getInt64(plan2, "id")

	subBody, _ := json.Marshal(map[string]any{"plan_id": plan1ID})
	subReq := httptest.NewRequest(http.MethodPost, "/api/v1/subscriptions", bytes.NewReader(subBody))
	subReq.Header.Set("Content-Type", "application/json")
	subReq = subReq.WithContext(middleware.ContextWithUser(subReq.Context(), middleware.UserInfo{UserID: 1}))
	srec := httptest.NewRecorder()
	h.Subscribe(srec, subReq)

	created, err := decodeMap(srec.Body)
	if err != nil {
		t.Fatalf("decode: %v", err)
	}
	subID := getInt64(created, "id")

	changeBody, _ := json.Marshal(map[string]any{"new_plan_id": plan2ID})
	changeReq := httptest.NewRequest(http.MethodPost, "/api/v1/subscriptions/"+strconv.FormatInt(subID, 10)+"/change-plan", bytes.NewReader(changeBody))
	changeReq = pathvar.WithVars(changeReq, map[string]string{"id": strconv.FormatInt(subID, 10)})
	changeReq.Header.Set("Content-Type", "application/json")
	changeReq = changeReq.WithContext(middleware.ContextWithUser(changeReq.Context(), middleware.UserInfo{UserID: 1}))
	rec := httptest.NewRecorder()
	h.ChangePlan(rec, changeReq)

	if rec.Code != http.StatusBadRequest {
		t.Errorf("expected 400 for pending subscription, got %d", rec.Code)
	}
}
