package rest

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strconv"
	"strings"
	"sync"
	"testing"

	"github.com/zeromicro/go-zero/rest/pathvar"

	domain "github.com/beeleelee/mall/domain/catalog"
	"github.com/beeleelee/mall/domain/kernel"
)

type fakeCatalogRepo struct {
	mu       sync.Mutex
	products map[kernel.ID]*domain.Product
	skus     map[domain.SKU]kernel.ID
}

func newFakeCatalogRepo() *fakeCatalogRepo {
	return &fakeCatalogRepo{
		products: make(map[kernel.ID]*domain.Product),
		skus:     make(map[domain.SKU]kernel.ID),
	}
}

func (f *fakeCatalogRepo) Save(_ context.Context, p *domain.Product) error {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.products[p.ID] = p
	f.skus[p.SKU] = p.ID
	return nil
}

func (f *fakeCatalogRepo) FindByID(_ context.Context, id kernel.ID) (*domain.Product, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	p, ok := f.products[id]
	if !ok {
		return nil, kernel.NewDomainError(kernel.ErrNotFound, "product not found")
	}
	return p, nil
}

func (f *fakeCatalogRepo) FindBySKU(_ context.Context, sku domain.SKU) (*domain.Product, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	id, ok := f.skus[sku]
	if !ok {
		return nil, kernel.NewDomainError(kernel.ErrNotFound, "product not found")
	}
	p, ok := f.products[id]
	if !ok {
		return nil, kernel.NewDomainError(kernel.ErrNotFound, "product not found")
	}
	return p, nil
}

func (f *fakeCatalogRepo) Search(_ context.Context, query string, opts domain.SearchOptions) (*domain.SearchResult, error) {
	f.mu.Lock()
	defer f.mu.Unlock()

	limit := opts.Limit
	if limit <= 0 {
		limit = 20
	}

	cursorID := int64(0)
	if opts.Cursor != "" {
		id, err := decodeCursorREST(opts.Cursor)
		if err != nil {
			return nil, kernel.NewDomainError(kernel.ErrInvalidArgument, "invalid cursor")
		}
		cursorID = id
	}

	var filtered []*domain.Product
	for _, p := range f.products {
		q := query
		if q == "" {
			q = opts.FulltextQuery
		}
		if q != "" && !matchQueryREST(p, q) {
			continue
		}
		if opts.Category != "" && p.Category != opts.Category {
			continue
		}
		if opts.MinPrice > 0 && p.Price.Amount < opts.MinPrice {
			continue
		}
		if opts.MaxPrice > 0 && p.Price.Amount > opts.MaxPrice {
			continue
		}
		if cursorID > 0 && p.ID.Int64() >= cursorID {
			continue
		}
		filtered = append(filtered, p)
	}

	if len(filtered) > limit {
		return &domain.SearchResult{
			Products:   filtered[:limit],
			HasMore:    true,
			NextCursor: encodeCursorREST(filtered[limit-1].ID.Int64()),
		}, nil
	}

	return &domain.SearchResult{
		Products: filtered,
		HasMore:  false,
	}, nil
}

func (f *fakeCatalogRepo) Delete(_ context.Context, id kernel.ID) error {
	f.mu.Lock()
	defer f.mu.Unlock()
	p, ok := f.products[id]
	if !ok {
		return kernel.NewDomainError(kernel.ErrNotFound, "product not found")
	}
	delete(f.skus, p.SKU)
	delete(f.products, id)
	return nil
}

func matchQueryREST(p *domain.Product, query string) bool {
	q := strings.ToLower(query)
	return strings.Contains(strings.ToLower(p.Name), q) ||
		strings.Contains(strings.ToLower(p.Description), q) ||
		strings.Contains(strings.ToLower(string(p.SKU)), q)
}

func encodeCursorREST(id int64) domain.Cursor {
	return domain.Cursor(base64.RawURLEncoding.EncodeToString([]byte(strconv.FormatInt(id, 10))))
}

func decodeCursorREST(c domain.Cursor) (int64, error) {
	data, err := base64.RawURLEncoding.DecodeString(string(c))
	if err != nil {
		return 0, err
	}
	return strconv.ParseInt(string(data), 10, 64)
}

type fakeLoggerCatalog struct{}

func (fakeLoggerCatalog) Debug(_ context.Context, _ string, _ ...kernel.LogField)          {}
func (fakeLoggerCatalog) Info(_ context.Context, _ string, _ ...kernel.LogField)           {}
func (fakeLoggerCatalog) Warn(_ context.Context, _ string, _ ...kernel.LogField)           {}
func (fakeLoggerCatalog) Error(_ context.Context, _ string, _ error, _ ...kernel.LogField) {}

func seedProduct(t *testing.T, repo *fakeCatalogRepo, sf *kernel.Snowflake, sku string, name string, amount int64) kernel.ID {
	t.Helper()
	id, _ := sf.NextID()
	p, err := domain.NewProduct(id, domain.SKU(sku), name, "desc", "cat", 0, domain.Money{Amount: amount, Currency: "USD"}, nil)
	if err != nil {
		t.Fatal(err)
	}
	if err := repo.Save(context.Background(), p); err != nil {
		t.Fatal(err)
	}
	return id
}

