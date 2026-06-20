package rest

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/zeromicro/go-zero/rest/pathvar"

	"github.com/beeleelee/mall/domain/kernel"
	domain "github.com/beeleelee/mall/domain/payment"
	"github.com/beeleelee/mall/interfaces/middleware"
)

func TestPaymentHandler_CreateMandate(t *testing.T) {
	h := newTestPaymentHandler(t)

	body, _ := json.Marshal(map[string]any{
		"max_amount":  10000,
		"merchant_id": 1,
		"expiry":      time.Now().Add(24 * time.Hour).Format(time.RFC3339),
	})
	req := userRequest(t, http.MethodPost, "/api/v1/payments/mandates", body, 42)
	rec := httptest.NewRecorder()
	h.CreateMandate(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", rec.Code, rec.Body.String())
	}

	var resp map[string]any
	if err := json.Unmarshal(rec.Body.Bytes(), &resp); err != nil {
		t.Fatalf("decode: %v", err)
	}
	if resp["status"] != "requested" {
		t.Fatalf("expected status requested, got %v", resp["status"])
	}
	if resp["user_id"] != float64(42) {
		t.Fatalf("expected user_id 42, got %v", resp["user_id"])
	}
}

func TestPaymentHandler_GetMandate(t *testing.T) {
	h := newTestPaymentHandler(t)

	body, _ := json.Marshal(map[string]any{
		"mandate_id":  100,
		"max_amount":  10000,
		"merchant_id": 1,
		"expiry":      time.Now().Add(24 * time.Hour).Format(time.RFC3339),
	})
	req := userRequest(t, http.MethodPost, "/api/v1/payments/mandates", body, 42)
	rec := httptest.NewRecorder()
	h.CreateMandate(rec, req)

	req2 := httptest.NewRequest(http.MethodGet, "/api/v1/payments/mandates/100", nil)
	req2 = req2.WithContext(middleware.ContextWithUser(req2.Context(), middleware.UserInfo{UserID: 42}))
	req2 = pathvar.WithVars(req2, map[string]string{"id": "100"})

	rec2 := httptest.NewRecorder()
	h.GetMandate(rec2, req2)

	if rec2.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", rec2.Code, rec2.Body.String())
	}
}

func TestPaymentHandler_ApproveAndExecute(t *testing.T) {
	h := newTestPaymentHandler(t)

	body, _ := json.Marshal(map[string]any{
		"mandate_id":  200,
		"max_amount":  10000,
		"merchant_id": 1,
		"expiry":      time.Now().Add(24 * time.Hour).Format(time.RFC3339),
	})
	req := userRequest(t, http.MethodPost, "/api/v1/payments/mandates", body, 42)
	rec := httptest.NewRecorder()
	h.CreateMandate(rec, req)

	approveBody, _ := json.Marshal(map[string]any{"signature": "sig-1"})
	req2 := httptest.NewRequest(http.MethodPost, "/api/v1/payments/mandates/200/approve", bytes.NewReader(approveBody))
	req2 = req2.WithContext(middleware.ContextWithUser(req2.Context(), middleware.UserInfo{UserID: 42}))
	req2 = pathvar.WithVars(req2, map[string]string{"id": "200"})
	rec2 := httptest.NewRecorder()
	h.ApproveMandate(rec2, req2)

	if rec2.Code != http.StatusOK {
		t.Fatalf("expected 200 approve, got %d: %s", rec2.Code, rec2.Body.String())
	}

	execBody, _ := json.Marshal(map[string]any{"token": "tok-1"})
	req3 := httptest.NewRequest(http.MethodPost, "/api/v1/payments/mandates/200/execute", bytes.NewReader(execBody))
	req3 = req3.WithContext(middleware.ContextWithUser(req3.Context(), middleware.UserInfo{UserID: 42}))
	req3 = pathvar.WithVars(req3, map[string]string{"id": "200"})
	rec3 := httptest.NewRecorder()
	h.ExecuteMandate(rec3, req3)

	if rec3.Code != http.StatusOK {
		t.Fatalf("expected 200 execute, got %d: %s", rec3.Code, rec3.Body.String())
	}

	var resp map[string]any
	json.Unmarshal(rec3.Body.Bytes(), &resp)
	if resp["status"] != "executed" {
		t.Fatalf("expected executed, got %v", resp["status"])
	}
}

func newTestPaymentHandler(t *testing.T) *PaymentHandler {
	t.Helper()
	repo := newFakePaymentRepo()
	svc := domain.NewPaymentService(repo, fakeLoggerPayment{})
	sf, err := kernel.NewSnowflake(2)
	if err != nil {
		t.Fatalf("NewSnowflake: %v", err)
	}
	return NewPaymentHandler(svc, sf)
}

type fakePaymentRepo struct {
	mandates map[kernel.ID]*domain.Mandate
}

func newFakePaymentRepo() *fakePaymentRepo {
	return &fakePaymentRepo{mandates: make(map[kernel.ID]*domain.Mandate)}
}

func (r *fakePaymentRepo) Save(_ context.Context, m *domain.Mandate) error {
	r.mandates[m.ID] = m
	return nil
}
func (r *fakePaymentRepo) FindByID(_ context.Context, id kernel.ID) (*domain.Mandate, error) {
	m, ok := r.mandates[id]
	if !ok {
		return nil, kernel.NewDomainError(kernel.ErrNotFound, "not found")
	}
	return m, nil
}
func (r *fakePaymentRepo) FindByUserID(_ context.Context, uid kernel.ID) ([]*domain.Mandate, error) {
	var res []*domain.Mandate
	for _, m := range r.mandates {
		if m.UserID == uid {
			res = append(res, m)
		}
	}
	return res, nil
}
func (r *fakePaymentRepo) FindActiveByUser(_ context.Context, uid kernel.ID) ([]*domain.Mandate, error) {
	var res []*domain.Mandate
	for _, m := range r.mandates {
		if m.UserID == uid && m.IsActive() {
			res = append(res, m)
		}
	}
	return res, nil
}
func (r *fakePaymentRepo) Delete(_ context.Context, id kernel.ID) error {
	delete(r.mandates, id)
	return nil
}

type fakeLoggerPayment struct{}

func (fakeLoggerPayment) Debug(_ context.Context, _ string, _ ...kernel.LogField)          {}
func (fakeLoggerPayment) Info(_ context.Context, _ string, _ ...kernel.LogField)           {}
func (fakeLoggerPayment) Warn(_ context.Context, _ string, _ ...kernel.LogField)           {}
func (fakeLoggerPayment) Error(_ context.Context, _ string, _ error, _ ...kernel.LogField) {}
