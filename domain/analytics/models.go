package analytics

type DashboardOverview struct {
	Revenue           RevenueSummary `json:"revenue"`
	Orders            OrderSummary   `json:"orders"`
	Users             UserSummary    `json:"users"`
	Products          ProductSummary `json:"products"`
	LowStockCount     int            `json:"low_stock_count" db:"low_stock_count"`
	AverageOrderValue int64          `json:"average_order_value" db:"average_order_value"`
	CancellationRate  float64        `json:"cancellation_rate" db:"cancellation_rate"`
	RecentOrders      []RecentOrder  `json:"recent_orders"`
}

type RevenueSummary struct {
	Today     int64 `json:"today" db:"today"`
	ThisWeek  int64 `json:"this_week" db:"this_week"`
	ThisMonth int64 `json:"this_month" db:"this_month"`
	AllTime   int64 `json:"all_time" db:"all_time"`
}

type OrderSummary struct {
	Total      int `json:"total" db:"total"`
	Confirmed  int `json:"confirmed" db:"confirmed"`
	Processing int `json:"processing" db:"processing"`
	Shipped    int `json:"shipped" db:"shipped"`
	Delivered  int `json:"delivered" db:"delivered"`
	Returned   int `json:"returned" db:"returned"`
	Cancelled  int `json:"cancelled" db:"cancelled"`
}

type UserSummary struct {
	Total     int `json:"total" db:"total"`
	Active    int `json:"active" db:"active"`
	Suspended int `json:"suspended" db:"suspended"`
}

type ProductSummary struct {
	Total  int `json:"total" db:"total"`
	Active int `json:"active" db:"active"`
}

type RecentOrder struct {
	ID         int64  `json:"id" db:"id"`
	UserID     int64  `json:"user_id" db:"user_id"`
	GrandTotal int64  `json:"grand_total" db:"grand_total"`
	Status     string `json:"status" db:"status"`
	CreatedAt  string `json:"created_at" db:"created_at"`
}

type DailyRevenue struct {
	Day        string `json:"day" db:"day"`
	Revenue    int64  `json:"revenue" db:"revenue"`
	OrderCount int    `json:"order_count" db:"order_count"`
}

type ProductRevenue struct {
	ProductID  int64  `json:"product_id" db:"product_id"`
	Name       string `json:"name" db:"name"`
	Revenue    int64  `json:"revenue" db:"revenue"`
	OrderCount int    `json:"order_count" db:"order_count"`
}

type CategoryRevenue struct {
	CategoryID   int64  `json:"category_id" db:"category_id"`
	CategoryName string `json:"category_name" db:"category_name"`
	Revenue      int64  `json:"revenue" db:"revenue"`
	ProductCount int    `json:"product_count" db:"product_count"`
}

type DailyOrderCount struct {
	Day   string `json:"day" db:"day"`
	Count int    `json:"count" db:"count"`
}

type StatusCount struct {
	Status string `json:"status" db:"status"`
	Count  int    `json:"count" db:"count"`
}

type DailyUserCount struct {
	Day   string `json:"day" db:"day"`
	Count int    `json:"count" db:"count"`
}

type ProductSales struct {
	ProductID    int64  `json:"product_id" db:"product_id"`
	Name         string `json:"name" db:"name"`
	QuantitySold int    `json:"quantity_sold" db:"quantity_sold"`
	Revenue      int64  `json:"revenue" db:"revenue"`
	OrderCount   int    `json:"order_count" db:"order_count"`
}

type CategoryCount struct {
	CategoryID   int64  `json:"category_id" db:"category_id"`
	CategoryName string `json:"category_name" db:"category_name"`
	ProductCount int    `json:"product_count" db:"product_count"`
}

type InventorySummary struct {
	TotalProducts   int `json:"total_products" db:"total_products"`
	TotalStock      int `json:"total_stock" db:"total_stock"`
	LowStockCount   int `json:"low_stock_count" db:"low_stock_count"`
	OutOfStockCount int `json:"out_of_stock_count" db:"out_of_stock_count"`
}
