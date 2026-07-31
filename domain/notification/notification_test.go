package notification

import (
	"context"
	"testing"

	"github.com/beeleelee/mall/domain/kernel"
)

func TestNewNotification_Success(t *testing.T) {
	n, err := NewNotification(1, 100, NotificationTypeOrder, "Order Confirmed", "Your order is confirmed")
	if err != nil {
		t.Fatal(err)
	}
	if n.ID != 1 || n.UserID != 100 {
		t.Errorf("unexpected ids: %d %d", n.ID, n.UserID)
	}
	if n.Read {
		t.Error("expected unread notification")
	}
	if n.IsUnread() != true {
		t.Error("expected IsUnread true")
	}
}

func TestNewNotification_Invalid(t *testing.T) {
	cases := []struct {
		name   string
		id     kernel.ID
		userID kernel.ID
		ntype  NotificationType
		title  string
		body   string
	}{
		{"zero id", 0, 100, NotificationTypeOrder, "t", "b"},
		{"zero user", 1, 0, NotificationTypeOrder, "t", "b"},
		{"empty type", 1, 100, "", "t", "b"},
		{"empty title", 1, 100, NotificationTypeOrder, "", "b"},
	}
	for _, c := range cases {
		_, err := NewNotification(c.id, c.userID, c.ntype, c.title, c.body)
		if !kernel.IsInvalidArgument(err) {
			t.Errorf("%s: expected invalid argument, got %v", c.name, err)
		}
	}
}

func TestNotification_MarkRead(t *testing.T) {
	n, _ := NewNotification(1, 100, NotificationTypeOrder, "t", "b")
	if err := n.MarkRead(); err != nil {
		t.Fatal(err)
	}
	if !n.Read {
		t.Error("expected read after MarkRead")
	}
	if n.IsUnread() {
		t.Error("expected not unread after MarkRead")
	}
}

func TestNotification_MarkRead_Twice(t *testing.T) {
	n, _ := NewNotification(1, 100, NotificationTypeOrder, "t", "b")
	n.MarkRead()
	if err := n.MarkRead(); !kernel.IsConflict(err) {
		t.Fatalf("expected conflict, got %v", err)
	}
}

func TestNewNotificationPreferences_Success(t *testing.T) {
	p, err := NewNotificationPreferences(1, 100)
	if err != nil {
		t.Fatal(err)
	}
	if !p.EmailEnabled || !p.InAppEnabled {
		t.Error("expected both channels enabled by default")
	}
	if !p.Allows(ChannelEmail, NotificationTypeOrder) {
		t.Error("expected email allowed by default")
	}
}

func TestNotificationPreferences_SetChannel(t *testing.T) {
	p, _ := NewNotificationPreferences(1, 100)
	if err := p.SetChannel(ChannelEmail, false); err != nil {
		t.Fatal(err)
	}
	if p.Allows(ChannelEmail, NotificationTypeOrder) {
		t.Error("expected email disabled")
	}
	if !p.Allows(ChannelInApp, NotificationTypeOrder) {
		t.Error("expected in-app still enabled")
	}
	if err := p.SetChannel("sms", true); !kernel.IsInvalidArgument(err) {
		t.Fatalf("expected invalid argument, got %v", err)
	}
}

func TestNotificationPreferences_SetType(t *testing.T) {
	p, _ := NewNotificationPreferences(1, 100)
	p.SetType(NotificationTypeSubscription, true)
	if !p.Allows(ChannelEmail, NotificationTypeSubscription) {
		t.Error("expected subscription type allowed")
	}
	if p.Allows(ChannelEmail, NotificationTypeRefund) {
		t.Error("expected refund type not allowed")
	}
	p.SetType(NotificationTypeSubscription, false)
	if p.Allows(ChannelEmail, NotificationTypeSubscription) {
		t.Error("expected subscription type disabled after removal")
	}
}

func TestNotificationService_NotifyInApp_NoWriter(t *testing.T) {
	svc := newTestService(&mockEmailSender{})
	err := svc.NotifyInApp(context.Background(), 1, 100, NotificationTypeOrder, "t", "b")
	if err != nil {
		t.Fatalf("expected no-op without writer, got %v", err)
	}
}

