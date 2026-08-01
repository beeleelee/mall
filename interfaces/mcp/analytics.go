package mcp

import (
	"context"
	"encoding/json"

	analytics "github.com/beeleelee/mall/domain/analytics"
	identity "github.com/beeleelee/mall/domain/identity"
	"github.com/beeleelee/mall/domain/kernel"
)

type AnalyticsMCPHandler struct {
	analyticsSvc *analytics.AnalyticsService
	users        identity.UserRepository
}

func NewAnalyticsMCPHandler(
	analyticsSvc *analytics.AnalyticsService,
	users identity.UserRepository,
) *AnalyticsMCPHandler {
	return &AnalyticsMCPHandler{
		analyticsSvc: analyticsSvc,
		users:        users,
	}
}

var analyticsTools = []ToolDefinition{
	{
		Name:        "get_dashboard_overview",
		Description: "[Admin] Get the dashboard overview with revenue, order, user, product, and inventory summaries",
		InputSchema: InputSchema{
			Type: "object",
			Properties: map[string]PropertySchema{
				"admin_user_id": {Type: "number", Description: "Admin user ID"},
			},
		},
	},
	{
		Name:        "get_revenue_analytics",
		Description: "[Admin] Get revenue analytics grouped by day, product, or category",
		InputSchema: InputSchema{
			Type: "object",
			Properties: map[string]PropertySchema{
				"admin_user_id": {Type: "number", Description: "Admin user ID"},
				"group":         {Type: "string", Description: "Grouping: day, product, or category (default day)"},
				"days":          {Type: "number", Description: "Number of days to include (default 30)"},
				"limit":         {Type: "number", Description: "Max rows to return (optional)"},
			},
		},
	},
	{
		Name:        "get_order_analytics",
		Description: "[Admin] Get order analytics with status breakdown, orders per day, and cancellation rate",
		InputSchema: InputSchema{
			Type: "object",
			Properties: map[string]PropertySchema{
				"admin_user_id": {Type: "number", Description: "Admin user ID"},
				"days":          {Type: "number", Description: "Number of days to include (default 30)"},
			},
		},
	},
	{
		Name:        "get_user_analytics",
		Description: "[Admin] Get user analytics with new users per day and status breakdown",
		InputSchema: InputSchema{
			Type: "object",
			Properties: map[string]PropertySchema{
				"admin_user_id": {Type: "number", Description: "Admin user ID"},
				"days":          {Type: "number", Description: "Number of days to include (default 30)"},
			},
		},
	},
	{
		Name:        "get_product_analytics",
		Description: "[Admin] Get product analytics with top sellers, category breakdown, and inventory summary",
		InputSchema: InputSchema{
			Type: "object",
			Properties: map[string]PropertySchema{
				"admin_user_id": {Type: "number", Description: "Admin user ID"},
				"limit":         {Type: "number", Description: "Max top sellers to return (default 10)"},
			},
		},
	},
}

func (h *AnalyticsMCPHandler) ListTools() []ToolDefinition {
	return analyticsTools
}

func (h *AnalyticsMCPHandler) HandleTool(ctx context.Context, name string, raw json.RawMessage) (any, error) {
	var adminCheck struct {
		AdminUserID int64 `json:"admin_user_id"`
	}
	if err := json.Unmarshal(raw, &adminCheck); err != nil {
		return nil, kernel.NewDomainError(kernel.ErrInvalidArgument, "invalid arguments")
	}
	if err := h.checkAdmin(ctx, kernel.ID(adminCheck.AdminUserID)); err != nil {
		return nil, err
	}

	switch name {
	case "get_dashboard_overview":
		return h.callDashboard(ctx)
	case "get_revenue_analytics":
		return h.callRevenue(ctx, raw)
	case "get_order_analytics":
		return h.callOrders(ctx, raw)
	case "get_user_analytics":
		return h.callUsers(ctx, raw)
	case "get_product_analytics":
		return h.callProducts(ctx, raw)
	default:
		return nil, kernel.NewDomainError(kernel.ErrInvalidArgument, "unknown tool: "+name)
	}
}

func (h *AnalyticsMCPHandler) checkAdmin(ctx context.Context, userID kernel.ID) error {
	if userID <= 0 {
		return kernel.NewDomainError(kernel.ErrPermissionDenied, "admin_user_id must be positive")
	}
	user, err := h.users.FindByID(ctx, userID)
	if err != nil {
		return kernel.NewDomainError(kernel.ErrPermissionDenied, "admin authentication failed: user not found")
	}
	if !user.HasRole(identity.UserRoleAdmin) {
		return kernel.NewDomainError(kernel.ErrPermissionDenied, "user is not an admin")
	}
	return nil
}

