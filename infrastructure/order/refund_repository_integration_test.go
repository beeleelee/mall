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

type refundIntegrationFixture struct {
	repo    *PostgresRefundRepository
	db      *sqlx.DB
	schema  string
	cleanup func()
}

func newRefundIntegrationFixture(t *testing.T) *refundIntegrationFixture {
	t.Helper()

	pgDial := func() bool {
		conn, err := net.DialTimeout("tcp", "localhost:5432", 3*time.Second)
		if err != nil {
			return false
		}
		conn.Close()
		return true
	}
	if !pgDial() {
		t.Skip("integration: need postgres running on localhost:5432")
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

	// Create orders table first for FK reference
	if _, err := db.Exec(`CREATE TABLE IF NOT EXISTS orders (id BIGINT PRIMARY KEY)`); err != nil {
		db.Close()
		t.Fatalf("create orders table: %v", err)
	}

	if _, err := db.Exec(refundUpSQL); err != nil {
		db.Close()
		t.Fatalf("apply migration: %v", err)
	}

	repo := NewPostgresRefundRepository(db)
	cleanup := func() {
		db.Exec(fmt.Sprintf(`DROP SCHEMA "%s" CASCADE`, schema))
		db.Close()
	}

	return &refundIntegrationFixture{
		repo:    repo,
		db:      db,
		schema:  schema,
		cleanup: cleanup,
	}
}

const refundUpSQL = `
CREATE TABLE IF NOT EXISTS refunds (
    id BIGINT PRIMARY KEY,
    order_id BIGINT NOT NULL REFERENCES orders(id),
    mandate_id BIGINT,
    amount BIGINT NOT NULL,
    reason TEXT NOT NULL DEFAULT '',
    status VARCHAR(50) NOT NULL DEFAULT 'pending',
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    processed_at TIMESTAMPTZ,
    failed_at TIMESTAMPTZ,
    failure_reason TEXT
);

CREATE INDEX IF NOT EXISTS idx_refunds_order_id ON refunds(order_id);
CREATE INDEX IF NOT EXISTS idx_refunds_status ON refunds(status);
`

func TestPostgresRefundRepository_SaveAndFindByID(t *testing.T) {
	f := newRefundIntegrationFixture(t)
	defer f.cleanup()
	ctx := context.Background()

	f.db.ExecContext(ctx, `INSERT INTO orders (id) VALUES (100)`)

	refund, err := domain.NewRefund(1, 100, 0, 2500, "buyer returned item")
	if err != nil {
		t.Fatalf("NewRefund: %v", err)
	}

	if err := f.repo.Save(ctx, refund); err != nil {
		t.Fatalf("Save: %v", err)
	}

	found, err := f.repo.FindByID(ctx, 1)
	if err != nil {
		t.Fatalf("FindByID: %v", err)
	}

	if found.ID != 1 {
		t.Errorf("expected ID 1, got %d", found.ID)
	}
	if found.OrderID != 100 {
		t.Errorf("expected OrderID 100, got %d", found.OrderID)
	}
	if found.Amount != 2500 {
		t.Errorf("expected Amount 2500, got %d", found.Amount)
	}
	if found.Status != domain.RefundStatusPending {
		t.Errorf("expected Status pending, got %s", found.Status)
	}
}

func TestPostgresRefundRepository_SaveAndFindByOrderID(t *testing.T) {
	f := newRefundIntegrationFixture(t)
	defer f.cleanup()
	ctx := context.Background()

	f.db.ExecContext(ctx, `INSERT INTO orders (id) VALUES (200)`)

	r1, _ := domain.NewRefund(1, 200, 0, 2500, "return")
	r2, _ := domain.NewRefund(2, 200, 0, 1000, "partial")

	if err := f.repo.Save(ctx, r1); err != nil {
		t.Fatalf("Save r1: %v", err)
	}
	if err := f.repo.Save(ctx, r2); err != nil {
		t.Fatalf("Save r2: %v", err)
	}

	refunds, err := f.repo.FindByOrderID(ctx, 200)
	if err != nil {
		t.Fatalf("FindByOrderID: %v", err)
	}

	if len(refunds) != 2 {
		t.Fatalf("expected 2 refunds, got %d", len(refunds))
	}
}

func TestPostgresRefundRepository_FindByID_NotFound(t *testing.T) {
	f := newRefundIntegrationFixture(t)
	defer f.cleanup()
	ctx := context.Background()

	_, err := f.repo.FindByID(ctx, 999)
	if err == nil {
		t.Fatal("expected error for non-existent refund")
	}
	if !kernel.IsNotFound(err) {
		t.Errorf("expected IsNotFound, got %v", err)
	}
}

func TestPostgresRefundRepository_UpdateStatus(t *testing.T) {
	f := newRefundIntegrationFixture(t)
	defer f.cleanup()
	ctx := context.Background()

	f.db.ExecContext(ctx, `INSERT INTO orders (id) VALUES (300)`)

	refund, _ := domain.NewRefund(1, 300, 0, 2500, "return")
	if err := f.repo.Save(ctx, refund); err != nil {
		t.Fatalf("Save: %v", err)
	}

	refund.MarkProcessed()
	if err := f.repo.Save(ctx, refund); err != nil {
		t.Fatalf("Save after MarkProcessed: %v", err)
	}

	found, err := f.repo.FindByID(ctx, 1)
	if err != nil {
		t.Fatalf("FindByID: %v", err)
	}
	if found.Status != domain.RefundStatusProcessed {
		t.Errorf("expected processed, got %s", found.Status)
	}
	if found.ProcessedAt == nil {
		t.Error("expected ProcessedAt to be set")
	}
}
