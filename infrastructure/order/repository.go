package order

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"time"

	"github.com/jmoiron/sqlx"
	"github.com/redis/go-redis/v9"

	checkout "github.com/beeleelee/mall/domain/checkout"
	domain "github.com/beeleelee/mall/domain/order"
	"github.com/beeleelee/mall/domain/kernel"
)

type orderRow struct {
	ID              int64            `db:"id"`
	UserID          int64            `db:"user_id"`
	CheckoutID      int64            `db:"checkout_id"`
	CartID          int64            `db:"cart_id"`
	Items           json.RawMessage  `db:"items"`
	ShippingAddress json.RawMessage  `db:"shipping_address"`
	BillingAddress  json.RawMessage  `db:"billing_address"`
	ShippingOption  json.RawMessage  `db:"shipping_option"`
	PaymentHandler  string           `db:"payment_handler"`
	Subtotal        int64            `db:"subtotal"`
	ShippingCost    int64            `db:"shipping_cost"`
	TaxAmount       int64            `db:"tax_amount"`
	GrandTotal      int64            `db:"grand_total"`
	Status          string           `db:"status"`
	TrackingNumber  string           `db:"tracking_number"`
	Carrier         string           `db:"carrier"`
	ConfirmedAt     time.Time        `db:"confirmed_at"`
	ProcessingAt    *time.Time       `db:"processing_at"`
	ShippedAt       *time.Time       `db:"shipped_at"`
	DeliveredAt     *time.Time       `db:"delivered_at"`
	ReturnedAt      *time.Time       `db:"returned_at"`
	CancelledAt     *time.Time       `db:"cancelled_at"`
	CreatedAt       time.Time        `db:"created_at"`
	UpdatedAt       time.Time        `db:"updated_at"`
}

func (r orderRow) toDomain() (*domain.Order, error) {
	var items []domain.OrderLineItem
	if len(r.Items) > 0 {
		if err := json.Unmarshal(r.Items, &items); err != nil {
			return nil, kernel.NewDomainErrorWithCause(kernel.ErrInternal, "unmarshal order items", err)
		}
	}

	var shippingAddr checkout.Address
	if len(r.ShippingAddress) > 0 {
		json.Unmarshal(r.ShippingAddress, &shippingAddr)
	}
	var billingAddr checkout.Address
	if len(r.BillingAddress) > 0 {
		json.Unmarshal(r.BillingAddress, &billingAddr)
	}
	var shippingOpt checkout.ShippingOption
	if len(r.ShippingOption) > 0 {
		json.Unmarshal(r.ShippingOption, &shippingOpt)
	}

	return domain.NewOrderFromSnapshot(
		kernel.ID(r.ID),
		kernel.ID(r.UserID),
		kernel.ID(r.CheckoutID),
		kernel.ID(r.CartID),
		items,
		shippingAddr,
		billingAddr,
		shippingOpt,
		r.PaymentHandler,
		r.Subtotal,
		r.ShippingCost,
		r.TaxAmount,
		r.GrandTotal,
		domain.OrderStatus(r.Status),
		r.TrackingNumber,
		r.Carrier,
		r.ConfirmedAt,
		r.ProcessingAt,
		r.ShippedAt,
		r.DeliveredAt,
		r.ReturnedAt,
		r.CancelledAt,
		r.CreatedAt,
		r.UpdatedAt,
	), nil
}

