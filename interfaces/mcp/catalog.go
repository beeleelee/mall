package mcp

import (
	"context"
	"encoding/json"

	domain "github.com/beeleelee/mall/domain/catalog"
	"github.com/beeleelee/mall/domain/kernel"
)

type CatalogMCPHandler struct {
	svc   *domain.CatalogService
	tools []ToolDefinition
}

func NewCatalogMCPHandler(svc *domain.CatalogService) *CatalogMCPHandler {
	return &CatalogMCPHandler{
		svc:   svc,
		tools: catalogTools,
	}
}

var catalogTools = []ToolDefinition{
	{
		Name:        "search_catalog",
		Description: "Search for products by query, category, or price range",
		InputSchema: InputSchema{
			Type: "object",
			Properties: map[string]PropertySchema{
				"query":     {Type: "string", Description: "Full-text search query (matches name, description, category)"},
				"category":  {Type: "string", Description: "Filter by category"},
				"min_price": {Type: "number", Description: "Minimum price in cents"},
				"max_price": {Type: "number", Description: "Maximum price in cents"},
				"cursor":    {Type: "string", Description: "Pagination cursor from previous response"},
				"limit":     {Type: "number", Description: "Max results per page (default 20, max 100)"},
			},
		},
	},
	{
		Name:        "lookup_catalog",
		Description: "Look up a product by its SKU",
		InputSchema: InputSchema{
			Type: "object",
			Properties: map[string]PropertySchema{
				"sku": {Type: "string", Description: "Product SKU (stock keeping unit)"},
			},
		},
	},
	{
		Name:        "get_product",
		Description: "Get a product by its numeric ID",
		InputSchema: InputSchema{
			Type: "object",
			Properties: map[string]PropertySchema{
				"id": {Type: "number", Description: "Product numeric ID"},
			},
		},
	},
}

func (h *CatalogMCPHandler) ListTools() []ToolDefinition {
	return h.tools
}

func (h *CatalogMCPHandler) HandleTool(ctx context.Context, name string, raw json.RawMessage) (any, error) {
	switch name {
	case "search_catalog":
		return h.callSearch(ctx, raw)
	case "lookup_catalog":
		return h.callLookup(ctx, raw)
	case "get_product":
		return h.callGetProduct(ctx, raw)
	default:
		return nil, kernel.NewDomainError(kernel.ErrInvalidArgument, "unknown tool: "+name)
	}
}

type searchArgs struct {
	Query    string `json:"query"`
	Category string `json:"category"`
	MinPrice *int64 `json:"min_price"`
	MaxPrice *int64 `json:"max_price"`
	Cursor   string `json:"cursor"`
	Limit    int    `json:"limit"`
}

func (h *CatalogMCPHandler) callSearch(ctx context.Context, raw json.RawMessage) (any, error) {
	var args searchArgs
	if err := json.Unmarshal(raw, &args); err != nil {
		return nil, kernel.NewDomainError(kernel.ErrInvalidArgument, "invalid arguments")
	}

	opts := domain.SearchOptions{
		Category:      args.Category,
		Cursor:        domain.Cursor(args.Cursor),
		Limit:         args.Limit,
		FulltextQuery: args.Query,
	}
	if args.MinPrice != nil {
		opts.MinPrice = *args.MinPrice
	}
	if args.MaxPrice != nil {
		opts.MaxPrice = *args.MaxPrice
	}

	result, err := h.svc.Search(ctx, opts.FulltextQuery, opts)
	if err != nil {
		return nil, err
	}

	products := make([]map[string]any, len(result.Products))
	for i, p := range result.Products {
		products[i] = productToMap(p)
	}

	return map[string]any{
		"products":    products,
		"next_cursor": string(result.NextCursor),
		"has_more":    result.HasMore,
	}, nil
}

type lookupArgs struct {
	SKU string `json:"sku"`
}

func (h *CatalogMCPHandler) callLookup(ctx context.Context, raw json.RawMessage) (any, error) {
	var args lookupArgs
	if err := json.Unmarshal(raw, &args); err != nil {
		return nil, kernel.NewDomainError(kernel.ErrInvalidArgument, "invalid arguments")
	}
	if args.SKU == "" {
		return nil, kernel.NewDomainError(kernel.ErrInvalidArgument, "sku is required")
	}

	p, err := h.svc.Lookup(ctx, domain.SKU(args.SKU))
	if err != nil {
		return nil, err
	}

	return productToMap(p), nil
}

type getProductArgs struct {
	ID int64 `json:"id"`
}

func (h *CatalogMCPHandler) callGetProduct(ctx context.Context, raw json.RawMessage) (any, error) {
	var args getProductArgs
	if err := json.Unmarshal(raw, &args); err != nil {
		return nil, kernel.NewDomainError(kernel.ErrInvalidArgument, "invalid arguments")
	}
	if args.ID <= 0 {
		return nil, kernel.NewDomainError(kernel.ErrInvalidArgument, "id must be positive")
	}

	p, err := h.svc.GetProduct(ctx, kernel.ID(args.ID))
	if err != nil {
		return nil, err
	}

	return productToMap(p), nil
}

func productToMap(p *domain.Product) map[string]any {
	return map[string]any{
		"id":          p.ID.Int64(),
		"sku":         string(p.SKU),
		"name":        p.Name,
		"description": p.Description,
		"category":    p.Category,
		"price": map[string]any{
			"amount":   p.Price.Amount,
			"currency": p.Price.Currency,
		},
		"status":     string(p.Status),
		"attributes": p.Attributes,
	}
}
