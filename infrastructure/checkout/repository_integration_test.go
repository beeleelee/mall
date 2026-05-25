package checkout

import (
	"context"
	"fmt"
	"math/rand"
	"net"
	"testing"
	"time"

	_ "github.com/jackc/pgx/v5/stdlib"
	"github.com/jmoiron/sqlx"
	"github.com/redis/go-redis/v9"

	domain "github.com/beeleelee/mall/domain/checkout"
	"github.com/beeleelee/mall/domain/kernel"
)

type integrationFixture struct {
	repo    *PostgresCheckoutRepository
	db      *sqlx.DB
	rdb     *redis.Client
	schema  string
	cleanup func()
}

func servicesUp() bool {
	pg, err := net.DialTimeout("tcp", "localhost:5432", 3*time.Second)
	if err != nil {
		return false
	}
	pg.Close()

	rd, err := net.DialTimeout("tcp", "localhost:6379", 3*time.Second)
	if err != nil {
		return false
	}
	rd.Close()

	return true
}

func newIntegrationFixture(t *testing.T) *integrationFixture {
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

	rdb := redis.NewClient(&redis.Options{
		Addr: "localhost:6379",
		DB:   2,
	})

	repo := NewPostgresCheckoutRepository(db, rdb)

	cleanup := func() {
		rdb.FlushDB(context.Background())
		rdb.Close()
		db.Exec(fmt.Sprintf(`DROP SCHEMA "%s" CASCADE`, schema))
		db.Close()
	}

	return &integrationFixture{
		repo:   repo,
		db:     db,
		rdb:    rdb,
		schema: schema,
		cleanup: cleanup,
	}
}

const upSQL = `
CREATE TABLE IF NOT EXISTS checkout_sessions (
    id               BIGINT PRIMARY KEY,
    user_id          BIGINT NOT NULL,
    cart_id          BIGINT NOT NULL,
    cart_snapshot    JSONB NOT NULL DEFAULT '{}',
    shipping_address JSONB,
    billing_address  JSONB,
    shipping_option  JSONB,
    payment_handler  TEXT NOT NULL DEFAULT '',
    subtotal         BIGINT NOT NULL DEFAULT 0,
    shipping_cost    BIGINT NOT NULL DEFAULT 0,
    tax_amount       BIGINT NOT NULL DEFAULT 0,
    grand_total      BIGINT NOT NULL DEFAULT 0,
    status           TEXT NOT NULL DEFAULT 'incomplete',
    completed_at     TIMESTAMPTZ,
    created_at       TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at       TIMESTAMPTZ NOT NULL DEFAULT now()
);
`

func sampleSnapshot() domain.CartSnapshot {
	return domain.NewCartSnapshot([]domain.CartSnapshotItem{
		{ProductID: 100, SKU: "SKU001", Name: "Product 1", Quantity: 2, UnitPrice: 1000},
	})
}

func TestPostgresCheckoutRepository_SaveAndFindByID(t *testing.T) {
	f := newIntegrationFixture(t)
	defer f.cleanup()
	ctx := context.Background()

	session, err := domain.NewCheckoutSession(1, 42, 10, sampleSnapshot())
	if err != nil {
		t.Fatal(err)
	}

	if err := f.repo.Save(ctx, session); err != nil {
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
	if found.Status != domain.CheckoutStatusIncomplete {
		t.Errorf("expected incomplete, got %s", found.Status)
	}
}

func TestPostgresCheckoutRepository_FindByUserID(t *testing.T) {
	f := newIntegrationFixture(t)
	defer f.cleanup()
	ctx := context.Background()

	session, _ := domain.NewCheckoutSession(1, 42, 10, sampleSnapshot())
	f.repo.Save(ctx, session)

	found, err := f.repo.FindByUserID(ctx, 42)
	if err != nil {
		t.Fatal(err)
	}
	if found.ID != 1 {
		t.Errorf("expected ID 1, got %d", found.ID)
	}
}

func TestPostgresCheckoutRepository_UpdateSession(t *testing.T) {
	f := newIntegrationFixture(t)
	defer f.cleanup()
	ctx := context.Background()

	session, _ := domain.NewCheckoutSession(1, 42, 10, sampleSnapshot())
	f.repo.Save(ctx, session)

	addr := domain.Address{Line1: "123 Main St", City: "Portland", State: "OR", PostalCode: "97201", Country: "US"}
	session.SetShippingAddress(addr)
	f.repo.Save(ctx, session)

	found, err := f.repo.FindByID(ctx, 1)
	if err != nil {
		t.Fatal(err)
	}
	if found.ShippingAddress == nil {
		t.Fatal("expected non-nil shipping address")
	}
	if found.ShippingAddress.City != "Portland" {
		t.Errorf("expected Portland, got %s", found.ShippingAddress.City)
	}
}

func TestPostgresCheckoutRepository_NotFound(t *testing.T) {
	f := newIntegrationFixture(t)
	defer f.cleanup()

	_, err := f.repo.FindByID(context.Background(), 999)
	if !kernel.IsNotFound(err) {
		t.Errorf("expected not found, got %v", err)
	}
}

func TestPostgresCheckoutRepository_Delete(t *testing.T) {
	f := newIntegrationFixture(t)
	defer f.cleanup()
	ctx := context.Background()

	session, _ := domain.NewCheckoutSession(1, 42, 10, sampleSnapshot())
	f.repo.Save(ctx, session)

	if err := f.repo.Delete(ctx, 1); err != nil {
		t.Fatal(err)
	}

	_, err := f.repo.FindByID(ctx, 1)
	if !kernel.IsNotFound(err) {
		t.Errorf("expected not found after delete, got %v", err)
	}
}
