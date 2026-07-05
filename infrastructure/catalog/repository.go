package catalog

import (
	"context"
	"database/sql"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"strconv"
	"strings"
	"time"

	"github.com/jmoiron/sqlx"
	"github.com/redis/go-redis/v9"

	domain "github.com/beeleelee/mall/domain/catalog"
	"github.com/beeleelee/mall/domain/kernel"
)

type productRow struct {
	ID            int64           `db:"id"`
	SKU           string          `db:"sku"`
	Name          string          `db:"name"`
	Description   string          `db:"description"`
	Category      string          `db:"category"`
	PriceAmount   int64           `db:"price_amount"`
	PriceCurrency string          `db:"price_currency"`
	Status        string          `db:"status"`
	Attributes    json.RawMessage `db:"attributes"`
	SearchVector  *string         `db:"search_vector"`
	CreatedAt     time.Time       `db:"created_at"`
	UpdatedAt     time.Time       `db:"updated_at"`
}

type PostgresProductRepository struct {
	db    *sqlx.DB
	redis *redis.Client
	ttl   time.Duration
}

func NewPostgresProductRepository(db *sqlx.DB, redis *redis.Client) *PostgresProductRepository {
	return &PostgresProductRepository{
		db:    db,
		redis: redis,
		ttl:   15 * time.Minute,
	}
}

func (r *PostgresProductRepository) Save(ctx context.Context, product *domain.Product) error {
	attrs, err := json.Marshal(product.Attributes)
	if err != nil {
		return kernel.NewDomainErrorWithCause(kernel.ErrInternal, "marshal attributes", err)
	}

	_, err = r.db.ExecContext(ctx, `
		INSERT INTO products (id, sku, name, description, category, price_amount, price_currency, status, attributes, created_at, updated_at)
		VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11)
		ON CONFLICT (id) DO UPDATE SET
			sku = EXCLUDED.sku,
			name = EXCLUDED.name,
			description = EXCLUDED.description,
			category = EXCLUDED.category,
			price_amount = EXCLUDED.price_amount,
			price_currency = EXCLUDED.price_currency,
			status = EXCLUDED.status,
			attributes = EXCLUDED.attributes,
			updated_at = EXCLUDED.updated_at
	`, product.ID.Int64(), string(product.SKU), product.Name, product.Description,
		product.Category, product.Price.Amount, product.Price.Currency,
		string(product.Status), attrs, product.CreatedAt, product.UpdatedAt)
	if err != nil {
		return kernel.NewDomainErrorWithCause(kernel.ErrInternal, "save product", err)
	}

	r.invalidateCache(ctx, product.ID, product.SKU)

	return nil
}

func (r *PostgresProductRepository) FindByID(ctx context.Context, id kernel.ID) (*domain.Product, error) {
	cacheKey := r.idCacheKey(id)
	if product, err := r.readCache(ctx, cacheKey); err == nil && product != nil {
		return product, nil
	}

	return r.querySingle(ctx, "SELECT * FROM products WHERE id = $1", id.Int64())
}

func (r *PostgresProductRepository) FindBySKU(ctx context.Context, sku domain.SKU) (*domain.Product, error) {
	cacheKey := r.skuCacheKey(sku)
	if product, err := r.readCache(ctx, cacheKey); err == nil && product != nil {
		return product, nil
	}

	return r.querySingle(ctx, "SELECT * FROM products WHERE sku = $1", string(sku))
}

