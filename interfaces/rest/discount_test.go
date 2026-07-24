package rest

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"sync"
	"testing"
	"time"

	domain "github.com/beeleelee/mall/domain/discount"
	"github.com/beeleelee/mall/domain/kernel"
)

type fakeDiscountRepo struct {
	mu   sync.Mutex
	byID map[kernel.ID]*domain.DiscountCode
}

func newFakeDiscountRepo() *fakeDiscountRepo {
	return &fakeDiscountRepo{byID: make(map[kernel.ID]*domain.DiscountCode)}
}

func (f *fakeDiscountRepo) Save(_ context.Context, code *domain.DiscountCode) error {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.byID[code.ID] = code
	return nil
}

func (f *fakeDiscountRepo) FindByCode(_ context.Context, code string) (*domain.DiscountCode, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	for _, dc := range f.byID {
		if dc.Code == code {
			return dc, nil
		}
	}
	return nil, kernel.NewDomainError(kernel.ErrNotFound, "discount code not found")
}

func (f *fakeDiscountRepo) IncrementUsage(_ context.Context, id kernel.ID) error {
	f.mu.Lock()
	defer f.mu.Unlock()
	dc, ok := f.byID[id]
	if !ok {
		return kernel.NewDomainError(kernel.ErrNotFound, "not found")
	}
	dc.Use()
	return nil
}

func newTestDiscountHandler(t *testing.T) *DiscountHandler {
	t.Helper()
	repo := newFakeDiscountRepo()
	logger := fakeLog{}
	svc := domain.NewDiscountService(repo, logger)
	sf, err := kernel.NewSnowflake(1)
	if err != nil {
		t.Fatalf("NewSnowflake: %v", err)
	}
	return NewDiscountHandler(svc, sf)
}

