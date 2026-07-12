package analytics

import (
	"context"
	"fmt"
	"math/rand"
	"net"
	"testing"
	"time"

	_ "github.com/jackc/pgx/v5/stdlib"
	"github.com/jmoiron/sqlx"
)

type fixture struct {
	repo   *PostgresAnalyticsRepository
	db     *sqlx.DB
	schema string
	cleanup func()
}

func servicesUp() bool {
	pg, err := net.DialTimeout("tcp", "localhost:5432", 3*time.Second)
	if err != nil {
		return false
	}
	pg.Close()
	return true
}

func newFixture(t *testing.T) *fixture {
	t.Helper()
	if !servicesUp() {
		t.Skip("integration: need 'docker compose up postgres' running")
	}

	dsn := "postgres://mall:mall_dev@localhost:5432/mall?sslmode=disable"
	db, err := sqlx.Connect("pgx", dsn)
	if err != nil {
		t.Fatalf("connect postgres: %v", err)
	}
	db.SetMaxOpenConns(2)

	schema := fmt.Sprintf("test_%08x", rand.Int63())[:16]
	if _, err := db.Exec(fmt.Sprintf(`CREATE SCHEMA IF NOT EXISTS "%s"`, schema)); err != nil {
		db.Close()
		t.Fatalf("create schema: %v", err)
	}
	if _, err := db.Exec(fmt.Sprintf(`SET search_path TO "%s", public`, schema)); err != nil {
		db.Close()
		t.Fatalf("set search_path: %v", err)
	}

	schemaSQL := `
	CREATE TABLE users (
		id BIGINT PRIMARY KEY,
		email VARCHAR(255) NOT NULL UNIQUE,
		name VARCHAR(500) NOT NULL,
		password_hash TEXT NOT NULL,
		status VARCHAR(50) NOT NULL DEFAULT 'active',
		roles JSONB NOT NULL DEFAULT '["customer"]',
		created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
		updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
	);
	CREATE TABLE products (
		id BIGINT PRIMARY KEY,
		sku VARCHAR(255) NOT NULL UNIQUE,
		name VARCHAR(500) NOT NULL,
		description TEXT NOT NULL DEFAULT '',
		category VARCHAR(255) NOT NULL DEFAULT '',
		category_id BIGINT,
		price_amount BIGINT NOT NULL,
		price_currency VARCHAR(3) NOT NULL DEFAULT 'USD',
		status VARCHAR(50) NOT NULL DEFAULT 'active',
		attributes JSONB NOT NULL DEFAULT '{}',
		created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
		updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
	);
	CREATE TABLE categories (
		id BIGINT PRIMARY KEY,
		name VARCHAR(255) NOT NULL,
		slug VARCHAR(255) NOT NULL UNIQUE,
		parent_id BIGINT,
		sort_order INT NOT NULL DEFAULT 0,
		created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
		updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
	);
	CREATE TABLE orders (
		id BIGINT PRIMARY KEY,
		user_id BIGINT NOT NULL,
		checkout_id BIGINT NOT NULL,
		cart_id BIGINT NOT NULL,
		items JSONB NOT NULL DEFAULT '[]',
		shipping_address JSONB NOT NULL DEFAULT '{}',
		billing_address JSONB NOT NULL DEFAULT '{}',
		shipping_option JSONB NOT NULL DEFAULT '{}',
		payment_handler VARCHAR(255) NOT NULL DEFAULT '',
		subtotal BIGINT NOT NULL DEFAULT 0,
		shipping_cost BIGINT NOT NULL DEFAULT 0,
		tax_amount BIGINT NOT NULL DEFAULT 0,
		grand_total BIGINT NOT NULL DEFAULT 0,
		status VARCHAR(50) NOT NULL DEFAULT 'confirmed',
		tracking_number VARCHAR(255) NOT NULL DEFAULT '',
		carrier VARCHAR(255) NOT NULL DEFAULT '',
		confirmed_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
		processing_at TIMESTAMPTZ,
		shipped_at TIMESTAMPTZ,
		delivered_at TIMESTAMPTZ,
		returned_at TIMESTAMPTZ,
		cancelled_at TIMESTAMPTZ,
		created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
		updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
	);
	CREATE TABLE inventory (
		id BIGINT PRIMARY KEY,
		product_id BIGINT NOT NULL UNIQUE,
		quantity_available INT NOT NULL DEFAULT 0,
		reserved_quantity INT NOT NULL DEFAULT 0,
		low_stock_threshold INT NOT NULL DEFAULT 10,
		created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
		updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
	);
	`
	if _, err := db.Exec(schemaSQL); err != nil {
		db.Close()
		t.Fatalf("create tables: %v", err)
	}

	cleanup := func() {
		db.Exec(fmt.Sprintf(`DROP SCHEMA "%s" CASCADE`, schema))
		db.Close()
	}

	repo := NewPostgresAnalyticsRepository(db)
	return &fixture{
		repo:    repo,
		db:      db,
		schema:  schema,
		cleanup: cleanup,
	}
}

