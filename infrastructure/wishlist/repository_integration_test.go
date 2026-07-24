package wishlist

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

	"github.com/beeleelee/mall/domain/kernel"
	domain "github.com/beeleelee/mall/domain/wishlist"
)

type integrationFixture struct {
	repo    *PostgresWishlistRepository
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

	repo := NewPostgresWishlistRepository(db)

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
CREATE TABLE IF NOT EXISTS wishlists (
    id         BIGINT PRIMARY KEY,
    user_id    BIGINT NOT NULL UNIQUE,
    items      JSONB NOT NULL DEFAULT '[]',
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);
`

func TestWishlistRepository_SaveAndFindByUserID(t *testing.T) {
	f := newIntegrationFixture(t)
	defer f.cleanup()
	ctx := context.Background()

	w, _ := domain.NewWishlist(1, 100)
	if err := f.repo.Save(ctx, w); err != nil {
		t.Fatal(err)
	}

	found, err := f.repo.FindByUserID(ctx, 100)
	if err != nil {
		t.Fatal(err)
	}
	if found.ID != 1 {
		t.Errorf("ID = %d, want 1", found.ID)
	}
	if found.UserID != 100 {
		t.Errorf("UserID = %d, want 100", found.UserID)
	}
	if found.ItemCount() != 0 {
		t.Errorf("ItemCount = %d, want 0", found.ItemCount())
	}
}

func TestWishlistRepository_FindByUserID_NotFound(t *testing.T) {
	f := newIntegrationFixture(t)
	defer f.cleanup()

	_, err := f.repo.FindByUserID(context.Background(), 999)
	if !kernel.IsNotFound(err) {
		t.Errorf("expected not found, got %v", err)
	}
}

func TestWishlistRepository_SaveAndUpdate(t *testing.T) {
	f := newIntegrationFixture(t)
	defer f.cleanup()
	ctx := context.Background()

	w, _ := domain.NewWishlist(1, 100)
	if err := f.repo.Save(ctx, w); err != nil {
		t.Fatal(err)
	}

	w.AddItem(200)
	if err := f.repo.Save(ctx, w); err != nil {
		t.Fatal(err)
	}

	found, err := f.repo.FindByUserID(ctx, 100)
	if err != nil {
		t.Fatal(err)
	}
	if found.ItemCount() != 1 {
		t.Errorf("ItemCount = %d, want 1", found.ItemCount())
	}
	if !found.Contains(200) {
		t.Error("expected wishlist to contain product 200")
	}
}

func TestWishlistRepository_AddAndRemoveItems(t *testing.T) {
	f := newIntegrationFixture(t)
	defer f.cleanup()
	ctx := context.Background()

	w, _ := domain.NewWishlist(1, 100)
	w.AddItem(200)
	w.AddItem(201)
	if err := f.repo.Save(ctx, w); err != nil {
		t.Fatal(err)
	}

	found, _ := f.repo.FindByUserID(ctx, 100)
	if found.ItemCount() != 2 {
		t.Errorf("ItemCount = %d, want 2", found.ItemCount())
	}

	found.RemoveItem(200)
	if err := f.repo.Save(ctx, found); err != nil {
		t.Fatal(err)
	}

	updated, _ := f.repo.FindByUserID(ctx, 100)
	if updated.ItemCount() != 1 {
		t.Errorf("ItemCount = %d, want 1 after removal", updated.ItemCount())
	}
	if updated.Contains(200) {
		t.Error("expected wishlist to not contain product 200")
	}
	if !updated.Contains(201) {
		t.Error("expected wishlist to contain product 201")
	}
}

func TestWishlistRepository_Clear(t *testing.T) {
	f := newIntegrationFixture(t)
	defer f.cleanup()
	ctx := context.Background()

	w, _ := domain.NewWishlist(1, 100)
	w.AddItem(200)
	w.AddItem(201)
	f.repo.Save(ctx, w)

	w.Clear()
	f.repo.Save(ctx, w)

	found, _ := f.repo.FindByUserID(ctx, 100)
	if found.ItemCount() != 0 {
		t.Errorf("ItemCount = %d, want 0 after clear", found.ItemCount())
	}
}

func TestWishlistRepository_Delete(t *testing.T) {
	f := newIntegrationFixture(t)
	defer f.cleanup()
	ctx := context.Background()

	w, _ := domain.NewWishlist(1, 100)
	f.repo.Save(ctx, w)

	if err := f.repo.Delete(ctx, 1); err != nil {
		t.Fatal(err)
	}

	_, err := f.repo.FindByUserID(ctx, 100)
	if !kernel.IsNotFound(err) {
		t.Errorf("expected not found after delete, got %v", err)
	}
}
