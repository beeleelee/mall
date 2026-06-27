package payment

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
	domain "github.com/beeleelee/mall/domain/payment"
)

type integrationFixture struct {
	repo    *PostgresMandateRepository
	db      *sqlx.DB
	schema  string
	cleanup func()
}

const upSQL = `
CREATE TABLE IF NOT EXISTS mandates (
    id BIGINT PRIMARY KEY,
    user_id BIGINT NOT NULL,
    status TEXT NOT NULL,
    scope JSONB NOT NULL,
    signature TEXT NOT NULL DEFAULT '',
    token TEXT NOT NULL DEFAULT '',
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

	repo := NewPostgresMandateRepository(db)

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

func newTestMandate(id, userID kernel.ID, status domain.MandateStatus) *domain.Mandate {
	return &domain.Mandate{
		AggregateRoot: kernel.NewAggregateRoot(id),
		UserID:        userID,
		Status:        status,
		Scope: domain.MandateScope{
			MaxAmount:  50000,
			MerchantID: 1,
			Expiry:     time.Now().Add(24 * time.Hour),
		},
		Signature: "test_sig",
		Token:     "test_token",
	}
}

func TestIntegration_MandateSaveAndFindByID(t *testing.T) {
	f := newIntegrationFixture(t)
	defer f.cleanup()

	ctx := context.Background()
	m := newTestMandate(1, 42, domain.MandateStatusApproved)

	if err := f.repo.Save(ctx, m); err != nil {
		t.Fatalf("Save: %v", err)
	}

	found, err := f.repo.FindByID(ctx, 1)
	if err != nil {
		t.Fatalf("FindByID: %v", err)
	}
	if found.ID != 1 {
		t.Errorf("expected ID 1, got %d", found.ID)
	}
	if found.Status != domain.MandateStatusApproved {
		t.Errorf("expected status %s, got %s", domain.MandateStatusApproved, found.Status)
	}
	if found.UserID != 42 {
		t.Errorf("expected user 42, got %d", found.UserID)
	}
}

func TestIntegration_MandateNotFound(t *testing.T) {
	f := newIntegrationFixture(t)
	defer f.cleanup()

	_, err := f.repo.FindByID(context.Background(), 999)
	if !kernel.IsNotFound(err) {
		t.Errorf("expected NotFound, got %v", err)
	}
}

func TestIntegration_MandateFindByUserID(t *testing.T) {
	f := newIntegrationFixture(t)
	defer f.cleanup()

	ctx := context.Background()

	m1 := newTestMandate(1, 42, domain.MandateStatusApproved)
	m2 := newTestMandate(2, 42, domain.MandateStatusExecuted)
	m3 := newTestMandate(3, 99, domain.MandateStatusApproved)

	if err := f.repo.Save(ctx, m1); err != nil {
		t.Fatalf("Save m1: %v", err)
	}
	if err := f.repo.Save(ctx, m2); err != nil {
		t.Fatalf("Save m2: %v", err)
	}
	if err := f.repo.Save(ctx, m3); err != nil {
		t.Fatalf("Save m3: %v", err)
	}

	mandates, err := f.repo.FindByUserID(ctx, 42)
	if err != nil {
		t.Fatalf("FindByUserID: %v", err)
	}
	if len(mandates) != 2 {
		t.Fatalf("expected 2 mandates for user 42, got %d", len(mandates))
	}
}

func TestIntegration_MandateFindActiveByUser(t *testing.T) {
	f := newIntegrationFixture(t)
	defer f.cleanup()

	ctx := context.Background()

	m1 := newTestMandate(1, 42, domain.MandateStatusApproved)
	m2 := newTestMandate(2, 42, domain.MandateStatusCancelled)

	if err := f.repo.Save(ctx, m1); err != nil {
		t.Fatalf("Save m1: %v", err)
	}
	if err := f.repo.Save(ctx, m2); err != nil {
		t.Fatalf("Save m2: %v", err)
	}

	active, err := f.repo.FindActiveByUser(ctx, 42)
	if err != nil {
		t.Fatalf("FindActiveByUser: %v", err)
	}
	if len(active) != 1 {
		t.Fatalf("expected 1 active mandate, got %d", len(active))
	}
	if active[0].ID != 1 {
		t.Errorf("expected mandate ID 1, got %d", active[0].ID)
	}
}

func TestIntegration_MandateDelete(t *testing.T) {
	f := newIntegrationFixture(t)
	defer f.cleanup()

	ctx := context.Background()

	m := newTestMandate(1, 42, domain.MandateStatusApproved)
	if err := f.repo.Save(ctx, m); err != nil {
		t.Fatalf("Save: %v", err)
	}

	if err := f.repo.Delete(ctx, 1); err != nil {
		t.Fatalf("Delete: %v", err)
	}

	_, err := f.repo.FindByID(ctx, 1)
	if !kernel.IsNotFound(err) {
		t.Errorf("expected NotFound after delete, got %v", err)
	}
}

func TestIntegration_MandateUpdate(t *testing.T) {
	f := newIntegrationFixture(t)
	defer f.cleanup()

	ctx := context.Background()

	m := newTestMandate(1, 42, domain.MandateStatusApproved)
	if err := f.repo.Save(ctx, m); err != nil {
		t.Fatalf("Save initial: %v", err)
	}

	m.Status = domain.MandateStatusExecuted
	if err := f.repo.Save(ctx, m); err != nil {
		t.Fatalf("Save updated: %v", err)
	}

	found, _ := f.repo.FindByID(ctx, 1)
	if found.Status != domain.MandateStatusExecuted {
		t.Errorf("expected status %s, got %s", domain.MandateStatusExecuted, found.Status)
	}
}
