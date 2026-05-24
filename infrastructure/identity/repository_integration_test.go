package identity

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

	domain "github.com/beeleelee/mall/domain/identity"
	"github.com/beeleelee/mall/domain/kernel"
	"github.com/beeleelee/mall/infrastructure/database"
)

type integrationFixture struct {
	repo    *PostgresUserRepository
	db      *sqlx.DB
	rdb     *redis.Client
	schema  string
	cleanup func()
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

	migrator := database.NewMigrator(db)
	if err := migrator.Up(); err != nil {
		db.Exec(fmt.Sprintf(`DROP SCHEMA IF EXISTS "%s" CASCADE`, schema))
		db.Close()
		t.Fatalf("migrate: %v", err)
	}

	rdb := redis.NewClient(&redis.Options{
		Addr: "localhost:6379",
		DB:   2,
	})
	if err := rdb.Ping(context.Background()).Err(); err != nil {
		db.Exec(fmt.Sprintf(`DROP SCHEMA IF EXISTS "%s" CASCADE`, schema))
		db.Close()
		rdb.Close()
		t.Fatalf("connect redis: %v", err)
	}
	rdb.FlushDB(context.Background())

	repo := NewPostgresUserRepository(db, rdb)

	return &integrationFixture{
		repo:   repo,
		db:     db,
		rdb:    rdb,
		schema: schema,
		cleanup: func() {
			db.Exec(fmt.Sprintf(`DROP SCHEMA IF EXISTS "%s" CASCADE`, schema))
			rdb.FlushDB(context.Background())
			db.Close()
			rdb.Close()
		},
	}
}

func servicesUp() bool {
	pg, err := net.DialTimeout("tcp", "localhost:5432", 2*time.Second)
	if err != nil {
		return false
	}
	pg.Close()

	rd, err := net.DialTimeout("tcp", "localhost:6379", 2*time.Second)
	if err != nil {
		return false
	}
	rd.Close()

	return true
}

func testPassword() *domain.Password {
	return domain.NewPasswordFromHash("$2a$10$testhashforintegrationtestingonly123456")
}

var ctx = context.Background()

func TestIntegrationUser_SaveAndFindByID(t *testing.T) {
	f := newIntegrationFixture(t)
	defer f.cleanup()

	u, _ := domain.NewUser(1, "alice@example.com", "Alice", testPassword(), nil)
	if err := f.repo.Save(ctx, u); err != nil {
		t.Fatalf("save: %v", err)
	}

	got, err := f.repo.FindByID(ctx, 1)
	if err != nil {
		t.Fatalf("find by id: %v", err)
	}
	if got.Email != "alice@example.com" {
		t.Errorf("email: expected %q, got %q", "alice@example.com", got.Email)
	}
	if got.Name != "Alice" {
		t.Errorf("name: expected %q, got %q", "Alice", got.Name)
	}
	if got.Status != domain.UserStatusActive {
		t.Errorf("status: expected %q, got %q", domain.UserStatusActive, got.Status)
	}
}

func TestIntegrationUser_SaveAndFindByEmail(t *testing.T) {
	f := newIntegrationFixture(t)
	defer f.cleanup()

	u, _ := domain.NewUser(1, "bob@example.com", "Bob", testPassword(), nil)
	f.repo.Save(ctx, u)

	got, err := f.repo.FindByEmail(ctx, "bob@example.com")
	if err != nil {
		t.Fatalf("find by email: %v", err)
	}
	if got.Name != "Bob" {
		t.Errorf("name: expected %q, got %q", "Bob", got.Name)
	}
}

func TestIntegrationUser_FindByID_NotFound(t *testing.T) {
	f := newIntegrationFixture(t)
	defer f.cleanup()

	_, err := f.repo.FindByID(ctx, 99999)
	if !kernel.IsNotFound(err) {
		t.Errorf("expected NotFound, got %v", err)
	}
}

func TestIntegrationUser_FindByEmail_NotFound(t *testing.T) {
	f := newIntegrationFixture(t)
	defer f.cleanup()

	_, err := f.repo.FindByEmail(ctx, "nobody@example.com")
	if !kernel.IsNotFound(err) {
		t.Errorf("expected NotFound, got %v", err)
	}
}

func TestIntegrationUser_Delete(t *testing.T) {
	f := newIntegrationFixture(t)
	defer f.cleanup()

	u, _ := domain.NewUser(1, "delete@example.com", "To Delete", testPassword(), nil)
	f.repo.Save(ctx, u)

	if err := f.repo.Delete(ctx, 1); err != nil {
		t.Fatalf("delete: %v", err)
	}

	_, err := f.repo.FindByID(ctx, 1)
	if !kernel.IsNotFound(err) {
		t.Errorf("expected NotFound after delete, got %v", err)
	}
}

func TestIntegrationUser_Delete_NotFound(t *testing.T) {
	f := newIntegrationFixture(t)
	defer f.cleanup()

	err := f.repo.Delete(ctx, 99999)
	if !kernel.IsNotFound(err) {
		t.Errorf("expected NotFound, got %v", err)
	}
}

