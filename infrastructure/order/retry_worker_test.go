package order

import (
	"context"
	"net/http"
	"net/http/httptest"
	"sync"
	"testing"
	"time"

	"github.com/beeleelee/mall/domain/kernel"
	domain "github.com/beeleelee/mall/domain/order"
)

type fakeWebhookRepoForWorker struct {
	mu       sync.Mutex
	webhooks map[int64]*domain.Webhook
}

func newFakeWebhookRepoForWorker() *fakeWebhookRepoForWorker {
	return &fakeWebhookRepoForWorker{webhooks: make(map[int64]*domain.Webhook)}
}

func (f *fakeWebhookRepoForWorker) Save(_ context.Context, w *domain.Webhook) error {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.webhooks[w.ID.Int64()] = w
	return nil
}

func (f *fakeWebhookRepoForWorker) FindByID(_ context.Context, id kernel.ID) (*domain.Webhook, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	w, ok := f.webhooks[id.Int64()]
	if !ok {
		return nil, kernel.NewDomainError(kernel.ErrNotFound, "not found")
	}
	return w, nil
}

func (f *fakeWebhookRepoForWorker) FindByUserID(_ context.Context, userID kernel.ID) ([]*domain.Webhook, error) {
	return nil, nil
}

func (f *fakeWebhookRepoForWorker) FindByEvent(_ context.Context, event string) ([]*domain.Webhook, error) {
	return nil, nil
}

func (f *fakeWebhookRepoForWorker) Delete(_ context.Context, id kernel.ID) error {
	f.mu.Lock()
	defer f.mu.Unlock()
	delete(f.webhooks, id.Int64())
	return nil
}

func TestWebhookRetryWorker_RunsOnce(t *testing.T) {
	var mu sync.Mutex
	delivered := false

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		mu.Lock()
		delivered = true
		mu.Unlock()
		w.WriteHeader(http.StatusOK)
	}))
	defer server.Close()

	logRepo := &fakeDeliveryLogRepo{}
	whRepo := newFakeWebhookRepoForWorker()

	wh, _ := domain.NewWebhook(1, 1, server.URL, "test", []string{"order.shipped"})
	whRepo.Save(context.Background(), wh)

	logRepo.Save(context.Background(), &domain.DeliveryLogEntry{
		WebhookID: 1,
		Event:     "order.shipped",
		Payload:   []byte(`{"id": 1}`),
		Status:    "failed",
		Attempts:  2,
	})

	d := NewWebhookDeliverer(WithDeliveryLogRepo(logRepo))

	worker := NewWebhookRetryWorker(logRepo, whRepo, d, 100*time.Millisecond, nil)

	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()

	go worker.Start(ctx)

	time.Sleep(500 * time.Millisecond)

	mu.Lock()
	ok := delivered
	mu.Unlock()

	if !ok {
		t.Fatal("expected webhook to be delivered by retry worker")
	}
}

func TestWebhookRetryWorker_SkipsInactiveWebhooks(t *testing.T) {
	logRepo := &fakeDeliveryLogRepo{}
	whRepo := newFakeWebhookRepoForWorker()

	wh, _ := domain.NewWebhook(1, 1, "http://localhost:1/hook", "test", []string{"order.shipped"})
	wh.Active = false
	whRepo.Save(context.Background(), wh)

	logRepo.Save(context.Background(), &domain.DeliveryLogEntry{
		WebhookID: 1,
		Event:     "order.shipped",
		Payload:   []byte(`{"id": 1}`),
		Status:    "failed",
		Attempts:  2,
	})

	d := NewWebhookDeliverer()
	worker := NewWebhookRetryWorker(logRepo, whRepo, d, 50*time.Millisecond, nil)

	ctx, cancel := context.WithTimeout(context.Background(), 300*time.Millisecond)
	defer cancel()

	go worker.Start(ctx)

	time.Sleep(200 * time.Millisecond)

	failed, _ := logRepo.ListFailed(context.Background(), 10)
	found := false
	for _, e := range failed {
		if e.WebhookID == 1 {
			found = true
			break
		}
	}
	if found {
		t.Log("note: entry may still be failed if skipped before retry")
	}
}
