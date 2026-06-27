package mcp

import (
	"context"
	"encoding/json"

	domain "github.com/beeleelee/mall/domain/cart"
	"github.com/beeleelee/mall/domain/kernel"
)

type CartMCPHandler struct {
	svc *domain.CartService
	sf  *kernel.Snowflake
}

func NewCartMCPHandler(svc *domain.CartService, sf *kernel.Snowflake) *CartMCPHandler {
	return &CartMCPHandler{svc: svc, sf: sf}
}

var cartTools = []ToolDefinition{
	{
		Name:        "get_cart",
		Description: "Get the current cart for a user",
		InputSchema: InputSchema{
			Type: "object",
			Properties: map[string]PropertySchema{
				"user_id": {Type: "number", Description: "User ID"},
			},
		},
	},
	{
		Name:        "add_cart_item",
		Description: "Add an item to a user's cart (creates cart if needed)",
		InputSchema: InputSchema{
			Type: "object",
			Properties: map[string]PropertySchema{
				"user_id":    {Type: "number", Description: "User ID"},
				"product_id": {Type: "number", Description: "Product ID"},
				"sku":        {Type: "string", Description: "Product SKU"},
				"name":       {Type: "string", Description: "Product name"},
				"quantity":   {Type: "number", Description: "Quantity to add"},
				"unit_price": {Type: "number", Description: "Unit price in cents"},
				"image_url":  {Type: "string", Description: "Product image URL (optional)"},
			},
		},
	},
	{
		Name:        "update_cart_item",
		Description: "Update the quantity of an item in the cart",
		InputSchema: InputSchema{
			Type: "object",
			Properties: map[string]PropertySchema{
				"user_id":    {Type: "number", Description: "User ID"},
				"product_id": {Type: "number", Description: "Product ID"},
				"quantity":   {Type: "number", Description: "New quantity"},
			},
		},
	},
	{
		Name:        "remove_cart_item",
		Description: "Remove an item from the cart",
		InputSchema: InputSchema{
			Type: "object",
			Properties: map[string]PropertySchema{
				"user_id":    {Type: "number", Description: "User ID"},
				"product_id": {Type: "number", Description: "Product ID"},
			},
		},
	},
	{
		Name:        "clear_cart",
		Description: "Remove all items from the cart",
		InputSchema: InputSchema{
			Type: "object",
			Properties: map[string]PropertySchema{
				"user_id": {Type: "number", Description: "User ID"},
			},
		},
	},
}

func (h *CartMCPHandler) ListTools() []ToolDefinition {
	return cartTools
}

func (h *CartMCPHandler) HandleTool(ctx context.Context, name string, raw json.RawMessage) (any, error) {
	switch name {
	case "get_cart":
		return h.callGetCart(ctx, raw)
	case "add_cart_item":
		return h.callAddItem(ctx, raw)
	case "update_cart_item":
		return h.callUpdateQuantity(ctx, raw)
	case "remove_cart_item":
		return h.callRemoveItem(ctx, raw)
	case "clear_cart":
		return h.callClearCart(ctx, raw)
	default:
		return nil, kernel.NewDomainError(kernel.ErrInvalidArgument, "unknown tool: "+name)
	}
}

type getCartArgs struct {
	UserID int64 `json:"user_id"`
}

func (h *CartMCPHandler) callGetCart(ctx context.Context, raw json.RawMessage) (any, error) {
	var args getCartArgs
	if err := json.Unmarshal(raw, &args); err != nil {
		return nil, kernel.NewDomainError(kernel.ErrInvalidArgument, "invalid arguments")
	}
	if args.UserID <= 0 {
		return nil, kernel.NewDomainError(kernel.ErrInvalidArgument, "user_id must be positive")
	}

	cart, err := h.svc.GetCart(ctx, kernel.ID(args.UserID))
	if err != nil {
		return nil, err
	}

	return cartToMap(cart), nil
}

type addCartItemArgs struct {
	UserID    int64  `json:"user_id"`
	ProductID int64  `json:"product_id"`
	SKU       string `json:"sku"`
	Name      string `json:"name"`
	Quantity  int    `json:"quantity"`
	UnitPrice int64  `json:"unit_price"`
	ImageURL  string `json:"image_url,omitempty"`
}

