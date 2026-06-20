package discount

import (
	"context"
	"database/sql"
	"time"

	"github.com/jmoiron/sqlx"

	domain "github.com/beeleelee/mall/domain/discount"
	"github.com/beeleelee/mall/domain/kernel"
)

type discountCodeRow struct {
	ID          int64     `db:"id"`
	Code        string    `db:"code"`
	Type        string    `db:"type"`
	Value       int64     `db:"value"`
	MinPurchase int64     `db:"min_purchase"`
	MaxUsages   int       `db:"max_usages"`
	UsedCount   int       `db:"used_count"`
	Expiry      time.Time `db:"expiry"`
	Active      bool      `db:"active"`
	Stackable   bool      `db:"stackable"`
	CreatedAt   time.Time `db:"created_at"`
	UpdatedAt   time.Time `db:"updated_at"`
}

func (r discountCodeRow) toDomain() *domain.DiscountCode {
	return &domain.DiscountCode{
		AggregateRoot: kernel.NewAggregateRoot(kernel.ID(r.ID)),
		Code:          r.Code,
		Type:          domain.DiscountType(r.Type),
		Value:         r.Value,
		MinPurchase:   r.MinPurchase,
		MaxUsages:     r.MaxUsages,
		UsedCount:     r.UsedCount,
		Expiry:        r.Expiry,
		Active:        r.Active,
		Stackable:     r.Stackable,
	}
}

func fromDomain(d *domain.DiscountCode) discountCodeRow {
	return discountCodeRow{
		ID:          d.ID.Int64(),
		Code:        d.Code,
		Type:        string(d.Type),
		Value:       d.Value,
		MinPurchase: d.MinPurchase,
		MaxUsages:   d.MaxUsages,
		UsedCount:   d.UsedCount,
		Expiry:      d.Expiry,
		Active:      d.Active,
		Stackable:   d.Stackable,
	}
}

type PostgresDiscountRepository struct {
	db *sqlx.DB
}

func NewPostgresDiscountRepository(db *sqlx.DB) *PostgresDiscountRepository {
	return &PostgresDiscountRepository{db: db}
}

func (r *PostgresDiscountRepository) Save(ctx context.Context, code *domain.DiscountCode) error {
	row := fromDomain(code)
	_, err := r.db.ExecContext(ctx, `
		INSERT INTO discount_codes (id, code, type, value, min_purchase, max_usages, used_count, expiry, active, stackable, created_at, updated_at)
		VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, NOW(), NOW())
		ON CONFLICT (id) DO UPDATE SET
			code = EXCLUDED.code,
			type = EXCLUDED.type,
			value = EXCLUDED.value,
			min_purchase = EXCLUDED.min_purchase,
			max_usages = EXCLUDED.max_usages,
			used_count = EXCLUDED.used_count,
			expiry = EXCLUDED.expiry,
			active = EXCLUDED.active,
			stackable = EXCLUDED.stackable,
			updated_at = NOW()
	`, row.ID, row.Code, row.Type, row.Value, row.MinPurchase, row.MaxUsages, row.UsedCount, row.Expiry, row.Active, row.Stackable)
	if err != nil {
		return kernel.NewDomainErrorWithCause(kernel.ErrInternal, "save discount code", err)
	}
	return nil
}

func (r *PostgresDiscountRepository) FindByCode(ctx context.Context, code string) (*domain.DiscountCode, error) {
	var row discountCodeRow
	err := r.db.GetContext(ctx, &row, `SELECT * FROM discount_codes WHERE code = $1`, code)
	if err == sql.ErrNoRows {
		return nil, kernel.NewDomainError(kernel.ErrNotFound, "discount code not found")
	}
	if err != nil {
		return nil, kernel.NewDomainErrorWithCause(kernel.ErrInternal, "find discount code", err)
	}
	return row.toDomain(), nil
}

func (r *PostgresDiscountRepository) IncrementUsage(ctx context.Context, id kernel.ID) error {
	_, err := r.db.ExecContext(ctx, `UPDATE discount_codes SET used_count = used_count + 1, updated_at = NOW() WHERE id = $1`, id.Int64())
	if err != nil {
		return kernel.NewDomainErrorWithCause(kernel.ErrInternal, "increment discount usage", err)
	}
	return nil
}
