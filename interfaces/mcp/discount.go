package mcp

import (
	"context"
	"encoding/json"
	"time"

	"github.com/beeleelee/mall/domain/kernel"

	domain "github.com/beeleelee/mall/domain/discount"
)

type DiscountMCPHandler struct {
	svc *domain.DiscountService
	sf  *kernel.Snowflake
}

func NewDiscountMCPHandler(svc *domain.DiscountService, sf *kernel.Snowflake) *DiscountMCPHandler {
	return &DiscountMCPHandler{svc: svc, sf: sf}
}

var discountTools = []ToolDefinition{
	{
		Name:        "create_discount_code",
		Description: "Create a new discount code",
		InputSchema: InputSchema{
			Type: "object",
			Properties: map[string]PropertySchema{
				"code":          {Type: "string", Description: "Discount code string"},
				"discount_type": {Type: "string", Description: "Discount type: percentage or fixed", Enum: []string{"percentage", "fixed"}},
				"value":         {Type: "number", Description: "Discount value (percentage or amount in cents)"},
				"min_purchase":  {Type: "number", Description: "Minimum purchase amount in cents (optional)"},
				"max_usages":    {Type: "number", Description: "Maximum number of usages (optional)"},
				"expiry":        {Type: "string", Description: "Expiry date in RFC3339 format (optional)"},
				"stackable":     {Type: "string", Description: "Whether this code can be stacked (optional, default false)"},
			},
		},
	},
	{
		Name:        "validate_discount_code",
		Description: "Check if a discount code is valid for a given subtotal",
		InputSchema: InputSchema{
			Type: "object",
			Properties: map[string]PropertySchema{
				"code":     {Type: "string", Description: "Discount code"},
				"subtotal": {Type: "number", Description: "Subtotal amount in cents"},
			},
		},
	},
	{
		Name:        "apply_discount_code",
		Description: "Apply a discount code and get the final amount",
		InputSchema: InputSchema{
			Type: "object",
			Properties: map[string]PropertySchema{
				"code":     {Type: "string", Description: "Discount code"},
				"subtotal": {Type: "number", Description: "Subtotal amount in cents"},
			},
		},
	},
	{
		Name:        "deactivate_discount_code",
		Description: "Deactivate an existing discount code",
		InputSchema: InputSchema{
			Type: "object",
			Properties: map[string]PropertySchema{
				"code": {Type: "string", Description: "Discount code to deactivate"},
			},
		},
	},
}

func (h *DiscountMCPHandler) ListTools() []ToolDefinition {
	return discountTools
}

func (h *DiscountMCPHandler) HandleTool(ctx context.Context, name string, raw json.RawMessage) (any, error) {
	switch name {
	case "create_discount_code":
		return h.callCreateCode(ctx, raw)
	case "validate_discount_code":
		return h.callValidateCode(ctx, raw)
	case "apply_discount_code":
		return h.callApplyCode(ctx, raw)
	case "deactivate_discount_code":
		return h.callDeactivateCode(ctx, raw)
	default:
		return nil, kernel.NewDomainError(kernel.ErrInvalidArgument, "unknown tool: "+name)
	}
}

type createDiscountArgs struct {
	Code         string `json:"code"`
	DiscountType string `json:"discount_type"`
	Value        int64  `json:"value"`
	MinPurchase  int64  `json:"min_purchase"`
	MaxUsages    int    `json:"max_usages"`
	Expiry       string `json:"expiry"`
	Stackable    string `json:"stackable"`
}

func (h *DiscountMCPHandler) callCreateCode(ctx context.Context, raw json.RawMessage) (any, error) {
	var args createDiscountArgs
	if err := json.Unmarshal(raw, &args); err != nil {
		return nil, kernel.NewDomainError(kernel.ErrInvalidArgument, "invalid arguments")
	}
	if args.Code == "" {
		return nil, kernel.NewDomainError(kernel.ErrInvalidArgument, "code is required")
	}

	var dt domain.DiscountType
	switch args.DiscountType {
	case "percentage":
		dt = domain.DiscountTypePercentage
	case "fixed":
		dt = domain.DiscountTypeFlat
	default:
		return nil, kernel.NewDomainError(kernel.ErrInvalidArgument, "discount_type must be percentage or fixed")
	}

	var expiry time.Time
	if args.Expiry != "" {
		var err error
		expiry, err = time.Parse(time.RFC3339, args.Expiry)
		if err != nil {
			return nil, kernel.NewDomainError(kernel.ErrInvalidArgument, "invalid expiry format, use RFC3339")
		}
	}

	stackable := args.Stackable == "true"

	id, err := h.sf.NextID()
	if err != nil {
		return nil, err
	}

	dc, err := h.svc.CreateCode(ctx, id, args.Code, dt, args.Value, args.MinPurchase, args.MaxUsages, expiry, stackable)
	if err != nil {
		return nil, err
	}

	return map[string]any{
		"code":          dc.Code,
		"discount_type": string(dc.Type),
		"value":         dc.Value,
		"min_purchase":  dc.MinPurchase,
		"max_usages":    dc.MaxUsages,
		"usage_count":   dc.UsedCount,
		"active":        dc.Active,
		"stackable":     dc.Stackable,
	}, nil
}

type validateDiscountArgs struct {
	Code     string `json:"code"`
	Subtotal int64  `json:"subtotal"`
}

func (h *DiscountMCPHandler) callValidateCode(ctx context.Context, raw json.RawMessage) (any, error) {
	var args validateDiscountArgs
	if err := json.Unmarshal(raw, &args); err != nil {
		return nil, kernel.NewDomainError(kernel.ErrInvalidArgument, "invalid arguments")
	}

	dc, valid := h.svc.ValidateCode(ctx, args.Code, args.Subtotal)
	if dc == nil {
		return map[string]any{
			"valid":  false,
			"reason": "code not found",
		}, nil
	}
	if !valid {
		return map[string]any{
			"valid":  false,
			"reason": "code expired, maxed out, or below minimum purchase",
		}, nil
	}

	return map[string]any{
		"valid":         true,
		"code":          dc.Code,
		"discount_type": string(dc.Type),
		"value":         dc.Value,
		"stackable":     dc.Stackable,
	}, nil
}

func (h *DiscountMCPHandler) callApplyCode(ctx context.Context, raw json.RawMessage) (any, error) {
	var args validateDiscountArgs
	if err := json.Unmarshal(raw, &args); err != nil {
		return nil, kernel.NewDomainError(kernel.ErrInvalidArgument, "invalid arguments")
	}

	finalAmount, applied, err := h.svc.ApplyCode(ctx, args.Code, args.Subtotal)
	if err != nil {
		return nil, err
	}

	return map[string]any{
		"applied":      applied,
		"original":     args.Subtotal,
		"final_amount": finalAmount,
	}, nil
}

func (h *DiscountMCPHandler) callDeactivateCode(ctx context.Context, raw json.RawMessage) (any, error) {
	var args validateDiscountArgs
	if err := json.Unmarshal(raw, &args); err != nil {
		return nil, kernel.NewDomainError(kernel.ErrInvalidArgument, "invalid arguments")
	}

	if err := h.svc.DeactivateCode(ctx, args.Code); err != nil {
		return nil, err
	}

	return map[string]any{
		"code":   args.Code,
		"active": false,
	}, nil
}
