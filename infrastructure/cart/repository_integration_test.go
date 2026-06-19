package cart

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

	domain "github.com/beeleelee/mall/domain/cart"
	"github.com/beeleelee/mall/domain/kernel"
)

type integrationFixture struct {
	repo    *PostgresCartRepository
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
		DB:   1,
	})

	repo := NewPostgresCartRepository(db, rdb)

	cleanup := func() {
		rdb.FlushDB(context.Background())
		rdb.Close()
		db.Exec(fmt.Sprintf(`DROP SCHEMA "%s" CASCADE`, schema))
		db.Close()
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
CREATE TABLE IF NOT EXISTS carts (
    id         BIGINT PRIMARY KEY,
    user_id    BIGINT NOT NULL UNIQUE,
    items      JSONB NOT NULL DEFAULT '[]',
    status     TEXT NOT NULL DEFAULT 'active',
    created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT now()
);
`

func TestPostgresCartRepository_SaveAndFindByUserID(t *testing.T) {
	f := newIntegrationFixture(t)
	defer f.cleanup()
	ctx := context.Background()

	cart, err := domain.NewCart(1, 42)
	if err != nil {
		t.Fatal(err)
	}
	cart.AddItem(domain.CartItem{ProductID: 100, SKU: "SKU001", Name: "Product 1", Quantity: 2, UnitPrice: 1000})

	if err := f.repo.Save(ctx, cart); err != nil {
		t.Fatal(err)
	}

	found, err := f.repo.FindByUserID(ctx, 42)
	if err != nil {
		t.Fatal(err)
	}
	if found.ID != 1 {
		t.Errorf("expected cart ID 1, got %d", found.ID)
	}
	if len(found.Items) != 1 {
		t.Errorf("expected 1 item, got %d", len(found.Items))
	}
	if found.Items[0].ProductID != 100 {
		t.Errorf("expected product 100, got %d", found.Items[0].ProductID)
	}
}

func TestPostgresCartRepository_FindByID(t *testing.T) {
	f := newIntegrationFixture(t)
	defer f.cleanup()
	ctx := context.Background()

	cart, err := domain.NewCart(42, 100)
	if err != nil {
		t.Fatal(err)
	}
	if err := f.repo.Save(ctx, cart); err != nil {
		t.Fatal(err)
	}

	found, err := f.repo.FindByID(ctx, 42)
	if err != nil {
		t.Fatal(err)
	}
	if found.UserID != 100 {
		t.Errorf("expected user 100, got %d", found.UserID)
	}
}

func TestPostgresCartRepository_UpdateCart(t *testing.T) {
	f := newIntegrationFixture(t)
	defer f.cleanup()
	ctx := context.Background()

	cart, _ := domain.NewCart(1, 42)
	cart.AddItem(domain.CartItem{ProductID: 100, SKU: "A", Name: "A", Quantity: 2, UnitPrice: 1000})
	if err := f.repo.Save(ctx, cart); err != nil {
		t.Fatal(err)
	}

	cart.AddItem(domain.CartItem{ProductID: 101, SKU: "B", Name: "B", Quantity: 1, UnitPrice: 2000})
	if err := f.repo.Save(ctx, cart); err != nil {
		t.Fatal(err)
	}

	found, err := f.repo.FindByUserID(ctx, 42)
	if err != nil {
		t.Fatal(err)
	}
	if len(found.Items) != 2 {
		t.Errorf("expected 2 items, got %d", len(found.Items))
	}
}

func TestPostgresCartRepository_Delete(t *testing.T) {
	f := newIntegrationFixture(t)
	defer f.cleanup()
	ctx := context.Background()

	cart, _ := domain.NewCart(1, 42)
	f.repo.Save(ctx, cart)

	if err := f.repo.Delete(ctx, 1); err != nil {
		t.Fatal(err)
	}

	_, err := f.repo.FindByUserID(ctx, 42)
	if !kernel.IsNotFound(err) {
		t.Errorf("expected not found after delete, got %v", err)
	}
}

func TestPostgresCartRepository_NotFound(t *testing.T) {
	f := newIntegrationFixture(t)
	defer f.cleanup()

	_, err := f.repo.FindByUserID(context.Background(), 999)
	if !kernel.IsNotFound(err) {
		t.Errorf("expected not found, got %v", err)
	}
}