func fromDomain(o *domain.Order) (orderRow, error) {
	items, err := json.Marshal(o.Items)
	if err != nil {
		return orderRow{}, kernel.NewDomainErrorWithCause(kernel.ErrInternal, "marshal order items", err)
	}
	shippingAddr, _ := json.Marshal(o.ShippingAddress)
	billingAddr, _ := json.Marshal(o.BillingAddress)
	shippingOpt, _ := json.Marshal(o.ShippingOption)

	return orderRow{
		ID:              o.ID.Int64(),
		UserID:          o.UserID.Int64(),
		CheckoutID:      o.CheckoutID.Int64(),
		CartID:          o.CartID.Int64(),
		Items:           items,
		ShippingAddress: shippingAddr,
		BillingAddress:  billingAddr,
		ShippingOption:  shippingOpt,
		PaymentHandler:  o.PaymentHandler,
		Subtotal:        o.Subtotal,
		ShippingCost:    o.ShippingCost,
		TaxAmount:       o.TaxAmount,
		GrandTotal:      o.GrandTotal,
		Status:          string(o.Status),
		TrackingNumber:  o.TrackingNumber,
		Carrier:         o.Carrier,
		ConfirmedAt:     o.ConfirmedAt,
		ProcessingAt:    o.ProcessingAt,
		ShippedAt:       o.ShippedAt,
		DeliveredAt:     o.DeliveredAt,
		ReturnedAt:      o.ReturnedAt,
		CancelledAt:     o.CancelledAt,
		CreatedAt:       o.CreatedAt,
		UpdatedAt:       o.UpdatedAt,
	}, nil
}

type PostgresOrderRepository struct {
	db    *sqlx.DB
	redis *redis.Client
	ttl   time.Duration
}

func NewPostgresOrderRepository(db *sqlx.DB, rdb *redis.Client) *PostgresOrderRepository {
	return &PostgresOrderRepository{
		db:    db,
		redis: rdb,
		ttl:   15 * time.Minute,
	}
}

func (r *PostgresOrderRepository) Save(ctx context.Context, order *domain.Order) error {
	row, err := fromDomain(order)
	if err != nil {
		return err
	}

	_, err = r.db.ExecContext(ctx, `
		INSERT INTO orders (id, user_id, checkout_id, cart_id, items, shipping_address, billing_address, shipping_option, payment_handler, subtotal, shipping_cost, tax_amount, grand_total, status, tracking_number, carrier, confirmed_at, processing_at, shipped_at, delivered_at, returned_at, cancelled_at, created_at, updated_at)
		VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11, $12, $13, $14, $15, $16, $17, $18, $19, $20, $21, $22, $23, $24)
		ON CONFLICT (id) DO UPDATE SET
			user_id = EXCLUDED.user_id,
			checkout_id = EXCLUDED.checkout_id,
			cart_id = EXCLUDED.cart_id,
			items = EXCLUDED.items,
			shipping_address = EXCLUDED.shipping_address,
			billing_address = EXCLUDED.billing_address,
			shipping_option = EXCLUDED.shipping_option,
			payment_handler = EXCLUDED.payment_handler,
			subtotal = EXCLUDED.subtotal,
			shipping_cost = EXCLUDED.shipping_cost,
			tax_amount = EXCLUDED.tax_amount,
			grand_total = EXCLUDED.grand_total,
			status = EXCLUDED.status,
			tracking_number = EXCLUDED.tracking_number,
			carrier = EXCLUDED.carrier,
			confirmed_at = EXCLUDED.confirmed_at,
			processing_at = EXCLUDED.processing_at,
			shipped_at = EXCLUDED.shipped_at,
			delivered_at = EXCLUDED.delivered_at,
			returned_at = EXCLUDED.returned_at,
			cancelled_at = EXCLUDED.cancelled_at,
			updated_at = EXCLUDED.updated_at
	`, row.ID, row.UserID, row.CheckoutID, row.CartID, row.Items,
		row.ShippingAddress, row.BillingAddress, row.ShippingOption, row.PaymentHandler,
		row.Subtotal, row.ShippingCost, row.TaxAmount, row.GrandTotal,
		row.Status, row.TrackingNumber, row.Carrier,
		row.ConfirmedAt, row.ProcessingAt, row.ShippedAt, row.DeliveredAt, row.ReturnedAt, row.CancelledAt,
		row.CreatedAt, row.UpdatedAt)
	if err != nil {
		return kernel.NewDomainErrorWithCause(kernel.ErrInternal, "save order", err)
	}

	r.invalidateCache(ctx, order.ID, order.UserID)
	return nil
}

