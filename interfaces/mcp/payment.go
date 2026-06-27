package mcp

import (
	"context"
	"encoding/json"

	"github.com/beeleelee/mall/domain/kernel"

	domain "github.com/beeleelee/mall/domain/payment"
)

type PaymentMCPHandler struct {
	svc *domain.PaymentService
	sf  *kernel.Snowflake
}

func NewPaymentMCPHandler(svc *domain.PaymentService, sf *kernel.Snowflake) *PaymentMCPHandler {
	return &PaymentMCPHandler{svc: svc, sf: sf}
}

var paymentTools = []ToolDefinition{
	{
		Name:        "create_mandate",
		Description: "Request a new payment mandate",
		InputSchema: InputSchema{
			Type: "object",
			Properties: map[string]PropertySchema{
				"user_id":    {Type: "number", Description: "User ID"},
				"max_amount": {Type: "number", Description: "Maximum amount in cents"},
			},
		},
	},
	{
		Name:        "list_mandates",
		Description: "List all mandates for a user",
		InputSchema: InputSchema{
			Type: "object",
			Properties: map[string]PropertySchema{
				"user_id": {Type: "number", Description: "User ID"},
			},
		},
	},
	{
		Name:        "approve_mandate",
		Description: "Approve a mandate with a signature",
		InputSchema: InputSchema{
			Type: "object",
			Properties: map[string]PropertySchema{
				"id":        {Type: "number", Description: "Mandate ID"},
				"signature": {Type: "string", Description: "Approval signature"},
			},
		},
	},
	{
		Name:        "execute_mandate",
		Description: "Execute a mandate with a payment token",
		InputSchema: InputSchema{
			Type: "object",
			Properties: map[string]PropertySchema{
				"id":    {Type: "number", Description: "Mandate ID"},
				"token": {Type: "string", Description: "Payment token"},
			},
		},
	},
}

func (h *PaymentMCPHandler) ListTools() []ToolDefinition {
	return paymentTools
}

func (h *PaymentMCPHandler) HandleTool(ctx context.Context, name string, raw json.RawMessage) (any, error) {
	switch name {
	case "create_mandate":
		return h.callCreateMandate(ctx, raw)
	case "list_mandates":
		return h.callListMandates(ctx, raw)
	case "approve_mandate":
		return h.callApproveMandate(ctx, raw)
	case "execute_mandate":
		return h.callExecuteMandate(ctx, raw)
	default:
		return nil, kernel.NewDomainError(kernel.ErrInvalidArgument, "unknown tool: "+name)
	}
}

type createMandateArgs struct {
	UserID    int64 `json:"user_id"`
	MaxAmount int64 `json:"max_amount"`
}

func (h *PaymentMCPHandler) callCreateMandate(ctx context.Context, raw json.RawMessage) (any, error) {
	var args createMandateArgs
	if err := json.Unmarshal(raw, &args); err != nil {
		return nil, kernel.NewDomainError(kernel.ErrInvalidArgument, "invalid arguments")
	}
	if args.UserID <= 0 {
		return nil, kernel.NewDomainError(kernel.ErrInvalidArgument, "user_id must be positive")
	}

	id, err := h.sf.NextID()
	if err != nil {
		return nil, err
	}

	m, err := h.svc.RequestMandate(ctx, id, kernel.ID(args.UserID), domain.MandateScope{
		MaxAmount: args.MaxAmount,
	})
	if err != nil {
		return nil, err
	}

	return mandateToMap(m), nil
}

type listMandatesArgs struct {
	UserID int64 `json:"user_id"`
}

func (h *PaymentMCPHandler) callListMandates(ctx context.Context, raw json.RawMessage) (any, error) {
	var args listMandatesArgs
	if err := json.Unmarshal(raw, &args); err != nil {
		return nil, kernel.NewDomainError(kernel.ErrInvalidArgument, "invalid arguments")
	}
	if args.UserID <= 0 {
		return nil, kernel.NewDomainError(kernel.ErrInvalidArgument, "user_id must be positive")
	}

	mandates, err := h.svc.ListUserMandates(ctx, kernel.ID(args.UserID))
	if err != nil {
		return nil, err
	}

	result := make([]map[string]any, len(mandates))
	for i, m := range mandates {
		result[i] = mandateToMap(m)
	}
	return result, nil
}

type approveMandateArgs struct {
	ID        int64  `json:"id"`
	Signature string `json:"signature"`
}

func (h *PaymentMCPHandler) callApproveMandate(ctx context.Context, raw json.RawMessage) (any, error) {
	var args approveMandateArgs
	if err := json.Unmarshal(raw, &args); err != nil {
		return nil, kernel.NewDomainError(kernel.ErrInvalidArgument, "invalid arguments")
	}

	m, err := h.svc.ApproveMandate(ctx, kernel.ID(args.ID), args.Signature)
	if err != nil {
		return nil, err
	}

	return mandateToMap(m), nil
}

type executeMandateArgs struct {
	ID    int64  `json:"id"`
	Token string `json:"token"`
}

func (h *PaymentMCPHandler) callExecuteMandate(ctx context.Context, raw json.RawMessage) (any, error) {
	var args executeMandateArgs
	if err := json.Unmarshal(raw, &args); err != nil {
		return nil, kernel.NewDomainError(kernel.ErrInvalidArgument, "invalid arguments")
	}

	m, err := h.svc.ExecuteMandate(ctx, kernel.ID(args.ID), args.Token)
	if err != nil {
		return nil, err
	}

	return mandateToMap(m), nil
}

func mandateToMap(m *domain.Mandate) map[string]any {
	return map[string]any{
		"id":         m.ID.Int64(),
		"user_id":    m.UserID.Int64(),
		"status":     string(m.Status),
		"max_amount": m.Scope.MaxAmount,
		"signature":  m.Signature,
		"token":      m.Token,
	}
}
