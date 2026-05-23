package catalog

import (
	"context"
	"fmt"
	"math/rand"
	"net"
	"testing"
	"time"

	_ "github.com/jackc/pgx/v5/stdlib"
	"github.com/jmoiron/sqlx"
	"github.com/redis/go-redis/v9"

	domain "github.com/beeleelee/mall/domain/catalog"
	"github.com/beeleelee/mall/domain/kernel"
	"github.com/beeleelee/mall/infrastructure/database"
)

type integrationFixture struct {
	repo    *PostgresProductRepository
	db      *sqlx.DB
	rdb     *redis.Client
	schema  string
	cleanup func()
}

func newIntegrationFixture(t *testing.T) *integrationFixture {
	t.Helper()

	if !servicesUp() {
		t.Skip("integration: need 'docker compose up postgres redis' running")
	}

	dsn := "postgres://mall:mall_dev@localhost:5432/mall?sslmode=disable"

	db, err := sqlx.Connect("pgx", dsn)
	if err != nil {
		t.Fatalf("connect postgres: %v", err)
	}
	db.SetMaxOpenConns(4)

	schema := fmt.Sprintf("test_%08x", rand.Int63())[:16]
	if _, err := db.Exec(fmt.Sprintf(`CREATE SCHEMA IF NOT EXISTS "%s"`, schema)); err != nil {
		db.Close()
		t.Fatalf("create schema: %v", err)
	}
	if _, err := db.Exec(fmt.Sprintf(`SET search_path TO "%s", public`, schema)); err != nil {
		db.Close()
		t.Fatalf("set search_path: %v", err)
	}

	migrator := database.NewMigrator(db)
	if err := migrator.Up(); err != nil {
		db.Exec(fmt.Sprintf(`DROP SCHEMA IF EXISTS "%s" CASCADE`, schema))
		db.Close()
		t.Fatalf("migrate: %v", err)
	}

	rdb := redis.NewClient(&redis.Options{
		Addr: "localhost:6379",
		DB:   1,
	})
	if err := rdb.Ping(context.Background()).Err(); err != nil {
		db.Exec(fmt.Sprintf(`DROP SCHEMA IF EXISTS "%s" CASCADE`, schema))
		db.Close()
		rdb.Close()
		t.Fatalf("connect redis: %v", err)
	}
	rdb.FlushDB(context.Background())

	repo := NewPostgresProductRepository(db, rdb)

	return &integrationFixture{
		repo:   repo,
		db:     db,
		rdb:    rdb,
		schema: schema,
		cleanup: func() {
			db.Exec(fmt.Sprintf(`DROP SCHEMA IF EXISTS "%s" CASCADE`, schema))
			rdb.FlushDB(context.Background())
			db.Close()
			rdb.Close()
		},
	}
}

func servicesUp() bool {
	pg, err := net.DialTimeout("tcp", "localhost:5432", 2*time.Second)
	if err != nil {
		return false
	}
	pg.Close()

	rd, err := net.DialTimeout("tcp", "localhost:6379", 2*time.Second)
	if err != nil {
		return false
	}
	rd.Close()

	return true
}

func seedProducts(t *testing.T, repo *PostgresProductRepository, n int) {
	t.Helper()
	ctx := context.Background()
	for i := int64(1); i <= int64(n); i++ {
		p, err := domain.NewProduct(
			kernel.ID(i),
			domain.SKU(fmt.Sprintf("SKU-%03d", i)),
			fmt.Sprintf("Product %d", i),
			fmt.Sprintf("Description for product %d", i),
			"category-a",
			domain.Money{Amount: i * 1000, Currency: "USD"},
			nil,
		)
		if err != nil {
			t.Fatalf("new product %d: %v", i, err)
		}
		if err := repo.Save(ctx, p); err != nil {
			t.Fatalf("save product %d: %v", i, err)
		}
	}
}

var ctx = context.Background()