func TestNotificationService_NotifyInApp_WithWriter(t *testing.T) {
	w := &mockInAppWriter{}
	svc := newTestServiceWithWriter(&mockEmailSender{}, w)
	if err := svc.NotifyInApp(context.Background(), 1, 100, NotificationTypeOrder, "Hello", "Body"); err != nil {
		t.Fatal(err)
	}
	if len(w.written) != 1 {
		t.Fatalf("expected 1 in-app notification, got %d", len(w.written))
	}
	if w.written[0].Title != "Hello" {
		t.Errorf("expected title Hello, got %q", w.written[0].Title)
	}
}

func TestNotificationService_NotifyInApp_PreferenceDisabled(t *testing.T) {
	w := &mockInAppWriter{}
	prefRepo := &mockPrefRepo{prefs: map[kernel.ID]*NotificationPreferences{100: mustPrefs(t, 1, 100, false, false)}}
	svc := NewNotificationService(&mockEmailSender{}, discardLogger{}, WithInAppWriter(w), WithPreferenceRepository(prefRepo))
	if err := svc.NotifyInApp(context.Background(), 1, 100, NotificationTypeOrder, "t", "b"); err != nil {
		t.Fatal(err)
	}
	if len(w.written) != 0 {
		t.Fatalf("expected 0 in-app notifications when disabled, got %d", len(w.written))
	}
}

func TestNotificationService_SubscriptionEmailPref(t *testing.T) {
	sender := &mockEmailSender{}
	prefRepo := &mockPrefRepo{prefs: map[kernel.ID]*NotificationPreferences{100: mustPrefs(t, 1, 100, true, true)}}
	svc := NewNotificationService(sender, discardLogger{}, WithPreferenceRepository(prefRepo))
	if err := svc.SendSubscriptionRenewed(context.Background(), 100, "a@example.com", "A", 5, "Pro", 1999); err != nil {
		t.Fatal(err)
	}
	if len(sender.sent) != 1 {
		t.Fatalf("expected 1 email, got %d", len(sender.sent))
	}
}

func TestNotificationService_SubscriptionEmailPref_DisabledType(t *testing.T) {
	sender := &mockEmailSender{}
	prefs, _ := NewNotificationPreferences(1, 100)
	prefs.SetType(NotificationTypeOrder, true)
	prefRepo := &mockPrefRepo{prefs: map[kernel.ID]*NotificationPreferences{100: prefs}}
	svc := NewNotificationService(sender, discardLogger{}, WithPreferenceRepository(prefRepo))
	if err := svc.SendSubscriptionRenewed(context.Background(), 100, "a@example.com", "A", 5, "Pro", 1999); err != nil {
		t.Fatal(err)
	}
	if len(sender.sent) != 0 {
		t.Fatalf("expected 0 emails when type disabled, got %d", len(sender.sent))
	}
}

type mockInAppWriter struct {
	written []*Notification
}

func (w *mockInAppWriter) Write(_ context.Context, n *Notification) error {
	w.written = append(w.written, n)
	return nil
}

type mockPrefRepo struct {
	prefs map[kernel.ID]*NotificationPreferences
}

func (r *mockPrefRepo) Get(_ context.Context, userID kernel.ID) (*NotificationPreferences, error) {
	p, ok := r.prefs[userID]
	if !ok {
		return nil, kernel.NewDomainError(kernel.ErrNotFound, "preferences not found")
	}
	return p, nil
}

func (r *mockPrefRepo) Upsert(_ context.Context, p *NotificationPreferences) error {
	r.prefs[p.UserID] = p
	return nil
}

func mustPrefs(t *testing.T, id, userID kernel.ID, email, inApp bool) *NotificationPreferences {
	t.Helper()
	p, err := NewNotificationPreferences(id, userID)
	if err != nil {
		t.Fatal(err)
	}
	if !email {
		p.SetChannel(ChannelEmail, false)
	}
	if !inApp {
		p.SetChannel(ChannelInApp, false)
	}
	return p
}

func newTestServiceWithWriter(sender EmailSender, w InAppWriter) *NotificationService {
	return NewNotificationService(sender, discardLogger{}, WithInAppWriter(w))
}
