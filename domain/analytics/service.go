package analytics

import "context"

type AnalyticsService struct {
	repo AnalyticsRepository
}

func NewAnalyticsService(repo AnalyticsRepository) *AnalyticsService {
	return &AnalyticsService{repo: repo}
}

func (s *AnalyticsService) GetDashboardOverview(ctx context.Context) (*DashboardOverview, error) {
	return s.repo.GetDashboardOverview(ctx)
}

func (s *AnalyticsService) GetRevenueByDay(ctx context.Context, days int) ([]*DailyRevenue, error) {
	return s.repo.GetRevenueByDay(ctx, days)
}

func (s *AnalyticsService) GetRevenueByProduct(ctx context.Context, limit int) ([]*ProductRevenue, error) {
	return s.repo.GetRevenueByProduct(ctx, limit)
}

func (s *AnalyticsService) GetRevenueByCategory(ctx context.Context) ([]*CategoryRevenue, error) {
	return s.repo.GetRevenueByCategory(ctx)
}

func (s *AnalyticsService) GetAverageOrderValue(ctx context.Context) (int64, error) {
	return s.repo.GetAverageOrderValue(ctx)
}

func (s *AnalyticsService) GetOrdersPerDay(ctx context.Context, days int) ([]*DailyOrderCount, error) {
	return s.repo.GetOrdersPerDay(ctx, days)
}

func (s *AnalyticsService) GetOrderStatusBreakdown(ctx context.Context) ([]*StatusCount, error) {
	return s.repo.GetOrderStatusBreakdown(ctx)
}

func (s *AnalyticsService) GetCancellationRate(ctx context.Context) (float64, error) {
	return s.repo.GetCancellationRate(ctx)
}

func (s *AnalyticsService) GetNewUsersPerDay(ctx context.Context, days int) ([]*DailyUserCount, error) {
	return s.repo.GetNewUsersPerDay(ctx, days)
}

func (s *AnalyticsService) GetUserStatusBreakdown(ctx context.Context) ([]*StatusCount, error) {
	return s.repo.GetUserStatusBreakdown(ctx)
}

func (s *AnalyticsService) GetTopSellers(ctx context.Context, limit int) ([]*ProductSales, error) {
	return s.repo.GetTopSellers(ctx, limit)
}

func (s *AnalyticsService) GetProductsByCategory(ctx context.Context) ([]*CategoryCount, error) {
	return s.repo.GetProductsByCategory(ctx)
}

func (s *AnalyticsService) GetZeroOrderProductCount(ctx context.Context) (int, error) {
	return s.repo.GetZeroOrderProductCount(ctx)
}

func (s *AnalyticsService) GetInventorySummary(ctx context.Context) (*InventorySummary, error) {
	return s.repo.GetInventorySummary(ctx)
}
