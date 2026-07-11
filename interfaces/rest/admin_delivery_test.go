package rest

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"sync"
	"testing"

	orderdomain "github.com/beeleelee/mall/domain/order"
)

type fakeDeliveryLogRepo struct {
	mu      sync.Mutex
	entries []orderdomain.DeliveryLogEntry
}

func (f *fakeDeliveryLogRepo) Save(_ context.Context, entry *orderdomain.DeliveryLogEntry) error {
	f.mu.Lock()
	defer f.mu.Unlock()
	if entry.ID == 0 {
		entry.ID = int64(len(f.entries) + 1)
	}
	f.entries = append(f.entries, *entry)
	return nil
}

func (f *fakeDeliveryLogRepo) MarkRetried(_ context.Context, logID int64) error {
	f.mu.Lock()
	defer f.mu.Unlock()
	for i, e := range f.entries {
		if e.ID == logID {
			f.entries[i].Status = "retried"
			return nil
		}
	}
	return nil
}

func (f *fakeDeliveryLogRepo) MarkDelivered(_ context.Context, logID int64) error {
	f.mu.Lock()
	defer f.mu.Unlock()
	for i, e := range f.entries {
		if e.ID == logID {
			f.entries[i].Status = "delivered"
			return nil
		}
	}
	return nil
}

func (f *fakeDeliveryLogRepo) ListFailed(_ context.Context, limit int) ([]orderdomain.DeliveryLogEntry, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	var result []orderdomain.DeliveryLogEntry
	for _, e := range f.entries {
		if e.Status == "failed" || e.Status == "" {
			result = append(result, e)
		}
	}
	return result, nil
}

func (f *fakeDeliveryLogRepo) ListFailedDueForRetry(_ context.Context, limit int) ([]orderdomain.DeliveryLogEntry, error) {
	return f.ListFailed(context.Background(), limit)
}

func TestAdminHandler_ListFailedDeliveries_Empty(t *testing.T) {
	handler := &AdminHandler{deliveryLogRepo: &fakeDeliveryLogRepo{}}

	req := httptest.NewRequest(http.MethodGet, "/api/v1/admin/webhooks/failed", nil)
	w := httptest.NewRecorder()

	handler.ListFailedDeliveries(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", w.Code)
	}

	var resp []deliveryLogResponse
	if err := json.NewDecoder(w.Body).Decode(&resp); err != nil {
		t.Fatal(err)
	}
	if len(resp) != 0 {
		t.Fatalf("expected empty list, got %d items", len(resp))
	}
}

func TestAdminHandler_ListFailedDeliveries_WithEntries(t *testing.T) {
	repo := &fakeDeliveryLogRepo{}
	repo.Save(context.Background(), &orderdomain.DeliveryLogEntry{
		WebhookID: 1,
		Event:     "order.shipped",
		Payload:   []byte(`{}`),
		Status:    "failed",
		Error:     "connection refused",
		Attempts:  3,
	})
	repo.Save(context.Background(), &orderdomain.DeliveryLogEntry{
		WebhookID: 2,
		Event:     "order.confirmed",
		Payload:   []byte(`{}`),
		Status:    "failed",
		Error:     "timeout",
		Attempts:  1,
	})

	handler := &AdminHandler{deliveryLogRepo: repo}
	req := httptest.NewRequest(http.MethodGet, "/api/v1/admin/webhooks/failed", nil)
	w := httptest.NewRecorder()

	handler.ListFailedDeliveries(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", w.Code)
	}

	var resp []deliveryLogResponse
	if err := json.NewDecoder(w.Body).Decode(&resp); err != nil {
		t.Fatal(err)
	}
	if len(resp) != 2 {
		t.Fatalf("expected 2 entries, got %d", len(resp))
	}
	if resp[0].Event != "order.shipped" {
		t.Fatalf("expected first event 'order.shipped', got %q", resp[0].Event)
	}
}
