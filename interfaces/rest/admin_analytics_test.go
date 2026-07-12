package rest

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"sync"
	"testing"

	"github.com/beeleelee/mall/domain/analytics"
)

type fakeAnalyticsRepo struct {
	mu sync.Mutex
}

func newFakeAnalyticsRepo() *fakeAnalyticsRepo {
	return &fakeAnalyticsRepo{}
}

func (f *fakeAnalyticsRepo) GetDashboardOverview(_ context.Context) (*analytics.DashboardOverview, error) {
	return &analytics.DashboardOverview{
		Revenue:           analytics.RevenueSummary{Today: 10000, ThisWeek: 75000, ThisMonth: 300000, AllTime: 5000000},
		Orders:            analytics.OrderSummary{Total: 100, Confirmed: 10, Processing: 20, Shipped: 30, Delivered: 35, Returned: 3, Cancelled: 2},
		Users:             analytics.UserSummary{Total: 50, Active: 48, Suspended: 2},
		Products:          analytics.ProductSummary{Total: 200, Active: 180},
		LowStockCount:     5,
		AverageOrderValue: 5000,
		CancellationRate:  0.02,
		RecentOrders: []analytics.RecentOrder{
			{ID: 1, UserID: 1, GrandTotal: 5000, Status: "delivered", CreatedAt: "2026-07-10T12:00:00Z"},
		},
	}, nil
}

func (f *fakeAnalyticsRepo) GetRevenueByDay(_ context.Context, _ int) ([]*analytics.DailyRevenue, error) {
	return []*analytics.DailyRevenue{{Day: "2026-07-10", Revenue: 10000, OrderCount: 2}}, nil
}

func (f *fakeAnalyticsRepo) GetRevenueByProduct(_ context.Context, _ int) ([]*analytics.ProductRevenue, error) {
	return []*analytics.ProductRevenue{{ProductID: 1, Name: "Widget", Revenue: 5000, OrderCount: 1}}, nil
}

func (f *fakeAnalyticsRepo) GetRevenueByCategory(_ context.Context) ([]*analytics.CategoryRevenue, error) {
	return []*analytics.CategoryRevenue{{CategoryID: 1, CategoryName: "Electronics", Revenue: 10000, ProductCount: 5}}, nil
}

func (f *fakeAnalyticsRepo) GetAverageOrderValue(_ context.Context) (int64, error) {
	return 5000, nil
}

func (f *fakeAnalyticsRepo) GetOrdersPerDay(_ context.Context, _ int) ([]*analytics.DailyOrderCount, error) {
	return []*analytics.DailyOrderCount{{Day: "2026-07-10", Count: 2}}, nil
}

func (f *fakeAnalyticsRepo) GetOrderStatusBreakdown(_ context.Context) ([]*analytics.StatusCount, error) {
	return []*analytics.StatusCount{{Status: "delivered", Count: 35}}, nil
}

func (f *fakeAnalyticsRepo) GetCancellationRate(_ context.Context) (float64, error) {
	return 0.02, nil
}

func (f *fakeAnalyticsRepo) GetNewUsersPerDay(_ context.Context, _ int) ([]*analytics.DailyUserCount, error) {
	return []*analytics.DailyUserCount{{Day: "2026-07-10", Count: 3}}, nil
}

func (f *fakeAnalyticsRepo) GetUserStatusBreakdown(_ context.Context) ([]*analytics.StatusCount, error) {
	return []*analytics.StatusCount{{Status: "active", Count: 48}}, nil
}

func (f *fakeAnalyticsRepo) GetTopSellers(_ context.Context, _ int) ([]*analytics.ProductSales, error) {
	return []*analytics.ProductSales{{ProductID: 1, Name: "Widget", QuantitySold: 10, Revenue: 5000, OrderCount: 1}}, nil
}

func (f *fakeAnalyticsRepo) GetProductsByCategory(_ context.Context) ([]*analytics.CategoryCount, error) {
	return []*analytics.CategoryCount{{CategoryID: 1, CategoryName: "Electronics", ProductCount: 180}}, nil
}

func (f *fakeAnalyticsRepo) GetZeroOrderProductCount(_ context.Context) (int, error) {
	return 20, nil
}

func (f *fakeAnalyticsRepo) GetInventorySummary(_ context.Context) (*analytics.InventorySummary, error) {
	return &analytics.InventorySummary{TotalProducts: 200, TotalStock: 5000, LowStockCount: 5, OutOfStockCount: 2}, nil
}

func newTestAdminHandlerWithAnalytics() *AdminHandler {
	svc := analytics.NewAnalyticsService(newFakeAnalyticsRepo())
	return &AdminHandler{
		analyticsSvc: svc,
	}
}