func TestIntegration_SaveAndFindByID(t *testing.T) {
	f := newIntegrationFixture(t)
	defer f.cleanup()

	p, _ := domain.NewProduct(1, "INT-SKU-001", "Integration T-Shirt", "A test tee", "clothing", domain.Money{Amount: 1999, Currency: "USD"}, nil)
	if err := f.repo.Save(ctx, p); err != nil {
		t.Fatalf("save: %v", err)
	}

	got, err := f.repo.FindByID(ctx, 1)
	if err != nil {
		t.Fatalf("find by id: %v", err)
	}
	if got.Name != "Integration T-Shirt" {
		t.Errorf("name: expected %q, got %q", "Integration T-Shirt", got.Name)
	}
	if got.SKU != "INT-SKU-001" {
		t.Errorf("sku: expected %q, got %q", "INT-SKU-001", got.SKU)
	}
	if got.Price.Amount != 1999 {
		t.Errorf("price: expected %d, got %d", 1999, got.Price.Amount)
	}
}

func TestIntegration_SaveAndFindBySKU(t *testing.T) {
	f := newIntegrationFixture(t)
	defer f.cleanup()

	p, _ := domain.NewProduct(1, "INT-SKU-002", "SKU Lookup", "desc", "cat", domain.Money{Amount: 999, Currency: "USD"}, nil)
	f.repo.Save(ctx, p)

	got, err := f.repo.FindBySKU(ctx, "INT-SKU-002")
	if err != nil {
		t.Fatalf("find by sku: %v", err)
	}
	if got.Name != "SKU Lookup" {
		t.Errorf("name: expected %q, got %q", "SKU Lookup", got.Name)
	}
}

func TestIntegration_FindByID_NotFound(t *testing.T) {
	f := newIntegrationFixture(t)
	defer f.cleanup()

	_, err := f.repo.FindByID(ctx, 99999)
	if !kernel.IsNotFound(err) {
		t.Errorf("expected NotFound, got %v", err)
	}
}

func TestIntegration_FindBySKU_NotFound(t *testing.T) {
	f := newIntegrationFixture(t)
	defer f.cleanup()

	_, err := f.repo.FindBySKU(ctx, "NONEXISTENT")
	if !kernel.IsNotFound(err) {
		t.Errorf("expected NotFound, got %v", err)
	}
}

func TestIntegration_SearchByName(t *testing.T) {
	f := newIntegrationFixture(t)
	defer f.cleanup()
	seedProducts(t, f.repo, 5)

	result, err := f.repo.Search(ctx, "Product 3", domain.SearchOptions{})
	if err != nil {
		t.Fatalf("search: %v", err)
	}
	if len(result.Products) != 1 {
		t.Fatalf("expected 1 result, got %d", len(result.Products))
	}
	if result.Products[0].SKU != "SKU-003" {
		t.Errorf("expected SKU-003, got %s", result.Products[0].SKU)
	}
}

func TestIntegration_SearchByCategory(t *testing.T) {
	f := newIntegrationFixture(t)
	defer f.cleanup()

	p1, _ := domain.NewProduct(1, "CAT-A", "A", "desc", "electronics", domain.Money{Amount: 100, Currency: "USD"}, nil)
	p2, _ := domain.NewProduct(2, "CAT-B", "B", "desc", "clothing", domain.Money{Amount: 200, Currency: "USD"}, nil)
	f.repo.Save(ctx, p1)
	f.repo.Save(ctx, p2)

	result, err := f.repo.Search(ctx, "", domain.SearchOptions{Category: "electronics"})
	if err != nil {
		t.Fatalf("search: %v", err)
	}
	if len(result.Products) != 1 {
		t.Fatalf("expected 1 result, got %d", len(result.Products))
	}
	if result.Products[0].SKU != "CAT-A" {
		t.Errorf("expected CAT-A, got %s", result.Products[0].SKU)
	}
}

