package inventory

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

	domain "github.com/beeleelee/mall/domain/inventory"
	"github.com/beeleelee/mall/domain/kernel"
)

type integrationFixture struct {
	repo    *PostgresInventoryRepository
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
	_ = pg.Close()

	rd, err := net.DialTimeout("tcp", "localhost:6379", 3*time.Second)
	if err != nil {
		return false
	}
	_ = rd.Close()

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
		_ = db.Close()
		t.Fatalf("create schema: %v", err)
	}
	if _, err := db.Exec(fmt.Sprintf(`SET search_path TO "%s", public`, schema)); err != nil {
		_ = db.Close()
		t.Fatalf("set search_path: %v", err)
	}

	if _, err := db.Exec(upSQL); err != nil {
		_ = db.Close()
		t.Fatalf("apply migration: %v", err)
	}

	rdb := redis.NewClient(&redis.Options{
		Addr: "localhost:6379",
		DB:   2,
	})

	repo := NewPostgresInventoryRepository(db, rdb)

	cleanup := func() {
		_ = rdb.FlushDB(context.Background())
		_ = rdb.Close()
		_, _ = db.Exec(fmt.Sprintf(`DROP SCHEMA "%s" CASCADE`, schema))
		_ = db.Close()
	}

	return &integrationFixture{
		repo:    repo,
		db:      db,
		rdb:     rdb,
		schema:  schema,
		cleanup: cleanup,
	}
}

const upSQL = `
CREATE TABLE IF NOT EXISTS products (
    id              BIGINT PRIMARY KEY,
    sku             TEXT NOT NULL,
    name            TEXT NOT NULL,
    description     TEXT NOT NULL DEFAULT '',
    category        TEXT NOT NULL DEFAULT '',
    price_amount    BIGINT NOT NULL DEFAULT 0,
    price_currency  TEXT NOT NULL DEFAULT 'USD',
    status          TEXT NOT NULL DEFAULT 'active',
    attributes      JSONB NOT NULL DEFAULT '{}',
    created_at      TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at      TIMESTAMPTZ NOT NULL DEFAULT now()
);

CREATE TABLE IF NOT EXISTS inventory (
    id                 BIGINT PRIMARY KEY,
    product_id         BIGINT NOT NULL UNIQUE REFERENCES products(id),
    quantity_available INT NOT NULL DEFAULT 0,
    reserved_quantity  INT NOT NULL DEFAULT 0,
    low_stock_threshold INT NOT NULL DEFAULT 10,
    created_at         TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at         TIMESTAMPTZ NOT NULL DEFAULT now()
);
`

func TestSaveAndFindByProductID(t *testing.T) {
	f := newIntegrationFixture(t)
	defer f.cleanup()

	ctx := context.Background()

	_, err := f.db.ExecContext(ctx, `INSERT INTO products (id, sku, name, price_amount, price_currency) VALUES (100, 'TEST-001', 'Test Product', 1000, 'USD')`)
	if err != nil {
		t.Fatalf("seed product: %v", err)
	}

	item, err := domain.NewInventoryItem(1, 100, 50, 10)
	if err != nil {
		t.Fatalf("NewInventoryItem: %v", err)
	}

	if err := f.repo.Save(ctx, item); err != nil {
		t.Fatalf("Save: %v", err)
	}

	found, err := f.repo.FindByProductID(ctx, 100)
	if err != nil {
		t.Fatalf("FindByProductID: %v", err)
	}

	if found.QuantityAvailable != 50 {
		t.Errorf("expected 50, got %d", found.QuantityAvailable)
	}
}

