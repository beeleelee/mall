package rest

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strconv"
	"testing"

	"github.com/zeromicro/go-zero/rest/pathvar"

	"github.com/beeleelee/mall/domain/kernel"
	domain "github.com/beeleelee/mall/domain/order"
)

type fakeWebhookRepo struct {
	webhooks map[kernel.ID]*domain.Webhook
}

func newFakeWebhookRepo() *fakeWebhookRepo {
	return &fakeWebhookRepo{webhooks: make(map[kernel.ID]*domain.Webhook)}
}

func (f *fakeWebhookRepo) Save(_ context.Context, w *domain.Webhook) error {
	f.webhooks[w.ID] = w
	return nil
}

func (f *fakeWebhookRepo) FindByID(_ context.Context, id kernel.ID) (*domain.Webhook, error) {
	w, ok := f.webhooks[id]
	if !ok {
		return nil, kernel.NewDomainError(kernel.ErrNotFound, "webhook not found")
	}
	return w, nil
}

func (f *fakeWebhookRepo) FindByUserID(_ context.Context, userID kernel.ID) ([]*domain.Webhook, error) {
	var result []*domain.Webhook
	for _, w := range f.webhooks {
		if w.UserID == userID {
			result = append(result, w)
		}
	}
	return result, nil
}

func (f *fakeWebhookRepo) FindByEvent(_ context.Context, event string) ([]*domain.Webhook, error) {
	var result []*domain.Webhook
	for _, w := range f.webhooks {
		if !w.Active {
			continue
		}
		for _, e := range w.Events {
			if e == event {
				result = append(result, w)
				break
			}
		}
	}
	return result, nil
}

func (f *fakeWebhookRepo) Delete(_ context.Context, id kernel.ID) error {
	delete(f.webhooks, id)
	return nil
}

type webhookTestFixture struct {
	handler *WebhookHandler
	repo    *fakeWebhookRepo
}

func newWebhookTestFixture(t *testing.T) *webhookTestFixture {
	t.Helper()
	repo := newFakeWebhookRepo()
	sf, err := kernel.NewSnowflake(1)
	if err != nil {
		t.Fatalf("NewSnowflake: %v", err)
	}
	svc := domain.NewWebhookService(repo, sf)
	return &webhookTestFixture{handler: NewWebhookHandler(svc), repo: repo}
}

func TestWebhookHandler_Register_Success(t *testing.T) {
	f := newWebhookTestFixture(t)
	body := map[string]any{
		"url":    "https://example.com/hook",
		"secret": "mysecret",
		"events": []string{"order.confirmed", "order.shipped"},
	}
	data, _ := json.Marshal(body)
	req := userRequest(t, http.MethodPost, "/api/v1/webhooks", data, 1)
	rec := httptest.NewRecorder()
	f.handler.Register(rec, req)

	if rec.Code != http.StatusCreated {
		t.Fatalf("expected 201, got %d: %s", rec.Code, rec.Body.String())
	}

	var resp webhookResponse
	if err := json.NewDecoder(rec.Body).Decode(&resp); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	if resp.URL != "https://example.com/hook" {
		t.Errorf("expected URL, got %s", resp.URL)
	}
	if resp.UserID != 1 {
		t.Errorf("expected UserID 1, got %d", resp.UserID)
	}
	if !resp.Active {
		t.Errorf("expected active to be true")
	}
}

func TestWebhookHandler_Register_EmptyURL(t *testing.T) {
	f := newWebhookTestFixture(t)
	body := map[string]any{
		"url":    "",
		"secret": "mysecret",
		"events": []string{},
	}
	data, _ := json.Marshal(body)
	req := userRequest(t, http.MethodPost, "/api/v1/webhooks", data, 1)
	rec := httptest.NewRecorder()
	f.handler.Register(rec, req)

	if rec.Code != http.StatusBadRequest {
		t.Errorf("expected 400 for empty URL, got %d", rec.Code)
	}
}

func TestWebhookHandler_ListByUser_Success(t *testing.T) {
	f := newWebhookTestFixture(t)

	// seed webhooks via repo
	sf, _ := kernel.NewSnowflake(1)
	id1, _ := sf.NextID()
	id2, _ := sf.NextID()
	id3, _ := sf.NextID()
	w1, _ := domain.NewWebhook(id1, 1, "https://example.com/hook1", "s1", []string{"order.created"})
	w2, _ := domain.NewWebhook(id2, 1, "https://example.com/hook2", "s2", []string{"order.shipped"})
	w3, _ := domain.NewWebhook(id3, 2, "https://example.com/hook3", "s3", []string{"order.created"})
	f.repo.Save(context.Background(), w1)
	f.repo.Save(context.Background(), w2)
	f.repo.Save(context.Background(), w3)

	req := userRequest(t, http.MethodGet, "/api/v1/webhooks", nil, 1)
	rec := httptest.NewRecorder()
	f.handler.ListByUser(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", rec.Code)
	}

	var resp []webhookResponse
	if err := json.NewDecoder(rec.Body).Decode(&resp); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	if len(resp) != 2 {
		t.Fatalf("expected 2 webhooks for user 1, got %d", len(resp))
	}
}

func TestWebhookHandler_Delete_Success(t *testing.T) {
	f := newWebhookTestFixture(t)
	body := map[string]any{
		"url":    "https://example.com/hook",
		"secret": "s",
		"events": []string{"order.created"},
	}
	data, _ := json.Marshal(body)
	regReq := userRequest(t, http.MethodPost, "/api/v1/webhooks", data, 1)
	regRec := httptest.NewRecorder()
	f.handler.Register(regRec, regReq)

	var created webhookResponse
	json.NewDecoder(regRec.Body).Decode(&created)

	idStr := strconv.FormatInt(created.ID, 10)
	delReq := userRequest(t, http.MethodDelete, "/api/v1/webhooks/"+idStr, nil, 1)
	delReq = pathvar.WithVars(delReq, map[string]string{"id": idStr})
	rec := httptest.NewRecorder()
	f.handler.Delete(rec, delReq)

	if rec.Code != http.StatusNoContent {
		t.Errorf("expected 204, got %d", rec.Code)
	}
}

func TestWebhookHandler_Delete_WrongUser(t *testing.T) {
	f := newWebhookTestFixture(t)
	sf, _ := kernel.NewSnowflake(1)
	id, _ := sf.NextID()
	w, _ := domain.NewWebhook(id, 1, "https://example.com/hook", "s", []string{"order.created"})
	f.repo.Save(context.Background(), w)

	idStr := strconv.FormatInt(w.ID.Int64(), 10)
	delReq := userRequest(t, http.MethodDelete, "/api/v1/webhooks/"+idStr, nil, 2)
	delReq = pathvar.WithVars(delReq, map[string]string{"id": idStr})
	rec := httptest.NewRecorder()
	f.handler.Delete(rec, delReq)

	if rec.Code != http.StatusForbidden {
		t.Errorf("expected 403 for wrong user, got %d", rec.Code)
	}
}

func TestWebhookHandler_Unauthenticated(t *testing.T) {
	f := newWebhookTestFixture(t)
	req := httptest.NewRequest(http.MethodPost, "/api/v1/webhooks", nil)
	rec := httptest.NewRecorder()
	f.handler.Register(rec, req)

	if rec.Code != http.StatusUnauthorized {
		t.Errorf("expected 401, got %d", rec.Code)
	}
}