func (r *PostgresOrderRepository) FindByID(ctx context.Context, id kernel.ID) (*domain.Order, error) {
	cacheKey := r.idCacheKey(id)
	if order, err := r.readCache(ctx, cacheKey); err == nil && order != nil {
		return order, nil
	}

	var row orderRow
	err := r.db.GetContext(ctx, &row, `SELECT * FROM orders WHERE id = $1`, id.Int64())
	if err == sql.ErrNoRows {
		return nil, kernel.NewDomainError(kernel.ErrNotFound, "order not found")
	}
	if err != nil {
		return nil, kernel.NewDomainErrorWithCause(kernel.ErrInternal, "find order by id", err)
	}

	order, err := row.toDomain()
	if err != nil {
		return nil, err
	}
	r.writeCache(ctx, cacheKey, order)
	return order, nil
}

func (r *PostgresOrderRepository) FindByUserID(ctx context.Context, userID kernel.ID) ([]*domain.Order, error) {
	var rows []orderRow
	err := r.db.SelectContext(ctx, &rows, `SELECT * FROM orders WHERE user_id = $1 ORDER BY created_at DESC`, userID.Int64())
	if err == sql.ErrNoRows || len(rows) == 0 {
		return nil, kernel.NewDomainError(kernel.ErrNotFound, "no orders found for user")
	}
	if err != nil {
		return nil, kernel.NewDomainErrorWithCause(kernel.ErrInternal, "find orders by user", err)
	}

	result := make([]*domain.Order, 0, len(rows))
	for _, row := range rows {
		order, err := row.toDomain()
		if err != nil {
			return nil, err
		}
		result = append(result, order)
	}
	return result, nil
}

func (r *PostgresOrderRepository) FindByCheckoutID(ctx context.Context, checkoutID kernel.ID) (*domain.Order, error) {
	var row orderRow
	err := r.db.GetContext(ctx, &row, `SELECT * FROM orders WHERE checkout_id = $1`, checkoutID.Int64())
	if err == sql.ErrNoRows {
		return nil, kernel.NewDomainError(kernel.ErrNotFound, "order not found for checkout")
	}
	if err != nil {
		return nil, kernel.NewDomainErrorWithCause(kernel.ErrInternal, "find order by checkout id", err)
	}
	return row.toDomain()
}

func (r *PostgresOrderRepository) Delete(ctx context.Context, id kernel.ID) error {
	var userID int64
	err := r.db.GetContext(ctx, &userID, `SELECT user_id FROM orders WHERE id = $1`, id.Int64())
	if err == sql.ErrNoRows {
		return nil
	}
	if err != nil {
		return kernel.NewDomainErrorWithCause(kernel.ErrInternal, "get order user_id", err)
	}

	_, err = r.db.ExecContext(ctx, `DELETE FROM orders WHERE id = $1`, id.Int64())
	if err != nil {
		return kernel.NewDomainErrorWithCause(kernel.ErrInternal, "delete order", err)
	}

	r.invalidateCache(ctx, id, kernel.ID(userID))
	return nil
}

func (r *PostgresOrderRepository) idCacheKey(id kernel.ID) string {
	return fmt.Sprintf("order:id:%d", id.Int64())
}

func (r *PostgresOrderRepository) readCache(ctx context.Context, key string) (*domain.Order, error) {
	data, err := r.redis.Get(ctx, key).Bytes()
	if err == redis.Nil {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}

	var row orderRow
	if err := json.Unmarshal(data, &row); err != nil {
		return nil, err
	}

	return row.toDomain()
}

func (r *PostgresOrderRepository) writeCache(ctx context.Context, key string, order *domain.Order) {
	row, err := fromDomain(order)
	if err != nil {
		return
	}
	data, err := json.Marshal(row)
	if err != nil {
		return
	}
	r.redis.Set(ctx, key, data, r.ttl)
}

func (r *PostgresOrderRepository) invalidateCache(ctx context.Context, id, userID kernel.ID) {
	r.redis.Del(ctx, r.idCacheKey(id))
	if userID > 0 {
		r.redis.Del(ctx, fmt.Sprintf("order:user:%d", userID.Int64()))
	}
}
