package notification

import (
	"context"
	"fmt"
	"math/rand"
	"net"
	"testing"
	"time"

	_ "github.com/jackc/pgx/v5/stdlib"
	"github.com/jmoiron/sqlx"

	"github.com/beeleelee/mall/domain/kernel"
	domain "github.com/beeleelee/mall/domain/notification"
)

const upSQL = `
CREATE TABLE IF NOT EXISTS notifications (
    id BIGINT PRIMARY KEY,
    user_id BIGINT NOT NULL,
    type TEXT NOT NULL,
    title TEXT NOT NULL,
    body TEXT NOT NULL DEFAULT '',
    read BOOLEAN NOT NULL DEFAULT FALSE,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);
CREATE INDEX IF NOT EXISTS idx_notifications_user_id ON notifications(user_id);
CREATE INDEX IF NOT EXISTS idx_notifications_user_read ON notifications(user_id, read);

CREATE TABLE IF NOT EXISTS notification_preferences (
    id BIGINT PRIMARY KEY,
    user_id BIGINT NOT NULL UNIQUE,
    email_enabled BOOLEAN NOT NULL DEFAULT TRUE,
    in_app_enabled BOOLEAN NOT NULL DEFAULT TRUE,
    types JSONB,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);
`

type fixture struct {
	notifRepo *PostgresNotificationRepository
	prefRepo  *PostgresNotificationPreferenceRepository
	db        *sqlx.DB
	cleanup   func()
}

func newFixture(t *testing.T) *fixture {
	t.Helper()

	if !servicesUp() {
		t.Skip("integration: need 'docker compose up postgres redis' running")
	}

	dsn := "postgres://mall:mall_dev@localhost:5432/mall?sslmode=disable"

	db, err := sqlx.Connect("pgx", dsn)
	if err != nil {
		t.Fatalf("connect postgres: %v", err)
	}
	db.SetMaxOpenConns(4)

	schema := fmt.Sprintf("test_%08x", rand.Int63())[:16]
	if _, err := db.Exec(fmt.Sprintf(`CREATE SCHEMA IF NOT EXISTS "%s"`, schema)); err != nil {
		db.Close()
		t.Fatalf("create schema: %v", err)
	}
	if _, err := db.Exec(fmt.Sprintf(`SET search_path TO "%s", public`, schema)); err != nil {
		db.Close()
		t.Fatalf("set search_path: %v", err)
	}

	if _, err := db.Exec(upSQL); err != nil {
		db.Close()
		t.Fatalf("apply migration: %v", err)
	}

	return &fixture{
		notifRepo: NewPostgresNotificationRepository(db),
		prefRepo:  NewPostgresNotificationPreferenceRepository(db),
		db:        db,
		cleanup: func() {
			db.Exec(fmt.Sprintf(`DROP SCHEMA IF EXISTS "%s" CASCADE`, schema))
			db.Close()
		},
	}
}

func TestNotificationRepository_SaveAndFind(t *testing.T) {
	f := newFixture(t)
	defer f.cleanup()

	ctx := context.Background()
	n, _ := domain.NewNotification(1, 100, domain.NotificationTypeOrder, "Order Confirmed", "Your order has been confirmed.")

	if err := f.notifRepo.Save(ctx, n); err != nil {
		t.Fatalf("save: %v", err)
	}

	found, err := f.notifRepo.FindByID(ctx, 1)
	if err != nil {
		t.Fatalf("find by id: %v", err)
	}
	if found.Title != "Order Confirmed" || found.Body != "Your order has been confirmed." {
		t.Fatalf("unexpected notification: %+v", found)
	}
	if !found.IsUnread() {
		t.Error("expected unread notification")
	}
}

