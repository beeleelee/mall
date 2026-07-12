package analytics

import (
	"context"
	"testing"
)

func newTestService() *AnalyticsService {
	return NewAnalyticsService(newFakeAnalyticsRepo())
}

func TestAnalyticsService_DashboardOverview(t *testing.T) {
	svc := newTestService()
	dash, err := svc.GetDashboardOverview(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	if dash.Revenue.AllTime != 5000000 {
		t.Errorf("expected all time revenue 5000000, got %d", dash.Revenue.AllTime)
	}
	if dash.Orders.Total != 100 {
		t.Errorf("expected 100 orders, got %d", dash.Orders.Total)
	}
	if dash.Users.Total != 50 {
		t.Errorf("expected 50 users, got %d", dash.Users.Total)
	}
	if dash.Products.Total != 200 {
		t.Errorf("expected 200 products, got %d", dash.Products.Total)
	}
	if dash.LowStockCount != 5 {
		t.Errorf("expected 5 low stock, got %d", dash.LowStockCount)
	}
	if len(dash.RecentOrders) != 1 {
		t.Errorf("expected 1 recent order, got %d", len(dash.RecentOrders))
	}
}

func TestAnalyticsService_RevenueByDay(t *testing.T) {
	svc := newTestService()
	rows, err := svc.GetRevenueByDay(context.Background(), 30)
	if err != nil {
		t.Fatal(err)
	}
	if len(rows) != 1 {
		t.Fatalf("expected 1 row, got %d", len(rows))
	}
	if rows[0].Revenue != 10000 {
		t.Errorf("expected revenue 10000, got %d", rows[0].Revenue)
	}
}

func TestAnalyticsService_RevenueByProduct(t *testing.T) {
	svc := newTestService()
	rows, err := svc.GetRevenueByProduct(context.Background(), 10)
	if err != nil {
		t.Fatal(err)
	}
	if len(rows) != 1 {
		t.Fatalf("expected 1 row, got %d", len(rows))
	}
	if rows[0].Name != "Widget" {
		t.Errorf("expected Widget, got %s", rows[0].Name)
	}
}

func TestAnalyticsService_OrderStatusBreakdown(t *testing.T) {
	svc := newTestService()
	rows, err := svc.GetOrderStatusBreakdown(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	if len(rows) != 1 {
		t.Fatalf("expected 1 row, got %d", len(rows))
	}
	if rows[0].Status != "delivered" {
		t.Errorf("expected delivered, got %s", rows[0].Status)
	}
}

func TestAnalyticsService_TopSellers(t *testing.T) {
	svc := newTestService()
	rows, err := svc.GetTopSellers(context.Background(), 10)
	if err != nil {
		t.Fatal(err)
	}
	if len(rows) != 1 {
		t.Fatalf("expected 1 row, got %d", len(rows))
	}
	if rows[0].QuantitySold != 10 {
		t.Errorf("expected 10 sold, got %d", rows[0].QuantitySold)
	}
}

func TestAnalyticsService_InventorySummary(t *testing.T) {
	svc := newTestService()
	summary, err := svc.GetInventorySummary(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	if summary.TotalProducts != 200 {
		t.Errorf("expected 200 total products, got %d", summary.TotalProducts)
	}
	if summary.OutOfStockCount != 2 {
		t.Errorf("expected 2 out of stock, got %d", summary.OutOfStockCount)
	}
}

func TestAnalyticsService_ZeroOrderProductCount(t *testing.T) {
	svc := newTestService()
	count, err := svc.GetZeroOrderProductCount(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	if count != 20 {
		t.Errorf("expected 20, got %d", count)
	}
}