func (r *PostgresProductRepository) Search(ctx context.Context, query string, opts domain.SearchOptions) (*domain.SearchResult, error) {
	limit := opts.Limit
	if limit <= 0 {
		limit = 20
	}
	if limit > 100 {
		limit = 100
	}

	cursorID := int64(0)
	if opts.Cursor != "" {
		var err error
		cursorID, err = decodeCursor(opts.Cursor)
		if err != nil {
			return nil, kernel.NewDomainError(kernel.ErrInvalidArgument, "invalid cursor")
		}
	}

	where := []string{"1 = 1"}
	args := []any{}
	argIdx := 1

	where, args = addFilter(where, args, &argIdx, "status", string(opts.Status))
	where, args = addFilter(where, args, &argIdx, "category", opts.Category)
	where, args = addIntFilter(where, args, &argIdx, "price_amount", opts.MinPrice, ">=")
	where, args = addIntFilter(where, args, &argIdx, "price_amount", opts.MaxPrice, "<=")

	if opts.FulltextQuery != "" {
		where = append(where, fmt.Sprintf("search_vector @@ plainto_tsquery('english', $%d)", argIdx))
		args = append(args, opts.FulltextQuery)
		argIdx++
	} else if query != "" {
		where = append(where, fmt.Sprintf("name ILIKE '%%' || $%d || '%%'", argIdx))
		args = append(args, query)
		argIdx++
	}

	if cursorID > 0 {
		where = append(where, fmt.Sprintf("id < $%d", argIdx))
		args = append(args, cursorID)
		argIdx++
	}

	orderBy := "id DESC"
	if opts.FulltextQuery != "" {
		orderBy = fmt.Sprintf("ts_rank(search_vector, plainto_tsquery('english', $%d)) DESC", argIdx)
		args = append(args, opts.FulltextQuery)
		argIdx++
	}

	sqlQuery := fmt.Sprintf(`
		SELECT * FROM products
		WHERE %s
		ORDER BY %s
		LIMIT $%d
	`, strings.Join(where, " AND "), orderBy, argIdx)
	args = append(args, limit+1)

	rows, err := r.db.QueryxContext(ctx, sqlQuery, args...)
	if err != nil {
		return nil, kernel.NewDomainErrorWithCause(kernel.ErrInternal, "search products", err)
	}
	defer func() { _ = rows.Close() }()

	products := []*domain.Product{}
	for rows.Next() {
		row, err := r.scanRow(rows)
		if err != nil {
			return nil, err
		}
		product, err := row.toDomain()
		if err != nil {
			return nil, err
		}
		products = append(products, product)
	}
	if err := rows.Err(); err != nil {
		return nil, kernel.NewDomainErrorWithCause(kernel.ErrInternal, "iterate search results", err)
	}

	result := &domain.SearchResult{}

	if len(products) > limit {
		result.Products = products[:limit]
		result.HasMore = true
		result.NextCursor = encodeCursor(products[limit-1].ID.Int64())
	} else {
		result.Products = products
		result.HasMore = false
	}

	return result, nil
}

func (r *PostgresProductRepository) Delete(ctx context.Context, id kernel.ID) error {
	sku, err := r.getSKUByID(ctx, id)
	if err != nil {
		return err
	}

	_, err = r.db.ExecContext(ctx, "DELETE FROM products WHERE id = $1", id.Int64())
	if err != nil {
		return kernel.NewDomainErrorWithCause(kernel.ErrInternal, "delete product", err)
	}

	r.invalidateCache(ctx, id, domain.SKU(sku))

	return nil
}

func (r *PostgresProductRepository) getSKUByID(ctx context.Context, id kernel.ID) (string, error) {
	var sku string
	err := r.db.GetContext(ctx, &sku, "SELECT sku FROM products WHERE id = $1", id.Int64())
	if err == sql.ErrNoRows {
		return "", kernel.NewDomainError(kernel.ErrNotFound, "product not found")
	}
	if err != nil {
		return "", kernel.NewDomainErrorWithCause(kernel.ErrInternal, "get product sku", err)
	}
	return sku, nil
}

func (r *PostgresProductRepository) querySingle(ctx context.Context, query string, args ...any) (*domain.Product, error) {
	row, err := r.scanSingle(ctx, query, args...)
	if err != nil {
		return nil, err
	}

	product, err := row.toDomain()
	if err != nil {
		return nil, err
	}

	r.writeCache(ctx, r.idCacheKey(product.ID), product)
	r.writeCache(ctx, r.skuCacheKey(product.SKU), product)

	return product, nil
}

func (r *PostgresProductRepository) scanSingle(ctx context.Context, query string, args ...any) (productRow, error) {
	var row productRow
	err := r.db.GetContext(ctx, &row, query, args...)
	if err == sql.ErrNoRows {
		return row, kernel.NewDomainError(kernel.ErrNotFound, "product not found")
	}
	if err != nil {
		return row, kernel.NewDomainErrorWithCause(kernel.ErrInternal, "query product", err)
	}
	return row, nil
}