func TestIntegration_SearchByPriceRange(t *testing.T) {
	f := newIntegrationFixture(t)
	defer f.cleanup()
	seedProducts(t, f.repo, 10)

	result, err := f.repo.Search(ctx, "", domain.SearchOptions{MinPrice: 5000, MaxPrice: 8000})
	if err != nil {
		t.Fatalf("search: %v", err)
	}
	if len(result.Products) != 4 {
		t.Fatalf("expected 4 results (IDs 5-8), got %d", len(result.Products))
	}
}

func TestIntegration_SearchWithStatusFilter(t *testing.T) {
	f := newIntegrationFixture(t)
	defer f.cleanup()

	p1, _ := domain.NewProduct(1, "STA-A", "Active", "desc", "cat", domain.Money{Amount: 100, Currency: "USD"}, nil)
	p2, _ := domain.NewProduct(2, "STA-B", "Inactive", "desc", "cat", domain.Money{Amount: 200, Currency: "USD"}, nil)
	p2.ChangeStatus(domain.ProductStatusInactive)
	p3, _ := domain.NewProduct(3, "STA-C", "Discontinued", "desc", "cat", domain.Money{Amount: 300, Currency: "USD"}, nil)
	p3.ChangeStatus(domain.ProductStatusDiscontinued)
	f.repo.Save(ctx, p1)
	f.repo.Save(ctx, p2)
	f.repo.Save(ctx, p3)

	result, err := f.repo.Search(ctx, "", domain.SearchOptions{Status: domain.ProductStatusDiscontinued})
	if err != nil {
		t.Fatalf("search: %v", err)
	}
	if len(result.Products) != 1 {
		t.Fatalf("expected 1 result, got %d", len(result.Products))
	}
	if result.Products[0].SKU != "STA-C" {
		t.Errorf("expected STA-C, got %s", result.Products[0].SKU)
	}
}

func TestIntegration_SearchPagination(t *testing.T) {
	f := newIntegrationFixture(t)
	defer f.cleanup()
	seedProducts(t, f.repo, 25)

	page1, err := f.repo.Search(ctx, "", domain.SearchOptions{Limit: 10})
	if err != nil {
		t.Fatalf("page 1: %v", err)
	}
	if len(page1.Products) != 10 {
		t.Fatalf("page 1: expected 10, got %d", len(page1.Products))
	}
	if !page1.HasMore {
		t.Fatal("page 1: expected HasMore")
	}
	if page1.NextCursor == "" {
		t.Fatal("page 1: expected non-empty NextCursor")
	}

	page2, err := f.repo.Search(ctx, "", domain.SearchOptions{Limit: 10, Cursor: page1.NextCursor})
	if err != nil {
		t.Fatalf("page 2: %v", err)
	}
	if len(page2.Products) != 10 {
		t.Fatalf("page 2: expected 10, got %d", len(page2.Products))
	}
	if !page2.HasMore {
		t.Fatal("page 2: expected HasMore")
	}

	page3, err := f.repo.Search(ctx, "", domain.SearchOptions{Limit: 10, Cursor: page2.NextCursor})
	if err != nil {
		t.Fatalf("page 3: %v", err)
	}
	if len(page3.Products) != 5 {
		t.Fatalf("page 3: expected 5, got %d", len(page3.Products))
	}
	if page3.HasMore {
		t.Fatal("page 3: expected HasMore=false")
	}

	seen := make(map[int64]bool)
	for _, p := range page1.Products {
		seen[p.ID.Int64()] = true
	}
	for _, p := range page2.Products {
		if seen[p.ID.Int64()] {
			t.Errorf("duplicate %d on page 2", p.ID.Int64())
		}
		seen[p.ID.Int64()] = true
	}
	for _, p := range page3.Products {
		if seen[p.ID.Int64()] {
			t.Errorf("duplicate %d on page 3", p.ID.Int64())
		}
		seen[p.ID.Int64()] = true
	}
	if len(seen) != 25 {
		t.Errorf("expected 25 unique IDs, got %d", len(seen))
	}
}

