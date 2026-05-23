package catalog

import (
	"context"
	"encoding/base64"
	"sort"
	"strconv"
	"strings"
	"sync"

	"github.com/beeleelee/mall/domain/kernel"
)

type fakeProductRepository struct {
	mu       sync.Mutex
	products map[kernel.ID]*Product
	skus     map[SKU]kernel.ID
}

func newFakeProductRepository() *fakeProductRepository {
	return &fakeProductRepository{
		products: make(map[kernel.ID]*Product),
		skus:     make(map[SKU]kernel.ID),
	}
}

func (f *fakeProductRepository) Save(_ context.Context, product *Product) error {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.products[product.ID] = product
	f.skus[product.SKU] = product.ID
	return nil
}

func (f *fakeProductRepository) FindByID(_ context.Context, id kernel.ID) (*Product, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	p, ok := f.products[id]
	if !ok {
		return nil, kernel.NewDomainError(kernel.ErrNotFound, "product not found")
	}
	return p, nil
}

func (f *fakeProductRepository) FindBySKU(_ context.Context, sku SKU) (*Product, error) {
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

func (f *fakeProductRepository) Search(_ context.Context, query string, opts SearchOptions) (*SearchResult, error) {
	f.mu.Lock()
	defer f.mu.Unlock()

	limit := opts.Limit
	if limit <= 0 {
		limit = 20
	}

	cursorID := int64(0)
	if opts.Cursor != "" {
		id, err := decodeCursor(opts.Cursor)
		if err != nil {
			return nil, kernel.NewDomainError(kernel.ErrInvalidArgument, "invalid cursor")
		}
		cursorID = id
	}

	filtered := make([]*Product, 0, len(f.products))
	for _, p := range f.products {
		if !matchQuery(p, query) {
			continue
		}
		if opts.Status != "" && p.Status != opts.Status {
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

	sort.Slice(filtered, func(i, j int) bool {
		return filtered[i].ID > filtered[j].ID
	})

	result := &SearchResult{}

	if len(filtered) > limit {
		result.Products = filtered[:limit]
		result.HasMore = true
		result.NextCursor = encodeCursor(filtered[limit-1].ID.Int64())
	} else {
		result.Products = filtered
		result.HasMore = false
	}

	return result, nil
}

func (f *fakeProductRepository) Delete(_ context.Context, id kernel.ID) error {
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

func matchQuery(p *Product, query string) bool {
	if query == "" {
		return true
	}
	q := strings.ToLower(query)
	return strings.Contains(strings.ToLower(p.Name), q) ||
		strings.Contains(strings.ToLower(p.Description), q) ||
		strings.Contains(strings.ToLower(string(p.SKU)), q)
}

func encodeCursor(id int64) Cursor {
	return Cursor(base64.RawURLEncoding.EncodeToString([]byte(strconv.FormatInt(id, 10))))
}

func decodeCursor(c Cursor) (int64, error) {
	data, err := base64.RawURLEncoding.DecodeString(string(c))
	if err != nil {
		return 0, err
	}
	return strconv.ParseInt(string(data), 10, 64)
}

type fakeLogger struct{}

func (fakeLogger) Debug(_ context.Context, _ string, _ ...kernel.LogField) {}
func (fakeLogger) Info(_ context.Context, _ string, _ ...kernel.LogField)  {}
func (fakeLogger) Warn(_ context.Context, _ string, _ ...kernel.LogField)  {}
func (fakeLogger) Error(_ context.Context, _ string, _ error, _ ...kernel.LogField) {}
