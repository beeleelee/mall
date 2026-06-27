package mcp

import (
	"context"
	"encoding/json"

	domain "github.com/beeleelee/mall/domain/checkout"
	"github.com/beeleelee/mall/domain/kernel"
)

type CheckoutMCPHandler struct {
	svc *domain.CheckoutService
	sf  *kernel.Snowflake
}

func NewCheckoutMCPHandler(svc *domain.CheckoutService, sf *kernel.Snowflake) *CheckoutMCPHandler {
	return &CheckoutMCPHandler{svc: svc, sf: sf}
}

var checkoutTools = []ToolDefinition{
	{
		Name:        "create_checkout",
		Description: "Create a new checkout session from cart items",
		InputSchema: InputSchema{
			Type: "object",
			Properties: map[string]PropertySchema{
				"user_id": {Type: "number", Description: "User ID"},
				"cart_id": {Type: "number", Description: "Cart ID"},
				"items":   {Type: "string", Description: "JSON array of {product_id, sku, name, quantity, unit_price, image_url}"},
			},
		},
	},
	{
		Name:        "get_checkout",
		Description: "Get a checkout session by ID",
		InputSchema: InputSchema{
			Type: "object",
			Properties: map[string]PropertySchema{
				"id": {Type: "number", Description: "Checkout ID"},
			},
		},
	},
	{
		Name:        "set_shipping_address",
		Description: "Set the shipping address on a checkout",
		InputSchema: InputSchema{
			Type: "object",
			Properties: map[string]PropertySchema{
				"id":          {Type: "number", Description: "Checkout ID"},
				"line1":       {Type: "string", Description: "Address line 1"},
				"line2":       {Type: "string", Description: "Address line 2 (optional)"},
				"city":        {Type: "string", Description: "City"},
				"state":       {Type: "string", Description: "State or province"},
				"postal_code": {Type: "string", Description: "Postal code"},
				"country":     {Type: "string", Description: "Country code"},
			},
		},
	},
	{
		Name:        "set_billing_address",
		Description: "Set the billing address on a checkout",
		InputSchema: InputSchema{
			Type: "object",
			Properties: map[string]PropertySchema{
				"id":          {Type: "number", Description: "Checkout ID"},
				"line1":       {Type: "string", Description: "Address line 1"},
				"line2":       {Type: "string", Description: "Address line 2 (optional)"},
				"city":        {Type: "string", Description: "City"},
				"state":       {Type: "string", Description: "State or province"},
				"postal_code": {Type: "string", Description: "Postal code"},
				"country":     {Type: "string", Description: "Country code"},
			},
		},
	},
	{
		Name:        "select_shipping_option",
		Description: "Select a shipping option for the checkout",
		InputSchema: InputSchema{
			Type: "object",
			Properties: map[string]PropertySchema{
				"id":        {Type: "number", Description: "Checkout ID"},
				"option_id": {Type: "string", Description: "Shipping option identifier"},
				"name":      {Type: "string", Description: "Shipping option name"},
				"cost":      {Type: "number", Description: "Shipping cost in cents"},
				"estimated": {Type: "string", Description: "Estimated delivery time (optional)"},
			},
		},
	},
	{
		Name:        "select_payment_handler",
		Description: "Select a payment handler for the checkout",
		InputSchema: InputSchema{
			Type: "object",
			Properties: map[string]PropertySchema{
				"id":      {Type: "number", Description: "Checkout ID"},
				"handler": {Type: "string", Description: "Payment handler identifier (e.g. ap2_mandate, stripe)"},
			},
		},
	},
	{
		Name:        "complete_checkout",
		Description: "Complete a checkout session",
		InputSchema: InputSchema{
			Type: "object",
			Properties: map[string]PropertySchema{
				"id":           {Type: "number", Description: "Checkout ID"},
				"continue_url": {Type: "string", Description: "URL for continued payment flow (optional)"},
			},
		},
	},
	{
		Name:        "cancel_checkout",
		Description: "Cancel a checkout session",
		InputSchema: InputSchema{
			Type: "object",
			Properties: map[string]PropertySchema{
				"id": {Type: "number", Description: "Checkout ID"},
			},
		},
	},
}

func (h *CheckoutMCPHandler) ListTools() []ToolDefinition {
	return checkoutTools
}

func (h *CheckoutMCPHandler) HandleTool(ctx context.Context, name string, raw json.RawMessage) (any, error) {
	switch name {
	case "create_checkout":
		return h.callCreate(ctx, raw)
	case "get_checkout":
		return h.callGetCheckout(ctx, raw)
	case "set_shipping_address":
		return h.callSetShippingAddress(ctx, raw)
	case "set_billing_address":
		return h.callSetBillingAddress(ctx, raw)
	case "select_shipping_option":
		return h.callSelectShippingOption(ctx, raw)
	case "select_payment_handler":
		return h.callSelectPaymentHandler(ctx, raw)
	case "complete_checkout":
		return h.callComplete(ctx, raw)
	case "cancel_checkout":
		return h.callCancel(ctx, raw)
	default:
		return nil, kernel.NewDomainError(kernel.ErrInvalidArgument, "unknown tool: "+name)
	}
}

