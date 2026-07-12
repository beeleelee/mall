package analytics

import (
	"context"
	"sync"
)

type fakeAnalyticsRepo struct {
	mu sync.Mutex

	dashboard        *DashboardOverview
	revenueByDay     []*DailyRevenue
	revenueByProduct []*ProductRevenue
	revenueByCat     []*CategoryRevenue
	avgOrderValue    int64
	ordersPerDay     []*DailyOrderCount
	statusBreakdown  []*StatusCount
	cancelRate       float64
	usersPerDay      []*DailyUserCount
	userStatus       []*StatusCount
	topSellers       []*ProductSales
	prodByCat        []*CategoryCount
	zeroOrderCount   int
	inventorySum     *InventorySummary
}

func newFakeAnalyticsRepo() *fakeAnalyticsRepo {
	return &fakeAnalyticsRepo{
		dashboard: &DashboardOverview{
			Revenue:           RevenueSummary{Today: 10000, ThisWeek: 75000, ThisMonth: 300000, AllTime: 5000000},
			Orders:            OrderSummary{Total: 100, Confirmed: 10, Processing: 20, Shipped: 30, Delivered: 35, Returned: 3, Cancelled: 2},
			Users:             UserSummary{Total: 50, Active: 48, Suspended: 2},
			Products:          ProductSummary{Total: 200, Active: 180},
			LowStockCount:     5,
			AverageOrderValue: 5000,
			CancellationRate:  0.02,
			RecentOrders: []RecentOrder{
				{ID: 1, UserID: 1, GrandTotal: 5000, Status: "delivered", CreatedAt: "2026-07-10T12:00:00Z"},
			},
		},
		revenueByDay:     []*DailyRevenue{{Day: "2026-07-10", Revenue: 10000, OrderCount: 2}},
		revenueByProduct: []*ProductRevenue{{ProductID: 1, Name: "Widget", Revenue: 5000, OrderCount: 1}},
		revenueByCat:     []*CategoryRevenue{{CategoryID: 1, CategoryName: "Electronics", Revenue: 10000, ProductCount: 5}},
		avgOrderValue:    5000,
		ordersPerDay:     []*DailyOrderCount{{Day: "2026-07-10", Count: 2}},
		statusBreakdown:  []*StatusCount{{Status: "delivered", Count: 35}},
		cancelRate:       0.02,
		usersPerDay:      []*DailyUserCount{{Day: "2026-07-10", Count: 3}},
		userStatus:       []*StatusCount{{Status: "active", Count: 48}},
		topSellers:       []*ProductSales{{ProductID: 1, Name: "Widget", QuantitySold: 10, Revenue: 5000, OrderCount: 1}},
		prodByCat:        []*CategoryCount{{CategoryID: 1, CategoryName: "Electronics", ProductCount: 180}},
		zeroOrderCount:   20,
		inventorySum:     &InventorySummary{TotalProducts: 200, TotalStock: 5000, LowStockCount: 5, OutOfStockCount: 2},
	}
}

func (f *fakeAnalyticsRepo) GetDashboardOverview(_ context.Context) (*DashboardOverview, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	return f.dashboard, nil
}

func (f *fakeAnalyticsRepo) GetRevenueByDay(_ context.Context, _ int) ([]*DailyRevenue, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	return f.revenueByDay, nil
}

func (f *fakeAnalyticsRepo) GetRevenueByProduct(_ context.Context, _ int) ([]*ProductRevenue, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	return f.revenueByProduct, nil
}

func (f *fakeAnalyticsRepo) GetRevenueByCategory(_ context.Context) ([]*CategoryRevenue, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	return f.revenueByCat, nil
}

func (f *fakeAnalyticsRepo) GetAverageOrderValue(_ context.Context) (int64, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	return f.avgOrderValue, nil
}

func (f *fakeAnalyticsRepo) GetOrdersPerDay(_ context.Context, _ int) ([]*DailyOrderCount, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	return f.ordersPerDay, nil
}

func (f *fakeAnalyticsRepo) GetOrderStatusBreakdown(_ context.Context) ([]*StatusCount, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	return f.statusBreakdown, nil
}

func (f *fakeAnalyticsRepo) GetCancellationRate(_ context.Context) (float64, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	return f.cancelRate, nil
}

func (f *fakeAnalyticsRepo) GetNewUsersPerDay(_ context.Context, _ int) ([]*DailyUserCount, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	return f.usersPerDay, nil
}

func (f *fakeAnalyticsRepo) GetUserStatusBreakdown(_ context.Context) ([]*StatusCount, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	return f.userStatus, nil
}

func (f *fakeAnalyticsRepo) GetTopSellers(_ context.Context, _ int) ([]*ProductSales, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	return f.topSellers, nil
}

func (f *fakeAnalyticsRepo) GetProductsByCategory(_ context.Context) ([]*CategoryCount, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	return f.prodByCat, nil
}

func (f *fakeAnalyticsRepo) GetZeroOrderProductCount(_ context.Context) (int, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	return f.zeroOrderCount, nil
}

func (f *fakeAnalyticsRepo) GetInventorySummary(_ context.Context) (*InventorySummary, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	return f.inventorySum, nil
}