func TestNotificationRepository_FindByUserID(t *testing.T) {
	f := newFixture(t)
	defer f.cleanup()

	ctx := context.Background()
	for i := int64(1); i <= 3; i++ {
		n, _ := domain.NewNotification(kernel.ID(i), 200, domain.NotificationTypeShipping, "Shipping", "Update")
		if err := f.notifRepo.Save(ctx, n); err != nil {
			t.Fatalf("save %d: %v", i, err)
		}
	}

	list, err := f.notifRepo.FindByUserID(ctx, 200, 10)
	if err != nil {
		t.Fatalf("find by user: %v", err)
	}
	if len(list) != 3 {
		t.Fatalf("expected 3 notifications, got %d", len(list))
	}

	list, err = f.notifRepo.FindByUserID(ctx, 200, 1)
	if err != nil {
		t.Fatalf("find by user limit: %v", err)
	}
	if len(list) != 1 {
		t.Fatalf("expected 1 notification with limit, got %d", len(list))
	}
}

func TestNotificationRepository_MarkRead(t *testing.T) {
	f := newFixture(t)
	defer f.cleanup()

	ctx := context.Background()
	n, _ := domain.NewNotification(1, 300, domain.NotificationTypeOrder, "Title", "Body")
	if err := f.notifRepo.Save(ctx, n); err != nil {
		t.Fatalf("save: %v", err)
	}

	if err := f.notifRepo.MarkRead(ctx, 1, 300); err != nil {
		t.Fatalf("mark read: %v", err)
	}

	found, _ := f.notifRepo.FindByID(ctx, 1)
	if found.Read != true {
		t.Error("expected notification marked read")
	}

	if err := f.notifRepo.MarkRead(ctx, 999, 300); err == nil {
		t.Error("expected error for missing notification")
	}
}

func TestNotificationRepository_MarkAllRead(t *testing.T) {
	f := newFixture(t)
	defer f.cleanup()

	ctx := context.Background()
	for i := int64(1); i <= 3; i++ {
		n, _ := domain.NewNotification(kernel.ID(i), 400, domain.NotificationTypeOrder, "Title", "Body")
		if err := f.notifRepo.Save(ctx, n); err != nil {
			t.Fatalf("save %d: %v", i, err)
		}
	}

	count, err := f.notifRepo.UnreadCount(ctx, 400)
	if err != nil {
		t.Fatalf("unread count: %v", err)
	}
	if count != 3 {
		t.Fatalf("expected 3 unread, got %d", count)
	}

	if err := f.notifRepo.MarkAllRead(ctx, 400); err != nil {
		t.Fatalf("mark all read: %v", err)
	}

	count, err = f.notifRepo.UnreadCount(ctx, 400)
	if err != nil {
		t.Fatalf("unread count: %v", err)
	}
	if count != 0 {
		t.Fatalf("expected 0 unread, got %d", count)
	}
}

func TestNotificationPreferenceRepository_UpsertAndGet(t *testing.T) {
	f := newFixture(t)
	defer f.cleanup()

	ctx := context.Background()
	prefs, _ := domain.NewNotificationPreferences(1, 500)
	if err := prefs.SetChannel(domain.ChannelEmail, false); err != nil {
		t.Fatalf("set channel: %v", err)
	}
	prefs.SetType(domain.NotificationTypeOrder, true)

	if err := f.prefRepo.Upsert(ctx, prefs); err != nil {
		t.Fatalf("upsert: %v", err)
	}

	found, err := f.prefRepo.Get(ctx, 500)
	if err != nil {
		t.Fatalf("get: %v", err)
	}
	if found.EmailEnabled != false {
		t.Error("expected email disabled")
	}
	if found.Types == nil || len(*found.Types) != 1 || (*found.Types)[0] != domain.NotificationTypeOrder {
		t.Errorf("unexpected types: %+v", found.Types)
	}

	if _, err := f.prefRepo.Get(ctx, 999); err == nil {
		t.Error("expected not found for missing preferences")
	}
}

func servicesUp() bool {
	pg, err := net.DialTimeout("tcp", "localhost:5432", 3*time.Second)
	if err != nil {
		return false
	}
	pg.Close()
	return true
}
