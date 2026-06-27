package mcp

import (
	"context"
	"encoding/json"

	"github.com/beeleelee/mall/domain/kernel"
	domain "github.com/beeleelee/mall/domain/order"
)

type OrderMCPHandler struct {
	svc *domain.OrderService
}

func NewOrderMCPHandler(svc *domain.OrderService) *OrderMCPHandler {
	return &OrderMCPHandler{svc: svc}
}

var orderTools = []ToolDefinition{
	{
		Name:        "list_orders",
		Description: "List all orders for a user",
		InputSchema: InputSchema{
			Type: "object",
			Properties: map[string]PropertySchema{
				"user_id": {Type: "number", Description: "User ID"},
			},
		},
	},
	{
		Name:        "get_order",
		Description: "Get an order by its ID",
		InputSchema: InputSchema{
			Type: "object",
			Properties: map[string]PropertySchema{
				"id": {Type: "number", Description: "Order ID"},
			},
		},
	},
}

func (h *OrderMCPHandler) ListTools() []ToolDefinition {
	return orderTools
}

func (h *OrderMCPHandler) HandleTool(ctx context.Context, name string, raw json.RawMessage) (any, error) {
	switch name {
	case "list_orders":
		return h.callListOrders(ctx, raw)
	case "get_order":
		return h.callGetOrder(ctx, raw)
	default:
		return nil, kernel.NewDomainError(kernel.ErrInvalidArgument, "unknown tool: "+name)
	}
}

type listOrdersArgs struct {
	UserID int64 `json:"user_id"`
}

func (h *OrderMCPHandler) callListOrders(ctx context.Context, raw json.RawMessage) (any, error) {
	var args listOrdersArgs
	if err := json.Unmarshal(raw, &args); err != nil {
		return nil, kernel.NewDomainError(kernel.ErrInvalidArgument, "invalid arguments")
	}
	if args.UserID <= 0 {
		return nil, kernel.NewDomainError(kernel.ErrInvalidArgument, "user_id must be positive")
	}

	orders, err := h.svc.GetOrdersByUser(ctx, kernel.ID(args.UserID))
	if err != nil {
		return nil, err
	}

	result := make([]map[string]any, len(orders))
	for i, o := range orders {
		result[i] = orderToMap(o)
	}
	return result, nil
}

type getOrderArgs struct {
	ID int64 `json:"id"`
}

func (h *OrderMCPHandler) callGetOrder(ctx context.Context, raw json.RawMessage) (any, error) {
	var args getOrderArgs
	if err := json.Unmarshal(raw, &args); err != nil {
		return nil, kernel.NewDomainError(kernel.ErrInvalidArgument, "invalid arguments")
	}
	if args.ID <= 0 {
		return nil, kernel.NewDomainError(kernel.ErrInvalidArgument, "id must be positive")
	}

	order, err := h.svc.GetOrder(ctx, kernel.ID(args.ID))
	if err != nil {
		return nil, err
	}

	return orderToMap(order), nil
}

func orderToMap(order *domain.Order) map[string]any {
	items := make([]map[string]any, len(order.Items))
	for i, item := range order.Items {
		items[i] = map[string]any{
			"product_id":  item.ProductID.Int64(),
			"sku":         item.SKU,
			"name":        item.Name,
			"quantity":    item.Quantity,
			"unit_price":  item.UnitPrice,
			"total_price": item.TotalPrice,
		}
	}

	return map[string]any{
		"id":          order.ID.Int64(),
		"user_id":     order.UserID.Int64(),
		"checkout_id": order.CheckoutID.Int64(),
		"cart_id":     order.CartID.Int64(),
		"items":       items,
		"shipping_address": map[string]any{
			"line1":       order.ShippingAddress.Line1,
			"line2":       order.ShippingAddress.Line2,
			"city":        order.ShippingAddress.City,
			"state":       order.ShippingAddress.State,
			"postal_code": order.ShippingAddress.PostalCode,
			"country":     order.ShippingAddress.Country,
		},
		"billing_address": map[string]any{
			"line1":       order.BillingAddress.Line1,
			"line2":       order.BillingAddress.Line2,
			"city":        order.BillingAddress.City,
			"state":       order.BillingAddress.State,
			"postal_code": order.BillingAddress.PostalCode,
			"country":     order.BillingAddress.Country,
		},
		"shipping_option": map[string]any{
			"id":        order.ShippingOption.ID,
			"name":      order.ShippingOption.Name,
			"cost":      order.ShippingOption.Cost,
			"estimated": order.ShippingOption.Estimated,
		},
		"payment_handler": order.PaymentHandler,
		"subtotal":        order.Subtotal,
		"shipping_cost":   order.ShippingCost,
		"tax_amount":      order.TaxAmount,
		"grand_total":     order.GrandTotal,
		"status":          string(order.Status),
		"tracking_number": order.TrackingNumber,
		"carrier":         order.Carrier,
	}
}
