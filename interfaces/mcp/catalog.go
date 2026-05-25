package mcp

import (
	"context"
	"encoding/json"
	"net/http"

	domain "github.com/beeleelee/mall/domain/catalog"
	"github.com/beeleelee/mall/domain/kernel"
)

type CatalogMCPHandler struct {
	svc *domain.CatalogService
}

func NewCatalogMCPHandler(svc *domain.CatalogService) *CatalogMCPHandler {
	return &CatalogMCPHandler{svc: svc}
}

type jsonRPCRequest struct {
	JSONRPC string          `json:"jsonrpc"`
	Method  string          `json:"method"`
	Params  json.RawMessage `json:"params,omitempty"`
	ID      json.RawMessage `json:"id"`
}

type jsonRPCResponse struct {
	JSONRPC string    `json:"jsonrpc"`
	Result  any       `json:"result,omitempty"`
	Error   *rpcError `json:"error,omitempty"`
	ID      any       `json:"id"`
}

type rpcError struct {
	Code    int    `json:"code"`
	Message string `json:"message"`
	Data    any    `json:"data,omitempty"`
}

type toolDefinition struct {
	Name        string      `json:"name"`
	Description string      `json:"description"`
	InputSchema inputSchema `json:"inputSchema"`
}

type inputSchema struct {
	Type       string                    `json:"type"`
	Properties map[string]propertySchema `json:"properties"`
}

type propertySchema struct {
	Type        string   `json:"type"`
	Description string   `json:"description"`
	Enum        []string `json:"enum,omitempty"`
}

type toolCallParams struct {
	Name      string          `json:"name"`
	Arguments json.RawMessage `json:"arguments"`
}

type toolContent struct {
	Type string `json:"type"`
	Text string `json:"text"`
}

var tools = []toolDefinition{
	{
		Name:        "search_catalog",
		Description: "Search for products by query, category, or price range",
		InputSchema: inputSchema{
			Type: "object",
			Properties: map[string]propertySchema{
				"query":     {Type: "string", Description: "Search query (matches name, description, SKU)"},
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
		InputSchema: inputSchema{
			Type: "object",
			Properties: map[string]propertySchema{
				"sku": {Type: "string", Description: "Product SKU (stock keeping unit)"},
			},
		},
	},
	{
		Name:        "get_product",
		Description: "Get a product by its numeric ID",
		InputSchema: inputSchema{
			Type: "object",
			Properties: map[string]propertySchema{
				"id": {Type: "number", Description: "Product numeric ID"},
			},
		},
	},
}

func (h *CatalogMCPHandler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		writeRPCError(w, nil, -32600, "only POST is accepted")
		return
	}

	var req jsonRPCRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeRPCError(w, nil, -32700, "parse error: invalid JSON")
		return
	}

	if req.JSONRPC != "2.0" {
		writeRPCError(w, req.ID, -32600, "invalid jsonrpc version")
		return
	}

	switch req.Method {
	case "tools/list":
		h.handleListTools(w, req)
	case "tools/call":
		h.handleCallTool(w, r, req)
	default:
		writeRPCError(w, req.ID, -32601, "method not found: "+req.Method)
	}
}

func (h *CatalogMCPHandler) handleListTools(w http.ResponseWriter, req jsonRPCRequest) {
	writeRPCResult(w, req.ID, map[string]any{
		"tools": tools,
	})
}

func (h *CatalogMCPHandler) handleCallTool(w http.ResponseWriter, r *http.Request, req jsonRPCRequest) {
	var params toolCallParams
	if err := json.Unmarshal(req.Params, &params); err != nil {
		writeRPCError(w, req.ID, -32602, "invalid params")
		return
	}

	var result any
	var err error

	ctx := r.Context()

	switch params.Name {
	case "search_catalog":
		result, err = h.callSearch(ctx, params.Arguments)
	case "lookup_catalog":
		result, err = h.callLookup(ctx, params.Arguments)
	case "get_product":
		result, err = h.callGetProduct(ctx, params.Arguments)
	default:
		writeRPCError(w, req.ID, -32601, "tool not found: "+params.Name)
		return
	}

	if err != nil {
		writeRPCError(w, req.ID, -32000, err.Error())
		return
	}

	data, _ := json.Marshal(result)
	writeRPCResult(w, req.ID, map[string]any{
		"content": []toolContent{
			{Type: "text", Text: string(data)},
		},
	})
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
		Category: args.Category,
		Cursor:   domain.Cursor(args.Cursor),
		Limit:    args.Limit,
	}
	if args.MinPrice != nil {
		opts.MinPrice = *args.MinPrice
	}
	if args.MaxPrice != nil {
		opts.MaxPrice = *args.MaxPrice
	}

	result, err := h.svc.Search(ctx, args.Query, opts)
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

func writeRPCResult(w http.ResponseWriter, id any, result any) {
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(jsonRPCResponse{
		JSONRPC: "2.0",
		Result:  result,
		ID:      id,
	})
}

func writeRPCError(w http.ResponseWriter, id any, code int, msg string) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK) // JSON-RPC uses 200 even for errors
	json.NewEncoder(w).Encode(jsonRPCResponse{
		JSONRPC: "2.0",
		Error: &rpcError{
			Code:    code,
			Message: msg,
		},
		ID: id,
	})
}
