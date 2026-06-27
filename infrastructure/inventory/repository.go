package inventory

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"time"

	"github.com/jmoiron/sqlx"
	"github.com/redis/go-redis/v9"

	domain "github.com/beeleelee/mall/domain/inventory"
	"github.com/beeleelee/mall/domain/kernel"
)

type inventoryRow struct {
	ID                int64     `db:"id"`
	ProductID         int64     `db:"product_id"`
	QuantityAvailable int       `db:"quantity_available"`
	ReservedQuantity  int       `db:"reserved_quantity"`
	LowStockThreshold int       `db:"low_stock_threshold"`
	CreatedAt         time.Time `db:"created_at"`
	UpdatedAt         time.Time `db:"updated_at"`
}

func (r inventoryRow) toDomain() *domain.InventoryItem {
	item := &domain.InventoryItem{
		AggregateRoot:     kernel.NewAggregateRoot(kernel.ID(r.ID)),
		ProductID:         kernel.ID(r.ProductID),
		QuantityAvailable: r.QuantityAvailable,
		ReservedQuantity:  r.ReservedQuantity,
		LowStockThreshold: r.LowStockThreshold,
	}
	item.CreatedAt = r.CreatedAt
	item.UpdatedAt = r.UpdatedAt
	return item
}

func fromDomain(item *domain.InventoryItem) inventoryRow {
	return inventoryRow{
		ID:                item.ID.Int64(),
		ProductID:         item.ProductID.Int64(),
		QuantityAvailable: item.QuantityAvailable,
		ReservedQuantity:  item.ReservedQuantity,
		LowStockThreshold: item.LowStockThreshold,
		CreatedAt:         item.CreatedAt,
		UpdatedAt:         item.UpdatedAt,
	}
}

type PostgresInventoryRepository struct {
	db    *sqlx.DB
	redis *redis.Client
	ttl   time.Duration
}

func NewPostgresInventoryRepository(db *sqlx.DB, rdb *redis.Client) *PostgresInventoryRepository {
	return &PostgresInventoryRepository{
		db:    db,
		redis: rdb,
		ttl:   15 * time.Minute,
	}
}

func (r *PostgresInventoryRepository) Save(ctx context.Context, item *domain.InventoryItem) error {
	row := fromDomain(item)
	_, err := r.db.ExecContext(ctx, `
		INSERT INTO inventory (id, product_id, quantity_available, reserved_quantity, low_stock_threshold, created_at, updated_at)
		VALUES ($1, $2, $3, $4, $5, $6, $7)
		ON CONFLICT (id) DO UPDATE SET
			product_id = EXCLUDED.product_id,
			quantity_available = EXCLUDED.quantity_available,
			reserved_quantity = EXCLUDED.reserved_quantity,
			low_stock_threshold = EXCLUDED.low_stock_threshold,
			updated_at = EXCLUDED.updated_at
	`, row.ID, row.ProductID, row.QuantityAvailable, row.ReservedQuantity, row.LowStockThreshold, row.CreatedAt, row.UpdatedAt)
	if err != nil {
		return kernel.NewDomainErrorWithCause(kernel.ErrInternal, "save inventory", err)
	}

	r.invalidateCache(ctx, item.ProductID)
	return nil
}

func (r *PostgresInventoryRepository) FindByProductID(ctx context.Context, productID kernel.ID) (*domain.InventoryItem, error) {
	cacheKey := r.cacheKey(productID)
	if item, err := r.readCache(ctx, cacheKey); err == nil && item != nil {
		return item, nil
	}

	var row inventoryRow
	err := r.db.GetContext(ctx, &row, `SELECT * FROM inventory WHERE product_id = $1`, productID.Int64())
	if err == sql.ErrNoRows {
		return nil, kernel.NewDomainError(kernel.ErrNotFound, "inventory not found for product")
	}
	if err != nil {
		return nil, kernel.NewDomainErrorWithCause(kernel.ErrInternal, "find inventory by product", err)
	}

	item := row.toDomain()
	r.writeCache(ctx, cacheKey, item)
	return item, nil
}

func (r *PostgresInventoryRepository) FindAll(ctx context.Context, offset, limit int) ([]*domain.InventoryItem, error) {
	if limit <= 0 || limit > 100 {
		limit = 20
	}

	var rows []inventoryRow
	err := r.db.SelectContext(ctx, &rows, `SELECT * FROM inventory ORDER BY product_id ASC LIMIT $1 OFFSET $2`, limit, offset)
	if err != nil {
		return nil, kernel.NewDomainErrorWithCause(kernel.ErrInternal, "find all inventory", err)
	}

	result := make([]*domain.InventoryItem, 0, len(rows))
	for _, row := range rows {
		result = append(result, row.toDomain())
	}
	return result, nil
}

func (r *PostgresInventoryRepository) FindLowStock(ctx context.Context, threshold int) ([]*domain.InventoryItem, error) {
	var rows []inventoryRow
	err := r.db.SelectContext(ctx, &rows, `SELECT * FROM inventory WHERE quantity_available <= $1 ORDER BY quantity_available ASC`, threshold)
	if err != nil {
		return nil, kernel.NewDomainErrorWithCause(kernel.ErrInternal, "find low stock inventory", err)
	}

	result := make([]*domain.InventoryItem, 0, len(rows))
	for _, row := range rows {
		result = append(result, row.toDomain())
	}
	return result, nil
}

func (r *PostgresInventoryRepository) Delete(ctx context.Context, id kernel.ID) error {
	var productID int64
	err := r.db.GetContext(ctx, &productID, `SELECT product_id FROM inventory WHERE id = $1`, id.Int64())
	if err == sql.ErrNoRows {
		return nil
	}
	if err != nil {
		return kernel.NewDomainErrorWithCause(kernel.ErrInternal, "get inventory product_id", err)
	}

	_, err = r.db.ExecContext(ctx, `DELETE FROM inventory WHERE id = $1`, id.Int64())
	if err != nil {
		return kernel.NewDomainErrorWithCause(kernel.ErrInternal, "delete inventory", err)
	}

	r.invalidateCache(ctx, kernel.ID(productID))
	return nil
}

func (r *PostgresInventoryRepository) cacheKey(productID kernel.ID) string {
	return fmt.Sprintf("inventory:product:%d", productID.Int64())
}

func (r *PostgresInventoryRepository) readCache(ctx context.Context, key string) (*domain.InventoryItem, error) {
	data, err := r.redis.Get(ctx, key).Bytes()
	if err == redis.Nil {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}

	var row inventoryRow
	if err := json.Unmarshal(data, &row); err != nil {
		return nil, err
	}
	return row.toDomain(), nil
}

func (r *PostgresInventoryRepository) writeCache(ctx context.Context, key string, item *domain.InventoryItem) {
	row := fromDomain(item)
	data, err := json.Marshal(row)
	if err != nil {
		return
	}
	r.redis.Set(ctx, key, data, r.ttl)
}

func (r *PostgresInventoryRepository) invalidateCache(ctx context.Context, productID kernel.ID) {
	r.redis.Del(ctx, r.cacheKey(productID))
}
