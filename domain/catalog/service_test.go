package catalog

import (
	"context"
	"testing"

	"github.com/beeleelee/mall/domain/kernel"
)

func usdPrice(amount int64) Money {
	return Money{Amount: amount, Currency: "USD"}
}

func newService() *CatalogService {
	return NewCatalogService(newFakeProductRepository(), fakeLogger{})
}

func createProduct(t *testing.T, svc *CatalogService, id int64, sku SKU, name string, price Money) *Product {
	t.Helper()
	p, err := svc.CreateProduct(context.Background(), kernel.ID(id), sku, name, "desc", "cat", 0, price, nil)
	if err != nil {
		t.Fatalf("CreateProduct(%d, %s, %s) failed: %v", id, sku, name, err)
	}
	return p
}

func TestCatalogService_CreateAndGetProduct(t *testing.T) {
	svc := newService()
	p := createProduct(t, svc, 1, "SKU-001", "Phone", usdPrice(99900))

	got, err := svc.GetProduct(context.Background(), p.ID)
	if err != nil {
		t.Fatalf("GetProduct failed: %v", err)
	}
	if got.Name != "Phone" {
		t.Errorf("expected name Phone, got %s", got.Name)
	}
	if got.SKU != "SKU-001" {
		t.Errorf("expected SKU SKU-001, got %s", got.SKU)
	}
	if got.Price.Amount != 99900 {
		t.Errorf("expected price 99900, got %d", got.Price.Amount)
	}
}

func TestCatalogService_CreateAndLookup(t *testing.T) {
	svc := newService()
	createProduct(t, svc, 1, "SKU-LOOKUP", "Tablet", usdPrice(49900))

	got, err := svc.Lookup(context.Background(), "SKU-LOOKUP")
	if err != nil {
		t.Fatalf("Lookup failed: %v", err)
	}
	if got.Name != "Tablet" {
		t.Errorf("expected name Tablet, got %s", got.Name)
	}
}

func TestCatalogService_SearchByQuery(t *testing.T) {
	svc := newService()
	createProduct(t, svc, 1, "SKU-S1", "Wireless Mouse", usdPrice(2500))
	createProduct(t, svc, 2, "SKU-S2", "Mechanical Keyboard", usdPrice(8900))
	createProduct(t, svc, 3, "SKU-S3", "USB-C Hub", usdPrice(3500))

	result, err := svc.Search(context.Background(), "mouse", SearchOptions{})
	if err != nil {
		t.Fatalf("Search failed: %v", err)
	}
	if len(result.Products) != 1 {
		t.Fatalf("expected 1 result, got %d", len(result.Products))
	}
	if result.Products[0].SKU != "SKU-S1" {
		t.Errorf("expected SKU-S1, got %s", result.Products[0].SKU)
	}
}

func TestCatalogService_SearchByCategory(t *testing.T) {
	svc := newService()
	createProduct(t, svc, 1, "SKU-C1", "Laptop", usdPrice(99900))
	createProduct(t, svc, 2, "SKU-C2", "Monitor", usdPrice(29900))
	p3, _ := NewProduct(3, "SKU-C3", "Mouse Pad", "desc", "accessories", 0, usdPrice(1500), nil)
	svc.repo.Save(context.Background(), p3)

	result, err := svc.Search(context.Background(), "", SearchOptions{Category: "cat"})
	if err != nil {
		t.Fatalf("Search failed: %v", err)
	}
	if len(result.Products) != 2 {
		t.Fatalf("expected 2 results, got %d", len(result.Products))
	}
}

func TestCatalogService_SearchByPriceRange(t *testing.T) {
	svc := newService()
	createProduct(t, svc, 1, "SKU-P1", "Cheap Widget", usdPrice(500))
	createProduct(t, svc, 2, "SKU-P2", "Mid Widget", usdPrice(2500))
	createProduct(t, svc, 3, "SKU-P3", "Pricey Widget", usdPrice(7500))

	result, err := svc.Search(context.Background(), "", SearchOptions{MinPrice: 2000, MaxPrice: 5000})
	if err != nil {
		t.Fatalf("Search failed: %v", err)
	}
	if len(result.Products) != 1 {
		t.Fatalf("expected 1 result, got %d", len(result.Products))
	}
	if result.Products[0].SKU != "SKU-P2" {
		t.Errorf("expected SKU-P2, got %s", result.Products[0].SKU)
	}
}

func TestCatalogService_SearchDefaultStatus(t *testing.T) {
	svc := newService()
	createProduct(t, svc, 1, "SKU-D1", "Active Item", usdPrice(1000))
	p2, _ := NewProduct(2, "SKU-D2", "Inactive Item", "desc", "cat", 0, usdPrice(2000), nil)
	p2.ChangeStatus(ProductStatusInactive)
	svc.repo.Save(context.Background(), p2)

	result, err := svc.Search(context.Background(), "", SearchOptions{})
	if err != nil {
		t.Fatalf("Search failed: %v", err)
	}
	if len(result.Products) != 1 {
		t.Fatalf("expected 1 active product, got %d", len(result.Products))
	}
	if result.Products[0].SKU != "SKU-D1" {
		t.Errorf("expected SKU-D1, got %s", result.Products[0].SKU)
	}
}

