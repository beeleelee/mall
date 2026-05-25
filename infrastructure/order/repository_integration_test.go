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
	"github.com/redis/go-redis/v9"

	checkout "github.com/beeleelee/mall/domain/checkout"
	domain "github.com/beeleelee/mall/domain/order"
	"github.com/beeleelee/mall/domain/kernel"
)

type integrationFixture struct {
	repo    *PostgresOrderRepository
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
		DB:   3,
	})

	repo := NewPostgresOrderRepository(db, rdb)

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
CREATE TABLE IF NOT EXISTS orders (
    id               BIGINT PRIMARY KEY,
    user_id          BIGINT NOT NULL,
    checkout_id      BIGINT NOT NULL,
    cart_id          BIGINT NOT NULL,
    items            JSONB NOT NULL DEFAULT '[]',
    shipping_address JSONB NOT NULL DEFAULT '{}',
    billing_address  JSONB NOT NULL DEFAULT '{}',
    shipping_option  JSONB NOT NULL DEFAULT '{}',
    payment_handler  TEXT NOT NULL DEFAULT '',
    subtotal         BIGINT NOT NULL DEFAULT 0,
    shipping_cost    BIGINT NOT NULL DEFAULT 0,
    tax_amount       BIGINT NOT NULL DEFAULT 0,
    grand_total      BIGINT NOT NULL DEFAULT 0,
    status           TEXT NOT NULL DEFAULT 'confirmed',
    tracking_number  TEXT NOT NULL DEFAULT '',
    carrier          TEXT NOT NULL DEFAULT '',
    confirmed_at     TIMESTAMPTZ NOT NULL DEFAULT now(),
    processing_at    TIMESTAMPTZ,
    shipped_at       TIMESTAMPTZ,
    delivered_at     TIMESTAMPTZ,
    returned_at      TIMESTAMPTZ,
    cancelled_at     TIMESTAMPTZ,
    created_at       TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at       TIMESTAMPTZ NOT NULL DEFAULT now()
);

CREATE UNIQUE INDEX IF NOT EXISTS idx_orders_checkout_id ON orders (checkout_id);
`

func completedCheckout() *checkout.CheckoutSession {
	snapshot := checkout.NewCartSnapshot([]checkout.CartSnapshotItem{
		{ProductID: 100, SKU: "SKU001", Name: "Product 1", Quantity: 2, UnitPrice: 1000},
	})
	s, _ := checkout.NewCheckoutSession(1, 42, 10, snapshot)
	addr := checkout.Address{Line1: "123 Main St", City: "Portland", State: "OR", PostalCode: "97201", Country: "US"}
	s.SetShippingAddress(addr)
	s.SetBillingAddress(addr)
	s.SelectShippingOption(checkout.ShippingOption{ID: "std", Name: "Standard", Cost: 500})
	s.SelectPaymentHandler("stripe")
	s.MarkReady()
	s.Complete()
	return s
}

func TestPostgresOrderRepository_SaveAndFindByID(t *testing.T) {
	f := newIntegrationFixture(t)
	defer f.cleanup()
	ctx := context.Background()

	order, err := domain.NewOrderFromCheckout(1, completedCheckout())
	if err != nil {
		t.Fatal(err)
	}

	if err := f.repo.Save(ctx, order); err != nil {
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
	if found.Status != domain.OrderStatusConfirmed {
		t.Errorf("expected confirmed, got %s", found.Status)
	}
}

func TestPostgresOrderRepository_FindByUserID(t *testing.T) {
	f := newIntegrationFixture(t)
	defer f.cleanup()
	ctx := context.Background()

	order1, _ := domain.NewOrderFromCheckout(1, completedCheckout())
	f.repo.Save(ctx, order1)

	s2 := completedCheckout()
	s2.ID = 2
	s2.CartID = 20
	order2, _ := domain.NewOrderFromCheckout(2, s2)
	f.repo.Save(ctx, order2)

	orders, err := f.repo.FindByUserID(ctx, 42)
	if err != nil {
		t.Fatal(err)
	}
	if len(orders) != 2 {
		t.Errorf("expected 2 orders, got %d", len(orders))
	}
}

func TestPostgresOrderRepository_UpdateOrder(t *testing.T) {
	f := newIntegrationFixture(t)
	defer f.cleanup()
	ctx := context.Background()

	order, _ := domain.NewOrderFromCheckout(1, completedCheckout())
	f.repo.Save(ctx, order)

	order, _ = f.repo.FindByID(ctx, 1)
	order.StartProcessing()
	f.repo.Save(ctx, order)

	found, _ := f.repo.FindByID(ctx, 1)
	if found.Status != domain.OrderStatusProcessing {
		t.Errorf("expected processing, got %s", found.Status)
	}
}

func TestPostgresOrderRepository_NotFound(t *testing.T) {
	f := newIntegrationFixture(t)
	defer f.cleanup()

	_, err := f.repo.FindByID(context.Background(), 999)
	if !kernel.IsNotFound(err) {
		t.Errorf("expected not found, got %v", err)
	}
}

func TestPostgresOrderRepository_Delete(t *testing.T) {
	f := newIntegrationFixture(t)
	defer f.cleanup()
	ctx := context.Background()

	order, _ := domain.NewOrderFromCheckout(1, completedCheckout())
	f.repo.Save(ctx, order)
	f.repo.Delete(ctx, 1)

	_, err := f.repo.FindByID(ctx, 1)
	if !kernel.IsNotFound(err) {
		t.Errorf("expected not found after delete, got %v", err)
	}
}
