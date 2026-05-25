package checkout

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"time"

	"github.com/jmoiron/sqlx"
	"github.com/redis/go-redis/v9"

	domain "github.com/beeleelee/mall/domain/checkout"
	"github.com/beeleelee/mall/domain/kernel"
)

type nullRawMessage []byte

func (m *nullRawMessage) Scan(src any) error {
	if src == nil {
		*m = nil
		return nil
	}
	*m = src.([]byte)
	return nil
}

func (m nullRawMessage) MarshalJSON() ([]byte, error) {
	if m == nil {
		return []byte("null"), nil
	}
	return []byte(m), nil
}

func (m *nullRawMessage) UnmarshalJSON(data []byte) error {
	if string(data) == "null" {
		*m = nil
		return nil
	}
	*m = data
	return nil
}

type sessionRow struct {
	ID              int64           `db:"id"`
	UserID          int64           `db:"user_id"`
	CartID          int64           `db:"cart_id"`
	CartSnapshot    nullRawMessage  `db:"cart_snapshot"`
	ShippingAddress nullRawMessage  `db:"shipping_address"`
	BillingAddress  nullRawMessage  `db:"billing_address"`
	ShippingOption  nullRawMessage  `db:"shipping_option"`
	PaymentHandler  string          `db:"payment_handler"`
	Subtotal        int64           `db:"subtotal"`
	ShippingCost    int64           `db:"shipping_cost"`
	TaxAmount       int64           `db:"tax_amount"`
	GrandTotal      int64           `db:"grand_total"`
	Status          string          `db:"status"`
	CompletedAt     *time.Time      `db:"completed_at"`
	CreatedAt       time.Time       `db:"created_at"`
	UpdatedAt       time.Time       `db:"updated_at"`
}

func (r sessionRow) toDomain() (*domain.CheckoutSession, error) {
	var snapshot domain.CartSnapshot
	if len(r.CartSnapshot) > 0 {
		if err := json.Unmarshal(r.CartSnapshot, &snapshot); err != nil {
			return nil, kernel.NewDomainErrorWithCause(kernel.ErrInternal, "unmarshal cart snapshot", err)
		}
	}

	var shippingAddr *domain.Address
	if len(r.ShippingAddress) > 0 {
		var addr domain.Address
		if err := json.Unmarshal(r.ShippingAddress, &addr); err != nil {
			return nil, kernel.NewDomainErrorWithCause(kernel.ErrInternal, "unmarshal shipping address", err)
		}
		shippingAddr = &addr
	}

	var billingAddr *domain.Address
	if len(r.BillingAddress) > 0 {
		var addr domain.Address
		if err := json.Unmarshal(r.BillingAddress, &addr); err != nil {
			return nil, kernel.NewDomainErrorWithCause(kernel.ErrInternal, "unmarshal billing address", err)
		}
		billingAddr = &addr
	}

	var shippingOpt *domain.ShippingOption
	if len(r.ShippingOption) > 0 {
		var opt domain.ShippingOption
		if err := json.Unmarshal(r.ShippingOption, &opt); err != nil {
			return nil, kernel.NewDomainErrorWithCause(kernel.ErrInternal, "unmarshal shipping option", err)
		}
		shippingOpt = &opt
	}

	return domain.NewCheckoutSessionFromSnapshot(
		kernel.ID(r.ID),
		kernel.ID(r.UserID),
		kernel.ID(r.CartID),
		snapshot,
		shippingAddr,
		billingAddr,
		shippingOpt,
		r.PaymentHandler,
		r.Subtotal,
		r.ShippingCost,
		r.TaxAmount,
		r.GrandTotal,
		domain.CheckoutStatus(r.Status),
		r.CompletedAt,
		r.CreatedAt,
		r.UpdatedAt,
	), nil
}

