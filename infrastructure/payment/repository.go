package payment

import (
	"context"
	"database/sql"
	"encoding/json"
	"time"

	"github.com/jmoiron/sqlx"

	"github.com/beeleelee/mall/domain/kernel"
	domain "github.com/beeleelee/mall/domain/payment"
)

type mandateRow struct {
	ID        int64     `db:"id"`
	UserID    int64     `db:"user_id"`
	Status    string    `db:"status"`
	Scope     []byte    `db:"scope"`
	Signature string    `db:"signature"`
	Token     string    `db:"token"`
	CreatedAt time.Time `db:"created_at"`
	UpdatedAt time.Time `db:"updated_at"`
}

func (r mandateRow) toDomain() (*domain.Mandate, error) {
	var scope domain.MandateScope
	if err := json.Unmarshal(r.Scope, &scope); err != nil {
		return nil, kernel.NewDomainErrorWithCause(kernel.ErrInternal, "unmarshal mandate scope", err)
	}

	m := &domain.Mandate{
		AggregateRoot: kernel.NewAggregateRoot(kernel.ID(r.ID)),
		UserID:        kernel.ID(r.UserID),
		Status:        domain.MandateStatus(r.Status),
		Scope:         scope,
		Signature:     r.Signature,
		Token:         r.Token,
	}
	m.CreatedAt = r.CreatedAt
	m.UpdatedAt = r.UpdatedAt
	return m, nil
}

func fromDomain(m *domain.Mandate) (mandateRow, error) {
	scope, err := json.Marshal(m.Scope)
	if err != nil {
		return mandateRow{}, kernel.NewDomainErrorWithCause(kernel.ErrInternal, "marshal mandate scope", err)
	}

	return mandateRow{
		ID:        m.ID.Int64(),
		UserID:    m.UserID.Int64(),
		Status:    string(m.Status),
		Scope:     scope,
		Signature: m.Signature,
		Token:     m.Token,
		CreatedAt: m.CreatedAt,
		UpdatedAt: m.UpdatedAt,
	}, nil
}

type PostgresMandateRepository struct {
	db *sqlx.DB
}

func NewPostgresMandateRepository(db *sqlx.DB) *PostgresMandateRepository {
	return &PostgresMandateRepository{db: db}
}

func (r *PostgresMandateRepository) Save(ctx context.Context, mandate *domain.Mandate) error {
	row, err := fromDomain(mandate)
	if err != nil {
		return err
	}

	_, err = r.db.ExecContext(ctx, `
		INSERT INTO mandates (id, user_id, status, scope, signature, token, created_at, updated_at)
		VALUES ($1, $2, $3, $4, $5, $6, $7, $8)
		ON CONFLICT (id) DO UPDATE SET
			user_id = EXCLUDED.user_id,
			status = EXCLUDED.status,
			scope = EXCLUDED.scope,
			signature = EXCLUDED.signature,
			token = EXCLUDED.token,
			updated_at = EXCLUDED.updated_at
	`, row.ID, row.UserID, row.Status, row.Scope, row.Signature, row.Token, row.CreatedAt, row.UpdatedAt)
	if err != nil {
		return kernel.NewDomainErrorWithCause(kernel.ErrInternal, "save mandate", err)
	}
	return nil
}

func (r *PostgresMandateRepository) FindByID(ctx context.Context, id kernel.ID) (*domain.Mandate, error) {
	var row mandateRow
	err := r.db.GetContext(ctx, &row, `SELECT * FROM mandates WHERE id = $1`, id.Int64())
	if err == sql.ErrNoRows {
		return nil, kernel.NewDomainError(kernel.ErrNotFound, "mandate not found")
	}
	if err != nil {
		return nil, kernel.NewDomainErrorWithCause(kernel.ErrInternal, "find mandate by id", err)
	}
	return row.toDomain()
}

func (r *PostgresMandateRepository) FindByUserID(ctx context.Context, userID kernel.ID) ([]*domain.Mandate, error) {
	var rows []mandateRow
	err := r.db.SelectContext(ctx, &rows, `SELECT * FROM mandates WHERE user_id = $1 ORDER BY created_at DESC`, userID.Int64())
	if err != nil {
		return nil, kernel.NewDomainErrorWithCause(kernel.ErrInternal, "find mandates by user", err)
	}

	result := make([]*domain.Mandate, 0, len(rows))
	for _, row := range rows {
		m, err := row.toDomain()
		if err != nil {
			return nil, err
		}
		result = append(result, m)
	}
	return result, nil
}

func (r *PostgresMandateRepository) FindActiveByUser(ctx context.Context, userID kernel.ID) ([]*domain.Mandate, error) {
	var rows []mandateRow
	err := r.db.SelectContext(ctx, &rows, `SELECT * FROM mandates WHERE user_id = $1 AND status IN ('approved', 'executed') ORDER BY created_at DESC`, userID.Int64())
	if err != nil {
		return nil, kernel.NewDomainErrorWithCause(kernel.ErrInternal, "find active mandates by user", err)
	}

	result := make([]*domain.Mandate, 0, len(rows))
	for _, row := range rows {
		m, err := row.toDomain()
		if err != nil {
			return nil, err
		}
		result = append(result, m)
	}
	return result, nil
}

func (r *PostgresMandateRepository) Delete(ctx context.Context, id kernel.ID) error {
	_, err := r.db.ExecContext(ctx, `DELETE FROM mandates WHERE id = $1`, id.Int64())
	if err != nil {
		return kernel.NewDomainErrorWithCause(kernel.ErrInternal, "delete mandate", err)
	}
	return nil
}
