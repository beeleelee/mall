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

type deliveryLogIntegrationFixture struct {
	repo     *PostgresWebhookDeliveryLogRepository
	whRepo   *PostgresWebhookRepository
	db       *sqlx.DB
	schema   string
	cleanup  func()
	sf       *kernel.Snowflake
	webhook  *domain.Webhook
}

func deliveryLogServicesUp() bool {
	pg, err := net.DialTimeout("tcp", "localhost:5432", 3*time.Second)
	if err != nil {
		return false
	}
	pg.Close()
	return true
}

func newDeliveryLogIntegrationFixture(t *testing.T) *deliveryLogIntegrationFixture {
	t.Helper()

	if !deliveryLogServicesUp() {
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
		t.Fatalf("apply webhooks migration: %v", err)
	}
	if _, err := db.Exec(deliveryLogUpSQL); err != nil {
		db.Close()
		t.Fatalf("apply delivery_log migration: %v", err)
	}

	sf, err := kernel.NewSnowflake(1)
	if err != nil {
		db.Close()
		t.Fatalf("NewSnowflake: %v", err)
	}

	wh, err := domain.NewWebhook(1, 42, "https://example.com/hook", "mysecret", []string{"order.shipped"})
	if err != nil {
		db.Close()
		t.Fatalf("new webhook: %v", err)
	}
	whRepo := NewPostgresWebhookRepository(db)
	if err := whRepo.Save(context.Background(), wh); err != nil {
		db.Close()
		t.Fatalf("save webhook: %v", err)
	}

	repo := NewPostgresWebhookDeliveryLogRepository(db, sf)

	cleanup := func() {
		db.Exec(fmt.Sprintf(`DROP SCHEMA "%s" CASCADE`, schema))
		db.Close()
	}

	return &deliveryLogIntegrationFixture{
		repo:    repo,
		whRepo:  whRepo,
		db:      db,
		schema:  schema,
		cleanup: cleanup,
		sf:      sf,
		webhook: wh,
	}
}

const deliveryLogUpSQL = `
CREATE TABLE IF NOT EXISTS webhook_delivery_log (
    id         BIGINT PRIMARY KEY,
    webhook_id BIGINT NOT NULL REFERENCES webhooks(id) ON DELETE CASCADE,
    event      VARCHAR(255) NOT NULL,
    payload    JSONB NOT NULL DEFAULT '{}',
    status     VARCHAR(50) NOT NULL DEFAULT 'failed',
    error      TEXT,
    attempts   INT NOT NULL DEFAULT 0,
    next_retry TIMESTAMPTZ,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);
CREATE INDEX IF NOT EXISTS idx_webhook_delivery_log_status ON webhook_delivery_log (status);
CREATE INDEX IF NOT EXISTS idx_webhook_delivery_log_next_retry ON webhook_delivery_log (next_retry);
CREATE INDEX IF NOT EXISTS idx_webhook_delivery_log_webhook ON webhook_delivery_log (webhook_id);
`

func TestPostgresWebhookDeliveryLogRepository_SaveAndListFailed(t *testing.T) {
	f := newDeliveryLogIntegrationFixture(t)
	defer f.cleanup()
	ctx := context.Background()

	entry := &domain.DeliveryLogEntry{
		WebhookID: 1,
		Event:     "order.shipped",
		Payload:   []byte(`{"order_id": 123}`),
		Status:    "failed",
		Error:     "connection refused",
		Attempts:  3,
	}
	if err := f.repo.Save(ctx, entry); err != nil {
		t.Fatal(err)
	}
	if entry.ID == 0 {
		t.Fatal("expected ID to be set after save")
	}

	failed, err := f.repo.ListFailed(ctx, 10)
	if err != nil {
		t.Fatal(err)
	}
	if len(failed) != 1 {
		t.Fatalf("expected 1 failed entry, got %d", len(failed))
	}
	if failed[0].ID != entry.ID {
		t.Fatalf("expected ID %d, got %d", entry.ID, failed[0].ID)
	}
	if failed[0].Status != "failed" {
		t.Fatalf("expected status 'failed', got %q", failed[0].Status)
	}
}

func TestPostgresWebhookDeliveryLogRepository_MarkRetried(t *testing.T) {
	f := newDeliveryLogIntegrationFixture(t)
	defer f.cleanup()
	ctx := context.Background()

	entry := &domain.DeliveryLogEntry{
		WebhookID: 1,
		Event:     "order.shipped",
		Payload:   []byte(`{}`),
		Status:    "failed",
		Attempts:  1,
	}
	if err := f.repo.Save(ctx, entry); err != nil {
		t.Fatal(err)
	}

	if err := f.repo.MarkRetried(ctx, entry.ID); err != nil {
		t.Fatal(err)
	}

	failed, _ := f.repo.ListFailed(ctx, 10)
	if len(failed) != 0 {
		t.Fatalf("expected 0 failed entries after retry, got %d", len(failed))
	}
}

func TestPostgresWebhookDeliveryLogRepository_MarkDelivered(t *testing.T) {
	f := newDeliveryLogIntegrationFixture(t)
	defer f.cleanup()
	ctx := context.Background()

	entry := &domain.DeliveryLogEntry{
		WebhookID: 1,
		Event:     "order.shipped",
		Payload:   []byte(`{}`),
		Status:    "failed",
		Attempts:  1,
	}
	if err := f.repo.Save(ctx, entry); err != nil {
		t.Fatal(err)
	}

	if err := f.repo.MarkDelivered(ctx, entry.ID); err != nil {
		t.Fatal(err)
	}

	failed, _ := f.repo.ListFailed(ctx, 10)
	if len(failed) != 0 {
		t.Fatalf("expected 0 failed entries after delivered, got %d", len(failed))
	}
}

func TestPostgresWebhookDeliveryLogRepository_ListFailedDueForRetry(t *testing.T) {
	f := newDeliveryLogIntegrationFixture(t)
	defer f.cleanup()
	ctx := context.Background()

	now := time.Now()

	pastDue := &domain.DeliveryLogEntry{
		WebhookID: 1,
		Event:     "order.shipped",
		Payload:   []byte(`{}`),
		Status:    "failed",
		Attempts:  2,
		NextRetry: &now,
	}
	if err := f.repo.Save(ctx, pastDue); err != nil {
		t.Fatal(err)
	}

	futureTime := now.Add(24 * time.Hour)
	futureDue := &domain.DeliveryLogEntry{
		WebhookID: 1,
		Event:     "order.shipped",
		Payload:   []byte(`{}`),
		Status:    "failed",
		Attempts:  2,
		NextRetry: &futureTime,
	}
	if err := f.repo.Save(ctx, futureDue); err != nil {
		t.Fatal(err)
	}

	noRetry := &domain.DeliveryLogEntry{
		WebhookID: 1,
		Event:     "order.shipped",
		Payload:   []byte(`{}`),
		Status:    "failed",
		Attempts:  2,
	}
	if err := f.repo.Save(ctx, noRetry); err != nil {
		t.Fatal(err)
	}

	due, err := f.repo.ListFailedDueForRetry(ctx, 10)
	if err != nil {
		t.Fatal(err)
	}
	if len(due) != 2 {
		t.Fatalf("expected 2 due for retry, got %d", len(due))
	}
}