type createCheckoutArgs struct {
	UserID int64  `json:"user_id"`
	CartID int64  `json:"cart_id"`
	Items  string `json:"items"`
}

func (h *CheckoutMCPHandler) callCreate(ctx context.Context, raw json.RawMessage) (any, error) {
	var args createCheckoutArgs
	if err := json.Unmarshal(raw, &args); err != nil {
		return nil, kernel.NewDomainError(kernel.ErrInvalidArgument, "invalid arguments")
	}
	if args.UserID <= 0 {
		return nil, kernel.NewDomainError(kernel.ErrInvalidArgument, "user_id must be positive")
	}
	if args.CartID <= 0 {
		return nil, kernel.NewDomainError(kernel.ErrInvalidArgument, "cart_id must be positive")
	}

	var items []domain.CartSnapshotItem
	if err := json.Unmarshal([]byte(args.Items), &items); err != nil {
		return nil, kernel.NewDomainError(kernel.ErrInvalidArgument, "items must be valid JSON array")
	}

	checkoutID, err := h.sf.NextID()
	if err != nil {
		return nil, err
	}

	session, err := h.svc.CreateCheckout(ctx, domain.CreateCheckoutInput{
		CheckoutID: checkoutID,
		UserID:     kernel.ID(args.UserID),
		CartID:     kernel.ID(args.CartID),
		CartItems:  items,
	})
	if err != nil {
		return nil, err
	}

	return checkoutToMap(session), nil
}

type getCheckoutArgs struct {
	ID int64 `json:"id"`
}

func (h *CheckoutMCPHandler) callGetCheckout(ctx context.Context, raw json.RawMessage) (any, error) {
	var args getCheckoutArgs
	if err := json.Unmarshal(raw, &args); err != nil {
		return nil, kernel.NewDomainError(kernel.ErrInvalidArgument, "invalid arguments")
	}
	if args.ID <= 0 {
		return nil, kernel.NewDomainError(kernel.ErrInvalidArgument, "id must be positive")
	}

	session, err := h.svc.GetCheckout(ctx, kernel.ID(args.ID))
	if err != nil {
		return nil, err
	}

	return checkoutToMap(session), nil
}

type setAddressArgs struct {
	ID         int64  `json:"id"`
	Line1      string `json:"line1"`
	Line2      string `json:"line2,omitempty"`
	City       string `json:"city"`
	State      string `json:"state"`
	PostalCode string `json:"postal_code"`
	Country    string `json:"country"`
}

func (h *CheckoutMCPHandler) callSetShippingAddress(ctx context.Context, raw json.RawMessage) (any, error) {
	var args setAddressArgs
	if err := json.Unmarshal(raw, &args); err != nil {
		return nil, kernel.NewDomainError(kernel.ErrInvalidArgument, "invalid arguments")
	}

	session, err := h.svc.SetShippingAddress(ctx, kernel.ID(args.ID), domain.Address{
		Line1:      args.Line1,
		Line2:      args.Line2,
		City:       args.City,
		State:      args.State,
		PostalCode: args.PostalCode,
		Country:    args.Country,
	})
	if err != nil {
		return nil, err
	}

	return checkoutToMap(session), nil
}

func (h *CheckoutMCPHandler) callSetBillingAddress(ctx context.Context, raw json.RawMessage) (any, error) {
	var args setAddressArgs
	if err := json.Unmarshal(raw, &args); err != nil {
		return nil, kernel.NewDomainError(kernel.ErrInvalidArgument, "invalid arguments")
	}

	session, err := h.svc.SetBillingAddress(ctx, kernel.ID(args.ID), domain.Address{
		Line1:      args.Line1,
		Line2:      args.Line2,
		City:       args.City,
		State:      args.State,
		PostalCode: args.PostalCode,
		Country:    args.Country,
	})
	if err != nil {
		return nil, err
	}

	return checkoutToMap(session), nil
}

type selectShippingArgs struct {
	ID        int64  `json:"id"`
	OptionID  string `json:"option_id"`
	Name      string `json:"name"`
	Cost      int64  `json:"cost"`
	Estimated string `json:"estimated,omitempty"`
}