func TestIntegration_SearchOrderByIDDesc(t *testing.T) {
	f := newIntegrationFixture(t)
	defer f.cleanup()

	seedProducts(t, f.repo, 5)

	result, err := f.repo.Search(ctx, "", domain.SearchOptions{Limit: 5})
	if err != nil {
		t.Fatalf("search: %v", err)
	}
	if len(result.Products) != 5 {
		t.Fatalf("expected 5, got %d", len(result.Products))
	}
	for i := 0; i < 4; i++ {
		if result.Products[i].ID <= result.Products[i+1].ID {
			t.Errorf("not sorted desc at %d: %d <= %d", i, result.Products[i].ID, result.Products[i+1].ID)
		}
	}
}

func TestIntegration_SearchEmptyResult(t *testing.T) {
	f := newIntegrationFixture(t)
	defer f.cleanup()

	result, err := f.repo.Search(ctx, "nonexistent", domain.SearchOptions{})
	if err != nil {
		t.Fatalf("search: %v", err)
	}
	if len(result.Products) != 0 {
		t.Errorf("expected 0, got %d", len(result.Products))
	}
	if result.HasMore {
		t.Error("expected HasMore=false")
	}
}

func TestIntegration_DeleteProduct(t *testing.T) {
	f := newIntegrationFixture(t)
	defer f.cleanup()

	p, _ := domain.NewProduct(1, "INT-DEL", "To Delete", "desc", "cat", domain.Money{Amount: 100, Currency: "USD"}, nil)
	f.repo.Save(ctx, p)

	if err := f.repo.Delete(ctx, 1); err != nil {
		t.Fatalf("delete: %v", err)
	}

	_, err := f.repo.FindByID(ctx, 1)
	if !kernel.IsNotFound(err) {
		t.Errorf("expected NotFound after delete, got %v", err)
	}
}

func TestIntegration_DeleteNotFound(t *testing.T) {
	f := newIntegrationFixture(t)
	defer f.cleanup()

	err := f.repo.Delete(ctx, 99999)
	if !kernel.IsNotFound(err) {
		t.Errorf("expected NotFound, got %v", err)
	}
}

func TestIntegration_CachePopulation(t *testing.T) {
	f := newIntegrationFixture(t)
	defer f.cleanup()

	p, _ := domain.NewProduct(1, "INT-CACHE", "Cache Test", "desc", "cat", domain.Money{Amount: 500, Currency: "USD"}, nil)
	f.repo.Save(ctx, p)

	// Clear product from any cached state and do a fresh read
	f.repo.FindByID(ctx, 1)

	key := f.repo.idCacheKey(1)
	val, err := f.rdb.Get(ctx, key).Result()
	if err != nil {
		t.Fatalf("redis get: %v", err)
	}
	if val == "" {
		t.Fatal("expected cached value, got empty")
	}

	skuKey := f.repo.skuCacheKey("INT-CACHE")
	skuVal, err := f.rdb.Get(ctx, skuKey).Result()
	if err != nil {
		t.Fatalf("redis get sku cache: %v", err)
	}
	if skuVal == "" {
		t.Fatal("expected cached sku value, got empty")
	}
}

func TestIntegration_CacheInvalidationOnSave(t *testing.T) {
	f := newIntegrationFixture(t)
	defer f.cleanup()

	p, _ := domain.NewProduct(1, "INT-INV", "Before", "desc", "cat", domain.Money{Amount: 100, Currency: "USD"}, nil)
	f.repo.Save(ctx, p)

	// Populate cache
	f.repo.FindByID(ctx, 1)
	if _, err := f.rdb.Get(ctx, f.repo.idCacheKey(1)).Result(); err != nil {
		t.Fatalf("expected cache hit before save: %v", err)
	}

	// Update product
	p.ChangePrice(domain.Money{Amount: 200, Currency: "USD"})
	f.repo.Save(ctx, p)

	// Cache should be invalidated
	_, err := f.rdb.Get(ctx, f.repo.idCacheKey(1)).Result()
	if err != redis.Nil {
		t.Errorf("expected cache miss after save, got %v", err)
	}
}

