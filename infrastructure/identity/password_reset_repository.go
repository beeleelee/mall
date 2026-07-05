package identity

import (
	"context"
	"database/sql"
	"time"

	"github.com/jmoiron/sqlx"

	domain "github.com/beeleelee/mall/domain/identity"
	"github.com/beeleelee/mall/domain/kernel"
)

type passwordResetTokenRow struct {
	ID        int64     `db:"id"`
	UserID    int64     `db:"user_id"`
	TokenHash string    `db:"token_hash"`
	ExpiresAt time.Time `db:"expires_at"`
	Used      bool      `db:"used"`
	CreatedAt time.Time `db:"created_at"`
	UpdatedAt time.Time `db:"updated_at"`
}

func (r passwordResetTokenRow) toDomain() *domain.PasswordResetToken {
	return &domain.PasswordResetToken{
		Entity:    kernel.NewEntity(kernel.ID(r.ID)),
		UserID:    kernel.ID(r.UserID),
		TokenHash: r.TokenHash,
		ExpiresAt: r.ExpiresAt,
		Used:      r.Used,
	}
}

type PostgresPasswordResetTokenRepository struct {
	db *sqlx.DB
}

func NewPostgresPasswordResetTokenRepository(db *sqlx.DB) *PostgresPasswordResetTokenRepository {
	return &PostgresPasswordResetTokenRepository{db: db}
}

func (r *PostgresPasswordResetTokenRepository) Save(ctx context.Context, token *domain.PasswordResetToken) error {
	_, err := r.db.ExecContext(ctx, `
		INSERT INTO password_reset_tokens (id, user_id, token_hash, expires_at, used, created_at, updated_at)
		VALUES ($1, $2, $3, $4, $5, $6, $7)
		ON CONFLICT (id) DO UPDATE SET
			used = EXCLUDED.used,
			updated_at = EXCLUDED.updated_at
	`, token.ID.Int64(), token.UserID.Int64(), token.TokenHash, token.ExpiresAt, token.Used, token.CreatedAt, token.UpdatedAt)
	if err != nil {
		return kernel.NewDomainErrorWithCause(kernel.ErrInternal, "save password reset token", err)
	}
	return nil
}

func (r *PostgresPasswordResetTokenRepository) FindByHash(ctx context.Context, hash string) (*domain.PasswordResetToken, error) {
	var row passwordResetTokenRow
	err := r.db.GetContext(ctx, &row, "SELECT * FROM password_reset_tokens WHERE token_hash = $1", hash)
	if err == sql.ErrNoRows {
		return nil, kernel.NewDomainError(kernel.ErrNotFound, "reset token not found")
	}
	if err != nil {
		return nil, kernel.NewDomainErrorWithCause(kernel.ErrInternal, "find reset token", err)
	}
	return row.toDomain(), nil
}

func (r *PostgresPasswordResetTokenRepository) MarkUsed(ctx context.Context, id kernel.ID) error {
	_, err := r.db.ExecContext(ctx, "UPDATE password_reset_tokens SET used = TRUE, updated_at = NOW() WHERE id = $1", id.Int64())
	if err != nil {
		return kernel.NewDomainErrorWithCause(kernel.ErrInternal, "mark reset token used", err)
	}
	return nil
}

func (r *PostgresPasswordResetTokenRepository) DeleteExpired(ctx context.Context) error {
	_, err := r.db.ExecContext(ctx, "DELETE FROM password_reset_tokens WHERE expires_at < NOW()")
	if err != nil {
		return kernel.NewDomainErrorWithCause(kernel.ErrInternal, "delete expired tokens", err)
	}
	return nil
}
