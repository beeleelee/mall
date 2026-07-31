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
	domain "github.com/beeleelee/mall/domain/notification"
	"github.com/beeleelee/mall/interfaces/middleware"
)

type fakeNotifRepo struct {
	mu     sync.Mutex
	notifs map[kernel.ID]*domain.Notification
}

func newFakeNotifRepo() *fakeNotifRepo {
	return &fakeNotifRepo{notifs: make(map[kernel.ID]*domain.Notification)}
}

func (f *fakeNotifRepo) Write(ctx context.Context, n *domain.Notification) error {
	return f.Save(ctx, n)
}

func (f *fakeNotifRepo) Save(_ context.Context, n *domain.Notification) error {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.notifs[n.ID] = n
	return nil
}

func (f *fakeNotifRepo) FindByID(_ context.Context, id kernel.ID) (*domain.Notification, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	if n, ok := f.notifs[id]; ok {
		return n, nil
	}
	return nil, kernel.NewDomainError(kernel.ErrNotFound, "notification not found")
}

func (f *fakeNotifRepo) FindByUserID(_ context.Context, userID kernel.ID, limit int) ([]*domain.Notification, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	result := []*domain.Notification{}
	for _, n := range f.notifs {
		if n.UserID == userID {
			result = append(result, n)
		}
	}
	if limit > 0 && len(result) > limit {
		result = result[:limit]
	}
	return result, nil
}

func (f *fakeNotifRepo) MarkRead(_ context.Context, id, userID kernel.ID) error {
	f.mu.Lock()
	defer f.mu.Unlock()
	n, ok := f.notifs[id]
	if !ok || n.UserID != userID {
		return kernel.NewDomainError(kernel.ErrNotFound, "notification not found")
	}
	n.Read = true
	return nil
}

func (f *fakeNotifRepo) MarkAllRead(_ context.Context, userID kernel.ID) error {
	f.mu.Lock()
	defer f.mu.Unlock()
	for _, n := range f.notifs {
		if n.UserID == userID {
			n.Read = true
		}
	}
	return nil
}

func (f *fakeNotifRepo) UnreadCount(_ context.Context, userID kernel.ID) (int, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	count := 0
	for _, n := range f.notifs {
		if n.UserID == userID && !n.Read {
			count++
		}
	}
	return count, nil
}

type fakeNotifPrefRepo struct {
	mu    sync.Mutex
	prefs map[kernel.ID]*domain.NotificationPreferences
}

type fakeNotifEmailSender struct{}

func (fakeNotifEmailSender) Send(context.Context, domain.EmailMessage) error { return nil }

func (r *fakeNotifPrefRepo) Get(_ context.Context, userID kernel.ID) (*domain.NotificationPreferences, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	if p, ok := r.prefs[userID]; ok {
		return p, nil
	}
	return nil, kernel.NewDomainError(kernel.ErrNotFound, "notification preferences not found")
}

func (r *fakeNotifPrefRepo) Upsert(_ context.Context, p *domain.NotificationPreferences) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.prefs[p.UserID] = p
	return nil
}

func newTestNotificationHandler(t *testing.T) *NotificationHandler {
	t.Helper()
	repo := newFakeNotifRepo()
	sf, err := kernel.NewSnowflake(1)
	if err != nil {
		t.Fatalf("NewSnowflake: %v", err)
	}
	prefRepo := &fakeNotifPrefRepo{prefs: make(map[kernel.ID]*domain.NotificationPreferences)}
	svc := domain.NewNotificationService(
		fakeNotifEmailSender{},
		fakeLog{},
		domain.WithNotificationRepository(repo),
		domain.WithInAppWriter(repo),
		domain.WithPreferenceRepository(prefRepo),
		domain.WithSnowflake(sf),
	)
	return NewNotificationHandler(svc)
}

func authNotifRequest(r *http.Request, userID int64) *http.Request {
	return r.WithContext(middleware.ContextWithUser(r.Context(), middleware.UserInfo{UserID: userID}))
}

func TestNotificationHandler_List(t *testing.T) {
	h := newTestNotificationHandler(t)

	h.svc.NotifyInApp(context.Background(), 1, 50, domain.NotificationTypeOrder, "Order Confirmed", "Body 1")
	h.svc.NotifyInApp(context.Background(), 2, 50, domain.NotificationTypeShipping, "Shipped", "Body 2")
	h.svc.NotifyInApp(context.Background(), 3, 60, domain.NotificationTypeOrder, "Other", "Body 3")

	req := httptest.NewRequest(http.MethodGet, "/api/v1/notifications", nil)
	req = authNotifRequest(req, 50)
	rec := httptest.NewRecorder()
	h.List(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", rec.Code, rec.Body.String())
	}
	var resp struct {
		Notifications []notificationResponse `json:"notifications"`
		UnreadCount   int                    `json:"unread_count"`
	}
	if err := json.NewDecoder(rec.Body).Decode(&resp); err != nil {
		t.Fatalf("decode: %v", err)
	}
	if len(resp.Notifications) != 2 {
		t.Fatalf("expected 2 notifications, got %d", len(resp.Notifications))
	}
	if resp.UnreadCount != 2 {
		t.Fatalf("expected 2 unread, got %d", resp.UnreadCount)
	}
}

