package analytics

import (
	"context"

	"github.com/jmoiron/sqlx"

	domain "github.com/beeleelee/mall/domain/analytics"
	"github.com/beeleelee/mall/domain/kernel"
)

type PostgresAnalyticsRepository struct {
	db *sqlx.DB
}

func NewPostgresAnalyticsRepository(db *sqlx.DB) *PostgresAnalyticsRepository {
	return &PostgresAnalyticsRepository{db: db}
}

func (r *PostgresAnalyticsRepository) GetDashboardOverview(ctx context.Context) (*domain.DashboardOverview, error) {
	dash := &domain.DashboardOverview{}

	if err := r.db.GetContext(ctx, &dash.Revenue, `
		SELECT
			COALESCE(SUM(grand_total) FILTER (WHERE created_at >= CURRENT_DATE), 0) AS today,
			COALESCE(SUM(grand_total) FILTER (WHERE created_at >= date_trunc('week', CURRENT_DATE)), 0) AS this_week,
			COALESCE(SUM(grand_total) FILTER (WHERE created_at >= date_trunc('month', CURRENT_DATE)), 0) AS this_month,
			COALESCE(SUM(grand_total), 0) AS all_time
		FROM orders WHERE status NOT IN ('cancelled', 'returned')
	`); err != nil {
		return nil, kernel.NewDomainErrorWithCause(kernel.ErrInternal, "dashboard revenue", err)
	}

	if err := r.db.GetContext(ctx, &dash.Orders, `
		SELECT
			COUNT(*) AS total,
			COALESCE(COUNT(*) FILTER (WHERE status = 'confirmed'), 0) AS confirmed,
			COALESCE(COUNT(*) FILTER (WHERE status = 'processing'), 0) AS processing,
			COALESCE(COUNT(*) FILTER (WHERE status = 'shipped'), 0) AS shipped,
			COALESCE(COUNT(*) FILTER (WHERE status = 'delivered'), 0) AS delivered,
			COALESCE(COUNT(*) FILTER (WHERE status = 'returned'), 0) AS returned,
			COALESCE(COUNT(*) FILTER (WHERE status = 'cancelled'), 0) AS cancelled
		FROM orders
	`); err != nil {
		return nil, kernel.NewDomainErrorWithCause(kernel.ErrInternal, "dashboard orders", err)
	}

	if err := r.db.GetContext(ctx, &dash.Users, `
		SELECT
			COUNT(*) AS total,
			COALESCE(COUNT(*) FILTER (WHERE status = 'active'), 0) AS active,
			COALESCE(COUNT(*) FILTER (WHERE status = 'suspended'), 0) AS suspended
		FROM users
	`); err != nil {
		return nil, kernel.NewDomainErrorWithCause(kernel.ErrInternal, "dashboard users", err)
	}

	if err := r.db.GetContext(ctx, &dash.Products, `
		SELECT
			COUNT(*) AS total,
			COALESCE(COUNT(*) FILTER (WHERE status = 'active'), 0) AS active
		FROM products
	`); err != nil {
		return nil, kernel.NewDomainErrorWithCause(kernel.ErrInternal, "dashboard products", err)
	}

	if err := r.db.GetContext(ctx, &dash.LowStockCount, `
		SELECT COUNT(*) FROM inventory WHERE quantity_available <= low_stock_threshold
	`); err != nil {
		return nil, kernel.NewDomainErrorWithCause(kernel.ErrInternal, "dashboard low stock", err)
	}

	if err := r.db.GetContext(ctx, &dash.AverageOrderValue, `
		SELECT COALESCE(ROUND(AVG(grand_total)), 0)
		FROM orders WHERE status NOT IN ('cancelled', 'returned')
	`); err != nil {
		return nil, kernel.NewDomainErrorWithCause(kernel.ErrInternal, "dashboard avg order value", err)
	}

	if err := r.db.GetContext(ctx, &dash.CancellationRate, `
		SELECT CASE WHEN COUNT(*) > 0
			THEN COUNT(*) FILTER (WHERE status = 'cancelled')::float / COUNT(*)::float
			ELSE 0 END
		FROM orders
	`); err != nil {
		return nil, kernel.NewDomainErrorWithCause(kernel.ErrInternal, "dashboard cancellation rate", err)
	}

	if err := r.db.SelectContext(ctx, &dash.RecentOrders, `
		SELECT id, user_id, grand_total, status, created_at::text AS created_at
		FROM orders ORDER BY created_at DESC LIMIT 10
	`); err != nil {
		return nil, kernel.NewDomainErrorWithCause(kernel.ErrInternal, "dashboard recent orders", err)
	}

	return dash, nil
}

