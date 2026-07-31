package subscription

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
	domain "github.com/beeleelee/mall/domain/subscription"
)

type integrationFixture struct {
	planRepo *PostgresPlanRepository
	subRepo  *PostgresSubscriptionRepository
	db       *sqlx.DB
	schema   string
	cleanup  func()
}

func servicesUp() bool {
	pg, err := net.DialTimeout("tcp", "localhost:5432", 3*time.Second)
	if err != nil {
		return false
	}
	pg.Close()
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

	planRepo := NewPostgresPlanRepository(db, nil)
	subRepo := NewPostgresSubscriptionRepository(db, nil)

	cleanup := func() {
		db.Exec(fmt.Sprintf(`DROP SCHEMA "%s" CASCADE`, schema))
		db.Close()
	}

	return &integrationFixture{
		planRepo: planRepo,
		subRepo:  subRepo,
		db:       db,
		schema:   schema,
		cleanup:  cleanup,
	}
}

const upSQL = `
CREATE TABLE subscription_plans (
    id BIGINT PRIMARY KEY,
    name TEXT NOT NULL,
    description TEXT NOT NULL DEFAULT '',
    amount BIGINT NOT NULL,
    interval TEXT NOT NULL,
    interval_count INT NOT NULL DEFAULT 1,
    trial_days INT NOT NULL DEFAULT 0,
    features JSONB NOT NULL DEFAULT '[]',
    status TEXT NOT NULL DEFAULT 'active',
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE TABLE subscriptions (
    id BIGINT PRIMARY KEY,
    user_id BIGINT NOT NULL,
    plan_id BIGINT NOT NULL REFERENCES subscription_plans(id),
    status TEXT NOT NULL DEFAULT 'pending',
    current_period_start TIMESTAMPTZ NOT NULL,
    current_period_end TIMESTAMPTZ NOT NULL,
    trial_ends_at TIMESTAMPTZ,
    cancelled_at TIMESTAMPTZ,
    payment_token TEXT NOT NULL DEFAULT '',
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);
`

func seedPlan(t *testing.T, repo *PostgresPlanRepository, id int64) *domain.Plan {
	t.Helper()
	p, err := domain.NewPlan(kernel.ID(id), fmt.Sprintf("Plan-%d", id), "", 999, "month", 1, 0, []string{"f1"})
	if err != nil {
		t.Fatal(err)
	}
	if err := repo.Save(context.Background(), p); err != nil {
		t.Fatal(err)
	}
	return p
}

func TestPlanRepository_SaveAndFindByID(t *testing.T) {
	f := newIntegrationFixture(t)
	defer f.cleanup()
	ctx := context.Background()

	p := seedPlan(t, f.planRepo, 1)

	found, err := f.planRepo.FindByID(ctx, 1)
	if err != nil {
		t.Fatal(err)
	}
	if found.Name != "Plan-1" {
		t.Errorf("Name = %s, want Plan-1", found.Name)
	}
	if found.Amount != 999 {
		t.Errorf("Amount = %d, want 999", found.Amount)
	}
	if found.Interval != "month" {
		t.Errorf("Interval = %s, want month", found.Interval)
	}
	if found.ID != p.ID {
		t.Errorf("ID = %d, want %d", found.ID, p.ID)
	}
}

func TestPlanRepository_FindByID_NotFound(t *testing.T) {
	f := newIntegrationFixture(t)
	defer f.cleanup()

	_, err := f.planRepo.FindByID(context.Background(), 999)
	if !kernel.IsNotFound(err) {
		t.Errorf("expected not found, got %v", err)
	}
}

func TestPlanRepository_FindAll(t *testing.T) {
	f := newIntegrationFixture(t)
	defer f.cleanup()
	ctx := context.Background()

	seedPlan(t, f.planRepo, 1)
	seedPlan(t, f.planRepo, 2)

	plans, err := f.planRepo.FindAll(ctx)
	if err != nil {
		t.Fatal(err)
	}
	if len(plans) != 2 {
		t.Errorf("len = %d, want 2", len(plans))
	}
}

func TestPlanRepository_FindActive(t *testing.T) {
	f := newIntegrationFixture(t)
	defer f.cleanup()
	ctx := context.Background()

	seedPlan(t, f.planRepo, 1)
	p2 := seedPlan(t, f.planRepo, 2)
	p2.Deactivate()
	f.planRepo.Save(ctx, p2)

	plans, err := f.planRepo.FindActive(ctx)
	if err != nil {
		t.Fatal(err)
	}
	if len(plans) != 1 {
		t.Errorf("len = %d, want 1 active plan", len(plans))
	}
}

