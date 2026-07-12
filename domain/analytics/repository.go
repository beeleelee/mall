package analytics

import "context"

type AnalyticsRepository interface {
	GetDashboardOverview(ctx context.Context) (*DashboardOverview, error)
	GetRevenueByDay(ctx context.Context, days int) ([]*DailyRevenue, error)
	GetRevenueByProduct(ctx context.Context, limit int) ([]*ProductRevenue, error)
	GetRevenueByCategory(ctx context.Context) ([]*CategoryRevenue, error)
	GetAverageOrderValue(ctx context.Context) (int64, error)
	GetOrdersPerDay(ctx context.Context, days int) ([]*DailyOrderCount, error)
	GetOrderStatusBreakdown(ctx context.Context) ([]*StatusCount, error)
	GetCancellationRate(ctx context.Context) (float64, error)
	GetNewUsersPerDay(ctx context.Context, days int) ([]*DailyUserCount, error)
	GetUserStatusBreakdown(ctx context.Context) ([]*StatusCount, error)
	GetTopSellers(ctx context.Context, limit int) ([]*ProductSales, error)
	GetProductsByCategory(ctx context.Context) ([]*CategoryCount, error)
	GetZeroOrderProductCount(ctx context.Context) (int, error)
	GetInventorySummary(ctx context.Context) (*InventorySummary, error)
}
