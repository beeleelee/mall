package order

import (
	"context"
	"net/http"
	"net/http/httptest"
	"sync"
	"testing"

	domain "github.com/beeleelee/mall/domain/order"
)

type fakeDeliveryLogRepo struct {
	mu      sync.Mutex
	entries []domain.DeliveryLogEntry
}

func (f *fakeDeliveryLogRepo) Save(_ context.Context, entry *domain.DeliveryLogEntry) error {
	f.mu.Lock()
	defer f.mu.Unlock()
	if entry.ID == 0 {
		entry.ID = int64(len(f.entries) + 1)
		f.entries = append(f.entries, *entry)
	} else {
		for i, e := range f.entries {
			if e.ID == entry.ID {
				f.entries[i] = *entry
				return nil
			}
		}
		f.entries = append(f.entries, *entry)
	}
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

func (f *fakeDeliveryLogRepo) ListFailed(_ context.Context, limit int) ([]domain.DeliveryLogEntry, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	var result []domain.DeliveryLogEntry
	for _, e := range f.entries {
		if e.Status == "failed" {
			result = append(result, e)
		}
	}
	return result, nil
}

func (f *fakeDeliveryLogRepo) ListFailedDueForRetry(_ context.Context, _ int) ([]domain.DeliveryLogEntry, error) {
	return f.ListFailed(context.Background(), 50)
}

func TestWebhookDeliverer_Success_LogsDelivered(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))
	defer server.Close()

	logRepo := &fakeDeliveryLogRepo{}
	d := NewWebhookDeliverer(WithDeliveryLogRepo(logRepo))

	wh, _ := domain.NewWebhook(1, 1, server.URL, "test-secret", []string{"order.shipped"})

	err := d.Deliver(context.Background(), wh, "order.shipped", []byte(`{"id":1}`))
	if err != nil {
		t.Fatalf("expected no error, got: %v", err)
	}

	if len(logRepo.entries) != 1 {
		t.Fatalf("expected 1 log entry, got %d", len(logRepo.entries))
	}
	if logRepo.entries[0].Status != "delivered" {
		t.Fatalf("expected status 'delivered', got %q", logRepo.entries[0].Status)
	}
}

func TestWebhookDeliverer_Failure_LogsFailed(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
	}))
	defer server.Close()

	logRepo := &fakeDeliveryLogRepo{}
	d := NewWebhookDeliverer(WithDeliveryLogRepo(logRepo))

	wh, _ := domain.NewWebhook(1, 1, server.URL, "test-secret", []string{"order.shipped"})

	err := d.Deliver(context.Background(), wh, "order.shipped", []byte(`{"id":1}`))
	if err == nil {
		t.Fatal("expected error for 500 response")
	}

	if len(logRepo.entries) == 0 {
		t.Fatal("expected at least 1 log entry")
	}
	last := logRepo.entries[len(logRepo.entries)-1]
	if last.Status != "failed" {
		t.Fatalf("expected final status 'failed', got %q", last.Status)
	}
	if last.NextRetry == nil {
		t.Fatal("expected next_retry to be set after final failure")
	}
}

func TestWebhookDeliverer_NoLogRepo_StillWorks(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))
	defer server.Close()

	d := NewWebhookDeliverer()
	wh, _ := domain.NewWebhook(1, 1, server.URL, "test-secret", []string{"order.shipped"})

	err := d.Deliver(context.Background(), wh, "order.shipped", []byte(`{"id":1}`))
	if err != nil {
		t.Fatalf("expected no error, got: %v", err)
	}
}

func TestWebhookDeliverer_SendsCorrectHeaders(t *testing.T) {
	var capturedSignature, capturedTimestamp string
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		capturedSignature = r.Header.Get("X-Signature-256")
		capturedTimestamp = r.Header.Get("X-Signature-Timestamp")
		w.WriteHeader(http.StatusOK)
	}))
	defer server.Close()

	d := NewWebhookDeliverer()
	wh, _ := domain.NewWebhook(1, 1, server.URL, "test-secret", []string{"order.shipped"})

	err := d.Deliver(context.Background(), wh, "order.shipped", []byte(`{"id":1}`))
	if err != nil {
		t.Fatalf("expected no error, got: %v", err)
	}

	if capturedSignature == "" {
		t.Fatal("expected X-Signature-256 header")
	}
	if capturedTimestamp == "" {
		t.Fatal("expected X-Signature-Timestamp header")
	}

	expectedSig := domain.SignWebhookPayload("test-secret", []byte(`{"id":1}`))
	if capturedSignature != expectedSig {
		t.Fatalf("expected signature %q, got %q", expectedSig, capturedSignature)
	}
}