func TestIntegration_CacheInvalidationOnDelete(t *testing.T) {
	f := newIntegrationFixture(t)
	defer f.cleanup()

	p, _ := domain.NewProduct(1, "INT-DELC", "Del Cache", "desc", "cat", domain.Money{Amount: 100, Currency: "USD"}, nil)
	f.repo.Save(ctx, p)

	f.repo.FindByID(ctx, 1)
	if _, err := f.rdb.Get(ctx, f.repo.idCacheKey(1)).Result(); err != nil {
		t.Fatalf("expected cache hit before delete: %v", err)
	}

	f.repo.Delete(ctx, 1)

	_, err := f.rdb.Get(ctx, f.repo.idCacheKey(1)).Result()
	if err != redis.Nil {
		t.Errorf("expected cache miss after delete, got %v", err)
	}
}

func TestIntegration_CacheTTL(t *testing.T) {
	f := newIntegrationFixture(t)
	defer f.cleanup()

	p, _ := domain.NewProduct(1, "INT-TTL", "TTL Test", "desc", "cat", domain.Money{Amount: 100, Currency: "USD"}, nil)
	f.repo.Save(ctx, p)
	f.repo.FindByID(ctx, 1)

	ttl, err := f.rdb.TTL(ctx, f.repo.idCacheKey(1)).Result()
	if err != nil {
		t.Fatalf("redis ttl: %v", err)
	}
	if ttl <= 0 {
		t.Fatalf("expected positive TTL, got %v", ttl)
	}
	if ttl > 20*time.Minute {
		t.Errorf("expected TTL ~15m, got %v", ttl)
	}
}

func TestIntegration_UpdateThenFind(t *testing.T) {
	f := newIntegrationFixture(t)
	defer f.cleanup()

	p, _ := domain.NewProduct(1, "INT-UPD", "Original", "original desc", "original cat", domain.Money{Amount: 1000, Currency: "USD"}, nil)
	f.repo.Save(ctx, p)

	p.ChangePrice(domain.Money{Amount: 2500, Currency: "USD"})
	p.UpdateDetails("Updated", "updated desc", "updated cat")
	f.repo.Save(ctx, p)

	got, err := f.repo.FindByID(ctx, 1)
	if err != nil {
		t.Fatalf("find: %v", err)
	}
	if got.Name != "Updated" {
		t.Errorf("name: expected %q, got %q", "Updated", got.Name)
	}
	if got.Price.Amount != 2500 {
		t.Errorf("price: expected %d, got %d", 2500, got.Price.Amount)
	}
	if got.Description != "updated desc" {
		t.Errorf("desc: expected %q, got %q", "updated desc", got.Description)
	}
	if got.Category != "updated cat" {
		t.Errorf("category: expected %q, got %q", "updated cat", got.Category)
	}
}

func TestIntegration_AttributesRoundTrip(t *testing.T) {
	f := newIntegrationFixture(t)
	defer f.cleanup()

	attrs := map[string]any{"color": "navy", "size": "L", "weight_kg": 0.5}
	p, _ := domain.NewProduct(1, "INT-ATTR", "Attr Test", "desc", "cat", domain.Money{Amount: 4999, Currency: "USD"}, attrs)
	f.repo.Save(ctx, p)

	got, err := f.repo.FindByID(ctx, 1)
	if err != nil {
		t.Fatalf("find: %v", err)
	}
	if got.Attributes["color"] != "navy" {
		t.Errorf("color: expected %q, got %v", "navy", got.Attributes["color"])
	}
	if got.Attributes["size"] != "L" {
		t.Errorf("size: expected %q, got %v", "L", got.Attributes["size"])
	}
	if got.Attributes["weight_kg"] != 0.5 {
		t.Errorf("weight_kg: expected 0.5, got %v", got.Attributes["weight_kg"])
	}
}
