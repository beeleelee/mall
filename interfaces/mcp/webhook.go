package mcp

import (
	"context"
	"encoding/json"

	"github.com/beeleelee/mall/domain/kernel"

	domain "github.com/beeleelee/mall/domain/order"
)

type WebhookMCPHandler struct {
	svc *domain.WebhookService
}

func NewWebhookMCPHandler(svc *domain.WebhookService) *WebhookMCPHandler {
	return &WebhookMCPHandler{svc: svc}
}

var webhookTools = []ToolDefinition{
	{
		Name:        "register_webhook",
		Description: "Register a new webhook for order events",
		InputSchema: InputSchema{
			Type: "object",
			Properties: map[string]PropertySchema{
				"user_id": {Type: "number", Description: "User ID"},
				"url":     {Type: "string", Description: "Webhook callback URL"},
				"secret":  {Type: "string", Description: "Secret for HMAC signing"},
				"events":  {Type: "string", Description: "JSON array of event names (e.g. [\"order.confirmed\",\"order.shipped\"])"},
			},
		},
	},
	{
		Name:        "list_webhooks",
		Description: "List all webhooks for a user",
		InputSchema: InputSchema{
			Type: "object",
			Properties: map[string]PropertySchema{
				"user_id": {Type: "number", Description: "User ID"},
			},
		},
	},
	{
		Name:        "delete_webhook",
		Description: "Delete a webhook",
		InputSchema: InputSchema{
			Type: "object",
			Properties: map[string]PropertySchema{
				"user_id": {Type: "number", Description: "User ID"},
				"id":      {Type: "number", Description: "Webhook ID"},
			},
		},
	},
}

func (h *WebhookMCPHandler) ListTools() []ToolDefinition {
	return webhookTools
}

func (h *WebhookMCPHandler) HandleTool(ctx context.Context, name string, raw json.RawMessage) (any, error) {
	switch name {
	case "register_webhook":
		return h.callRegister(ctx, raw)
	case "list_webhooks":
		return h.callList(ctx, raw)
	case "delete_webhook":
		return h.callDelete(ctx, raw)
	default:
		return nil, kernel.NewDomainError(kernel.ErrInvalidArgument, "unknown tool: "+name)
	}
}

type registerWebhookArgs struct {
	UserID int64  `json:"user_id"`
	URL    string `json:"url"`
	Secret string `json:"secret"`
	Events string `json:"events"`
}

func (h *WebhookMCPHandler) callRegister(ctx context.Context, raw json.RawMessage) (any, error) {
	var args registerWebhookArgs
	if err := json.Unmarshal(raw, &args); err != nil {
		return nil, kernel.NewDomainError(kernel.ErrInvalidArgument, "invalid arguments")
	}
	if args.UserID <= 0 {
		return nil, kernel.NewDomainError(kernel.ErrInvalidArgument, "user_id must be positive")
	}
	if args.URL == "" {
		return nil, kernel.NewDomainError(kernel.ErrInvalidArgument, "url must not be empty")
	}
	if args.Secret == "" {
		return nil, kernel.NewDomainError(kernel.ErrInvalidArgument, "secret must not be empty")
	}

	var events []string
	if err := json.Unmarshal([]byte(args.Events), &events); err != nil {
		return nil, kernel.NewDomainError(kernel.ErrInvalidArgument, "events must be a valid JSON array of strings")
	}

	wh, err := h.svc.Register(ctx, kernel.ID(args.UserID), args.URL, args.Secret, events)
	if err != nil {
		return nil, err
	}

	return webhookToMap(wh), nil
}

type listWebhooksArgs struct {
	UserID int64 `json:"user_id"`
}

func (h *WebhookMCPHandler) callList(ctx context.Context, raw json.RawMessage) (any, error) {
	var args listWebhooksArgs
	if err := json.Unmarshal(raw, &args); err != nil {
		return nil, kernel.NewDomainError(kernel.ErrInvalidArgument, "invalid arguments")
	}
	if args.UserID <= 0 {
		return nil, kernel.NewDomainError(kernel.ErrInvalidArgument, "user_id must be positive")
	}

	webhooks, err := h.svc.ListByUser(ctx, kernel.ID(args.UserID))
	if err != nil {
		return nil, err
	}

	result := make([]map[string]any, len(webhooks))
	for i, wh := range webhooks {
		result[i] = webhookToMap(wh)
	}
	return result, nil
}

type deleteWebhookArgs struct {
	UserID int64 `json:"user_id"`
	ID     int64 `json:"id"`
}

func (h *WebhookMCPHandler) callDelete(ctx context.Context, raw json.RawMessage) (any, error) {
	var args deleteWebhookArgs
	if err := json.Unmarshal(raw, &args); err != nil {
		return nil, kernel.NewDomainError(kernel.ErrInvalidArgument, "invalid arguments")
	}
	if args.UserID <= 0 {
		return nil, kernel.NewDomainError(kernel.ErrInvalidArgument, "user_id must be positive")
	}
	if args.ID <= 0 {
		return nil, kernel.NewDomainError(kernel.ErrInvalidArgument, "id must be positive")
	}

	if err := h.svc.Delete(ctx, kernel.ID(args.UserID), kernel.ID(args.ID)); err != nil {
		return nil, err
	}

	return map[string]string{"status": "deleted"}, nil
}

func webhookToMap(wh *domain.Webhook) map[string]any {
	return map[string]any{
		"id":     wh.ID.Int64(),
		"user_id": wh.UserID.Int64(),
		"url":    wh.URL,
		"events": wh.Events,
		"active": wh.Active,
	}
}
