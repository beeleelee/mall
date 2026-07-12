package analytics

type DashboardOverview struct {
	Revenue           RevenueSummary `json:"revenue"`
	Orders            OrderSummary   `json:"orders"`
	Users             UserSummary    `json:"users"`
	Products          ProductSummary `json:"products"`
	LowStockCount     int            `json:"low_stock_count"`
	AverageOrderValue int64          `json:"average_order_value"`
	CancellationRate  float64        `json:"cancellation_rate"`
	RecentOrders      []RecentOrder  `json:"recent_orders"`
}

type RevenueSummary struct {
	Today     int64 `json:"today"`
	ThisWeek  int64 `json:"this_week"`
	ThisMonth int64 `json:"this_month"`
	AllTime   int64 `json:"all_time"`
}

type OrderSummary struct {
	Total      int `json:"total"`
	Confirmed  int `json:"confirmed"`
	Processing int `json:"processing"`
	Shipped    int `json:"shipped"`
	Delivered  int `json:"delivered"`
	Returned   int `json:"returned"`
	Cancelled  int `json:"cancelled"`
}

type UserSummary struct {
	Total     int `json:"total"`
	Active    int `json:"active"`
	Suspended int `json:"suspended"`
}

type ProductSummary struct {
	Total  int `json:"total"`
	Active int `json:"active"`
}

type RecentOrder struct {
	ID         int64  `json:"id"`
	UserID     int64  `json:"user_id"`
	GrandTotal int64  `json:"grand_total"`
	Status     string `json:"status"`
	CreatedAt  string `json:"created_at"`
}

type DailyRevenue struct {
	Day        string `json:"day"`
	Revenue    int64  `json:"revenue"`
	OrderCount int    `json:"order_count"`
}

type ProductRevenue struct {
	ProductID  int64  `json:"product_id"`
	Name       string `json:"name"`
	Revenue    int64  `json:"revenue"`
	OrderCount int    `json:"order_count"`
}

type CategoryRevenue struct {
	CategoryID   int64  `json:"category_id"`
	CategoryName string `json:"category_name"`
	Revenue      int64  `json:"revenue"`
	ProductCount int    `json:"product_count"`
}

type DailyOrderCount struct {
	Day   string `json:"day"`
	Count int    `json:"count"`
}

type StatusCount struct {
	Status string `json:"status"`
	Count  int    `json:"count"`
}

type DailyUserCount struct {
	Day   string `json:"day"`
	Count int    `json:"count"`
}

type ProductSales struct {
	ProductID    int64  `json:"product_id"`
	Name         string `json:"name"`
	QuantitySold int    `json:"quantity_sold"`
	Revenue      int64  `json:"revenue"`
	OrderCount   int    `json:"order_count"`
}

type CategoryCount struct {
	CategoryID   int64  `json:"category_id"`
	CategoryName string `json:"category_name"`
	ProductCount int    `json:"product_count"`
}

type InventorySummary struct {
	TotalProducts   int `json:"total_products"`
	TotalStock      int `json:"total_stock"`
	LowStockCount   int `json:"low_stock_count"`
	OutOfStockCount int `json:"out_of_stock_count"`
}
