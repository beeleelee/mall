package cart

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"time"

	"github.com/jmoiron/sqlx"
	"github.com/redis/go-redis/v9"

	domain "github.com/beeleelee/mall/domain/cart"
	"github.com/beeleelee/mall/domain/kernel"
)

type cartRow struct {
	ID        int64           `db:"id"`
	UserID    int64           `db:"user_id"`
	Items     json.RawMessage `db:"items"`
	Status    string          `db:"status"`
	CreatedAt time.Time       `db:"created_at"`
	UpdatedAt time.Time       `db:"updated_at"`
}

func (r cartRow) toDomain() (*domain.Cart, error) {
	var items []domain.CartItem
	if len(r.Items) > 0 {
		if err := json.Unmarshal(r.Items, &items); err != nil {
			return nil, kernel.NewDomainErrorWithCause(kernel.ErrInternal, "unmarshal cart items", err)
		}
	}
	return domain.NewCartFromSnapshot(
		kernel.ID(r.ID),
		kernel.ID(r.UserID),
		items,
		domain.CartStatus(r.Status),
		r.CreatedAt,
		r.UpdatedAt,
	), nil
}

func fromDomain(c *domain.Cart) (cartRow, error) {
	items, err := json.Marshal(c.Items)
	if err != nil {
		return cartRow{}, kernel.NewDomainErrorWithCause(kernel.ErrInternal, "marshal cart items", err)
	}
	return cartRow{
		ID:        c.ID.Int64(),
		UserID:    c.UserID.Int64(),
		Items:     items,
		Status:    string(c.Status),
		CreatedAt: c.CreatedAt,
		UpdatedAt: c.UpdatedAt,
	}, nil
}

type PostgresCartRepository struct {
	db    *sqlx.DB
	redis *redis.Client
	ttl   time.Duration
}

func NewPostgresCartRepository(db *sqlx.DB, rdb *redis.Client) *PostgresCartRepository {
	return &PostgresCartRepository{
		db:    db,
		redis: rdb,
		ttl:   15 * time.Minute,
	}
}

func (r *PostgresCartRepository) Save(ctx context.Context, cart *domain.Cart) error {
	row, err := fromDomain(cart)
	if err != nil {
		return err
	}

	_, err = r.db.ExecContext(ctx, `
		INSERT INTO carts (id, user_id, items, status, created_at, updated_at)
		VALUES ($1, $2, $3, $4, $5, $6)
		ON CONFLICT (id) DO UPDATE SET
			user_id = EXCLUDED.user_id,
			items = EXCLUDED.items,
			status = EXCLUDED.status,
			updated_at = EXCLUDED.updated_at
	`, row.ID, row.UserID, row.Items, row.Status, row.CreatedAt, row.UpdatedAt)
	if err != nil {
		return kernel.NewDomainErrorWithCause(kernel.ErrInternal, "save cart", err)
	}

	r.invalidateCache(ctx, cart.ID, cart.UserID)

	return nil
}

func (r *PostgresCartRepository) FindByID(ctx context.Context, id kernel.ID) (*domain.Cart, error) {
	cacheKey := r.idCacheKey(id)
	if cart, err := r.readCache(ctx, cacheKey); err == nil && cart != nil {
		return cart, nil
	}

	var row cartRow
	err := r.db.GetContext(ctx, &row, `SELECT * FROM carts WHERE id = $1`, id.Int64())
	if err == sql.ErrNoRows {
		return nil, kernel.NewDomainError(kernel.ErrNotFound, "cart not found")
	}
	if err != nil {
		return nil, kernel.NewDomainErrorWithCause(kernel.ErrInternal, "find cart by id", err)
	}

	cart, err := row.toDomain()
	if err != nil {
		return nil, err
	}
	r.writeCache(ctx, cacheKey, cart)

	return cart, nil
}

func (r *PostgresCartRepository) FindByUserID(ctx context.Context, userID kernel.ID) (*domain.Cart, error) {
	cacheKey := r.userCacheKey(userID)
	if cart, err := r.readCache(ctx, cacheKey); err == nil && cart != nil {
		return cart, nil
	}

	var row cartRow
	err := r.db.GetContext(ctx, &row, `SELECT * FROM carts WHERE user_id = $1`, userID.Int64())
	if err == sql.ErrNoRows {
		return nil, kernel.NewDomainError(kernel.ErrNotFound, "cart not found")
	}
	if err != nil {
		return nil, kernel.NewDomainErrorWithCause(kernel.ErrInternal, "find cart by user", err)
	}

	cart, err := row.toDomain()
	if err != nil {
		return nil, err
	}
	r.writeCache(ctx, r.idCacheKey(cart.ID), cart)
	r.writeCache(ctx, cacheKey, cart)

	return cart, nil
}

func (r *PostgresCartRepository) Delete(ctx context.Context, id kernel.ID) error {
	var userID int64
	err := r.db.GetContext(ctx, &userID, `SELECT user_id FROM carts WHERE id = $1`, id.Int64())
	if err == sql.ErrNoRows {
		return nil
	}
	if err != nil {
		return kernel.NewDomainErrorWithCause(kernel.ErrInternal, "get cart user_id", err)
	}

	_, err = r.db.ExecContext(ctx, `DELETE FROM carts WHERE id = $1`, id.Int64())
	if err != nil {
		return kernel.NewDomainErrorWithCause(kernel.ErrInternal, "delete cart", err)
	}

	r.invalidateCache(ctx, id, kernel.ID(userID))
	return nil
}

func (r *PostgresCartRepository) idCacheKey(id kernel.ID) string {
	return fmt.Sprintf("cart:id:%d", id.Int64())
}

func (r *PostgresCartRepository) userCacheKey(userID kernel.ID) string {
	return fmt.Sprintf("cart:user:%d", userID.Int64())
}

func (r *PostgresCartRepository) readCache(ctx context.Context, key string) (*domain.Cart, error) {
	data, err := r.redis.Get(ctx, key).Bytes()
	if err == redis.Nil {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}

	var row cartRow
	if err := json.Unmarshal(data, &row); err != nil {
		return nil, err
	}

	return row.toDomain()
}

func (r *PostgresCartRepository) writeCache(ctx context.Context, key string, cart *domain.Cart) {
	row, err := fromDomain(cart)
	if err != nil {
		return
	}
	data, err := json.Marshal(row)
	if err != nil {
		return
	}
	r.redis.Set(ctx, key, data, r.ttl)
}

func (r *PostgresCartRepository) invalidateCache(ctx context.Context, id, userID kernel.ID) {
	r.redis.Del(ctx, r.idCacheKey(id))
	if userID > 0 {
		r.redis.Del(ctx, r.userCacheKey(userID))
	}
}
