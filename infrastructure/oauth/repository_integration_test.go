package oauth

import (
	"context"
	"fmt"
	"math/rand"
	"net"
	"testing"
	"time"

	_ "github.com/jackc/pgx/v5/stdlib"
	"github.com/jmoiron/sqlx"

	domain "github.com/beeleelee/mall/domain/oauth"
	"github.com/beeleelee/mall/domain/kernel"
)

type integrationFixture struct {
	clients domain.OAuthClientRepository
	codes   domain.AuthorizationCodeRepository
	tokens  domain.RefreshTokenRepository
	db      *sqlx.DB
	schema  string
	cleanup func()
}

func servicesUp() bool {
	conn, err := net.DialTimeout("tcp", "localhost:5432", 3*time.Second)
	if err != nil {
		return false
	}
	conn.Close()
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

	clients := NewPostgresOAuthClientRepository(db)
	codes := NewPostgresAuthorizationCodeRepository(db)
	tokens := NewPostgresRefreshTokenRepository(db)

	cleanup := func() {
		db.Exec(fmt.Sprintf(`DROP SCHEMA "%s" CASCADE`, schema))
		db.Close()
	}

	return &integrationFixture{
		clients: clients,
		codes:   codes,
		tokens:  tokens,
		db:      db,
		schema:  schema,
		cleanup: cleanup,
	}
}

const upSQL = `
CREATE TABLE IF NOT EXISTS oauth_clients (
    id            BIGINT PRIMARY KEY,
    client_id     TEXT NOT NULL UNIQUE,
    secret_hash   TEXT NOT NULL,
    redirect_uris TEXT NOT NULL DEFAULT '[]',
    scopes        TEXT NOT NULL DEFAULT '[]',
    status        TEXT NOT NULL DEFAULT 'active',
    created_at    TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at    TIMESTAMPTZ NOT NULL DEFAULT now()
);
CREATE TABLE IF NOT EXISTS oauth_authorization_codes (
    code         TEXT PRIMARY KEY,
    client_id    TEXT NOT NULL,
    user_id      BIGINT NOT NULL,
    redirect_uri TEXT NOT NULL,
    scope        TEXT NOT NULL DEFAULT '',
    expires_at   TIMESTAMPTZ NOT NULL,
    used         BOOLEAN NOT NULL DEFAULT FALSE,
    created_at   TIMESTAMPTZ NOT NULL DEFAULT now()
);
CREATE TABLE IF NOT EXISTS oauth_refresh_tokens (
    id         TEXT PRIMARY KEY,
    client_id  TEXT NOT NULL,
    user_id    BIGINT NOT NULL,
    scope      TEXT NOT NULL DEFAULT '',
    expires_at TIMESTAMPTZ NOT NULL,
    revoked    BOOLEAN NOT NULL DEFAULT FALSE,
    created_at TIMESTAMPTZ NOT NULL DEFAULT now()
);
`

func TestPostgresOAuthClientRepository_SaveAndFind(t *testing.T) {
	f := newIntegrationFixture(t)
	defer f.cleanup()
	ctx := context.Background()

	client, err := domain.NewClient(1, "integration-client", "my-secret", []string{"https://app.example.com/cb"}, []string{"read", "write"})
	if err != nil {
		t.Fatal(err)
	}

	if err := f.clients.Save(ctx, client); err != nil {
		t.Fatal(err)
	}

	found, err := f.clients.FindByClientID(ctx, "integration-client")
	if err != nil {
		t.Fatal(err)
	}
	if found.ClientID != "integration-client" {
		t.Errorf("expected integration-client, got %s", found.ClientID)
	}
	if !found.VerifySecret("my-secret") {
		t.Error("expected secret to verify")
	}
}

func TestPostgresOAuthClientRepository_FindByID(t *testing.T) {
	f := newIntegrationFixture(t)
	defer f.cleanup()
	ctx := context.Background()

	client, err := domain.NewClient(42, "id-lookup", "secret", []string{"https://example.com/cb"}, []string{"read"})
	if err != nil {
		t.Fatal(err)
	}
	if err := f.clients.Save(ctx, client); err != nil {
		t.Fatal(err)
	}

	found, err := f.clients.FindByID(ctx, 42)
	if err != nil {
		t.Fatal(err)
	}
	if found.ClientID != "id-lookup" {
		t.Errorf("expected id-lookup, got %s", found.ClientID)
	}
}

func TestPostgresOAuthClientRepository_NotFound(t *testing.T) {
	f := newIntegrationFixture(t)
	defer f.cleanup()

	_, err := f.clients.FindByClientID(context.Background(), "nonexistent")
	if !kernel.IsNotFound(err) {
		t.Errorf("expected not found, got %v", err)
	}
}

func TestPostgresAuthorizationCodeRepository_SaveAndFind(t *testing.T) {
	f := newIntegrationFixture(t)
	defer f.cleanup()
	ctx := context.Background()

	code, err := domain.NewAuthorizationCode("integration-client", 1, "https://example.com/cb", "read", 10*time.Minute)
	if err != nil {
		t.Fatal(err)
	}

	if err := f.codes.Save(ctx, code); err != nil {
		t.Fatal(err)
	}

	found, err := f.codes.FindByCode(ctx, code.Code)
	if err != nil {
		t.Fatal(err)
	}
	if found.ClientID != "integration-client" {
		t.Errorf("expected integration-client, got %s", found.ClientID)
	}
}

func TestPostgresAuthorizationCodeRepository_Delete(t *testing.T) {
	f := newIntegrationFixture(t)
	defer f.cleanup()
	ctx := context.Background()

	code, err := domain.NewAuthorizationCode("client", 1, "https://example.com/cb", "read", 10*time.Minute)
	if err != nil {
		t.Fatal(err)
	}
	if err := f.codes.Save(ctx, code); err != nil {
		t.Fatal(err)
	}

	if err := f.codes.Delete(ctx, code.Code); err != nil {
		t.Fatal(err)
	}

	_, err = f.codes.FindByCode(ctx, code.Code)
	if !kernel.IsNotFound(err) {
		t.Errorf("expected not found after delete, got %v", err)
	}
}

func TestPostgresRefreshTokenRepository_SaveAndFind(t *testing.T) {
	f := newIntegrationFixture(t)
	defer f.cleanup()
	ctx := context.Background()

	token, err := domain.NewRefreshToken("client", 1, "read", 30*24*time.Hour)
	if err != nil {
		t.Fatal(err)
	}

	if err := f.tokens.Save(ctx, token); err != nil {
		t.Fatal(err)
	}

	found, err := f.tokens.FindByID(ctx, token.ID)
	if err != nil {
		t.Fatal(err)
	}
	if found.ClientID != "client" {
		t.Errorf("expected client, got %s", found.ClientID)
	}
	if found.Revoked {
		t.Error("expected not revoked")
	}
}

func TestPostgresRefreshTokenRepository_Revoke(t *testing.T) {
	f := newIntegrationFixture(t)
	defer f.cleanup()
	ctx := context.Background()

	token, err := domain.NewRefreshToken("client", 1, "read", 30*24*time.Hour)
	if err != nil {
		t.Fatal(err)
	}
	if err := f.tokens.Save(ctx, token); err != nil {
		t.Fatal(err)
	}

	if err := f.tokens.Revoke(ctx, token.ID); err != nil {
		t.Fatal(err)
	}

	found, err := f.tokens.FindByID(ctx, token.ID)
	if err != nil {
		t.Fatal(err)
	}
	if !found.Revoked {
		t.Error("expected revoked")
	}
}