func TestCatalogService_SearchPagination(t *testing.T) {
	svc := newService()
	for i := int64(1); i <= 25; i++ {
		createProduct(t, svc, i, SKU("SKU-PG-"+string(rune('A'+i-1))), "Item", usdPrice(i*100))
	}

	page1, err := svc.Search(context.Background(), "", SearchOptions{Limit: 10})
	if err != nil {
		t.Fatalf("page 1 search failed: %v", err)
	}
	if len(page1.Products) != 10 {
		t.Errorf("expected 10 products on page 1, got %d", len(page1.Products))
	}
	if !page1.HasMore {
		t.Error("expected HasMore on page 1, got false")
	}
	if page1.NextCursor == "" {
		t.Error("expected NextCursor on page 1, got empty")
	}

	page2, err := svc.Search(context.Background(), "", SearchOptions{Limit: 10, Cursor: page1.NextCursor})
	if err != nil {
		t.Fatalf("page 2 search failed: %v", err)
	}
	if len(page2.Products) != 10 {
		t.Errorf("expected 10 products on page 2, got %d", len(page2.Products))
	}
	if !page2.HasMore {
		t.Error("expected HasMore on page 2, got false")
	}

	page3, err := svc.Search(context.Background(), "", SearchOptions{Limit: 10, Cursor: page2.NextCursor})
	if err != nil {
		t.Fatalf("page 3 search failed: %v", err)
	}
	if len(page3.Products) != 5 {
		t.Errorf("expected 5 products on page 3, got %d", len(page3.Products))
	}
	if page3.HasMore {
		t.Error("expected HasMore=false on last page")
	}

	ids := make(map[int64]bool)
	for _, p := range page1.Products {
		ids[p.ID.Int64()] = true
	}
	for _, p := range page2.Products {
		if ids[p.ID.Int64()] {
			t.Errorf("duplicate product %d on page 2", p.ID.Int64())
		}
		ids[p.ID.Int64()] = true
	}
	for _, p := range page3.Products {
		if ids[p.ID.Int64()] {
			t.Errorf("duplicate product %d on page 3", p.ID.Int64())
		}
		ids[p.ID.Int64()] = true
	}

	if len(ids) != 25 {
		t.Errorf("expected 25 unique products across 3 pages, got %d", len(ids))
	}
}

func TestCatalogService_SearchEmptyResult(t *testing.T) {
	svc := newService()
	result, err := svc.Search(context.Background(), "nonexistent", SearchOptions{})
	if err != nil {
		t.Fatalf("Search failed: %v", err)
	}
	if len(result.Products) != 0 {
		t.Errorf("expected 0 results, got %d", len(result.Products))
	}
	if result.HasMore {
		t.Error("expected HasMore=false for empty result")
	}
}

func TestCatalogService_SearchNoProducts(t *testing.T) {
	svc := newService()
	result, err := svc.Search(context.Background(), "", SearchOptions{})
	if err != nil {
		t.Fatalf("Search failed: %v", err)
	}
	if len(result.Products) != 0 {
		t.Errorf("expected 0 results, got %d", len(result.Products))
	}
	if result.HasMore {
		t.Error("expected HasMore=false for empty catalog")
	}
}

func TestCatalogService_LookupNotFound(t *testing.T) {
	svc := newService()
	_, err := svc.Lookup(context.Background(), "NONEXISTENT")
	if !kernel.IsNotFound(err) {
		t.Errorf("expected NotFound error, got %v", err)
	}
}

func TestCatalogService_LookupEmptySKU(t *testing.T) {
	svc := newService()
	_, err := svc.Lookup(context.Background(), "")
	if !kernel.IsInvalidArgument(err) {
		t.Errorf("expected InvalidArgument error, got %v", err)
	}
}

func TestCatalogService_GetProductNotFound(t *testing.T) {
	svc := newService()
	_, err := svc.GetProduct(context.Background(), 999)
	if !kernel.IsNotFound(err) {
		t.Errorf("expected NotFound error, got %v", err)
	}
}

func TestCatalogService_GetProductInvalidID(t *testing.T) {
	svc := newService()
	_, err := svc.GetProduct(context.Background(), 0)
	if !kernel.IsInvalidArgument(err) {
		t.Errorf("expected InvalidArgument error, got %v", err)
	}
}

func TestCatalogService_DeleteProduct(t *testing.T) {
	svc := newService()
	p := createProduct(t, svc, 1, "SKU-DEL", "To Delete", usdPrice(1000))

	_, err := svc.Lookup(context.Background(), "SKU-DEL")
	if err != nil {
		t.Fatalf("product should exist before delete: %v", err)
	}

	if err := svc.repo.Delete(context.Background(), p.ID); err != nil {
		t.Fatalf("Delete failed: %v", err)
	}

	_, err = svc.GetProduct(context.Background(), p.ID)
	if !kernel.IsNotFound(err) {
		t.Errorf("expected NotFound after delete, got %v", err)
	}
}

