package mcp

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	domain "github.com/beeleelee/mall/domain/catalog"
	"github.com/beeleelee/mall/domain/kernel"
)

type fakeRepo struct {
	products map[kernel.ID]*domain.Product
	skus     map[domain.SKU]kernel.ID
}

func newFakeRepo() *fakeRepo {
	return &fakeRepo{
		products: make(map[kernel.ID]*domain.Product),
		skus:     make(map[domain.SKU]kernel.ID),
	}
}

func (f *fakeRepo) Save(_ context.Context, p *domain.Product) error {
	f.products[p.ID] = p
	f.skus[p.SKU] = p.ID
	return nil
}

func (f *fakeRepo) FindByID(_ context.Context, id kernel.ID) (*domain.Product, error) {
	p, ok := f.products[id]
	if !ok {
		return nil, kernel.NewDomainError(kernel.ErrNotFound, "not found")
	}
	return p, nil
}

func (f *fakeRepo) FindBySKU(_ context.Context, sku domain.SKU) (*domain.Product, error) {
	id, ok := f.skus[sku]
	if !ok {
		return nil, kernel.NewDomainError(kernel.ErrNotFound, "not found")
	}
	return f.products[id], nil
}

func (f *fakeRepo) Search(_ context.Context, query string, _ domain.SearchOptions) (*domain.SearchResult, error) {
	var filtered []*domain.Product
	for _, p := range f.products {
		if query == "" {
			filtered = append(filtered, p)
		}
	}
	return &domain.SearchResult{Products: filtered}, nil
}

func (f *fakeRepo) Delete(_ context.Context, _ kernel.ID) error {
	return nil
}

type fakeLoggerMCP struct{}

func (fakeLoggerMCP) Debug(_ context.Context, _ string, _ ...kernel.LogField)          {}
func (fakeLoggerMCP) Info(_ context.Context, _ string, _ ...kernel.LogField)           {}
func (fakeLoggerMCP) Warn(_ context.Context, _ string, _ ...kernel.LogField)           {}
func (fakeLoggerMCP) Error(_ context.Context, _ string, _ error, _ ...kernel.LogField) {}

func newRouter(t *testing.T) (*MCPRouter, *fakeRepo) {
	t.Helper()
	repo := newFakeRepo()
	svc := domain.NewCatalogService(repo, fakeLoggerMCP{})
	router := NewMCPRouter()
	router.Register(NewCatalogMCPHandler(svc))
	return router, repo
}

func TestMCP_ToolsList(t *testing.T) {
	router, _ := newRouter(t)

	body, _ := json.Marshal(map[string]any{
		"jsonrpc": "2.0",
		"method":  "tools/list",
		"id":      1,
	})

	w := httptest.NewRecorder()
	r := httptest.NewRequest(http.MethodPost, "/mcp", bytes.NewReader(body))
	r.Header.Set("Content-Type", "application/json")
	router.ServeHTTP(w, r)

	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", w.Code)
	}

	var resp map[string]any
	if err := json.Unmarshal(w.Body.Bytes(), &resp); err != nil {
		t.Fatal(err)
	}

	if resp["jsonrpc"] != "2.0" {
		t.Errorf("expected jsonrpc 2.0, got %v", resp["jsonrpc"])
	}

	tools, ok := resp["result"].(map[string]any)["tools"].([]any)
	if !ok {
		t.Fatal("expected result.tools array")
	}
	if len(tools) != 3 {
		t.Fatalf("expected 3 tools, got %d", len(tools))
	}
}

func TestMCP_CallTool(t *testing.T) {
	router, repo := newRouter(t)

	sf, _ := kernel.NewSnowflake(1)
	id, _ := sf.NextID()
	p, err := domain.NewProduct(id, "SKU-MCP-001", "MCP Product", "test", "cat", domain.Money{Amount: 1000, Currency: "USD"}, nil)
	if err != nil {
		t.Fatal(err)
	}
	if err := repo.Save(context.Background(), p); err != nil {
		t.Fatal(err)
	}

	t.Run("call get_product", func(t *testing.T) {
		body, _ := json.Marshal(map[string]any{
			"jsonrpc": "2.0",
			"method":  "tools/call",
			"params": map[string]any{
				"name": "get_product",
				"arguments": map[string]any{
					"id": id.Int64(),
				},
			},
			"id": 2,
		})

		w := httptest.NewRecorder()
		r := httptest.NewRequest(http.MethodPost, "/mcp", bytes.NewReader(body))
		router.ServeHTTP(w, r)

		if w.Code != http.StatusOK {
			t.Fatalf("expected 200, got %d: %s", w.Code, w.Body.String())
		}

		var resp map[string]any
		json.Unmarshal(w.Body.Bytes(), &resp)
		if resp["error"] != nil {
			t.Fatalf("unexpected error: %v", resp["error"])
		}

		content, _ := resp["result"].(map[string]any)["content"].([]any)
		if len(content) == 0 {
			t.Fatal("expected content")
		}
	})

	t.Run("call lookup_catalog", func(t *testing.T) {
		body, _ := json.Marshal(map[string]any{
			"jsonrpc": "2.0",
			"method":  "tools/call",
			"params": map[string]any{
				"name": "lookup_catalog",
				"arguments": map[string]any{
					"sku": "SKU-MCP-001",
				},
			},
			"id": 3,
		})

		w := httptest.NewRecorder()
		r := httptest.NewRequest(http.MethodPost, "/mcp", bytes.NewReader(body))
		router.ServeHTTP(w, r)

		if w.Code != http.StatusOK {
			t.Fatalf("expected 200, got %d", w.Code)
		}

		var resp map[string]any
		json.Unmarshal(w.Body.Bytes(), &resp)
		if resp["error"] != nil {
			t.Fatalf("unexpected error: %v", resp["error"])
		}
	})

	t.Run("unknown tool", func(t *testing.T) {
		body, _ := json.Marshal(map[string]any{
			"jsonrpc": "2.0",
			"method":  "tools/call",
			"params": map[string]any{
				"name":      "nonexistent",
				"arguments": map[string]any{},
			},
			"id": 4,
		})

		w := httptest.NewRecorder()
		r := httptest.NewRequest(http.MethodPost, "/mcp", bytes.NewReader(body))
		router.ServeHTTP(w, r)

		var resp map[string]any
		json.Unmarshal(w.Body.Bytes(), &resp)
		errObj, ok := resp["error"].(map[string]any)
		if !ok {
			t.Fatal("expected error")
		}
		if errObj["code"] != float64(-32601) {
			t.Errorf("expected code -32601, got %v", errObj["code"])
		}
	})

	t.Run("invalid jsonrpc", func(t *testing.T) {
		body, _ := json.Marshal(map[string]any{
			"jsonrpc": "1.0",
			"method":  "tools/list",
			"id":      5,
		})

		w := httptest.NewRecorder()
		r := httptest.NewRequest(http.MethodPost, "/mcp", bytes.NewReader(body))
		router.ServeHTTP(w, r)

		var resp map[string]any
		json.Unmarshal(w.Body.Bytes(), &resp)
		if resp["error"] == nil {
			t.Fatal("expected error for invalid jsonrpc version")
		}
	})
}