func (h *CheckoutMCPHandler) callSelectShippingOption(ctx context.Context, raw json.RawMessage) (any, error) {
	var args selectShippingArgs
	if err := json.Unmarshal(raw, &args); err != nil {
		return nil, kernel.NewDomainError(kernel.ErrInvalidArgument, "invalid arguments")
	}

	session, err := h.svc.SelectShippingOption(ctx, kernel.ID(args.ID), domain.ShippingOption{
		ID:        args.OptionID,
		Name:      args.Name,
		Cost:      args.Cost,
		Estimated: args.Estimated,
	})
	if err != nil {
		return nil, err
	}

	return checkoutToMap(session), nil
}

type selectPaymentArgs struct {
	ID      int64  `json:"id"`
	Handler string `json:"handler"`
}

func (h *CheckoutMCPHandler) callSelectPaymentHandler(ctx context.Context, raw json.RawMessage) (any, error) {
	var args selectPaymentArgs
	if err := json.Unmarshal(raw, &args); err != nil {
		return nil, kernel.NewDomainError(kernel.ErrInvalidArgument, "invalid arguments")
	}

	session, err := h.svc.SelectPaymentHandler(ctx, kernel.ID(args.ID), args.Handler)
	if err != nil {
		return nil, err
	}

	return checkoutToMap(session), nil
}

type completeCheckoutArgs struct {
	ID          int64  `json:"id"`
	ContinueURL string `json:"continue_url,omitempty"`
}

func (h *CheckoutMCPHandler) callComplete(ctx context.Context, raw json.RawMessage) (any, error) {
	var args completeCheckoutArgs
	if err := json.Unmarshal(raw, &args); err != nil {
		return nil, kernel.NewDomainError(kernel.ErrInvalidArgument, "invalid arguments")
	}

	session, escalated, err := h.svc.StartComplete(ctx, kernel.ID(args.ID), args.ContinueURL)
	if err != nil {
		return nil, err
	}

	result := checkoutToMap(session)
	result["escalated"] = escalated
	return result, nil
}

type cancelCheckoutArgs struct {
	ID int64 `json:"id"`
}

func (h *CheckoutMCPHandler) callCancel(ctx context.Context, raw json.RawMessage) (any, error) {
	var args cancelCheckoutArgs
	if err := json.Unmarshal(raw, &args); err != nil {
		return nil, kernel.NewDomainError(kernel.ErrInvalidArgument, "invalid arguments")
	}

	session, err := h.svc.Cancel(ctx, kernel.ID(args.ID))
	if err != nil {
		return nil, err
	}

	return checkoutToMap(session), nil
}

func checkoutToMap(session *domain.CheckoutSession) map[string]any {
	items := make([]map[string]any, len(session.CartSnapshot.Items))
	for i, item := range session.CartSnapshot.Items {
		items[i] = map[string]any{
			"product_id": item.ProductID.Int64(),
			"sku":        item.SKU,
			"name":       item.Name,
			"quantity":   item.Quantity,
			"unit_price": item.UnitPrice,
			"image_url":  item.ImageURL,
		}
	}

	var sa map[string]any
	if session.ShippingAddress != nil {
		sa = map[string]any{
			"line1":       session.ShippingAddress.Line1,
			"line2":       session.ShippingAddress.Line2,
			"city":        session.ShippingAddress.City,
			"state":       session.ShippingAddress.State,
			"postal_code": session.ShippingAddress.PostalCode,
			"country":     session.ShippingAddress.Country,
		}
	}

	var ba map[string]any
	if session.BillingAddress != nil {
		ba = map[string]any{
			"line1":       session.BillingAddress.Line1,
			"line2":       session.BillingAddress.Line2,
			"city":        session.BillingAddress.City,
			"state":       session.BillingAddress.State,
			"postal_code": session.BillingAddress.PostalCode,
			"country":     session.BillingAddress.Country,
		}
	}

	var so map[string]any
	if session.ShippingOption != nil {
		so = map[string]any{
			"id":        session.ShippingOption.ID,
			"name":      session.ShippingOption.Name,
			"cost":      session.ShippingOption.Cost,
			"estimated": session.ShippingOption.Estimated,
		}
	}

	var completedAt *int64
	if session.CompletedAt != nil {
		t := session.CompletedAt.UnixMilli()
		completedAt = &t
	}

	return map[string]any{
		"id":               session.ID.Int64(),
		"user_id":          session.UserID.Int64(),
		"cart_id":          session.CartID.Int64(),
		"items":            items,
		"shipping_address": sa,
		"billing_address":  ba,
		"shipping_option":  so,
		"payment_handler":  session.PaymentHandler,
		"mandate_id":       session.MandateID.Int64(),
		"subtotal":         session.Subtotal,
		"shipping_cost":    session.ShippingCost,
		"tax_amount":       session.TaxAmount,
		"grand_total":      session.GrandTotal,
		"status":           string(session.Status),
		"continue_url":     session.ContinueURL,
		"completed_at":     completedAt,
		"created_at":       session.CreatedAt.UnixMilli(),
		"updated_at":       session.UpdatedAt.UnixMilli(),
	}
}
