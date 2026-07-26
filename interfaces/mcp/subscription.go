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
		Description: "Subscribe the current user to a plan",
		InputSchema: InputSchema{
			Type: "object",
			Properties: map[string]PropertySchema{
				"plan_id": {Type: "number", Description: "Plan ID to subscribe to"},
			},
		},
	},
	{
		Name:        "list_user_subscriptions",
		Description: "List all subscriptions for the current user",
		InputSchema: InputSchema{
			Type:       "object",
			Properties: map[string]PropertySchema{},
		},
	},
	{
		Name:        "cancel_subscription",
		Description: "Cancel an active subscription",
		InputSchema: InputSchema{
			Type: "object",
			Properties: map[string]PropertySchema{
				"subscription_id": {Type: "number", Description: "Subscription ID to cancel"},
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
		return h.callListUserSubscriptions(ctx)
	case "cancel_subscription":
		return h.callCancelSubscription(ctx, raw)
	case "change_subscription_plan":
		return h.callChangePlan(ctx, raw)
	default:
		return nil, kernel.NewDomainError(kernel.ErrInvalidArgument, "unknown tool: "+name)
	}
}

type planIDArgs struct {
	PlanID int64 `json:"plan_id"`
}

type subscribeArgs struct {
	PlanID int64 `json:"plan_id"`
}

type subscriptionIDArgs struct {
	SubscriptionID int64 `json:"subscription_id"`
}

type changePlanArgs struct {
	SubscriptionID int64 `json:"subscription_id"`
	NewPlanID      int64 `json:"new_plan_id"`
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
	sub, err := h.svc.Subscribe(ctx, 0, subscription.SubscribeRequest{PlanID: args.PlanID})
	if err != nil {
		return nil, err
	}
	return sub, nil
}

func (h *SubscriptionMCPHandler) callListUserSubscriptions(ctx context.Context) (any, error) {
	subs, err := h.svc.ListUserSubscriptions(ctx, 0)
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
	sub, err := h.svc.ChangePlan(ctx, args.SubscriptionID, args.NewPlanID)
	if err != nil {
		return nil, err
	}
	return sub, nil
}