func TestPlanRepository_Save_Update(t *testing.T) {
	f := newIntegrationFixture(t)
	defer f.cleanup()
	ctx := context.Background()

	p := seedPlan(t, f.planRepo, 1)
	p.Name = "Updated"
	if err := f.planRepo.Save(ctx, p); err != nil {
		t.Fatal(err)
	}

	found, _ := f.planRepo.FindByID(ctx, 1)
	if found.Name != "Updated" {
		t.Errorf("Name = %s, want Updated", found.Name)
	}
}

func TestSubscriptionRepository_SaveAndFindByID(t *testing.T) {
	f := newIntegrationFixture(t)
	defer f.cleanup()
	ctx := context.Background()

	p := seedPlan(t, f.planRepo, 1)
	sub, err := domain.NewSubscription(1, 100, p.ID, p)
	if err != nil {
		t.Fatal(err)
	}
	if err := f.subRepo.Save(ctx, sub); err != nil {
		t.Fatal(err)
	}

	found, err := f.subRepo.FindByID(ctx, 1)
	if err != nil {
		t.Fatal(err)
	}
	if found.UserID != 100 {
		t.Errorf("UserID = %d, want 100", found.UserID)
	}
	if found.PlanID != 1 {
		t.Errorf("PlanID = %d, want 1", found.PlanID)
	}
}

func TestSubscriptionRepository_FindByID_NotFound(t *testing.T) {
	f := newIntegrationFixture(t)
	defer f.cleanup()

	_, err := f.subRepo.FindByID(context.Background(), 999)
	if !kernel.IsNotFound(err) {
		t.Errorf("expected not found, got %v", err)
	}
}

func TestSubscriptionRepository_FindByUserID(t *testing.T) {
	f := newIntegrationFixture(t)
	defer f.cleanup()
	ctx := context.Background()

	p := seedPlan(t, f.planRepo, 1)
	s1, _ := domain.NewSubscription(1, 100, p.ID, p)
	s2, _ := domain.NewSubscription(2, 100, p.ID, p)
	f.subRepo.Save(ctx, s1)
	f.subRepo.Save(ctx, s2)

	subs, err := f.subRepo.FindByUserID(ctx, 100)
	if err != nil {
		t.Fatal(err)
	}
	if len(subs) != 2 {
		t.Errorf("len = %d, want 2", len(subs))
	}
}

func TestSubscriptionRepository_FindActiveByUserID(t *testing.T) {
	f := newIntegrationFixture(t)
	defer f.cleanup()
	ctx := context.Background()

	p := seedPlan(t, f.planRepo, 1)
	s, _ := domain.NewSubscription(1, 100, p.ID, p)
	s.Activate()
	f.subRepo.Save(ctx, s)

	found, err := f.subRepo.FindActiveByUserID(ctx, 100)
	if err != nil {
		t.Fatal(err)
	}
	if found.ID != 1 {
		t.Errorf("ID = %d, want 1", found.ID)
	}
}

func TestSubscriptionRepository_FindActiveByUserID_NotFound(t *testing.T) {
	f := newIntegrationFixture(t)
	defer f.cleanup()
	ctx := context.Background()

	p := seedPlan(t, f.planRepo, 1)
	s, _ := domain.NewSubscription(1, 100, p.ID, p)
	f.subRepo.Save(ctx, s)

	_, err := f.subRepo.FindActiveByUserID(ctx, 100)
	if !kernel.IsNotFound(err) {
		t.Errorf("expected not found for pending subscription, got %v", err)
	}
}

func TestSubscriptionRepository_FindDueForBilling(t *testing.T) {
	f := newIntegrationFixture(t)
	defer f.cleanup()
	ctx := context.Background()

	p := seedPlan(t, f.planRepo, 1)
	s, _ := domain.NewSubscription(1, 100, p.ID, p)
	s.Activate()
	s.CurrentPeriodEnd = time.Now().AddDate(0, 0, -1)
	f.subRepo.Save(ctx, s)

	due, err := f.subRepo.FindDueForBilling(ctx, time.Now())
	if err != nil {
		t.Fatal(err)
	}
	if len(due) != 1 {
		t.Errorf("len = %d, want 1 due subscription", len(due))
	}
}

func TestSubscriptionRepository_Save_Update(t *testing.T) {
	f := newIntegrationFixture(t)
	defer f.cleanup()
	ctx := context.Background()

	p := seedPlan(t, f.planRepo, 1)
	s, _ := domain.NewSubscription(1, 100, p.ID, p)
	f.subRepo.Save(ctx, s)

	s.Activate()
	if err := f.subRepo.Save(ctx, s); err != nil {
		t.Fatal(err)
	}

	found, _ := f.subRepo.FindByID(ctx, 1)
	if found.Status != domain.SubscriptionStatusActive {
		t.Errorf("Status = %s, want active", found.Status)
	}
}