func seedTestData(t *testing.T, f *fixture, now time.Time) {
	t.Helper()

	_, err := f.db.Exec(`
		INSERT INTO users (id, email, name, password_hash, status, created_at) VALUES
		(1, 'a@b.com', 'Alice', 'hash', 'active', $1),
		(2, 'b@b.com', 'Bob', 'hash', 'active', $1),
		(3, 'c@b.com', 'Charlie', 'hash', 'suspended', $1)
	`, now)
	if err != nil {
		t.Fatalf("seed users: %v", err)
	}

	_, err = f.db.Exec(`
		INSERT INTO categories (id, name, slug) VALUES
		(1, 'Electronics', 'electronics'),
		(2, 'Books', 'books')
	`)
	if err != nil {
		t.Fatalf("seed categories: %v", err)
	}

	_, err = f.db.Exec(`
		INSERT INTO products (id, sku, name, category, category_id, price_amount, status) VALUES
		(1, 'WIDGET', 'Widget', 'Electronics', 1, 5000, 'active'),
		(2, 'GADGET', 'Gadget', 'Electronics', 1, 3000, 'active'),
		(3, 'BOOK', 'Go Book', 'Books', 2, 2000, 'active')
	`)
	if err != nil {
		t.Fatalf("seed products: %v", err)
	}

	_, err = f.db.Exec(`
		INSERT INTO orders (id, user_id, checkout_id, cart_id, items, grand_total, status, created_at) VALUES
		(1, 1, 1, 1, '[{"product_id": 1, "name": "Widget", "quantity": 2, "unit_price": 5000, "total_price": 10000}]', 10000, 'delivered', $1),
		(2, 2, 2, 2, '[{"product_id": 2, "name": "Gadget", "quantity": 1, "unit_price": 3000, "total_price": 3000}]', 3000, 'delivered', $1),
		(3, 1, 3, 3, '[{"product_id": 1, "name": "Widget", "quantity": 1, "unit_price": 5000, "total_price": 5000}]', 5000, 'cancelled', $1)
	`, now)
	if err != nil {
		t.Fatalf("seed orders: %v", err)
	}

	_, err = f.db.Exec(`
		INSERT INTO inventory (id, product_id, quantity_available, low_stock_threshold) VALUES
		(1, 1, 50, 5),
		(2, 2, 2, 10),
		(3, 3, 0, 5)
	`)
	if err != nil {
		t.Fatalf("seed inventory: %v", err)
	}
}

func now() time.Time {
	return time.Date(2026, 7, 12, 10, 0, 0, 0, time.UTC)
}

func TestPostgresAnalyticsRepository_Dashboard(t *testing.T) {
	f := newFixture(t)
	defer f.cleanup()
	seedTestData(t, f, now())

	dash, err := f.repo.GetDashboardOverview(context.Background())
	if err != nil {
		t.Fatal(err)
	}

	if dash.Revenue.Today <= 0 {
		t.Errorf("expected revenue today > 0, got %d", dash.Revenue.Today)
	}
	if dash.Orders.Total != 3 {
		t.Errorf("expected 3 orders, got %d", dash.Orders.Total)
	}
	if dash.Users.Total != 3 {
		t.Errorf("expected 3 users, got %d", dash.Users.Total)
	}
	if dash.Products.Total != 3 {
		t.Errorf("expected 3 products, got %d", dash.Products.Total)
	}
	if dash.Products.Active != 3 {
		t.Errorf("expected 3 active products, got %d", dash.Products.Active)
	}
	if dash.LowStockCount != 2 {
		t.Errorf("expected 2 low stock, got %d", dash.LowStockCount)
	}
	if dash.AverageOrderValue <= 0 {
		t.Errorf("expected average > 0, got %d", dash.AverageOrderValue)
	}
	if dash.CancellationRate <= 0 {
		t.Errorf("expected cancellation rate > 0, got %f", dash.CancellationRate)
	}
	if len(dash.RecentOrders) != 3 {
		t.Errorf("expected 3 recent orders, got %d", len(dash.RecentOrders))
	}
}

