package mcp

import (
	"context"
	"encoding/json"
	"time"

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
				"user_id":     {Type: "number", Description: "User ID"},
				"max_amount":  {Type: "number", Description: "Maximum amount in cents"},
				"merchant_id": {Type: "number", Description: "Merchant ID"},
				"expiry":      {Type: "string", Description: "Expiry date in RFC3339 format (optional, defaults to +1 year)"},
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
		Name:        "get_mandate",
		Description: "Get a mandate by its ID",
		InputSchema: InputSchema{
			Type: "object",
			Properties: map[string]PropertySchema{
				"id": {Type: "number", Description: "Mandate ID"},
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
	{
		Name:        "settle_mandate",
		Description: "Settle a mandate after payment is captured",
		InputSchema: InputSchema{
			Type: "object",
			Properties: map[string]PropertySchema{
				"id": {Type: "number", Description: "Mandate ID"},
			},
		},
	},
	{
		Name:        "exchange_payment_token",
		Description: "Exchange a wallet payment token (e.g. Google Pay, Apple Pay) for a mandate execution",
		InputSchema: InputSchema{
			Type: "object",
			Properties: map[string]PropertySchema{
				"mandate_id": {Type: "number", Description: "Mandate ID"},
				"user_id":    {Type: "number", Description: "User ID"},
				"token":      {Type: "string", Description: "Wallet payment token (encrypted payment data)"},
				"provider":   {Type: "string", Description: "Wallet provider (google_pay, apple_pay, stripe)"},
			},
		},
	},
	{
		Name:        "cancel_mandate",
		Description: "Cancel a mandate",
		InputSchema: InputSchema{
			Type: "object",
			Properties: map[string]PropertySchema{
				"id": {Type: "number", Description: "Mandate ID"},
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
	case "get_mandate":
		return h.callGetMandate(ctx, raw)
	case "approve_mandate":
		return h.callApproveMandate(ctx, raw)
	case "execute_mandate":
		return h.callExecuteMandate(ctx, raw)
	case "settle_mandate":
		return h.callSettleMandate(ctx, raw)
	case "exchange_payment_token":
		return h.callExchangePaymentToken(ctx, raw)
	case "cancel_mandate":
		return h.callCancelMandate(ctx, raw)
	default:
		return nil, kernel.NewDomainError(kernel.ErrInvalidArgument, "unknown tool: "+name)
	}
}

type createMandateArgs struct {
	UserID     int64  `json:"user_id"`
	MaxAmount  int64  `json:"max_amount"`
	MerchantID int64  `json:"merchant_id"`
	Expiry     string `json:"expiry,omitempty"`
}

func (h *PaymentMCPHandler) callCreateMandate(ctx context.Context, raw json.RawMessage) (any, error) {
	var args createMandateArgs
	if err := json.Unmarshal(raw, &args); err != nil {
		return nil, kernel.NewDomainError(kernel.ErrInvalidArgument, "invalid arguments")
	}
	if args.UserID <= 0 {
		return nil, kernel.NewDomainError(kernel.ErrInvalidArgument, "user_id must be positive")
	}
	if args.MerchantID <= 0 {
		return nil, kernel.NewDomainError(kernel.ErrInvalidArgument, "merchant_id must be positive")
	}

	expiry := time.Now().AddDate(1, 0, 0)
	if args.Expiry != "" {
		var err error
		expiry, err = time.Parse(time.RFC3339, args.Expiry)
		if err != nil {
			return nil, kernel.NewDomainError(kernel.ErrInvalidArgument, "invalid expiry format, use RFC3339")
		}
	}

	id, err := h.sf.NextID()
	if err != nil {
		return nil, err
	}

	m, err := h.svc.RequestMandate(ctx, id, kernel.ID(args.UserID), domain.MandateScope{
		MaxAmount:  args.MaxAmount,
		MerchantID: kernel.ID(args.MerchantID),
		Expiry:     expiry,
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

type getMandateArgs struct {
	ID int64 `json:"id"`
}

func (h *PaymentMCPHandler) callGetMandate(ctx context.Context, raw json.RawMessage) (any, error) {
	var args getMandateArgs
	if err := json.Unmarshal(raw, &args); err != nil {
		return nil, kernel.NewDomainError(kernel.ErrInvalidArgument, "invalid arguments")
	}
	if args.ID <= 0 {
		return nil, kernel.NewDomainError(kernel.ErrInvalidArgument, "id must be positive")
	}

	m, err := h.svc.GetMandate(ctx, kernel.ID(args.ID))
	if err != nil {
		return nil, err
	}

	return mandateToMap(m), nil
}

type settleMandateArgs struct {
	ID int64 `json:"id"`
}

func (h *PaymentMCPHandler) callSettleMandate(ctx context.Context, raw json.RawMessage) (any, error) {
	var args settleMandateArgs
	if err := json.Unmarshal(raw, &args); err != nil {
		return nil, kernel.NewDomainError(kernel.ErrInvalidArgument, "invalid arguments")
	}
	if args.ID <= 0 {
		return nil, kernel.NewDomainError(kernel.ErrInvalidArgument, "id must be positive")
	}

	m, err := h.svc.SettleMandate(ctx, kernel.ID(args.ID))
	if err != nil {
		return nil, err
	}

	return mandateToMap(m), nil
}

type exchangePaymentTokenArgs struct {
	MandateID int64  `json:"mandate_id"`
	UserID    int64  `json:"user_id"`
	Token     string `json:"token"`
	Provider  string `json:"provider"`
}

func (h *PaymentMCPHandler) callExchangePaymentToken(ctx context.Context, raw json.RawMessage) (any, error) {
	var args exchangePaymentTokenArgs
	if err := json.Unmarshal(raw, &args); err != nil {
		return nil, kernel.NewDomainError(kernel.ErrInvalidArgument, "invalid arguments")
	}
	if args.MandateID <= 0 {
		return nil, kernel.NewDomainError(kernel.ErrInvalidArgument, "mandate_id must be positive")
	}
	if args.UserID <= 0 {
		return nil, kernel.NewDomainError(kernel.ErrInvalidArgument, "user_id must be positive")
	}
	if args.Token == "" {
		return nil, kernel.NewDomainError(kernel.ErrInvalidArgument, "token must not be empty")
	}
	if args.Provider == "" {
		return nil, kernel.NewDomainError(kernel.ErrInvalidArgument, "provider must not be empty")
	}

	m, err := h.svc.ExchangeWalletToken(ctx, kernel.ID(args.MandateID), args.Token, args.Provider, kernel.ID(args.UserID))
	if err != nil {
		return nil, err
	}

	return mandateToMap(m), nil
}

type cancelMandateArgs struct {
	ID int64 `json:"id"`
}

func (h *PaymentMCPHandler) callCancelMandate(ctx context.Context, raw json.RawMessage) (any, error) {
	var args cancelMandateArgs
	if err := json.Unmarshal(raw, &args); err != nil {
		return nil, kernel.NewDomainError(kernel.ErrInvalidArgument, "invalid arguments")
	}
	if args.ID <= 0 {
		return nil, kernel.NewDomainError(kernel.ErrInvalidArgument, "id must be positive")
	}

	m, err := h.svc.CancelMandate(ctx, kernel.ID(args.ID))
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
