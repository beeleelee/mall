package oauth

import (
	"context"
	"database/sql"
	"encoding/json"
	"time"

	"github.com/jmoiron/sqlx"

	"github.com/beeleelee/mall/domain/kernel"
	domain "github.com/beeleelee/mall/domain/oauth"
)

type clientRow struct {
	ID           int64     `db:"id"`
	ClientID     string    `db:"client_id"`
	SecretHash   string    `db:"secret_hash"`
	RedirectURIs string    `db:"redirect_uris"`
	Scopes       string    `db:"scopes"`
	Status       string    `db:"status"`
	CreatedAt    time.Time `db:"created_at"`
	UpdatedAt    time.Time `db:"updated_at"`
}

func (r clientRow) toDomain() (*domain.OAuthClient, error) {
	var redirectURIs, scopes []string
	if err := json.Unmarshal([]byte(r.RedirectURIs), &redirectURIs); err != nil {
		return nil, kernel.NewDomainErrorWithCause(kernel.ErrInternal, "unmarshal redirect_uris", err)
	}
	if err := json.Unmarshal([]byte(r.Scopes), &scopes); err != nil {
		return nil, kernel.NewDomainErrorWithCause(kernel.ErrInternal, "unmarshal scopes", err)
	}
	return domain.NewClientFromHash(
		kernel.ID(r.ID),
		r.ClientID,
		r.SecretHash,
		redirectURIs,
		scopes,
		domain.ClientStatus(r.Status),
	), nil
}

func fromClient(c *domain.OAuthClient) clientRow {
	redirectURIs, _ := json.Marshal(c.RedirectURIs)
	scopes, _ := json.Marshal(c.Scopes)
	return clientRow{
		ID:           c.ID.Int64(),
		ClientID:     c.ClientID,
		SecretHash:   c.SecretHash,
		RedirectURIs: string(redirectURIs),
		Scopes:       string(scopes),
		Status:       string(c.Status),
		CreatedAt:    c.CreatedAt,
		UpdatedAt:    c.UpdatedAt,
	}
}

type codeRow struct {
	Code        string    `db:"code"`
	ClientID    string    `db:"client_id"`
	UserID      int64     `db:"user_id"`
	RedirectURI string    `db:"redirect_uri"`
	Scope       string    `db:"scope"`
	ExpiresAt   time.Time `db:"expires_at"`
	Used        bool      `db:"used"`
	CreatedAt   time.Time `db:"created_at"`
}

func (r codeRow) toDomain() *domain.AuthorizationCode {
	return &domain.AuthorizationCode{
		Code:        r.Code,
		ClientID:    r.ClientID,
		UserID:      kernel.ID(r.UserID),
		RedirectURI: r.RedirectURI,
		Scope:       r.Scope,
		ExpiresAt:   r.ExpiresAt,
		Used:        r.Used,
	}
}

func fromCode(c *domain.AuthorizationCode) codeRow {
	return codeRow{
		Code:        c.Code,
		ClientID:    c.ClientID,
		UserID:      c.UserID.Int64(),
		RedirectURI: c.RedirectURI,
		Scope:       c.Scope,
		ExpiresAt:   c.ExpiresAt,
		Used:        c.Used,
	}
}

type tokenRow struct {
	ID        string    `db:"id"`
	ClientID  string    `db:"client_id"`
	UserID    int64     `db:"user_id"`
	Scope     string    `db:"scope"`
	ExpiresAt time.Time `db:"expires_at"`
	Revoked   bool      `db:"revoked"`
	CreatedAt time.Time `db:"created_at"`
}

func (r tokenRow) toDomain() *domain.RefreshToken {
	return &domain.RefreshToken{
		ID:        r.ID,
		ClientID:  r.ClientID,
		UserID:    kernel.ID(r.UserID),
		Scope:     r.Scope,
		ExpiresAt: r.ExpiresAt,
		Revoked:   r.Revoked,
	}
}

func fromToken(t *domain.RefreshToken) tokenRow {
	return tokenRow{
		ID:        t.ID,
		ClientID:  t.ClientID,
		UserID:    t.UserID.Int64(),
		Scope:     t.Scope,
		ExpiresAt: t.ExpiresAt,
		Revoked:   t.Revoked,
	}
}

type PostgresOAuthClientRepository struct {
	db *sqlx.DB
}

func NewPostgresOAuthClientRepository(db *sqlx.DB) *PostgresOAuthClientRepository {
	return &PostgresOAuthClientRepository{db: db}
}

func (r *PostgresOAuthClientRepository) Save(ctx context.Context, client *domain.OAuthClient) error {
	row := fromClient(client)
	_, err := r.db.ExecContext(ctx, `
		INSERT INTO oauth_clients (id, client_id, secret_hash, redirect_uris, scopes, status, created_at, updated_at)
		VALUES ($1, $2, $3, $4, $5, $6, $7, $8)
		ON CONFLICT (id) DO UPDATE SET
			client_id = EXCLUDED.client_id,
			secret_hash = EXCLUDED.secret_hash,
			redirect_uris = EXCLUDED.redirect_uris,
			scopes = EXCLUDED.scopes,
			status = EXCLUDED.status,
			updated_at = EXCLUDED.updated_at
	`, row.ID, row.ClientID, row.SecretHash, row.RedirectURIs, row.Scopes, row.Status, row.CreatedAt, row.UpdatedAt)
	if err != nil {
		return kernel.NewDomainErrorWithCause(kernel.ErrInternal, "save oauth client", err)
	}
	return nil
}

