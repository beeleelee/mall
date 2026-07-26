package mcp

import (
	"context"
	"encoding/json"

	"github.com/beeleelee/mall/application/subscription"
	"github.com/beeleelee/mall/domain/kernel"
)

type SubscriptionMCPHandler struct {
	svc   *subscription.SubscriptionAppService
	tools []ToolDefinition
}

func NewSubscriptionMCPHandler(svc *subscription.SubscriptionAppService) *SubscriptionMCPHandler {
	return &SubscriptionMCPHandler{
		svc:   svc,
		tools: subscriptionTools,
	}
}

var subscriptionTools = []ToolDefinition{
	{
		Name:        "list_subscription_plans",
		Description: "List all available subscription plans",
		InputSchema: InputSchema{
			Type:       "object",
			Properties: map[string]PropertySchema{},
		},
	},
	{
		Name:        "get_subscription_plan",
		Description: "Get details of a specific subscription plan by ID",
		InputSchema: InputSchema{
			Type: "object",
			Properties: map[string]PropertySchema{
				"plan_id": {Type: "number", Description: "Plan ID"},
			},
		},
	},
	{
		Name:        "subscribe",
		Description: "Subscribe a user to a plan",
		InputSchema: InputSchema{
			Type: "object",
			Properties: map[string]PropertySchema{
				"plan_id": {Type: "number", Description: "Plan ID to subscribe to"},
				"user_id": {Type: "number", Description: "User ID to subscribe"},
			},
		},
	},
	{
		Name:        "list_user_subscriptions",
		Description: "List all subscriptions for a user",
		InputSchema: InputSchema{
			Type: "object",
			Properties: map[string]PropertySchema{
				"user_id": {Type: "number", Description: "User ID to list subscriptions for"},
			},
		},
	},
	{
		Name:        "cancel_subscription",
		Description: "Cancel a subscription",
		InputSchema: InputSchema{
			Type: "object",
			Properties: map[string]PropertySchema{
				"subscription_id": {Type: "number", Description: "Subscription ID to cancel"},
				"user_id":         {Type: "number", Description: "User ID who owns the subscription"},
			},
		},
	},
	{
		Name:        "change_subscription_plan",
		Description: "Change the plan of an existing subscription",
		InputSchema: InputSchema{
			Type: "object",
			Properties: map[string]PropertySchema{
				"subscription_id": {Type: "number", Description: "Subscription ID"},
				"new_plan_id":     {Type: "number", Description: "New plan ID"},
				"user_id":         {Type: "number", Description: "User ID who owns the subscription"},
			},
		},
	},
	{
		Name:        "activate_subscription",
		Description: "Activate a pending, trialing, or past_due subscription",
		InputSchema: InputSchema{
			Type: "object",
			Properties: map[string]PropertySchema{
				"subscription_id": {Type: "number", Description: "Subscription ID to activate"},
				"user_id":         {Type: "number", Description: "User ID who owns the subscription"},
			},
		},
	},
}

func (h *SubscriptionMCPHandler) ListTools() []ToolDefinition {
	return h.tools
}

func (h *SubscriptionMCPHandler) HandleTool(ctx context.Context, name string, raw json.RawMessage) (any, error) {
	switch name {
	case "list_subscription_plans":
		return h.callListPlans(ctx)
	case "get_subscription_plan":
		return h.callGetPlan(ctx, raw)
	case "subscribe":
		return h.callSubscribe(ctx, raw)
	case "list_user_subscriptions":
		return h.callListUserSubscriptions(ctx, raw)
	case "cancel_subscription":
		return h.callCancelSubscription(ctx, raw)
	case "change_subscription_plan":
		return h.callChangePlan(ctx, raw)
	case "activate_subscription":
		return h.callActivateSubscription(ctx, raw)
	default:
		return nil, kernel.NewDomainError(kernel.ErrInvalidArgument, "unknown tool: "+name)
	}
}

type planIDArgs struct {
	PlanID int64 `json:"plan_id"`
}

type subscribeArgs struct {
	PlanID int64 `json:"plan_id"`
	UserID int64 `json:"user_id"`
}

type listUserSubsArgs struct {
	UserID int64 `json:"user_id"`
}

type subscriptionIDArgs struct {
	SubscriptionID int64 `json:"subscription_id"`
	UserID         int64 `json:"user_id"`
}

type changePlanArgs struct {
	SubscriptionID int64 `json:"subscription_id"`
	NewPlanID      int64 `json:"new_plan_id"`
	UserID         int64 `json:"user_id"`
}

func (h *SubscriptionMCPHandler) callListPlans(ctx context.Context) (any, error) {
	plans, err := h.svc.ListPlans(ctx)
	if err != nil {
		return nil, err
	}
	return plans, nil
}