func (r *PostgresAnalyticsRepository) GetRevenueByDay(ctx context.Context, days int) ([]*domain.DailyRevenue, error) {
	if days <= 0 || days > 365 {
		days = 30
	}
	var rows []*domain.DailyRevenue
	if err := r.db.SelectContext(ctx, &rows, `
		SELECT DATE(created_at)::text AS day,
			COALESCE(SUM(grand_total), 0) AS revenue,
			COUNT(*) AS order_count
		FROM orders
		WHERE status NOT IN ('cancelled', 'returned')
		  AND created_at >= CURRENT_DATE - make_interval(days => $1)
		GROUP BY day ORDER BY day
	`, days); err != nil {
		return nil, kernel.NewDomainErrorWithCause(kernel.ErrInternal, "revenue by day", err)
	}
	return rows, nil
}

func (r *PostgresAnalyticsRepository) GetRevenueByProduct(ctx context.Context, limit int) ([]*domain.ProductRevenue, error) {
	if limit <= 0 || limit > 100 {
		limit = 20
	}
	var rows []*domain.ProductRevenue
	if err := r.db.SelectContext(ctx, &rows, `
		SELECT
			(item->>'product_id')::bigint AS product_id,
			MAX(item->>'name') AS name,
			COALESCE(SUM((item->>'total_price')::numeric::bigint), 0) AS revenue,
			COUNT(*) AS order_count
		FROM orders o
		CROSS JOIN LATERAL jsonb_array_elements(o.items) AS item
		WHERE o.status NOT IN ('cancelled', 'returned')
		GROUP BY product_id
		ORDER BY revenue DESC LIMIT $1
	`, limit); err != nil {
		return nil, kernel.NewDomainErrorWithCause(kernel.ErrInternal, "revenue by product", err)
	}
	return rows, nil
}

func (r *PostgresAnalyticsRepository) GetRevenueByCategory(ctx context.Context) ([]*domain.CategoryRevenue, error) {
	var rows []*domain.CategoryRevenue
	if err := r.db.SelectContext(ctx, &rows, `
		SELECT
			COALESCE(p.category_id, 0) AS category_id,
			COALESCE(c.name, 'Uncategorized') AS category_name,
			COALESCE(SUM((item->>'total_price')::numeric::bigint), 0) AS revenue,
			COUNT(DISTINCT p.id) AS product_count
		FROM orders o
		CROSS JOIN LATERAL jsonb_array_elements(o.items) AS item
		JOIN products p ON p.id = (item->>'product_id')::bigint
		LEFT JOIN categories c ON c.id = p.category_id
		WHERE o.status NOT IN ('cancelled', 'returned')
		GROUP BY p.category_id, c.name
		ORDER BY revenue DESC
	`); err != nil {
		return nil, kernel.NewDomainErrorWithCause(kernel.ErrInternal, "revenue by category", err)
	}
	return rows, nil
}

func (r *PostgresAnalyticsRepository) GetAverageOrderValue(ctx context.Context) (int64, error) {
	var avg int64
	if err := r.db.GetContext(ctx, &avg, `
		SELECT COALESCE(ROUND(AVG(grand_total)), 0)
		FROM orders WHERE status NOT IN ('cancelled', 'returned')
	`); err != nil {
		return 0, kernel.NewDomainErrorWithCause(kernel.ErrInternal, "average order value", err)
	}
	return avg, nil
}

func (r *PostgresAnalyticsRepository) GetOrdersPerDay(ctx context.Context, days int) ([]*domain.DailyOrderCount, error) {
	if days <= 0 || days > 365 {
		days = 30
	}
	var rows []*domain.DailyOrderCount
	if err := r.db.SelectContext(ctx, &rows, `
		SELECT DATE(created_at)::text AS day, COUNT(*) AS count
		FROM orders
		WHERE created_at >= CURRENT_DATE - make_interval(days => $1)
		GROUP BY day ORDER BY day
	`, days); err != nil {
		return nil, kernel.NewDomainErrorWithCause(kernel.ErrInternal, "orders per day", err)
	}
	return rows, nil
}

func (r *PostgresAnalyticsRepository) GetOrderStatusBreakdown(ctx context.Context) ([]*domain.StatusCount, error) {
	var rows []*domain.StatusCount
	if err := r.db.SelectContext(ctx, &rows, `
		SELECT status, COUNT(*) AS count FROM orders GROUP BY status ORDER BY count DESC
	`); err != nil {
		return nil, kernel.NewDomainErrorWithCause(kernel.ErrInternal, "order status breakdown", err)
	}
	return rows, nil
}

func (r *PostgresAnalyticsRepository) GetCancellationRate(ctx context.Context) (float64, error) {
	var rate float64
	if err := r.db.GetContext(ctx, &rate, `
		SELECT CASE WHEN COUNT(*) > 0
			THEN COUNT(*) FILTER (WHERE status = 'cancelled')::float / COUNT(*)::float
			ELSE 0 END
		FROM orders
	`); err != nil {
		return 0, kernel.NewDomainErrorWithCause(kernel.ErrInternal, "cancellation rate", err)
	}
	return rate, nil
}