func fromDomain(s *domain.CheckoutSession) (sessionRow, error) {
	cartSnapshot, err := json.Marshal(s.CartSnapshot)
	if err != nil {
		return sessionRow{}, kernel.NewDomainErrorWithCause(kernel.ErrInternal, "marshal cart snapshot", err)
	}

	var shippingAddr nullRawMessage
	if s.ShippingAddress != nil {
		data, mErr := json.Marshal(s.ShippingAddress)
		if mErr != nil {
			return sessionRow{}, kernel.NewDomainErrorWithCause(kernel.ErrInternal, "marshal shipping address", err)
		}
		shippingAddr = data
	}
	var billingAddr nullRawMessage
	if s.BillingAddress != nil {
		data, mErr := json.Marshal(s.BillingAddress)
		if mErr != nil {
			return sessionRow{}, kernel.NewDomainErrorWithCause(kernel.ErrInternal, "marshal billing address", err)
		}
		billingAddr = data
	}
	var shippingOpt nullRawMessage
	if s.ShippingOption != nil {
		data, mErr := json.Marshal(s.ShippingOption)
		if mErr != nil {
			return sessionRow{}, kernel.NewDomainErrorWithCause(kernel.ErrInternal, "marshal shipping option", err)
		}
		shippingOpt = data
	}

	return sessionRow{
		ID:              s.ID.Int64(),
		UserID:          s.UserID.Int64(),
		CartID:          s.CartID.Int64(),
		CartSnapshot:    cartSnapshot,
		ShippingAddress: shippingAddr,
		BillingAddress:  billingAddr,
		ShippingOption:  shippingOpt,
		PaymentHandler:  s.PaymentHandler,
		Subtotal:        s.Subtotal,
		ShippingCost:    s.ShippingCost,
		TaxAmount:       s.TaxAmount,
		GrandTotal:      s.GrandTotal,
		Status:          string(s.Status),
		CompletedAt:     s.CompletedAt,
		CreatedAt:       s.CreatedAt,
		UpdatedAt:       s.UpdatedAt,
	}, nil
}

type PostgresCheckoutRepository struct {
	db    *sqlx.DB
	redis *redis.Client
	ttl   time.Duration
}

func NewPostgresCheckoutRepository(db *sqlx.DB, rdb *redis.Client) *PostgresCheckoutRepository {
	return &PostgresCheckoutRepository{
		db:    db,
		redis: rdb,
		ttl:   15 * time.Minute,
	}
}

func (r *PostgresCheckoutRepository) Save(ctx context.Context, session *domain.CheckoutSession) error {
	row, err := fromDomain(session)
	if err != nil {
		return err
	}

	_, err = r.db.ExecContext(ctx, `
		INSERT INTO checkout_sessions (id, user_id, cart_id, cart_snapshot, shipping_address, billing_address, shipping_option, payment_handler, subtotal, shipping_cost, tax_amount, grand_total, status, completed_at, created_at, updated_at)
		VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11, $12, $13, $14, $15, $16)
		ON CONFLICT (id) DO UPDATE SET
			user_id = EXCLUDED.user_id,
			cart_id = EXCLUDED.cart_id,
			cart_snapshot = EXCLUDED.cart_snapshot,
			shipping_address = EXCLUDED.shipping_address,
			billing_address = EXCLUDED.billing_address,
			shipping_option = EXCLUDED.shipping_option,
			payment_handler = EXCLUDED.payment_handler,
			subtotal = EXCLUDED.subtotal,
			shipping_cost = EXCLUDED.shipping_cost,
			tax_amount = EXCLUDED.tax_amount,
			grand_total = EXCLUDED.grand_total,
			status = EXCLUDED.status,
			completed_at = EXCLUDED.completed_at,
			updated_at = EXCLUDED.updated_at
	`, row.ID, row.UserID, row.CartID, row.CartSnapshot, row.ShippingAddress, row.BillingAddress, row.ShippingOption, row.PaymentHandler, row.Subtotal, row.ShippingCost, row.TaxAmount, row.GrandTotal, row.Status, row.CompletedAt, row.CreatedAt, row.UpdatedAt)
	if err != nil {
		return kernel.NewDomainErrorWithCause(kernel.ErrInternal, "save checkout session", err)
	}

	r.invalidateCache(ctx, session.ID, session.UserID)
	return nil
}