func TestPostgresAnalyticsRepository_RevenueByDay(t *testing.T) {
	f := newFixture(t)
	defer f.cleanup()
	seedTestData(t, f, now())

	rows, err := f.repo.GetRevenueByDay(context.Background(), 30)
	if err != nil {
		t.Fatal(err)
	}
	if len(rows) == 0 {
		t.Fatal("expected at least 1 row")
	}
	if rows[0].OrderCount == 0 {
		t.Errorf("expected order count > 0")
	}
}

func TestPostgresAnalyticsRepository_RevenueByProduct(t *testing.T) {
	f := newFixture(t)
	defer f.cleanup()
	seedTestData(t, f, now())

	rows, err := f.repo.GetRevenueByProduct(context.Background(), 10)
	if err != nil {
		t.Fatal(err)
	}
	if len(rows) == 0 {
		t.Fatal("expected at least 1 product")
	}
	if rows[0].Name != "Widget" {
		t.Errorf("expected Widget, got %s", rows[0].Name)
	}
}

func TestPostgresAnalyticsRepository_OrderStatusBreakdown(t *testing.T) {
	f := newFixture(t)
	defer f.cleanup()
	seedTestData(t, f, now())

	rows, err := f.repo.GetOrderStatusBreakdown(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	found := false
	for _, r := range rows {
		if r.Status == "delivered" {
			found = true
			if r.Count != 2 {
				t.Errorf("expected 2 delivered, got %d", r.Count)
			}
		}
	}
	if !found {
		t.Error("expected 'delivered' in status breakdown")
	}
}

func TestPostgresAnalyticsRepository_NewUsersPerDay(t *testing.T) {
	f := newFixture(t)
	defer f.cleanup()
	seedTestData(t, f, now())

	rows, err := f.repo.GetNewUsersPerDay(context.Background(), 30)
	if err != nil {
		t.Fatal(err)
	}
	if len(rows) == 0 {
		t.Fatal("expected at least 1 row")
	}
	if rows[0].Count != 3 {
		t.Errorf("expected 3 new users, got %d", rows[0].Count)
	}
}

func TestPostgresAnalyticsRepository_TopSellers(t *testing.T) {
	f := newFixture(t)
	defer f.cleanup()
	seedTestData(t, f, now())

	rows, err := f.repo.GetTopSellers(context.Background(), 10)
	if err != nil {
		t.Fatal(err)
	}
	if len(rows) == 0 {
		t.Fatal("expected at least 1 seller")
	}
	if rows[0].QuantitySold != 2 {
		t.Errorf("expected 2 sold for top seller, got %d", rows[0].QuantitySold)
	}
}

func TestPostgresAnalyticsRepository_RevenueByCategory(t *testing.T) {
	f := newFixture(t)
	defer f.cleanup()
	seedTestData(t, f, now())

	rows, err := f.repo.GetRevenueByCategory(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	if len(rows) == 0 {
		t.Fatal("expected at least 1 category")
	}
}

func TestPostgresAnalyticsRepository_InventorySummary(t *testing.T) {
	f := newFixture(t)
	defer f.cleanup()
	seedTestData(t, f, now())

	summary, err := f.repo.GetInventorySummary(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	if summary.TotalProducts != 3 {
		t.Errorf("expected 3 products, got %d", summary.TotalProducts)
	}
	if summary.OutOfStockCount != 1 {
		t.Errorf("expected 1 out of stock, got %d", summary.OutOfStockCount)
	}
}

func TestPostgresAnalyticsRepository_ProductsByCategory(t *testing.T) {
	f := newFixture(t)
	defer f.cleanup()
	seedTestData(t, f, now())

	rows, err := f.repo.GetProductsByCategory(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	if len(rows) == 0 {
		t.Fatal("expected at least 1 category")
	}
}

func TestPostgresAnalyticsRepository_ZeroOrderProductCount(t *testing.T) {
	f := newFixture(t)
	defer f.cleanup()
	seedTestData(t, f, now())

	count, err := f.repo.GetZeroOrderProductCount(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	// Product 3 (Go Book) has no orders
	if count != 1 {
		t.Errorf("expected 1 product with zero orders, got %d", count)
	}
}

func TestPostgresAnalyticsRepository_UserStatusBreakdown(t *testing.T) {
	f := newFixture(t)
	defer f.cleanup()
	seedTestData(t, f, now())

	rows, err := f.repo.GetUserStatusBreakdown(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	activeFound := false
	for _, r := range rows {
		if r.Status == "active" && r.Count == 2 {
			activeFound = true
		}
	}
	if !activeFound {
		t.Errorf("expected 2 active users in breakdown, got %v", rows)
	}
}