func writeDiscountValidate(t *testing.T, h *DiscountHandler, code string, subtotal int64) *httptest.ResponseRecorder {
	t.Helper()
	body, _ := json.Marshal(map[string]any{"code": code, "subtotal": subtotal})
	req := httptest.NewRequest(http.MethodPost, "/api/v1/discounts/validate", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()
	h.Validate(rec, req)
	return rec
}

func TestDiscountHandler_Create_Success(t *testing.T) {
	h := newTestDiscountHandler(t)
	body, _ := json.Marshal(map[string]any{
		"code": "SAVE10", "type": "flat", "value": 1000,
		"min_purchase": 5000, "max_usages": 100,
		"expiry": time.Now().Add(24 * time.Hour).Format(time.RFC3339),
		"stackable": false,
	})
	req := httptest.NewRequest(http.MethodPost, "/api/v1/discounts", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()
	h.Create(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", rec.Code, rec.Body.String())
	}
	var resp map[string]any
	if err := json.NewDecoder(rec.Body).Decode(&resp); err != nil {
		t.Fatalf("decode: %v", err)
	}
	if resp["code"] != "SAVE10" {
		t.Errorf("expected SAVE10, got %v", resp["code"])
	}
}

func TestDiscountHandler_Create_InvalidBody(t *testing.T) {
	h := newTestDiscountHandler(t)
	req := httptest.NewRequest(http.MethodPost, "/api/v1/discounts", bytes.NewReader([]byte("not json")))
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()
	h.Create(rec, req)

	if rec.Code != http.StatusBadRequest {
		t.Errorf("expected 400, got %d", rec.Code)
	}
}

func TestDiscountHandler_Create_InvalidExpiry(t *testing.T) {
	h := newTestDiscountHandler(t)
	body, _ := json.Marshal(map[string]any{
		"code": "BADEXP", "type": "flat", "value": 500,
		"min_purchase": 0, "max_usages": 10,
		"expiry": "not-a-date",
		"stackable": false,
	})
	req := httptest.NewRequest(http.MethodPost, "/api/v1/discounts", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()
	h.Create(rec, req)

	if rec.Code != http.StatusBadRequest {
		t.Errorf("expected 400, got %d", rec.Code)
	}
}

func TestDiscountHandler_Validate_Valid(t *testing.T) {
	h := newTestDiscountHandler(t)
	body, _ := json.Marshal(map[string]any{
		"code": "VALID10", "type": "flat", "value": 500,
		"min_purchase": 0, "max_usages": 10,
		"expiry": time.Now().Add(24 * time.Hour).Format(time.RFC3339),
		"stackable": false,
	})
	req := httptest.NewRequest(http.MethodPost, "/api/v1/discounts", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	h.Create(httptest.NewRecorder(), req)

	rec := writeDiscountValidate(t, h, "VALID10", 1000)
	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", rec.Code)
	}
	var resp map[string]any
	json.NewDecoder(rec.Body).Decode(&resp)
	if resp["valid"] != true {
		t.Errorf("expected valid=true, got %v", resp["valid"])
	}
}

func TestDiscountHandler_Validate_NotFound(t *testing.T) {
	h := newTestDiscountHandler(t)
	rec := writeDiscountValidate(t, h, "NONEXISTENT", 1000)
	if rec.Code != http.StatusNotFound {
		t.Errorf("expected 404, got %d", rec.Code)
	}
}

func TestDiscountHandler_Apply_Success(t *testing.T) {
	h := newTestDiscountHandler(t)
	body, _ := json.Marshal(map[string]any{
		"code": "APPLY10", "type": "flat", "value": 1000,
		"min_purchase": 0, "max_usages": 10,
		"expiry": time.Now().Add(24 * time.Hour).Format(time.RFC3339),
		"stackable": false,
	})
	req := httptest.NewRequest(http.MethodPost, "/api/v1/discounts", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	h.Create(httptest.NewRecorder(), req)

	applyBody, _ := json.Marshal(map[string]any{"code": "APPLY10", "subtotal": 5000})
	applyReq := httptest.NewRequest(http.MethodPost, "/api/v1/discounts/apply", bytes.NewReader(applyBody))
	applyReq.Header.Set("Content-Type", "application/json")
	applyRec := httptest.NewRecorder()
	h.Apply(applyRec, applyReq)

	if applyRec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", applyRec.Code)
	}
	var resp map[string]any
	json.NewDecoder(applyRec.Body).Decode(&resp)
	if resp["final"].(float64) != 4000 {
		t.Errorf("expected final=4000, got %v", resp["final"])
	}
	if resp["applied"] != true {
		t.Errorf("expected applied=true, got %v", resp["applied"])
	}
}

func TestDiscountHandler_Apply_NotFound(t *testing.T) {
	h := newTestDiscountHandler(t)
	body, _ := json.Marshal(map[string]any{"code": "MISSING", "subtotal": 5000})
	req := httptest.NewRequest(http.MethodPost, "/api/v1/discounts/apply", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()
	h.Apply(rec, req)

	if rec.Code != http.StatusNotFound {
		t.Errorf("expected 404, got %d", rec.Code)
	}
}

func TestDiscountHandler_Deactivate_Success(t *testing.T) {
	h := newTestDiscountHandler(t)
	body, _ := json.Marshal(map[string]any{
		"code": "DEACTIVATE", "type": "flat", "value": 500,
		"min_purchase": 0, "max_usages": 10,
		"expiry": time.Now().Add(24 * time.Hour).Format(time.RFC3339),
		"stackable": false,
	})
	req := httptest.NewRequest(http.MethodPost, "/api/v1/discounts", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	h.Create(httptest.NewRecorder(), req)

	deactBody, _ := json.Marshal(map[string]string{"code": "DEACTIVATE"})
	deactReq := httptest.NewRequest(http.MethodPost, "/api/v1/discounts/deactivate", bytes.NewReader(deactBody))
	deactReq.Header.Set("Content-Type", "application/json")
	deactRec := httptest.NewRecorder()
	h.Deactivate(deactRec, deactReq)

	if deactRec.Code != http.StatusNoContent {
		t.Errorf("expected 204, got %d", deactRec.Code)
	}

	rec := writeDiscountValidate(t, h, "DEACTIVATE", 5000)
	var resp map[string]any
	json.NewDecoder(rec.Body).Decode(&resp)
	if resp["valid"] != false {
		t.Errorf("expected valid=false after deactivate, got %v", resp["valid"])
	}
}

func TestDiscountHandler_Deactivate_NotFound(t *testing.T) {
	h := newTestDiscountHandler(t)
	body, _ := json.Marshal(map[string]string{"code": "NOEXIST"})
	req := httptest.NewRequest(http.MethodPost, "/api/v1/discounts/deactivate", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()
	h.Deactivate(rec, req)

	if rec.Code != http.StatusNotFound {
		t.Errorf("expected 404, got %d", rec.Code)
	}
}

func TestDiscountHandler_Create_InvalidValue(t *testing.T) {
	h := newTestDiscountHandler(t)
	body, _ := json.Marshal(map[string]any{
		"code": "BADVAL", "type": "percentage", "value": 150,
		"min_purchase": 0, "max_usages": 10,
		"expiry": time.Now().Add(24 * time.Hour).Format(time.RFC3339),
		"stackable": false,
	})
	req := httptest.NewRequest(http.MethodPost, "/api/v1/discounts", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()
	h.Create(rec, req)

	if rec.Code != http.StatusBadRequest {
		t.Errorf("expected 400, got %d", rec.Code)
	}
}