func (h *SubscriptionMCPHandler) callGetPlan(ctx context.Context, raw json.RawMessage) (any, error) {
	var args planIDArgs
	if err := json.Unmarshal(raw, &args); err != nil {
		return nil, kernel.NewDomainError(kernel.ErrInvalidArgument, "invalid arguments")
	}
	plan, err := h.svc.GetPlan(ctx, args.PlanID)
	if err != nil {
		return nil, err
	}
	return plan, nil
}

func (h *SubscriptionMCPHandler) callSubscribe(ctx context.Context, raw json.RawMessage) (any, error) {
	var args subscribeArgs
	if err := json.Unmarshal(raw, &args); err != nil {
		return nil, kernel.NewDomainError(kernel.ErrInvalidArgument, "invalid arguments")
	}
	if args.UserID <= 0 {
		return nil, kernel.NewDomainError(kernel.ErrInvalidArgument, "user_id is required")
	}
	if args.PlanID <= 0 {
		return nil, kernel.NewDomainError(kernel.ErrInvalidArgument, "plan_id is required")
	}
	sub, err := h.svc.Subscribe(ctx, args.UserID, subscription.SubscribeRequest{PlanID: args.PlanID})
	if err != nil {
		return nil, err
	}
	return sub, nil
}

func (h *SubscriptionMCPHandler) callListUserSubscriptions(ctx context.Context, raw json.RawMessage) (any, error) {
	var args listUserSubsArgs
	if err := json.Unmarshal(raw, &args); err != nil {
		return nil, kernel.NewDomainError(kernel.ErrInvalidArgument, "invalid arguments")
	}
	if args.UserID <= 0 {
		return nil, kernel.NewDomainError(kernel.ErrInvalidArgument, "user_id is required")
	}
	subs, err := h.svc.ListUserSubscriptions(ctx, args.UserID)
	if err != nil {
		return nil, err
	}
	return subs, nil
}

func (h *SubscriptionMCPHandler) callCancelSubscription(ctx context.Context, raw json.RawMessage) (any, error) {
	var args subscriptionIDArgs
	if err := json.Unmarshal(raw, &args); err != nil {
		return nil, kernel.NewDomainError(kernel.ErrInvalidArgument, "invalid arguments")
	}
	if args.UserID <= 0 {
		return nil, kernel.NewDomainError(kernel.ErrInvalidArgument, "user_id is required")
	}
	if args.SubscriptionID <= 0 {
		return nil, kernel.NewDomainError(kernel.ErrInvalidArgument, "subscription_id is required")
	}
	sub, err := h.svc.CancelSubscription(ctx, args.SubscriptionID)
	if err != nil {
		return nil, err
	}
	return sub, nil
}

func (h *SubscriptionMCPHandler) callChangePlan(ctx context.Context, raw json.RawMessage) (any, error) {
	var args changePlanArgs
	if err := json.Unmarshal(raw, &args); err != nil {
		return nil, kernel.NewDomainError(kernel.ErrInvalidArgument, "invalid arguments")
	}
	if args.UserID <= 0 {
		return nil, kernel.NewDomainError(kernel.ErrInvalidArgument, "user_id is required")
	}
	if args.SubscriptionID <= 0 {
		return nil, kernel.NewDomainError(kernel.ErrInvalidArgument, "subscription_id is required")
	}
	if args.NewPlanID <= 0 {
		return nil, kernel.NewDomainError(kernel.ErrInvalidArgument, "new_plan_id is required")
	}
	sub, err := h.svc.ChangePlan(ctx, args.SubscriptionID, args.NewPlanID)
	if err != nil {
		return nil, err
	}
	return sub, nil
}

type activateSubArgs struct {
	SubscriptionID int64 `json:"subscription_id"`
	UserID         int64 `json:"user_id"`
}

func (h *SubscriptionMCPHandler) callActivateSubscription(ctx context.Context, raw json.RawMessage) (any, error) {
	var args activateSubArgs
	if err := json.Unmarshal(raw, &args); err != nil {
		return nil, kernel.NewDomainError(kernel.ErrInvalidArgument, "invalid arguments")
	}
	if args.UserID <= 0 {
		return nil, kernel.NewDomainError(kernel.ErrInvalidArgument, "user_id is required")
	}
	if args.SubscriptionID <= 0 {
		return nil, kernel.NewDomainError(kernel.ErrInvalidArgument, "subscription_id is required")
	}
	sub, err := h.svc.ActivateSubscription(ctx, args.SubscriptionID)
	if err != nil {
		return nil, err
	}
	return sub, nil
}