func TestIntegrationUser_UpdateThenFind(t *testing.T) {
	f := newIntegrationFixture(t)
	defer f.cleanup()

	u, _ := domain.NewUser(1, "update@example.com", "Original", testPassword(), nil)
	f.repo.Save(ctx, u)

	u.ChangeName("Updated Name")
	newPW, _ := domain.NewPassword("newsecurepassword123")
	u.ChangePassword(newPW)
	f.repo.Save(ctx, u)

	got, err := f.repo.FindByID(ctx, 1)
	if err != nil {
		t.Fatalf("find: %v", err)
	}
	if got.Name != "Updated Name" {
		t.Errorf("name: expected %q, got %q", "Updated Name", got.Name)
	}
	if !got.VerifyPassword("newsecurepassword123") {
		t.Error("password should match updated value")
	}
}

func TestIntegrationUser_SuspendAndFind(t *testing.T) {
	f := newIntegrationFixture(t)
	defer f.cleanup()

	u, _ := domain.NewUser(1, "status@example.com", "Status Test", testPassword(), nil)
	f.repo.Save(ctx, u)

	u.Suspend()
	f.repo.Save(ctx, u)

	got, err := f.repo.FindByID(ctx, 1)
	if err != nil {
		t.Fatalf("find: %v", err)
	}
	if got.Status != domain.UserStatusSuspended {
		t.Errorf("status: expected %q, got %q", domain.UserStatusSuspended, got.Status)
	}
}

func TestIntegrationUser_WithRoles(t *testing.T) {
	f := newIntegrationFixture(t)
	defer f.cleanup()

	u, _ := domain.NewUser(1, "admin@example.com", "Admin", testPassword(), []domain.UserRole{domain.UserRoleAdmin, domain.UserRoleCustomer})
	f.repo.Save(ctx, u)

	got, err := f.repo.FindByEmail(ctx, "admin@example.com")
	if err != nil {
		t.Fatalf("find: %v", err)
	}
	if !got.HasRole(domain.UserRoleAdmin) {
		t.Error("expected admin role")
	}
	if !got.HasRole(domain.UserRoleCustomer) {
		t.Error("expected customer role")
	}
}

func TestIntegrationUser_EmailCaseInsensitive(t *testing.T) {
	f := newIntegrationFixture(t)
	defer f.cleanup()

	u, _ := domain.NewUser(1, "Case@Test.Com", "Case Test", testPassword(), nil)
	f.repo.Save(ctx, u)

	got, err := f.repo.FindByEmail(ctx, "case@test.com")
	if err != nil {
		t.Fatalf("find lowercase email: %v", err)
	}
	if got.Email != "case@test.com" {
		t.Errorf("expected normalized email, got %q", got.Email)
	}
}

func TestIntegrationUser_CachePopulation(t *testing.T) {
	f := newIntegrationFixture(t)
	defer f.cleanup()

	u, _ := domain.NewUser(1, "cache@example.com", "Cache Test", testPassword(), nil)
	f.repo.Save(ctx, u)

	f.repo.FindByID(ctx, 1)

	key := f.repo.idCacheKey(1)
	val, err := f.rdb.Get(ctx, key).Result()
	if err != nil {
		t.Fatalf("redis get: %v", err)
	}
	if val == "" {
		t.Fatal("expected cached value, got empty")
	}

	emailKey := f.repo.emailCacheKey("cache@example.com")
	emailVal, err := f.rdb.Get(ctx, emailKey).Result()
	if err != nil {
		t.Fatalf("redis get email cache: %v", err)
	}
	if emailVal == "" {
		t.Fatal("expected cached email value, got empty")
	}
}

func TestIntegrationUser_CacheInvalidationOnSave(t *testing.T) {
	f := newIntegrationFixture(t)
	defer f.cleanup()

	u, _ := domain.NewUser(1, "inv@example.com", "Before", testPassword(), nil)
	f.repo.Save(ctx, u)

	f.repo.FindByID(ctx, 1)
	if _, err := f.rdb.Get(ctx, f.repo.idCacheKey(1)).Result(); err != nil {
		t.Fatalf("expected cache hit before save: %v", err)
	}

	u.ChangeName("After")
	f.repo.Save(ctx, u)

	_, err := f.rdb.Get(ctx, f.repo.idCacheKey(1)).Result()
	if err != redis.Nil {
		t.Errorf("expected cache miss after save, got %v", err)
	}
}

func TestIntegrationUser_CacheInvalidationOnDelete(t *testing.T) {
	f := newIntegrationFixture(t)
	defer f.cleanup()

	u, _ := domain.NewUser(1, "delcache@example.com", "Del Cache", testPassword(), nil)
	f.repo.Save(ctx, u)

	f.repo.FindByID(ctx, 1)
	if _, err := f.rdb.Get(ctx, f.repo.idCacheKey(1)).Result(); err != nil {
		t.Fatalf("expected cache hit before delete: %v", err)
	}

	f.repo.Delete(ctx, 1)

	_, err := f.rdb.Get(ctx, f.repo.idCacheKey(1)).Result()
	if err != redis.Nil {
		t.Errorf("expected cache miss after delete, got %v", err)
	}
}
