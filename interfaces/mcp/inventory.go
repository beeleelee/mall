package mcp

import (
	"context"
	"encoding/json"

	"github.com/beeleelee/mall/domain/kernel"

	domain "github.com/beeleelee/mall/domain/inventory"
)

type InventoryMCPHandler struct {
	svc *domain.InventoryService
	sf  *kernel.Snowflake
}

func NewInventoryMCPHandler(svc *domain.InventoryService, sf *kernel.Snowflake) *InventoryMCPHandler {
	return &InventoryMCPHandler{svc: svc, sf: sf}
}

var inventoryTools = []ToolDefinition{
	{
		Name:        "set_stock",
		Description: "Set the stock level for a product (creates or updates)",
		InputSchema: InputSchema{
			Type: "object",
			Properties: map[string]PropertySchema{
				"product_id":          {Type: "number", Description: "Product ID"},
				"quantity":            {Type: "number", Description: "Stock quantity"},
				"low_stock_threshold": {Type: "number", Description: "Low stock threshold (optional, default 10)"},
			},
		},
	},
	{
		Name:        "get_stock",
		Description: "Get current stock level for a product",
		InputSchema: InputSchema{
			Type: "object",
			Properties: map[string]PropertySchema{
				"product_id": {Type: "number", Description: "Product ID"},
			},
		},
	},
	{
		Name:        "list_low_stock",
		Description: "List all products with stock below threshold",
		InputSchema: InputSchema{
			Type: "object",
			Properties: map[string]PropertySchema{
				"threshold": {Type: "number", Description: "Stock threshold (optional, default 10)"},
			},
		},
	},
}

func (h *InventoryMCPHandler) ListTools() []ToolDefinition {
	return inventoryTools
}

func (h *InventoryMCPHandler) HandleTool(ctx context.Context, name string, raw json.RawMessage) (any, error) {
	switch name {
	case "set_stock":
		return h.callSetStock(ctx, raw)
	case "get_stock":
		return h.callGetStock(ctx, raw)
	case "list_low_stock":
		return h.callListLowStock(ctx, raw)
	default:
		return nil, kernel.NewDomainError(kernel.ErrInvalidArgument, "unknown tool: "+name)
	}
}

type setStockArgs struct {
	ProductID         int64 `json:"product_id"`
	Quantity          int   `json:"quantity"`
	LowStockThreshold int   `json:"low_stock_threshold"`
}

func (h *InventoryMCPHandler) callSetStock(ctx context.Context, raw json.RawMessage) (any, error) {
	var args setStockArgs
	if err := json.Unmarshal(raw, &args); err != nil {
		return nil, kernel.NewDomainError(kernel.ErrInvalidArgument, "invalid arguments")
	}
	if args.ProductID <= 0 {
		return nil, kernel.NewDomainError(kernel.ErrInvalidArgument, "product_id must be positive")
	}

	id, err := h.sf.NextID()
	if err != nil {
		return nil, err
	}

	item, err := h.svc.SetStock(ctx, id, kernel.ID(args.ProductID), args.Quantity, args.LowStockThreshold)
	if err != nil {
		return nil, err
	}

	return inventoryItemToMap(item), nil
}

type getStockArgs struct {
	ProductID int64 `json:"product_id"`
}

func (h *InventoryMCPHandler) callGetStock(ctx context.Context, raw json.RawMessage) (any, error) {
	var args getStockArgs
	if err := json.Unmarshal(raw, &args); err != nil {
		return nil, kernel.NewDomainError(kernel.ErrInvalidArgument, "invalid arguments")
	}
	if args.ProductID <= 0 {
		return nil, kernel.NewDomainError(kernel.ErrInvalidArgument, "product_id must be positive")
	}

	item, err := h.svc.GetStock(ctx, kernel.ID(args.ProductID))
	if err != nil {
		return nil, err
	}

	return inventoryItemToMap(item), nil
}

type listLowStockArgs struct {
	Threshold int `json:"threshold"`
}

func (h *InventoryMCPHandler) callListLowStock(ctx context.Context, raw json.RawMessage) (any, error) {
	var args listLowStockArgs
	if err := json.Unmarshal(raw, &args); err != nil {
		return nil, kernel.NewDomainError(kernel.ErrInvalidArgument, "invalid arguments")
	}

	items, err := h.svc.ListLowStock(ctx, args.Threshold)
	if err != nil {
		return nil, err
	}

	result := make([]map[string]any, len(items))
	for i, item := range items {
		result[i] = inventoryItemToMap(item)
	}
	return result, nil
}

func inventoryItemToMap(item *domain.InventoryItem) map[string]any {
	return map[string]any{
		"product_id":          item.ProductID.Int64(),
		"quantity":            item.QuantityAvailable,
		"reserved":            item.ReservedQuantity,
		"available":           item.AvailableQuantity(),
		"low_stock_threshold": item.LowStockThreshold,
		"is_low_stock":        item.IsLowStock(),
	}
}
