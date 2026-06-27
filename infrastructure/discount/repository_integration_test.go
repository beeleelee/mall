package discount

import (
	"context"
	"fmt"
	"math/rand"
	"net"
	"testing"
	"time"

	_ "github.com/jackc/pgx/v5/stdlib"
	"github.com/jmoiron/sqlx"

	domain "github.com/beeleelee/mall/domain/discount"
	"github.com/beeleelee/mall/domain/kernel"
)

type integrationFixture struct {
	repo    *PostgresDiscountRepository
	db      *sqlx.DB
	schema  string
	cleanup func()
}

const upSQL = `
CREATE TABLE IF NOT EXISTS discount_codes (
    id BIGINT PRIMARY KEY,
    code TEXT NOT NULL UNIQUE,
    type TEXT NOT NULL,
    value BIGINT NOT NULL,
    min_purchase BIGINT NOT NULL DEFAULT 0,
    max_usages INT NOT NULL DEFAULT 0,
    used_count INT NOT NULL DEFAULT 0,
    expiry TIMESTAMPTZ NOT NULL,
    active BOOLEAN NOT NULL DEFAULT true,
    stackable BOOLEAN NOT NULL DEFAULT false,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);
`

func servicesUp() bool {
	pg, err := net.DialTimeout("tcp", "localhost:5432", 3*time.Second)
	if err != nil {
		return false
	}
	_ = pg.Close()
	return true
}

func newIntegrationFixture(t *testing.T) *integrationFixture {
	t.Helper()

	if !servicesUp() {
		t.Skip("integration: need 'docker compose up postgres' running")
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

	repo := NewPostgresDiscountRepository(db)

	cleanup := func() {
		_, _ = db.Exec(fmt.Sprintf(`DROP SCHEMA "%s" CASCADE`, schema))
		_ = db.Close()
	}

	return &integrationFixture{
		repo:    repo,
		db:      db,
		schema:  schema,
		cleanup: cleanup,
	}
}

func TestIntegration_DiscountSaveAndFind(t *testing.T) {
	f := newIntegrationFixture(t)
	defer f.cleanup()

	ctx := context.Background()
	expiry := time.Now().Add(30 * 24 * time.Hour)

	code, err := domain.NewDiscountCode(1, "SAVE10", domain.DiscountTypeFlat, 1000, 5000, 100, expiry, false)
	if err != nil {
		t.Fatalf("NewDiscountCode: %v", err)
	}

	if err := f.repo.Save(ctx, code); err != nil {
		t.Fatalf("Save: %v", err)
	}

	found, err := f.repo.FindByCode(ctx, "SAVE10")
	if err != nil {
		t.Fatalf("FindByCode: %v", err)
	}
	if found.Code != "SAVE10" {
		t.Errorf("expected code SAVE10, got %s", found.Code)
	}
	if found.Value != 1000 {
		t.Errorf("expected value 1000, got %d", found.Value)
	}
}

func TestIntegration_DiscountNotFound(t *testing.T) {
	f := newIntegrationFixture(t)
	defer f.cleanup()

	_, err := f.repo.FindByCode(context.Background(), "NONEXISTENT")
	if !kernel.IsNotFound(err) {
		t.Errorf("expected NotFound, got %v", err)
	}
}

func TestIntegration_DiscountUpdate(t *testing.T) {
	f := newIntegrationFixture(t)
	defer f.cleanup()

	ctx := context.Background()
	expiry := time.Now().Add(30 * 24 * time.Hour)

	code, _ := domain.NewDiscountCode(1, "UPDATE", domain.DiscountTypeFlat, 500, 0, 10, expiry, false)
	if err := f.repo.Save(ctx, code); err != nil {
		t.Fatalf("Save: %v", err)
	}

	code.Deactivate()
	if err := f.repo.Save(ctx, code); err != nil {
		t.Fatalf("Save after deactivate: %v", err)
	}

	found, err := f.repo.FindByCode(ctx, "UPDATE")
	if err != nil {
		t.Fatalf("FindByCode: %v", err)
	}
	if found.Active {
		t.Error("expected discount to be inactive after deactivate")
	}
}

func TestIntegration_DiscountIncrementUsage(t *testing.T) {
	f := newIntegrationFixture(t)
	defer f.cleanup()

	ctx := context.Background()
	expiry := time.Now().Add(30 * 24 * time.Hour)

	code, _ := domain.NewDiscountCode(1, "USAGE", domain.DiscountTypeFlat, 500, 0, 10, expiry, false)
	if err := f.repo.Save(ctx, code); err != nil {
		t.Fatalf("Save: %v", err)
	}

	if err := f.repo.IncrementUsage(ctx, 1); err != nil {
		t.Fatalf("IncrementUsage: %v", err)
	}

	found, _ := f.repo.FindByCode(ctx, "USAGE")
	if found.UsedCount != 1 {
		t.Errorf("expected used_count 1, got %d", found.UsedCount)
	}
}

func TestIntegration_DiscountOverwrite(t *testing.T) {
	f := newIntegrationFixture(t)
	defer f.cleanup()

	ctx := context.Background()
	expiry := time.Now().Add(30 * 24 * time.Hour)

	code, _ := domain.NewDiscountCode(1, "OVERWRITE", domain.DiscountTypeFlat, 100, 0, 10, expiry, false)
	if err := f.repo.Save(ctx, code); err != nil {
		t.Fatalf("Save initial: %v", err)
	}

	updated, _ := domain.NewDiscountCode(1, "OVERWRITE", domain.DiscountTypeFlat, 200, 0, 10, expiry, false)
	if err := f.repo.Save(ctx, updated); err != nil {
		t.Fatalf("Save updated: %v", err)
	}

	found, _ := f.repo.FindByCode(ctx, "OVERWRITE")
	if found.Value != 200 {
		t.Errorf("expected value 200 after overwrite, got %d", found.Value)
	}
}