func TestUpdateStock(t *testing.T) {
	f := newIntegrationFixture(t)
	defer f.cleanup()

	ctx := context.Background()

	_, _ = f.db.ExecContext(ctx, `INSERT INTO products (id, sku, name, price_amount, price_currency) VALUES (200, 'TEST-002', 'Test Product 2', 2000, 'USD')`)

	item, _ := domain.NewInventoryItem(2, 200, 100, 10)
	_ = f.repo.Save(ctx, item)

	_ = item.SetStock(75)
	_ = f.repo.Save(ctx, item)

	found, _ := f.repo.FindByProductID(ctx, 200)
	if found.QuantityAvailable != 75 {
		t.Errorf("expected 75, got %d", found.QuantityAvailable)
	}
}

func TestReserveAndConfirm(t *testing.T) {
	f := newIntegrationFixture(t)
	defer f.cleanup()

	ctx := context.Background()

	_, _ = f.db.ExecContext(ctx, `INSERT INTO products (id, sku, name, price_amount, price_currency) VALUES (300, 'TEST-003', 'Test Product 3', 3000, 'USD')`)

	item, _ := domain.NewInventoryItem(3, 300, 50, 10)
	_ = f.repo.Save(ctx, item)

	_ = item.Reserve(10)
	_ = f.repo.Save(ctx, item)

	found, _ := f.repo.FindByProductID(ctx, 300)
	if found.ReservedQuantity != 10 {
		t.Errorf("expected reserved 10, got %d", found.ReservedQuantity)
	}

	_ = item.ConfirmReservation(10)
	_ = f.repo.Save(ctx, item)

	found, _ = f.repo.FindByProductID(ctx, 300)
	if found.QuantityAvailable != 40 {
		t.Errorf("expected available 40, got %d", found.QuantityAvailable)
	}
}

func TestFindLowStock(t *testing.T) {
	f := newIntegrationFixture(t)
	defer f.cleanup()

	ctx := context.Background()

	_, _ = f.db.ExecContext(ctx, `INSERT INTO products (id, sku, name, price_amount, price_currency) VALUES (400, 'TEST-004', 'Test Product 4', 1000, 'USD')`)
	_, _ = f.db.ExecContext(ctx, `INSERT INTO products (id, sku, name, price_amount, price_currency) VALUES (401, 'TEST-005', 'Test Product 5', 2000, 'USD')`)
	_, _ = f.db.ExecContext(ctx, `INSERT INTO products (id, sku, name, price_amount, price_currency) VALUES (402, 'TEST-006', 'Test Product 6', 3000, 'USD')`)

	item4, _ := domain.NewInventoryItem(4, 400, 5, 10)
	item5, _ := domain.NewInventoryItem(5, 401, 20, 10)
	item6, _ := domain.NewInventoryItem(6, 402, 50, 10)

	_ = f.repo.Save(ctx, item4)
	_ = f.repo.Save(ctx, item5)
	_ = f.repo.Save(ctx, item6)

	items, err := f.repo.FindLowStock(ctx, 10)
	if err != nil {
		t.Fatalf("FindLowStock: %v", err)
	}

	if len(items) != 1 {
		t.Fatalf("expected 1 low stock item, got %d", len(items))
	}
	if items[0].ProductID != 400 {
		t.Errorf("expected product 400, got %d", items[0].ProductID)
	}
}

func TestFindAll(t *testing.T) {
	f := newIntegrationFixture(t)
	defer f.cleanup()

	ctx := context.Background()

	for i := 0; i < 5; i++ {
		pid := int64(500 + i)
		_, _ = f.db.ExecContext(ctx, `INSERT INTO products (id, sku, name, price_amount, price_currency) VALUES ($1, $2, $3, 1000, 'USD')`,
			pid, fmt.Sprintf("TEST-%03d", i), fmt.Sprintf("Product %d", i))
		item, _ := domain.NewInventoryItem(kernel.ID(10+i), kernel.ID(pid), 100, 10)
		_ = f.repo.Save(ctx, item)
	}

	items, err := f.repo.FindAll(ctx, 0, 3)
	if err != nil {
		t.Fatalf("FindAll: %v", err)
	}
	if len(items) != 3 {
		t.Errorf("expected 3 items, got %d", len(items))
	}
}
