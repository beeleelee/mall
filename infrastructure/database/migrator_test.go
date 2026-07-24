package database

import (
	"fmt"
	"math/rand"
	"net"
	"testing"
	"time"

	_ "github.com/jackc/pgx/v5/stdlib"
	"github.com/jmoiron/sqlx"
)

func servicesUp() bool {
	pg, err := net.DialTimeout("tcp", "localhost:5432", 3*time.Second)
	if err != nil {
		return false
	}
	pg.Close()
	return true
}

func newTestDB(t *testing.T) (*sqlx.DB, string, func()) {
	t.Helper()

	if !servicesUp() {
		t.Skip("integration: need 'docker compose up postgres' running")
	}

	dsn := "postgres://mall:mall_dev@localhost:5432/mall?sslmode=disable"
	db, err := sqlx.Connect("pgx", dsn)
	if err != nil {
		t.Fatalf("connect postgres: %v", err)
	}
	db.SetMaxOpenConns(2)

	schema := fmt.Sprintf("test_migrator_%08x", rand.Int63())[:20]
	if _, err := db.Exec(fmt.Sprintf(`CREATE SCHEMA IF NOT EXISTS "%s"`, schema)); err != nil {
		db.Close()
		t.Fatalf("create schema: %v", err)
	}
	if _, err := db.Exec(fmt.Sprintf(`SET search_path TO "%s", public`, schema)); err != nil {
		db.Exec(fmt.Sprintf(`DROP SCHEMA IF EXISTS "%s" CASCADE`, schema))
		db.Close()
		t.Fatalf("set search_path: %v", err)
	}
	if _, err := db.Exec(`CREATE EXTENSION IF NOT EXISTS ltree`); err != nil {
		db.Exec(fmt.Sprintf(`DROP SCHEMA IF EXISTS "%s" CASCADE`, schema))
		db.Close()
		t.Fatalf("create ltree extension: %v", err)
	}

	cleanup := func() {
		db.Exec(fmt.Sprintf(`DROP SCHEMA IF EXISTS "%s" CASCADE`, schema))
		db.Close()
	}

	return db, schema, cleanup
}

func TestMigrator_Up(t *testing.T) {
	db, _, cleanup := newTestDB(t)
	defer cleanup()

	migrator := NewMigrator(db)
	if err := migrator.Up(); err != nil {
		t.Fatalf("first Up: %v", err)
	}

	var count int
	if err := db.Get(&count, `SELECT COUNT(*) FROM schema_migrations`); err != nil {
		t.Fatalf("count migrations: %v", err)
	}
	if count == 0 {
		t.Error("expected at least 1 migration to be applied")
	}
}

func TestMigrator_Up_Idempotent(t *testing.T) {
	db, _, cleanup := newTestDB(t)
	defer cleanup()

	migrator := NewMigrator(db)
	if err := migrator.Up(); err != nil {
		t.Fatalf("first Up: %v", err)
	}

	if err := migrator.Up(); err != nil {
		t.Fatalf("second Up: %v", err)
	}

	var count int
	if err := db.Get(&count, `SELECT COUNT(*) FROM schema_migrations`); err != nil {
		t.Fatalf("count migrations: %v", err)
	}
	if count == 0 {
		t.Error("expected at least 1 migration to be applied")
	}
}

func TestMigrator_Up_RunsInOrder(t *testing.T) {
	db, _, cleanup := newTestDB(t)
	defer cleanup()

	migrator := NewMigrator(db)
	if err := migrator.Up(); err != nil {
		t.Fatalf("Up: %v", err)
	}

	var versions []int
	if err := db.Select(&versions, `SELECT version FROM schema_migrations ORDER BY version`); err != nil {
		t.Fatalf("select versions: %v", err)
	}
	if len(versions) == 0 {
		t.Fatal("expected at least 1 migration version")
	}
	for i, v := range versions {
		if i > 0 && v <= versions[i-1] {
			t.Errorf("versions not in ascending order: %d after %d", v, versions[i-1])
		}
	}
}

func TestMigrator_ensureTable(t *testing.T) {
	db, _, cleanup := newTestDB(t)
	defer cleanup()

	migrator := NewMigrator(db)
	if err := migrator.ensureTable(); err != nil {
		t.Fatalf("ensureTable: %v", err)
	}

	if err := migrator.ensureTable(); err != nil {
		t.Fatalf("ensureTable idempotent: %v", err)
	}
}

func TestMigrator_loadMigrations_Count(t *testing.T) {
	db, _, cleanup := newTestDB(t)
	defer cleanup()

	migrator := NewMigrator(db)
	migrations, err := migrator.loadMigrations()
	if err != nil {
		t.Fatalf("loadMigrations: %v", err)
	}
	if len(migrations) == 0 {
		t.Fatal("expected at least 1 migration")
	}
	if migrations[0].Version != 1 {
		t.Errorf("first migration version = %d, want 1", migrations[0].Version)
	}
}

func TestMigrator_loadMigrations_HasUpSQL(t *testing.T) {
	db, _, cleanup := newTestDB(t)
	defer cleanup()

	migrator := NewMigrator(db)
	migrations, err := migrator.loadMigrations()
	if err != nil {
		t.Fatalf("loadMigrations: %v", err)
	}
	for _, m := range migrations {
		if m.UpSQL == "" {
			t.Errorf("migration %d has no UpSQL", m.Version)
		}
	}
}