func TestCatalogHandler_Search(t *testing.T) {
	repo := newFakeCatalogRepo()
	svc := domain.NewCatalogService(repo, fakeLoggerCatalog{})
	h := NewCatalogHandler(svc)

	sf, _ := kernel.NewSnowflake(1)
	p1 := seedProduct(t, repo, sf, "SKU-001", "Widget Alpha", 1000)
	seedProduct(t, repo, sf, "SKU-002", "Widget Beta", 2000)
	seedProduct(t, repo, sf, "SKU-003", "Gadget Gamma", 3000)

	t.Run("all products", func(t *testing.T) {
		w := httptest.NewRecorder()
		r := httptest.NewRequest(http.MethodGet, "/api/v1/catalog/search", nil)
		h.Search(w, r)

		if w.Code != http.StatusOK {
			t.Fatalf("expected 200, got %d: %s", w.Code, w.Body.String())
		}

		var res map[string]any
		if err := json.Unmarshal(w.Body.Bytes(), &res); err != nil {
			t.Fatal(err)
		}
		products := res["products"].([]any)
		if len(products) != 3 {
			t.Fatalf("expected 3 products, got %d", len(products))
		}
	})

	t.Run("search by query", func(t *testing.T) {
		w := httptest.NewRecorder()
		r := httptest.NewRequest(http.MethodGet, "/api/v1/catalog/search?q=widget", nil)
		h.Search(w, r)

		if w.Code != http.StatusOK {
			t.Fatalf("expected 200, got %d", w.Code)
		}
		var res map[string]any
		json.Unmarshal(w.Body.Bytes(), &res)
		products := res["products"].([]any)
		if len(products) != 2 {
			t.Fatalf("expected 2, got %d", len(products))
		}
	})

	t.Run("search with limit", func(t *testing.T) {
		w := httptest.NewRecorder()
		r := httptest.NewRequest(http.MethodGet, "/api/v1/catalog/search?limit=2", nil)
		h.Search(w, r)

		if w.Code != http.StatusOK {
			t.Fatalf("expected 200, got %d", w.Code)
		}
		var res map[string]any
		json.Unmarshal(w.Body.Bytes(), &res)
		products := res["products"].([]any)
		if len(products) != 2 {
			t.Fatalf("expected 2, got %d", len(products))
		}
		if res["has_more"].(bool) != true {
			t.Error("expected has_more true")
		}
		if res["next_cursor"].(string) == "" {
			t.Error("expected non-empty cursor")
		}
	})

	t.Run("no results", func(t *testing.T) {
		w := httptest.NewRecorder()
		r := httptest.NewRequest(http.MethodGet, "/api/v1/catalog/search?q=nonexistent", nil)
		h.Search(w, r)

		if w.Code != http.StatusOK {
			t.Fatalf("expected 200, got %d", w.Code)
		}
		var res map[string]any
		json.Unmarshal(w.Body.Bytes(), &res)
		products := res["products"].([]any)
		if len(products) != 0 {
			t.Fatalf("expected 0, got %d", len(products))
		}
	})

	t.Run("get by id", func(t *testing.T) {
		w := httptest.NewRecorder()
		r := httptest.NewRequest(http.MethodGet, "/api/v1/catalog/products/"+strconv.FormatInt(p1.Int64(), 10), nil)
		r = pathvar.WithVars(r, map[string]string{"id": strconv.FormatInt(p1.Int64(), 10)})
		h.GetProduct(w, r)

		if w.Code != http.StatusOK {
			t.Fatalf("expected 200, got %d: %s", w.Code, w.Body.String())
		}
		var res map[string]any
		json.Unmarshal(w.Body.Bytes(), &res)
		if res["sku"] != "SKU-001" {
			t.Errorf("expected SKU-001, got %v", res["sku"])
		}
	})

	t.Run("lookup by sku", func(t *testing.T) {
		w := httptest.NewRecorder()
		r := httptest.NewRequest(http.MethodGet, "/api/v1/catalog/lookup?sku=SKU-002", nil)
		h.Lookup(w, r)

		if w.Code != http.StatusOK {
			t.Fatalf("expected 200, got %d", w.Code)
		}
		var res map[string]any
		json.Unmarshal(w.Body.Bytes(), &res)
		if res["name"] != "Widget Beta" {
			t.Errorf("expected Widget Beta, got %v", res["name"])
		}
	})

	t.Run("lookup missing sku", func(t *testing.T) {
		w := httptest.NewRecorder()
		r := httptest.NewRequest(http.MethodGet, "/api/v1/catalog/lookup?sku=MISSING", nil)
		h.Lookup(w, r)

		if w.Code != http.StatusNotFound {
			t.Fatalf("expected 404, got %d", w.Code)
		}
	})

	t.Run("get product not found", func(t *testing.T) {
		w := httptest.NewRecorder()
		r := httptest.NewRequest(http.MethodGet, "/api/v1/catalog/products/99999", nil)
		r = pathvar.WithVars(r, map[string]string{"id": "99999"})
		h.GetProduct(w, r)

		if w.Code != http.StatusNotFound {
			t.Fatalf("expected 404, got %d", w.Code)
		}
	})
}