func (r *PostgresCheckoutRepository) FindByID(ctx context.Context, id kernel.ID) (*domain.CheckoutSession, error) {
	cacheKey := r.idCacheKey(id)
	if session, err := r.readCache(ctx, cacheKey); err == nil && session != nil {
		return session, nil
	}

	var row sessionRow
	err := r.db.GetContext(ctx, &row, `SELECT * FROM checkout_sessions WHERE id = $1`, id.Int64())
	if err == sql.ErrNoRows {
		return nil, kernel.NewDomainError(kernel.ErrNotFound, "checkout session not found")
	}
	if err != nil {
		return nil, kernel.NewDomainErrorWithCause(kernel.ErrInternal, "find checkout by id", err)
	}

	session, err := row.toDomain()
	if err != nil {
		return nil, err
	}
	r.writeCache(ctx, cacheKey, session)
	return session, nil
}

func (r *PostgresCheckoutRepository) FindByUserID(ctx context.Context, userID kernel.ID) (*domain.CheckoutSession, error) {
	cacheKey := r.userCacheKey(userID)
	if session, err := r.readCache(ctx, cacheKey); err == nil && session != nil {
		return session, nil
	}

	var row sessionRow
	err := r.db.GetContext(ctx, &row, `SELECT * FROM checkout_sessions WHERE user_id = $1`, userID.Int64())
	if err == sql.ErrNoRows {
		return nil, kernel.NewDomainError(kernel.ErrNotFound, "checkout session not found")
	}
	if err != nil {
		return nil, kernel.NewDomainErrorWithCause(kernel.ErrInternal, "find checkout by user", err)
	}

	session, err := row.toDomain()
	if err != nil {
		return nil, err
	}
	r.writeCache(ctx, r.idCacheKey(session.ID), session)
	r.writeCache(ctx, cacheKey, session)
	return session, nil
}

func (r *PostgresCheckoutRepository) Delete(ctx context.Context, id kernel.ID) error {
	var userID int64
	err := r.db.GetContext(ctx, &userID, `SELECT user_id FROM checkout_sessions WHERE id = $1`, id.Int64())
	if err == sql.ErrNoRows {
		return nil
	}
	if err != nil {
		return kernel.NewDomainErrorWithCause(kernel.ErrInternal, "get checkout user_id", err)
	}

	_, err = r.db.ExecContext(ctx, `DELETE FROM checkout_sessions WHERE id = $1`, id.Int64())
	if err != nil {
		return kernel.NewDomainErrorWithCause(kernel.ErrInternal, "delete checkout session", err)
	}

	r.invalidateCache(ctx, id, kernel.ID(userID))
	return nil
}

func (r *PostgresCheckoutRepository) idCacheKey(id kernel.ID) string {
	return fmt.Sprintf("checkout:id:%d", id.Int64())
}

func (r *PostgresCheckoutRepository) userCacheKey(userID kernel.ID) string {
	return fmt.Sprintf("checkout:user:%d", userID.Int64())
}

func (r *PostgresCheckoutRepository) readCache(ctx context.Context, key string) (*domain.CheckoutSession, error) {
	data, err := r.redis.Get(ctx, key).Bytes()
	if err == redis.Nil {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}

	var row sessionRow
	if err := json.Unmarshal(data, &row); err != nil {
		return nil, err
	}

	return row.toDomain()
}

func (r *PostgresCheckoutRepository) writeCache(ctx context.Context, key string, session *domain.CheckoutSession) {
	row, err := fromDomain(session)
	if err != nil {
		return
	}
	data, err := json.Marshal(row)
	if err != nil {
		return
	}
	r.redis.Set(ctx, key, data, r.ttl)
}

func (r *PostgresCheckoutRepository) invalidateCache(ctx context.Context, id, userID kernel.ID) {
	r.redis.Del(ctx, r.idCacheKey(id))
	if userID > 0 {
		r.redis.Del(ctx, r.userCacheKey(userID))
	}
}
