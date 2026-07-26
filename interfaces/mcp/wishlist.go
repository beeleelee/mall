package mcp

import (
	"context"
	"encoding/json"

	"github.com/beeleelee/mall/domain/kernel"
	domain "github.com/beeleelee/mall/domain/wishlist"
)

type WishlistMCPHandler struct {
	svc   *domain.WishlistService
	tools []ToolDefinition
}

func NewWishlistMCPHandler(svc *domain.WishlistService) *WishlistMCPHandler {
	return &WishlistMCPHandler{
		svc:   svc,
		tools: wishlistTools,
	}
}

var wishlistTools = []ToolDefinition{
	{
		Name:        "get_wishlist",
		Description: "Get the wishlist for the current user",
		InputSchema: InputSchema{
			Type:       "object",
			Properties: map[string]PropertySchema{},
		},
	},
	{
		Name:        "add_wishlist_item",
		Description: "Add a product to the wishlist",
		InputSchema: InputSchema{
			Type: "object",
			Properties: map[string]PropertySchema{
				"product_id": {Type: "number", Description: "Product ID to add"},
			},
		},
	},
	{
		Name:        "remove_wishlist_item",
		Description: "Remove a product from the wishlist",
		InputSchema: InputSchema{
			Type: "object",
			Properties: map[string]PropertySchema{
				"product_id": {Type: "number", Description: "Product ID to remove"},
			},
		},
	},
	{
		Name:        "clear_wishlist",
		Description: "Clear all items from the wishlist",
		InputSchema: InputSchema{
			Type:       "object",
			Properties: map[string]PropertySchema{},
		},
	},
}

func (h *WishlistMCPHandler) ListTools() []ToolDefinition {
	return h.tools
}

func (h *WishlistMCPHandler) HandleTool(ctx context.Context, name string, raw json.RawMessage) (any, error) {
	switch name {
	case "get_wishlist":
		return h.callGet(ctx)
	case "add_wishlist_item":
		return h.callAddItem(ctx, raw)
	case "remove_wishlist_item":
		return h.callRemoveItem(ctx, raw)
	case "clear_wishlist":
		return h.callClear(ctx)
	default:
		return nil, kernel.NewDomainError(kernel.ErrInvalidArgument, "unknown tool: "+name)
	}
}

type wishlistProductIDArgs struct {
	ProductID int64 `json:"product_id"`
}

func buildWishlistResponse(w *domain.Wishlist) map[string]any {
	items := make([]map[string]any, len(w.Items))
	for i, item := range w.Items {
		items[i] = map[string]any{
			"product_id": item.ProductID.Int64(),
			"added_at":   item.AddedAt.Format("2006-01-02T15:04:05Z"),
		}
	}
	return map[string]any{
		"id":     w.ID.Int64(),
		"user_id": w.UserID.Int64(),
		"items":  items,
		"count":  len(items),
	}
}

func (h *WishlistMCPHandler) callGet(ctx context.Context) (any, error) {
	wl, err := h.svc.GetWishlist(ctx, 0)
	if err != nil {
		return nil, err
	}
	return buildWishlistResponse(wl), nil
}

func (h *WishlistMCPHandler) callAddItem(ctx context.Context, raw json.RawMessage) (any, error) {
	var args wishlistProductIDArgs
	if err := json.Unmarshal(raw, &args); err != nil {
		return nil, kernel.NewDomainError(kernel.ErrInvalidArgument, "invalid arguments")
	}
	if args.ProductID <= 0 {
		return nil, kernel.NewDomainError(kernel.ErrInvalidArgument, "product_id is required")
	}
	if err := h.svc.AddItem(ctx, 0, kernel.ID(args.ProductID)); err != nil {
		return nil, err
	}
	return map[string]any{"status": "added", "product_id": args.ProductID}, nil
}

func (h *WishlistMCPHandler) callRemoveItem(ctx context.Context, raw json.RawMessage) (any, error) {
	var args wishlistProductIDArgs
	if err := json.Unmarshal(raw, &args); err != nil {
		return nil, kernel.NewDomainError(kernel.ErrInvalidArgument, "invalid arguments")
	}
	if err := h.svc.RemoveItem(ctx, 0, kernel.ID(args.ProductID)); err != nil {
		return nil, err
	}
	return map[string]any{"status": "removed", "product_id": args.ProductID}, nil
}

func (h *WishlistMCPHandler) callClear(ctx context.Context) (any, error) {
	if err := h.svc.ClearWishlist(ctx, 0); err != nil {
		return nil, err
	}
	return map[string]any{"status": "cleared"}, nil
}
