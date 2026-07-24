package review

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
	domain "github.com/beeleelee/mall/domain/review"
)

type integrationFixture struct {
	repo    *PostgresReviewRepository
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

	repo := NewPostgresReviewRepository(db)

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
CREATE TABLE IF NOT EXISTS reviews (
    id         BIGINT PRIMARY KEY,
    product_id BIGINT NOT NULL,
    user_id    BIGINT NOT NULL,
    rating     INT NOT NULL,
    title      TEXT NOT NULL DEFAULT '',
    content    TEXT NOT NULL DEFAULT '',
    status     TEXT NOT NULL DEFAULT 'pending',
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);
`

func TestReviewRepository_SaveAndFindByID(t *testing.T) {
	f := newIntegrationFixture(t)
	defer f.cleanup()
	ctx := context.Background()

	rating, _ := domain.NewRating(4)
	rv, _ := domain.NewReview(1, 100, 200, rating, "Great product", "Really love this item")

	if err := f.repo.Save(ctx, rv); err != nil {
		t.Fatal(err)
	}

	found, err := f.repo.FindByID(ctx, 1)
	if err != nil {
		t.Fatal(err)
	}
	if found.ID != 1 {
		t.Errorf("ID = %d, want 1", found.ID)
	}
	if found.ProductID != 100 {
		t.Errorf("ProductID = %d, want 100", found.ProductID)
	}
	if found.UserID != 200 {
		t.Errorf("UserID = %d, want 200", found.UserID)
	}
	if found.Rating != rating {
		t.Errorf("Rating = %d, want %d", found.Rating, rating)
	}
	if found.Title != "Great product" {
		t.Errorf("Title = %q", found.Title)
	}
	if found.Content != "Really love this item" {
		t.Errorf("Content = %q", found.Content)
	}
	if found.Status != domain.ReviewStatusPending {
		t.Errorf("Status = %s, want pending", found.Status)
	}
}

func TestReviewRepository_NotFound(t *testing.T) {
	f := newIntegrationFixture(t)
	defer f.cleanup()

	_, err := f.repo.FindByID(context.Background(), 999)
	if !kernel.IsNotFound(err) {
		t.Errorf("expected not found, got %v", err)
	}
}

func TestReviewRepository_SaveAndUpdate(t *testing.T) {
	f := newIntegrationFixture(t)
	defer f.cleanup()
	ctx := context.Background()

	rating, _ := domain.NewRating(3)
	rv, _ := domain.NewReview(1, 100, 200, rating, "OK", "Decent product")

	if err := f.repo.Save(ctx, rv); err != nil {
		t.Fatal(err)
	}

	rv.Approve()
	if err := f.repo.Save(ctx, rv); err != nil {
		t.Fatal(err)
	}

	found, err := f.repo.FindByID(ctx, 1)
	if err != nil {
		t.Fatal(err)
	}
	if found.Status != domain.ReviewStatusApproved {
		t.Errorf("Status = %s, want approved", found.Status)
	}
}

func TestReviewRepository_FindByProduct(t *testing.T) {
	f := newIntegrationFixture(t)
	defer f.cleanup()
	ctx := context.Background()

	for i := int64(1); i <= 3; i++ {
		rating, _ := domain.NewRating(int((i % 5) + 1))
		rv, _ := domain.NewReview(kernel.ID(i), 100, kernel.ID(200+i), rating, "Title", "Content")
		f.repo.Save(ctx, rv)
	}

	result, err := f.repo.FindByProduct(ctx, 100, domain.ReviewQueryOptions{Limit: 10})
	if err != nil {
		t.Fatal(err)
	}
	if result.Total != 3 {
		t.Errorf("Total = %d, want 3", result.Total)
	}
	if len(result.Reviews) != 3 {
		t.Errorf("len(Reviews) = %d, want 3", len(result.Reviews))
	}
}

func TestReviewRepository_FindByProduct_Empty(t *testing.T) {
	f := newIntegrationFixture(t)
	defer f.cleanup()
	ctx := context.Background()

	result, err := f.repo.FindByProduct(ctx, 999, domain.ReviewQueryOptions{Limit: 10})
	if err != nil {
		t.Fatal(err)
	}
	if result.Total != 0 {
		t.Errorf("Total = %d, want 0", result.Total)
	}
	if len(result.Reviews) != 0 {
		t.Errorf("len(Reviews) = %d, want 0", len(result.Reviews))
	}
}

func TestReviewRepository_FindByUser(t *testing.T) {
	f := newIntegrationFixture(t)
	defer f.cleanup()
	ctx := context.Background()

	rating, _ := domain.NewRating(5)
	rv1, _ := domain.NewReview(1, 100, 200, rating, "A", "Content 1")
	rv2, _ := domain.NewReview(2, 101, 200, rating, "B", "Content 2")
	f.repo.Save(ctx, rv1)
	f.repo.Save(ctx, rv2)

	result, err := f.repo.FindByUser(ctx, 200, domain.ReviewQueryOptions{Limit: 10})
	if err != nil {
		t.Fatal(err)
	}
	if result.Total != 2 {
		t.Errorf("Total = %d, want 2", result.Total)
	}
}

func TestReviewRepository_FindByProductAndUser(t *testing.T) {
	f := newIntegrationFixture(t)
	defer f.cleanup()
	ctx := context.Background()

	rating, _ := domain.NewRating(4)
	rv, _ := domain.NewReview(1, 100, 200, rating, "Title", "Content")
	f.repo.Save(ctx, rv)

	found, err := f.repo.FindByProductAndUser(ctx, 100, 200)
	if err != nil {
		t.Fatal(err)
	}
	if found.ID != 1 {
		t.Errorf("ID = %d, want 1", found.ID)
	}
}

func TestReviewRepository_FindByProductAndUser_NotFound(t *testing.T) {
	f := newIntegrationFixture(t)
	defer f.cleanup()

	_, err := f.repo.FindByProductAndUser(context.Background(), 100, 999)
	if !kernel.IsNotFound(err) {
		t.Errorf("expected not found, got %v", err)
	}
}

func TestReviewRepository_FindAll(t *testing.T) {
	f := newIntegrationFixture(t)
	defer f.cleanup()
	ctx := context.Background()

	for i := int64(1); i <= 3; i++ {
		rating, _ := domain.NewRating(4)
		rv, _ := domain.NewReview(kernel.ID(i), kernel.ID(100+i), kernel.ID(200+i), rating, "T", "C")
		f.repo.Save(ctx, rv)
	}

	result, err := f.repo.FindAll(ctx, domain.ReviewQueryOptions{Limit: 10})
	if err != nil {
		t.Fatal(err)
	}
	if result.Total != 3 {
		t.Errorf("Total = %d, want 3", result.Total)
	}
}

func TestReviewRepository_FindAll_FilterByStatus(t *testing.T) {
	f := newIntegrationFixture(t)
	defer f.cleanup()
	ctx := context.Background()

	rating, _ := domain.NewRating(4)
	rv1, _ := domain.NewReview(1, 100, 200, rating, "T", "C")
	rv1.Approve()
	f.repo.Save(ctx, rv1)

	rv2, _ := domain.NewReview(2, 101, 201, rating, "T", "C")
	f.repo.Save(ctx, rv2)

	result, err := f.repo.FindAll(ctx, domain.ReviewQueryOptions{Limit: 10, Status: domain.ReviewStatusApproved})
	if err != nil {
		t.Fatal(err)
	}
	if result.Total != 1 {
		t.Errorf("Total = %d, want 1", result.Total)
	}
}

func TestReviewRepository_Delete(t *testing.T) {
	f := newIntegrationFixture(t)
	defer f.cleanup()
	ctx := context.Background()

	rating, _ := domain.NewRating(5)
	rv, _ := domain.NewReview(1, 100, 200, rating, "T", "C")
	f.repo.Save(ctx, rv)

	if err := f.repo.Delete(ctx, 1); err != nil {
		t.Fatal(err)
	}

	_, err := f.repo.FindByID(ctx, 1)
	if !kernel.IsNotFound(err) {
		t.Errorf("expected not found after delete, got %v", err)
	}
}

func TestReviewRepository_GetAverageRating(t *testing.T) {
	f := newIntegrationFixture(t)
	defer f.cleanup()
	ctx := context.Background()

	ratings := []int{4, 5, 3}
	for i, r := range ratings {
		rating, _ := domain.NewRating(r)
		rv, _ := domain.NewReview(kernel.ID(int64(i+1)), 100, kernel.ID(200+int64(i)), rating, "T", "C")
		rv.Approve()
		f.repo.Save(ctx, rv)
	}

	avg, err := f.repo.GetAverageRating(ctx, 100)
	if err != nil {
		t.Fatal(err)
	}
	if avg != 4.0 {
		t.Errorf("Average = %f, want 4.0", avg)
	}
}

func TestReviewRepository_GetAverageRating_NoApproved(t *testing.T) {
	f := newIntegrationFixture(t)
	defer f.cleanup()
	ctx := context.Background()

	rating, _ := domain.NewRating(4)
	rv, _ := domain.NewReview(1, 100, 200, rating, "T", "C")
	f.repo.Save(ctx, rv)

	avg, err := f.repo.GetAverageRating(ctx, 100)
	if err != nil {
		t.Fatal(err)
	}
	if avg != 0 {
		t.Errorf("Average = %f, want 0", avg)
	}
}

func TestReviewRepository_GetAverageRating_NoReviews(t *testing.T) {
	f := newIntegrationFixture(t)
	defer f.cleanup()

	avg, err := f.repo.GetAverageRating(context.Background(), 999)
	if err != nil {
		t.Fatal(err)
	}
	if avg != 0 {
		t.Errorf("Average = %f, want 0", avg)
	}
}