func (h *CartMCPHandler) callAddItem(ctx context.Context, raw json.RawMessage) (any, error) {
	var args addCartItemArgs
	if err := json.Unmarshal(raw, &args); err != nil {
		return nil, kernel.NewDomainError(kernel.ErrInvalidArgument, "invalid arguments")
	}
	if args.UserID <= 0 {
		return nil, kernel.NewDomainError(kernel.ErrInvalidArgument, "user_id must be positive")
	}
	if args.ProductID <= 0 {
		return nil, kernel.NewDomainError(kernel.ErrInvalidArgument, "product_id must be positive")
	}

	cartID, err := h.sf.NextID()
	if err != nil {
		return nil, err
	}

	cart, err := h.svc.AddItem(ctx, domain.AddItemInput{
		CartID:    cartID,
		UserID:    kernel.ID(args.UserID),
		ProductID: kernel.ID(args.ProductID),
		SKU:       args.SKU,
		Name:      args.Name,
		Quantity:  args.Quantity,
		UnitPrice: args.UnitPrice,
		ImageURL:  args.ImageURL,
	})
	if err != nil {
		return nil, err
	}

	return cartToMap(cart), nil
}

type updateCartItemArgs struct {
	UserID    int64 `json:"user_id"`
	ProductID int64 `json:"product_id"`
	Quantity  int   `json:"quantity"`
}

func (h *CartMCPHandler) callUpdateQuantity(ctx context.Context, raw json.RawMessage) (any, error) {
	var args updateCartItemArgs
	if err := json.Unmarshal(raw, &args); err != nil {
		return nil, kernel.NewDomainError(kernel.ErrInvalidArgument, "invalid arguments")
	}
	if args.UserID <= 0 {
		return nil, kernel.NewDomainError(kernel.ErrInvalidArgument, "user_id must be positive")
	}

	cart, err := h.svc.UpdateQuantity(ctx, kernel.ID(args.UserID), kernel.ID(args.ProductID), args.Quantity)
	if err != nil {
		return nil, err
	}

	return cartToMap(cart), nil
}

type removeCartItemArgs struct {
	UserID    int64 `json:"user_id"`
	ProductID int64 `json:"product_id"`
}

func (h *CartMCPHandler) callRemoveItem(ctx context.Context, raw json.RawMessage) (any, error) {
	var args removeCartItemArgs
	if err := json.Unmarshal(raw, &args); err != nil {
		return nil, kernel.NewDomainError(kernel.ErrInvalidArgument, "invalid arguments")
	}
	if args.UserID <= 0 {
		return nil, kernel.NewDomainError(kernel.ErrInvalidArgument, "user_id must be positive")
	}

	cart, err := h.svc.RemoveItem(ctx, kernel.ID(args.UserID), kernel.ID(args.ProductID))
	if err != nil {
		return nil, err
	}

	return cartToMap(cart), nil
}

type clearCartArgs struct {
	UserID int64 `json:"user_id"`
}

func (h *CartMCPHandler) callClearCart(ctx context.Context, raw json.RawMessage) (any, error) {
	var args clearCartArgs
	if err := json.Unmarshal(raw, &args); err != nil {
		return nil, kernel.NewDomainError(kernel.ErrInvalidArgument, "invalid arguments")
	}
	if args.UserID <= 0 {
		return nil, kernel.NewDomainError(kernel.ErrInvalidArgument, "user_id must be positive")
	}

	cart, err := h.svc.ClearCart(ctx, kernel.ID(args.UserID))
	if err != nil {
		return nil, err
	}

	return cartToMap(cart), nil
}

func cartToMap(cart *domain.Cart) map[string]any {
	total := cart.GetTotal()
	items := make([]map[string]any, len(cart.Items))
	for i, item := range cart.Items {
		items[i] = map[string]any{
			"product_id": item.ProductID.Int64(),
			"sku":        item.SKU,
			"name":       item.Name,
			"quantity":   item.Quantity,
			"unit_price": item.UnitPrice,
			"image_url":  item.ImageURL,
		}
	}
	return map[string]any{
		"id":         cart.ID.Int64(),
		"user_id":    cart.UserID.Int64(),
		"items":      items,
		"status":     string(cart.Status),
		"item_count": total.ItemCount,
		"subtotal":   total.Subtotal,
	}
}
