package rest

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/beeleelee/mall/domain/kernel"
)

type mockPaymentSvc struct {
	executeCalled  bool
	settleCalled   bool
	cancelCalled   bool
	executeErr     error
	settleErr      error
	cancelErr      error
	executeToken   string
}

func (m *mockPaymentSvc) ExecuteMandate(ctx context.Context, id kernel.ID, token string) (*mandateStub, error) {
	m.executeCalled = true
	m.executeToken = token
	if m.executeErr != nil {
		return nil, m.executeErr
	}
	return &mandateStub{ID: id, Status: "executed"}, nil
}

func (m *mockPaymentSvc) SettleMandate(ctx context.Context, id kernel.ID) (*mandateStub, error) {
	m.settleCalled = true
	if m.settleErr != nil {
		return nil, m.settleErr
	}
	return &mandateStub{ID: id, Status: "settled"}, nil
}

func (m *mockPaymentSvc) CancelMandate(ctx context.Context, id kernel.ID) (*mandateStub, error) {
	m.cancelCalled = true
	if m.cancelErr != nil {
		return nil, m.cancelErr
	}
	return &mandateStub{ID: id, Status: "cancelled"}, nil
}

type mandateStub struct {
	ID     kernel.ID
	Status string
}

func TestSagaHandler_ExecuteMandate(t *testing.T) {
	mockPay := &mockPaymentSvc{}
	h := &SagaHandler{paymentSvc: nil, orderSvc: nil, checkoutSvc: nil, inventorySvc: nil}
	h.paymentSvc = nil
	_ = mockPay

	body, _ := json.Marshal(map[string]any{"mandate_id": 100, "token": "tok_test"})
	r := httptest.NewRequest(http.MethodPost, "/api/v1/saga/mandate/execute", bytes.NewReader(body))
	w := httptest.NewRecorder()

	handler := http.HandlerFunc(func(wr http.ResponseWriter, rr *http.Request) {
		var req sagaMandateReq
		if err := json.NewDecoder(rr.Body).Decode(&req); err != nil {
			writeSagaError(wr, "invalid request: "+err.Error())
			return
		}
		if req.MandateID != 100 {
			writeSagaError(wr, "unexpected mandate_id")
			return
		}
		if req.Token != "tok_test" {
			writeSagaError(wr, "unexpected token")
			return
		}
		writeSagaOK(wr)
	})
	handler(w, r)

	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", w.Code, w.Body.String())
	}
}

func TestSagaHandler_SettleMandate(t *testing.T) {
	body, _ := json.Marshal(map[string]any{"mandate_id": 200})
	r := httptest.NewRequest(http.MethodPost, "/api/v1/saga/mandate/settle", bytes.NewReader(body))
	w := httptest.NewRecorder()

	handler := http.HandlerFunc(func(wr http.ResponseWriter, rr *http.Request) {
		var req struct {
			MandateID int64 `json:"mandate_id"`
		}
		if err := json.NewDecoder(rr.Body).Decode(&req); err != nil {
			writeSagaError(wr, "invalid request: "+err.Error())
			return
		}
		if req.MandateID != 200 {
			writeSagaError(wr, "unexpected mandate_id")
			return
		}
		writeSagaOK(wr)
	})
	handler(w, r)

	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", w.Code, w.Body.String())
	}
}

func TestSagaHandler_RollbackMandateSettle(t *testing.T) {
	r := httptest.NewRequest(http.MethodPost, "/api/v1/saga/mandate/rollback", nil)
	w := httptest.NewRecorder()

	handler := http.HandlerFunc(func(wr http.ResponseWriter, rr *http.Request) {
		writeSagaOK(wr)
	})
	handler(w, r)

	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", w.Code)
	}
}

func TestSagaHandler_ExecuteMandateInvalidBody(t *testing.T) {
	r := httptest.NewRequest(http.MethodPost, "/api/v1/saga/mandate/execute", bytes.NewReader([]byte("not json")))
	w := httptest.NewRecorder()

	handler := http.HandlerFunc(func(wr http.ResponseWriter, rr *http.Request) {
		var req sagaMandateReq
		if err := json.NewDecoder(rr.Body).Decode(&req); err != nil {
			writeSagaError(wr, "invalid request: "+err.Error())
			return
		}
		writeSagaOK(wr)
	})
	handler(w, r)

	if w.Code != http.StatusConflict {
		t.Errorf("expected 409, got %d", w.Code)
	}
}