func (r *PostgresAnalyticsRepository) GetNewUsersPerDay(ctx context.Context, days int) ([]*domain.DailyUserCount, error) {
	if days <= 0 || days > 365 {
		days = 30
	}
	var rows []*domain.DailyUserCount
	if err := r.db.SelectContext(ctx, &rows, `
		SELECT DATE(created_at)::text AS day, COUNT(*) AS count
		FROM users
		WHERE created_at >= CURRENT_DATE - make_interval(days => $1)
		GROUP BY day ORDER BY day
	`, days); err != nil {
		return nil, kernel.NewDomainErrorWithCause(kernel.ErrInternal, "new users per day", err)
	}
	return rows, nil
}

func (r *PostgresAnalyticsRepository) GetUserStatusBreakdown(ctx context.Context) ([]*domain.StatusCount, error) {
	var rows []*domain.StatusCount
	if err := r.db.SelectContext(ctx, &rows, `
		SELECT status, COUNT(*) AS count FROM users GROUP BY status ORDER BY count DESC
	`); err != nil {
		return nil, kernel.NewDomainErrorWithCause(kernel.ErrInternal, "user status breakdown", err)
	}
	return rows, nil
}

func (r *PostgresAnalyticsRepository) GetTopSellers(ctx context.Context, limit int) ([]*domain.ProductSales, error) {
	if limit <= 0 || limit > 100 {
		limit = 20
	}
	var rows []*domain.ProductSales
	if err := r.db.SelectContext(ctx, &rows, `
		SELECT
			(item->>'product_id')::bigint AS product_id,
			MAX(item->>'name') AS name,
			COALESCE(SUM((item->>'quantity')::int), 0) AS quantity_sold,
			COALESCE(SUM((item->>'total_price')::numeric::bigint), 0) AS revenue,
			COUNT(*) AS order_count
		FROM orders o
		CROSS JOIN LATERAL jsonb_array_elements(o.items) AS item
		WHERE o.status NOT IN ('cancelled', 'returned')
		GROUP BY product_id
		ORDER BY quantity_sold DESC LIMIT $1
	`, limit); err != nil {
		return nil, kernel.NewDomainErrorWithCause(kernel.ErrInternal, "top sellers", err)
	}
	return rows, nil
}

func (r *PostgresAnalyticsRepository) GetProductsByCategory(ctx context.Context) ([]*domain.CategoryCount, error) {
	var rows []*domain.CategoryCount
	if err := r.db.SelectContext(ctx, &rows, `
		SELECT
			COALESCE(p.category_id, 0) AS category_id,
			COALESCE(c.name, 'Uncategorized') AS category_name,
			COUNT(*) AS product_count
		FROM products p
		LEFT JOIN categories c ON c.id = p.category_id
		GROUP BY p.category_id, c.name
		ORDER BY product_count DESC
	`); err != nil {
		return nil, kernel.NewDomainErrorWithCause(kernel.ErrInternal, "products by category", err)
	}
	return rows, nil
}

func (r *PostgresAnalyticsRepository) GetZeroOrderProductCount(ctx context.Context) (int, error) {
	var count int
	if err := r.db.GetContext(ctx, &count, `
		SELECT COUNT(*) FROM products p
		WHERE NOT EXISTS (
			SELECT 1 FROM orders o
			CROSS JOIN LATERAL jsonb_array_elements(o.items) AS item
			WHERE (item->>'product_id')::bigint = p.id
		)
	`); err != nil {
		return 0, kernel.NewDomainErrorWithCause(kernel.ErrInternal, "zero order product count", err)
	}
	return count, nil
}

func (r *PostgresAnalyticsRepository) GetInventorySummary(ctx context.Context) (*domain.InventorySummary, error) {
	var summary domain.InventorySummary
	if err := r.db.GetContext(ctx, &summary, `
		SELECT
			COUNT(*) AS total_products,
			COALESCE(SUM(quantity_available), 0) AS total_stock,
			COALESCE(COUNT(*) FILTER (WHERE quantity_available <= low_stock_threshold AND quantity_available > 0), 0) AS low_stock_count,
			COALESCE(COUNT(*) FILTER (WHERE quantity_available = 0), 0) AS out_of_stock_count
		FROM inventory
	`); err != nil {
		return nil, kernel.NewDomainErrorWithCause(kernel.ErrInternal, "inventory summary", err)
	}
	return &summary, nil
}

var _ domain.AnalyticsRepository = (*PostgresAnalyticsRepository)(nil)
