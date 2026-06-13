package order

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
	domain "github.com/beeleelee/mall/domain/order"
)

type webhookIntegrationFixture struct {
	repo    *PostgresWebhookRepository
	db      *sqlx.DB
	schema  string
	cleanup func()
}

func webhookServicesUp() bool {
	pg, err := net.DialTimeout("tcp", "localhost:5432", 3*time.Second)
	if err != nil {
		return false
	}
	pg.Close()
	return true
}

func newWebhookIntegrationFixture(t *testing.T) *webhookIntegrationFixture {
	t.Helper()

	if !webhookServicesUp() {
		t.Skip("integration: need 'docker compose up postgres' running")
	}

	dsn := "postgres://mall:mall_dev@localhost:5432/mall?sslmode=disable"

	db, err := sqlx.Connect("pgx", dsn)
	if err != nil {
		t.Fatalf("connect postgres: %v", err)
	}
	db.SetMaxOpenConns(2)

	schema := fmt.Sprintf("test_%08x", rand.Int63())[:16]
	if _, err := db.Exec(fmt.Sprintf(`CREATE SCHEMA IF NOT EXISTS "%s"`, schema)); err != nil {
		db.Close()
		t.Fatalf("create schema: %v", err)
	}
	if _, err := db.Exec(fmt.Sprintf(`SET search_path TO "%s", public`, schema)); err != nil {
		db.Close()
		t.Fatalf("set search_path: %v", err)
	}

	if _, err := db.Exec(webhookUpSQL); err != nil {
		db.Close()
		t.Fatalf("apply migration: %v", err)
	}

	repo := NewPostgresWebhookRepository(db)

	cleanup := func() {
		db.Exec(fmt.Sprintf(`DROP SCHEMA "%s" CASCADE`, schema))
		db.Close()
	}

	return &webhookIntegrationFixture{
		repo:    repo,
		db:      db,
		schema:  schema,
		cleanup: cleanup,
	}
}

const webhookUpSQL = `
CREATE TABLE IF NOT EXISTS webhooks (
    id BIGINT PRIMARY KEY,
    user_id BIGINT NOT NULL,
    url TEXT NOT NULL,
    secret TEXT NOT NULL DEFAULT '',
    events JSONB NOT NULL DEFAULT '[]',
    active BOOLEAN NOT NULL DEFAULT true,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);
CREATE INDEX IF NOT EXISTS idx_webhooks_user_id ON webhooks(user_id);
`

func TestPostgresWebhookRepository_SaveAndFindByID(t *testing.T) {
	f := newWebhookIntegrationFixture(t)
	defer f.cleanup()
	ctx := context.Background()

	w, err := domain.NewWebhook(1, 42, "https://example.com/hook", "mysecret", []string{"order.confirmed", "order.shipped"})
	if err != nil {
		t.Fatal(err)
	}

	if err := f.repo.Save(ctx, w); err != nil {
		t.Fatal(err)
	}

	found, err := f.repo.FindByID(ctx, 1)
	if err != nil {
		t.Fatal(err)
	}
	if found.ID != 1 {
		t.Errorf("expected ID 1, got %d", found.ID)
	}
	if found.UserID != 42 {
		t.Errorf("expected user 42, got %d", found.UserID)
	}
	if found.URL != "https://example.com/hook" {
		t.Errorf("expected URL, got %s", found.URL)
	}
	if len(found.Events) != 2 {
		t.Errorf("expected 2 events, got %d", len(found.Events))
	}
}

func TestPostgresWebhookRepository_FindByUserID(t *testing.T) {
	f := newWebhookIntegrationFixture(t)
	defer f.cleanup()
	ctx := context.Background()

	w1, _ := domain.NewWebhook(1, 42, "https://example.com/hook1", "s1", []string{"order.created"})
	w2, _ := domain.NewWebhook(2, 42, "https://example.com/hook2", "s2", []string{"order.shipped"})
	w3, _ := domain.NewWebhook(3, 99, "https://example.com/hook3", "s3", []string{"order.created"})
	f.repo.Save(ctx, w1)
	f.repo.Save(ctx, w2)
	f.repo.Save(ctx, w3)

	webhooks, err := f.repo.FindByUserID(ctx, 42)
	if err != nil {
		t.Fatal(err)
	}
	if len(webhooks) != 2 {
		t.Errorf("expected 2 webhooks for user 42, got %d", len(webhooks))
	}
}

func TestPostgresWebhookRepository_FindByEvent(t *testing.T) {
	f := newWebhookIntegrationFixture(t)
	defer f.cleanup()
	ctx := context.Background()

	w1, _ := domain.NewWebhook(1, 42, "https://example.com/hook1", "s1", []string{"order.confirmed"})
	w2, _ := domain.NewWebhook(2, 42, "https://example.com/hook2", "s2", []string{"order.shipped", "order.delivered"})
	f.repo.Save(ctx, w1)
	f.repo.Save(ctx, w2)

	// Find by matching event
	result, err := f.repo.FindByEvent(ctx, "order.shipped")
	if err != nil {
		t.Fatal(err)
	}
	if len(result) != 1 {
		t.Errorf("expected 1 webhook for order.shipped, got %d", len(result))
	}

	// Find by non-matching event
	result, err = f.repo.FindByEvent(ctx, "order.cancelled")
	if err != nil {
		t.Fatal(err)
	}
	if len(result) != 0 {
		t.Errorf("expected 0 webhooks for order.cancelled, got %d", len(result))
	}
}

func TestPostgresWebhookRepository_NotFound(t *testing.T) {
	f := newWebhookIntegrationFixture(t)
	defer f.cleanup()

	_, err := f.repo.FindByID(context.Background(), 999)
	if !kernel.IsNotFound(err) {
		t.Errorf("expected not found, got %v", err)
	}
}

func TestPostgresWebhookRepository_Delete(t *testing.T) {
	f := newWebhookIntegrationFixture(t)
	defer f.cleanup()
	ctx := context.Background()

	w, _ := domain.NewWebhook(1, 42, "https://example.com/hook", "s", []string{"order.created"})
	f.repo.Save(ctx, w)

	if err := f.repo.Delete(ctx, 1); err != nil {
		t.Fatal(err)
	}

	_, err := f.repo.FindByID(ctx, 1)
	if !kernel.IsNotFound(err) {
		t.Errorf("expected not found after delete, got %v", err)
	}
}