func (r *PostgresProductRepository) scanRow(rows *sqlx.Rows) (productRow, error) {
	var row productRow
	if err := rows.StructScan(&row); err != nil {
		return row, kernel.NewDomainErrorWithCause(kernel.ErrInternal, "scan product row", err)
	}
	return row, nil
}

func (r *PostgresProductRepository) idCacheKey(id kernel.ID) string {
	return fmt.Sprintf("catalog:product:id:%d", id.Int64())
}

func (r *PostgresProductRepository) skuCacheKey(sku domain.SKU) string {
	return fmt.Sprintf("catalog:product:sku:%s", string(sku))
}

func (r *PostgresProductRepository) readCache(ctx context.Context, key string) (*domain.Product, error) {
	data, err := r.redis.Get(ctx, key).Bytes()
	if err == redis.Nil {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}

	var row productRow
	if err := json.Unmarshal(data, &row); err != nil {
		return nil, err
	}

	return row.toDomain()
}

func (r *PostgresProductRepository) writeCache(ctx context.Context, key string, product *domain.Product) {
	row := fromDomain(product)
	data, err := json.Marshal(row)
	if err != nil {
		return
	}
	r.redis.Set(ctx, key, data, r.ttl)
}

func (r *PostgresProductRepository) invalidateCache(ctx context.Context, id kernel.ID, sku domain.SKU) {
	r.redis.Del(ctx, r.idCacheKey(id))
	if sku != "" {
		r.redis.Del(ctx, r.skuCacheKey(sku))
	}
}

func (r productRow) toDomain() (*domain.Product, error) {
	attrs := map[string]any{}
	if len(r.Attributes) > 0 {
		if err := json.Unmarshal(r.Attributes, &attrs); err != nil {
			return nil, kernel.NewDomainErrorWithCause(kernel.ErrInternal, "unmarshal attributes", err)
		}
	}

	p := &domain.Product{
		AggregateRoot: kernel.NewAggregateRoot(kernel.ID(r.ID)),
		SKU:           domain.SKU(r.SKU),
		Name:          r.Name,
		Description:   r.Description,
		Category:      r.Category,
		Price: domain.Money{
			Amount:   r.PriceAmount,
			Currency: r.PriceCurrency,
		},
		Status:     domain.ProductStatus(r.Status),
		Attributes: attrs,
	}

	p.CreatedAt = r.CreatedAt
	p.UpdatedAt = r.UpdatedAt

	return p, nil
}

func fromDomain(p *domain.Product) productRow {
	attrs, _ := json.Marshal(p.Attributes)

	return productRow{
		ID:            p.ID.Int64(),
		SKU:           string(p.SKU),
		Name:          p.Name,
		Description:   p.Description,
		Category:      p.Category,
		PriceAmount:   p.Price.Amount,
		PriceCurrency: p.Price.Currency,
		Status:        string(p.Status),
		Attributes:    attrs,
		CreatedAt:     p.CreatedAt,
		UpdatedAt:     p.UpdatedAt,
	}
}

func encodeCursor(id int64) domain.Cursor {
	return domain.Cursor(base64.RawURLEncoding.EncodeToString([]byte(strconv.FormatInt(id, 10))))
}

func decodeCursor(c domain.Cursor) (int64, error) {
	data, err := base64.RawURLEncoding.DecodeString(string(c))
	if err != nil {
		return 0, err
	}
	return strconv.ParseInt(string(data), 10, 64)
}

func addFilter(where []string, args []any, idx *int, column, value string) ([]string, []any) {
	if value == "" {
		return where, args
	}
	where = append(where, fmt.Sprintf("%s = $%d", column, *idx))
	args = append(args, value)
	*idx++
	return where, args
}

func addLikeFilter(where []string, args []any, idx *int, column, value string) ([]string, []any) {
	if value == "" {
		return where, args
	}
	where = append(where, fmt.Sprintf("%s ILIKE '%%' || $%d || '%%'", column, *idx))
	args = append(args, value)
	*idx++
	return where, args
}

func addIntFilter(where []string, args []any, idx *int, column string, value int64, op string) ([]string, []any) {
	if value == 0 {
		return where, args
	}
	where = append(where, fmt.Sprintf("%s %s $%d", column, op, *idx))
	args = append(args, value)
	*idx++
	return where, args
}