func (r *PostgresOAuthClientRepository) FindByClientID(ctx context.Context, clientID string) (*domain.OAuthClient, error) {
	var row clientRow
	err := r.db.GetContext(ctx, &row, `SELECT * FROM oauth_clients WHERE client_id = $1`, clientID)
	if err == sql.ErrNoRows {
		return nil, kernel.NewDomainError(kernel.ErrNotFound, "oauth client not found")
	}
	if err != nil {
		return nil, kernel.NewDomainErrorWithCause(kernel.ErrInternal, "find oauth client by client_id", err)
	}
	return row.toDomain()
}

func (r *PostgresOAuthClientRepository) FindByID(ctx context.Context, id kernel.ID) (*domain.OAuthClient, error) {
	var row clientRow
	err := r.db.GetContext(ctx, &row, `SELECT * FROM oauth_clients WHERE id = $1`, id.Int64())
	if err == sql.ErrNoRows {
		return nil, kernel.NewDomainError(kernel.ErrNotFound, "oauth client not found")
	}
	if err != nil {
		return nil, kernel.NewDomainErrorWithCause(kernel.ErrInternal, "find oauth client by id", err)
	}
	return row.toDomain()
}

type PostgresAuthorizationCodeRepository struct {
	db *sqlx.DB
}

func NewPostgresAuthorizationCodeRepository(db *sqlx.DB) *PostgresAuthorizationCodeRepository {
	return &PostgresAuthorizationCodeRepository{db: db}
}

func (r *PostgresAuthorizationCodeRepository) Save(ctx context.Context, code *domain.AuthorizationCode) error {
	row := fromCode(code)
	_, err := r.db.ExecContext(ctx, `
		INSERT INTO oauth_authorization_codes (code, client_id, user_id, redirect_uri, scope, expires_at, used, created_at)
		VALUES ($1, $2, $3, $4, $5, $6, $7, $8)
		ON CONFLICT (code) DO UPDATE SET
			used = EXCLUDED.used
	`, row.Code, row.ClientID, row.UserID, row.RedirectURI, row.Scope, row.ExpiresAt, row.Used, row.CreatedAt)
	if err != nil {
		return kernel.NewDomainErrorWithCause(kernel.ErrInternal, "save authorization code", err)
	}
	return nil
}

func (r *PostgresAuthorizationCodeRepository) FindByCode(ctx context.Context, code string) (*domain.AuthorizationCode, error) {
	var row codeRow
	err := r.db.GetContext(ctx, &row, `SELECT * FROM oauth_authorization_codes WHERE code = $1`, code)
	if err == sql.ErrNoRows {
		return nil, kernel.NewDomainError(kernel.ErrNotFound, "authorization code not found")
	}
	if err != nil {
		return nil, kernel.NewDomainErrorWithCause(kernel.ErrInternal, "find authorization code", err)
	}
	return row.toDomain(), nil
}

func (r *PostgresAuthorizationCodeRepository) Delete(ctx context.Context, code string) error {
	_, err := r.db.ExecContext(ctx, `DELETE FROM oauth_authorization_codes WHERE code = $1`, code)
	if err != nil {
		return kernel.NewDomainErrorWithCause(kernel.ErrInternal, "delete authorization code", err)
	}
	return nil
}

type PostgresRefreshTokenRepository struct {
	db *sqlx.DB
}

func NewPostgresRefreshTokenRepository(db *sqlx.DB) *PostgresRefreshTokenRepository {
	return &PostgresRefreshTokenRepository{db: db}
}

func (r *PostgresRefreshTokenRepository) Save(ctx context.Context, token *domain.RefreshToken) error {
	row := fromToken(token)
	_, err := r.db.ExecContext(ctx, `
		INSERT INTO oauth_refresh_tokens (id, client_id, user_id, scope, expires_at, revoked, created_at)
		VALUES ($1, $2, $3, $4, $5, $6, $7)
		ON CONFLICT (id) DO UPDATE SET
			revoked = EXCLUDED.revoked
	`, row.ID, row.ClientID, row.UserID, row.Scope, row.ExpiresAt, row.Revoked, row.CreatedAt)
	if err != nil {
		return kernel.NewDomainErrorWithCause(kernel.ErrInternal, "save refresh token", err)
	}
	return nil
}

func (r *PostgresRefreshTokenRepository) FindByID(ctx context.Context, id string) (*domain.RefreshToken, error) {
	var row tokenRow
	err := r.db.GetContext(ctx, &row, `SELECT * FROM oauth_refresh_tokens WHERE id = $1`, id)
	if err == sql.ErrNoRows {
		return nil, kernel.NewDomainError(kernel.ErrNotFound, "refresh token not found")
	}
	if err != nil {
		return nil, kernel.NewDomainErrorWithCause(kernel.ErrInternal, "find refresh token", err)
	}
	return row.toDomain(), nil
}

func (r *PostgresRefreshTokenRepository) Revoke(ctx context.Context, id string) error {
	_, err := r.db.ExecContext(ctx, `UPDATE oauth_refresh_tokens SET revoked = TRUE WHERE id = $1`, id)
	if err != nil {
		return kernel.NewDomainErrorWithCause(kernel.ErrInternal, "revoke refresh token", err)
	}
	return nil
}