func (h *AnalyticsMCPHandler) callDashboard(ctx context.Context) (any, error) {
	return h.analyticsSvc.GetDashboardOverview(ctx)
}

type revenueAnalyticsArgs struct {
	Group string `json:"group,omitempty"`
	Days  int    `json:"days,omitempty"`
	Limit int    `json:"limit,omitempty"`
}

func (h *AnalyticsMCPHandler) callRevenue(ctx context.Context, raw json.RawMessage) (any, error) {
	var args revenueAnalyticsArgs
	if err := json.Unmarshal(raw, &args); err != nil {
		return nil, kernel.NewDomainError(kernel.ErrInvalidArgument, "invalid arguments")
	}
	if args.Days <= 0 {
		args.Days = 30
	}

	switch args.Group {
	case "product":
		rows, err := h.analyticsSvc.GetRevenueByProduct(ctx, args.Limit)
		if err != nil {
			return nil, err
		}
		return rows, nil
	case "category":
		rows, err := h.analyticsSvc.GetRevenueByCategory(ctx)
		if err != nil {
			return nil, err
		}
		return rows, nil
	default:
		rows, err := h.analyticsSvc.GetRevenueByDay(ctx, args.Days)
		if err != nil {
			return nil, err
		}
		avg, _ := h.analyticsSvc.GetAverageOrderValue(ctx)
		return map[string]any{
			"daily":               rows,
			"average_order_value": avg,
		}, nil
	}
}

type orderAnalyticsArgs struct {
	Days int `json:"days,omitempty"`
}

func (h *AnalyticsMCPHandler) callOrders(ctx context.Context, raw json.RawMessage) (any, error) {
	var args orderAnalyticsArgs
	if err := json.Unmarshal(raw, &args); err != nil {
		return nil, kernel.NewDomainError(kernel.ErrInvalidArgument, "invalid arguments")
	}
	if args.Days <= 0 {
		args.Days = 30
	}

	statusBreakdown, err := h.analyticsSvc.GetOrderStatusBreakdown(ctx)
	if err != nil {
		return nil, err
	}
	ordersPerDay, err := h.analyticsSvc.GetOrdersPerDay(ctx, args.Days)
	if err != nil {
		return nil, err
	}
	cancelRate, err := h.analyticsSvc.GetCancellationRate(ctx)
	if err != nil {
		return nil, err
	}

	return map[string]any{
		"status_breakdown":  statusBreakdown,
		"orders_per_day":    ordersPerDay,
		"cancellation_rate": cancelRate,
	}, nil
}

type userAnalyticsArgs struct {
	Days int `json:"days,omitempty"`
}

func (h *AnalyticsMCPHandler) callUsers(ctx context.Context, raw json.RawMessage) (any, error) {
	var args userAnalyticsArgs
	if err := json.Unmarshal(raw, &args); err != nil {
		return nil, kernel.NewDomainError(kernel.ErrInvalidArgument, "invalid arguments")
	}
	if args.Days <= 0 {
		args.Days = 30
	}

	newUsersPerDay, err := h.analyticsSvc.GetNewUsersPerDay(ctx, args.Days)
	if err != nil {
		return nil, err
	}
	statusBreakdown, err := h.analyticsSvc.GetUserStatusBreakdown(ctx)
	if err != nil {
		return nil, err
	}

	return map[string]any{
		"new_users_per_day": newUsersPerDay,
		"status_breakdown":  statusBreakdown,
	}, nil
}

type productAnalyticsArgs struct {
	Limit int `json:"limit,omitempty"`
}

func (h *AnalyticsMCPHandler) callProducts(ctx context.Context, raw json.RawMessage) (any, error) {
	var args productAnalyticsArgs
	if err := json.Unmarshal(raw, &args); err != nil {
		return nil, kernel.NewDomainError(kernel.ErrInvalidArgument, "invalid arguments")
	}

	topSellers, err := h.analyticsSvc.GetTopSellers(ctx, args.Limit)
	if err != nil {
		return nil, err
	}
	byCategory, err := h.analyticsSvc.GetProductsByCategory(ctx)
	if err != nil {
		return nil, err
	}
	zeroOrderCount, err := h.analyticsSvc.GetZeroOrderProductCount(ctx)
	if err != nil {
		return nil, err
	}
	inventorySummary, err := h.analyticsSvc.GetInventorySummary(ctx)
	if err != nil {
		return nil, err
	}

	return map[string]any{
		"top_sellers":       topSellers,
		"by_category":       byCategory,
		"zero_order_count":  zeroOrderCount,
		"inventory_summary": inventorySummary,
	}, nil
}