func TestNotificationHandler_List_Unauthenticated(t *testing.T) {
	h := newTestNotificationHandler(t)

	req := httptest.NewRequest(http.MethodGet, "/api/v1/notifications", nil)
	rec := httptest.NewRecorder()
	h.List(rec, req)

	if rec.Code != http.StatusUnauthorized {
		t.Fatalf("expected 401, got %d", rec.Code)
	}
}

func TestNotificationHandler_MarkRead(t *testing.T) {
	h := newTestNotificationHandler(t)

	h.svc.NotifyInApp(context.Background(), 1, 50, domain.NotificationTypeOrder, "Title", "Body")

	req := httptest.NewRequest(http.MethodPost, "/api/v1/notifications/1/read", nil)
	req = authNotifRequest(req, 50)
	req = pathvar.WithVars(req, map[string]string{"id": "1"})
	rec := httptest.NewRecorder()
	h.MarkRead(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", rec.Code, rec.Body.String())
	}

	notifs, _ := h.svc.ListByUser(context.Background(), 50, 0)
	if len(notifs) != 1 || !notifs[0].Read {
		t.Fatalf("expected notification marked read: %+v", notifs)
	}
}

func TestNotificationHandler_MarkRead_Ownership(t *testing.T) {
	h := newTestNotificationHandler(t)

	h.svc.NotifyInApp(context.Background(), 1, 50, domain.NotificationTypeOrder, "Title", "Body")

	req := httptest.NewRequest(http.MethodPost, "/api/v1/notifications/1/read", nil)
	req = authNotifRequest(req, 60)
	req = pathvar.WithVars(req, map[string]string{"id": "1"})
	rec := httptest.NewRecorder()
	h.MarkRead(rec, req)

	if rec.Code != http.StatusNotFound {
		t.Fatalf("expected 404 for other user's notification, got %d", rec.Code)
	}
}

func TestNotificationHandler_MarkAllRead(t *testing.T) {
	h := newTestNotificationHandler(t)

	h.svc.NotifyInApp(context.Background(), 1, 50, domain.NotificationTypeOrder, "Title", "Body")
	h.svc.NotifyInApp(context.Background(), 2, 50, domain.NotificationTypeShipping, "Title", "Body")
	h.svc.NotifyInApp(context.Background(), 3, 60, domain.NotificationTypeOrder, "Title", "Body")

	req := httptest.NewRequest(http.MethodPost, "/api/v1/notifications/mark-all-read", nil)
	req = authNotifRequest(req, 50)
	rec := httptest.NewRecorder()
	h.MarkAllRead(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", rec.Code)
	}

	notifs, _ := h.svc.ListByUser(context.Background(), 50, 0)
	for _, n := range notifs {
		if !n.Read {
			t.Fatalf("expected all notifications read: %+v", n)
		}
	}
}

func TestNotificationHandler_UnreadCount(t *testing.T) {
	h := newTestNotificationHandler(t)

	h.svc.NotifyInApp(context.Background(), 1, 50, domain.NotificationTypeOrder, "Title", "Body")
	h.svc.NotifyInApp(context.Background(), 2, 50, domain.NotificationTypeShipping, "Title", "Body")

	req := httptest.NewRequest(http.MethodGet, "/api/v1/notifications/unread-count", nil)
	req = authNotifRequest(req, 50)
	rec := httptest.NewRecorder()
	h.UnreadCount(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", rec.Code)
	}
	var resp map[string]any
	if err := json.NewDecoder(rec.Body).Decode(&resp); err != nil {
		t.Fatalf("decode: %v", err)
	}
	if resp["unread_count"].(float64) != 2 {
		t.Fatalf("expected 2 unread, got %v", resp["unread_count"])
	}
}

func TestNotificationHandler_GetPreferences_NotFound(t *testing.T) {
	h := newTestNotificationHandler(t)

	req := httptest.NewRequest(http.MethodGet, "/api/v1/notifications/preferences", nil)
	req = authNotifRequest(req, 50)
	rec := httptest.NewRecorder()
	h.GetPreferences(rec, req)

	if rec.Code != http.StatusNotFound {
		t.Fatalf("expected 404, got %d: %s", rec.Code, rec.Body.String())
	}
}

func TestNotificationHandler_UpdatePreferences(t *testing.T) {
	h := newTestNotificationHandler(t)

	body, _ := json.Marshal(map[string]any{
		"email_enabled": false,
		"types":         []string{"order", "shipping"},
	})
	req := httptest.NewRequest(http.MethodPut, "/api/v1/notifications/preferences", bytes.NewReader(body))
	req = authNotifRequest(req, 50)
	rec := httptest.NewRecorder()
	h.UpdatePreferences(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", rec.Code, rec.Body.String())
	}

	var resp preferencesResponse
	if err := json.NewDecoder(rec.Body).Decode(&resp); err != nil {
		t.Fatalf("decode: %v", err)
	}
	if resp.EmailEnabled != false {
		t.Error("expected email disabled")
	}
	if len(resp.Types) != 2 || resp.Types[0] != "order" {
		t.Fatalf("unexpected types: %+v", resp.Types)
	}

	getReq := httptest.NewRequest(http.MethodGet, "/api/v1/notifications/preferences", nil)
	getReq = authNotifRequest(getReq, 50)
	getRec := httptest.NewRecorder()
	h.GetPreferences(getRec, getReq)
	if getRec.Code != http.StatusOK {
		t.Fatalf("expected 200 after update, got %d", getRec.Code)
	}
}