func TestAdminHandler_Dashboard(t *testing.T) {
	h := newTestAdminHandlerWithAnalytics()
	w := httptest.NewRecorder()
	r := httptest.NewRequest(http.MethodGet, "/api/v1/admin/dashboard", nil)

	h.Dashboard(w, r)

	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", w.Code, w.Body.String())
	}

	var dash analytics.DashboardOverview
	if err := json.Unmarshal(w.Body.Bytes(), &dash); err != nil {
		t.Fatal(err)
	}
	if dash.Revenue.AllTime != 5000000 {
		t.Errorf("expected all time revenue 5000000, got %d", dash.Revenue.AllTime)
	}
	if dash.Orders.Total != 100 {
		t.Errorf("expected 100 orders, got %d", dash.Orders.Total)
	}
	if dash.Products.Total != 200 {
		t.Errorf("expected 200 products, got %d", dash.Products.Total)
	}
}

func TestAdminHandler_RevenueAnalytics(t *testing.T) {
	h := newTestAdminHandlerWithAnalytics()

	t.Run("daily", func(t *testing.T) {
		w := httptest.NewRecorder()
		r := httptest.NewRequest(http.MethodGet, "/api/v1/admin/analytics/revenue", nil)
		h.RevenueAnalytics(w, r)

		if w.Code != http.StatusOK {
			t.Fatalf("expected 200, got %d", w.Code)
		}
		var resp map[string]any
		json.Unmarshal(w.Body.Bytes(), &resp)
		if _, ok := resp["daily"]; !ok {
			t.Error("expected 'daily' key")
		}
		if _, ok := resp["average_order_value"]; !ok {
			t.Error("expected 'average_order_value' key")
		}
	})

	t.Run("by product", func(t *testing.T) {
		w := httptest.NewRecorder()
		r := httptest.NewRequest(http.MethodGet, "/api/v1/admin/analytics/revenue?group=product", nil)
		h.RevenueAnalytics(w, r)

		if w.Code != http.StatusOK {
			t.Fatalf("expected 200, got %d", w.Code)
		}
	})

	t.Run("by category", func(t *testing.T) {
		w := httptest.NewRecorder()
		r := httptest.NewRequest(http.MethodGet, "/api/v1/admin/analytics/revenue?group=category", nil)
		h.RevenueAnalytics(w, r)

		if w.Code != http.StatusOK {
			t.Fatalf("expected 200, got %d", w.Code)
		}
	})
}

func TestAdminHandler_OrderAnalytics(t *testing.T) {
	h := newTestAdminHandlerWithAnalytics()
	w := httptest.NewRecorder()
	r := httptest.NewRequest(http.MethodGet, "/api/v1/admin/analytics/orders", nil)

	h.OrderAnalytics(w, r)

	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", w.Code)
	}

	var resp map[string]any
	json.Unmarshal(w.Body.Bytes(), &resp)
	if _, ok := resp["status_breakdown"]; !ok {
		t.Error("expected 'status_breakdown' key")
	}
	if _, ok := resp["orders_per_day"]; !ok {
		t.Error("expected 'orders_per_day' key")
	}
	if _, ok := resp["cancellation_rate"]; !ok {
		t.Error("expected 'cancellation_rate' key")
	}
}

func TestAdminHandler_UserAnalytics(t *testing.T) {
	h := newTestAdminHandlerWithAnalytics()
	w := httptest.NewRecorder()
	r := httptest.NewRequest(http.MethodGet, "/api/v1/admin/analytics/users", nil)

	h.UserAnalytics(w, r)

	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", w.Code)
	}

	var resp map[string]any
	json.Unmarshal(w.Body.Bytes(), &resp)
	if _, ok := resp["new_users_per_day"]; !ok {
		t.Error("expected 'new_users_per_day' key")
	}
	if _, ok := resp["status_breakdown"]; !ok {
		t.Error("expected 'status_breakdown' key")
	}
}

func TestAdminHandler_ProductAnalytics(t *testing.T) {
	h := newTestAdminHandlerWithAnalytics()
	w := httptest.NewRecorder()
	r := httptest.NewRequest(http.MethodGet, "/api/v1/admin/analytics/products", nil)

	h.ProductAnalytics(w, r)

	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", w.Code)
	}

	var resp map[string]any
	json.Unmarshal(w.Body.Bytes(), &resp)
	if _, ok := resp["top_sellers"]; !ok {
		t.Error("expected 'top_sellers' key")
	}
	if _, ok := resp["zero_order_count"]; !ok {
		t.Error("expected 'zero_order_count' key")
	}
	if _, ok := resp["inventory_summary"]; !ok {
		t.Error("expected 'inventory_summary' key")
	}
}