func TestCatalogService_DeleteNotFound(t *testing.T) {
	svc := newService()
	err := svc.repo.Delete(context.Background(), 999)
	if !kernel.IsNotFound(err) {
		t.Errorf("expected NotFound error, got %v", err)
	}
}

func TestCatalogService_SearchWithExplicitStatus(t *testing.T) {
	svc := newService()
	p1, _ := NewProduct(1, "SKU-E1", "Active", "desc", "cat", 0, usdPrice(1000), nil)
	svc.repo.Save(context.Background(), p1)
	p2, _ := NewProduct(2, "SKU-E2", "Inactive", "desc", "cat", 0, usdPrice(2000), nil)
	p2.ChangeStatus(ProductStatusInactive)
	svc.repo.Save(context.Background(), p2)

	result, err := svc.Search(context.Background(), "", SearchOptions{Status: ProductStatusInactive})
	if err != nil {
		t.Fatalf("Search failed: %v", err)
	}
	if len(result.Products) != 1 {
		t.Fatalf("expected 1 inactive product, got %d", len(result.Products))
	}
	if result.Products[0].SKU != "SKU-E2" {
		t.Errorf("expected SKU-E2, got %s", result.Products[0].SKU)
	}
}

func TestCatalogService_LimitClampedToMax(t *testing.T) {
	svc := newService()
	for i := int64(1); i <= 150; i++ {
		createProduct(t, svc, i, SKU("SKU-LMT-"+string(rune('A'+(i-1)%26))), "Item", usdPrice(i*10))
	}

	result, err := svc.Search(context.Background(), "", SearchOptions{Limit: 200})
	if err != nil {
		t.Fatalf("Search failed: %v", err)
	}
	if len(result.Products) > 100 {
		t.Errorf("expected at most 100 products, got %d", len(result.Products))
	}
}

func TestCatalogService_SearchOrderByIDDesc(t *testing.T) {
	svc := newService()
	createProduct(t, svc, 1, "SKU-O1", "Oldest", usdPrice(100))
	createProduct(t, svc, 2, "SKU-O2", "Middle", usdPrice(200))
	createProduct(t, svc, 3, "SKU-O3", "Newest", usdPrice(300))

	result, err := svc.Search(context.Background(), "", SearchOptions{})
	if err != nil {
		t.Fatalf("Search failed: %v", err)
	}
	if len(result.Products) != 3 {
		t.Fatalf("expected 3 products, got %d", len(result.Products))
	}
	if result.Products[0].SKU != "SKU-O3" {
		t.Errorf("expected newest first (SKU-O3), got %s", result.Products[0].SKU)
	}
	if result.Products[2].SKU != "SKU-O1" {
		t.Errorf("expected oldest last (SKU-O1), got %s", result.Products[2].SKU)
	}
}

func TestCatalogService_CreateProductWithAttributes(t *testing.T) {
	svc := newService()
	attrs := map[string]any{"color": "black", "size": "XL"}
	p, err := svc.CreateProduct(context.Background(), 1, "SKU-ATTR", "T-Shirt", "Cotton tee", "clothing", 0, usdPrice(2999), attrs)
	if err != nil {
		t.Fatalf("CreateProduct failed: %v", err)
	}

	got, err := svc.GetProduct(context.Background(), p.ID)
	if err != nil {
		t.Fatalf("GetProduct failed: %v", err)
	}
	if got.Attributes["color"] != "black" {
		t.Errorf("expected color black, got %v", got.Attributes["color"])
	}
	if got.Attributes["size"] != "XL" {
		t.Errorf("expected size XL, got %v", got.Attributes["size"])
	}
}

func TestCatalogService_UpdateThenGet(t *testing.T) {
	svc := newService()
	p := createProduct(t, svc, 1, "SKU-UPD", "Old Name", usdPrice(1000))

	if err := p.ChangePrice(usdPrice(2000)); err != nil {
		t.Fatalf("ChangePrice failed: %v", err)
	}
	if err := p.UpdateDetails("New Name", "new desc", "new cat", 0); err != nil {
		t.Fatalf("UpdateDetails failed: %v", err)
	}
	if err := svc.repo.Save(context.Background(), p); err != nil {
		t.Fatalf("Save after update failed: %v", err)
	}

	got, err := svc.GetProduct(context.Background(), p.ID)
	if err != nil {
		t.Fatalf("GetProduct failed: %v", err)
	}
	if got.Name != "New Name" {
		t.Errorf("expected New Name, got %s", got.Name)
	}
	if got.Price.Amount != 2000 {
		t.Errorf("expected price 2000, got %d", got.Price.Amount)
	}
	if got.Description != "new desc" {
		t.Errorf("expected 'new desc', got %s", got.Description)
	}
	if got.Category != "new cat" {
		t.Errorf("expected 'new cat', got %s", got.Category)
	}
}
