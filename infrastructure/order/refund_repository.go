package order

import (
	"context"
	"database/sql"
	"time"

	"github.com/jmoiron/sqlx"

	"github.com/beeleelee/mall/domain/kernel"
	domain "github.com/beeleelee/mall/domain/order"
)

type refundRow struct {
	ID            int64      `db:"id"`
	OrderID       int64      `db:"order_id"`
	MandateID     *int64     `db:"mandate_id"`
	Amount        int64      `db:"amount"`
	Reason        string     `db:"reason"`
	Status        string     `db:"status"`
	CreatedAt     time.Time  `db:"created_at"`
	ProcessedAt   *time.Time `db:"processed_at"`
	FailedAt      *time.Time `db:"failed_at"`
	FailureReason string     `db:"failure_reason"`
}

func (r refundRow) toDomain() *domain.Refund {
	refund := &domain.Refund{
		AggregateRoot: kernel.NewAggregateRoot(kernel.ID(r.ID)),
		OrderID:       kernel.ID(r.OrderID),
		Amount:        r.Amount,
		Reason:        r.Reason,
		Status:        domain.RefundStatus(r.Status),
		CreatedAt:     r.CreatedAt,
		ProcessedAt:   r.ProcessedAt,
		FailedAt:      r.FailedAt,
		FailureReason: r.FailureReason,
	}
	if r.MandateID != nil {
		refund.MandateID = kernel.ID(*r.MandateID)
	}
	return refund
}

type PostgresRefundRepository struct {
	db *sqlx.DB
}

func NewPostgresRefundRepository(db *sqlx.DB) *PostgresRefundRepository {
	return &PostgresRefundRepository{db: db}
}

func (r *PostgresRefundRepository) Save(ctx context.Context, refund *domain.Refund) error {
	mandateID := (*int64)(nil)
	if refund.MandateID > 0 {
		v := refund.MandateID.Int64()
		mandateID = &v
	}
	_, err := r.db.ExecContext(ctx, `
		INSERT INTO refunds (id, order_id, mandate_id, amount, reason, status, created_at, processed_at, failed_at, failure_reason)
		VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10)
		ON CONFLICT (id) DO UPDATE SET
			status = EXCLUDED.status,
			processed_at = EXCLUDED.processed_at,
			failed_at = EXCLUDED.failed_at,
			failure_reason = EXCLUDED.failure_reason
	`,
		refund.ID.Int64(), refund.OrderID.Int64(), mandateID,
		refund.Amount, refund.Reason, string(refund.Status),
		refund.CreatedAt, refund.ProcessedAt, refund.FailedAt, refund.FailureReason,
	)
	if err != nil {
		return kernel.NewDomainErrorWithCause(kernel.ErrInternal, "save refund", err)
	}
	return nil
}

func (r *PostgresRefundRepository) FindByID(ctx context.Context, id kernel.ID) (*domain.Refund, error) {
	var row refundRow
	err := r.db.GetContext(ctx, &row, `SELECT * FROM refunds WHERE id = $1`, id.Int64())
	if err == sql.ErrNoRows {
		return nil, kernel.NewDomainError(kernel.ErrNotFound, "refund not found")
	}
	if err != nil {
		return nil, kernel.NewDomainErrorWithCause(kernel.ErrInternal, "find refund by id", err)
	}
	return row.toDomain(), nil
}

func (r *PostgresRefundRepository) FindByOrderID(ctx context.Context, orderID kernel.ID) ([]*domain.Refund, error) {
	var rows []refundRow
	err := r.db.SelectContext(ctx, &rows, `SELECT * FROM refunds WHERE order_id = $1 ORDER BY created_at DESC`, orderID.Int64())
	if err != nil {
		return nil, kernel.NewDomainErrorWithCause(kernel.ErrInternal, "find refunds by order id", err)
	}
	result := make([]*domain.Refund, len(rows))
	for i, row := range rows {
		result[i] = row.toDomain()
	}
	return result, nil
}
