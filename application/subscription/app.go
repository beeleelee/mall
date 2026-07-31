package subscription

import (
	"context"

	"github.com/beeleelee/mall/domain/kernel"
	domain "github.com/beeleelee/mall/domain/subscription"
)

type CreatePlanRequest struct {
	Name          string
	Description   string
	Amount        int64
	Interval      string
	IntervalCount int
	TrialDays     int
	Features      []string
}

type PlanResponse struct {
	ID            int64    `json:"id"`
	Name          string   `json:"name"`
	Description   string   `json:"description"`
	Amount        int64    `json:"amount"`
	Interval      string   `json:"interval"`
	IntervalCount int      `json:"interval_count"`
	TrialDays     int      `json:"trial_days"`
	Features      []string `json:"features"`
	Status        string   `json:"status"`
}

type SubscribeRequest struct {
	PlanID       int64
	PaymentToken string
}

type SubscriptionResponse struct {
	ID                 int64  `json:"id"`
	UserID             int64  `json:"user_id"`
	PlanID             int64  `json:"plan_id"`
	Status             string `json:"status"`
	CurrentPeriodStart string `json:"current_period_start"`
	CurrentPeriodEnd   string `json:"current_period_end"`
	TrialEndsAt        string `json:"trial_ends_at,omitempty"`
	CancelledAt        string `json:"cancelled_at,omitempty"`
	PaymentToken       string `json:"payment_token,omitempty"`
}

type SubscriptionAppService struct {
	svc *domain.SubscriptionService
	sf  *kernel.Snowflake
}

func NewSubscriptionAppService(svc *domain.SubscriptionService, sf *kernel.Snowflake) *SubscriptionAppService {
	return &SubscriptionAppService{svc: svc, sf: sf}
}

func planToResponse(p *domain.Plan) *PlanResponse {
	return &PlanResponse{
		ID:            p.ID.Int64(),
		Name:          p.Name,
		Description:   p.Description,
		Amount:        p.Amount,
		Interval:      p.Interval,
		IntervalCount: p.IntervalCount,
		TrialDays:     p.TrialDays,
		Features:      p.Features,
		Status:        string(p.Status),
	}
}

func subToResponse(s *domain.Subscription) *SubscriptionResponse {
	resp := &SubscriptionResponse{
		ID:                 s.ID.Int64(),
		UserID:             s.UserID.Int64(),
		PlanID:             s.PlanID.Int64(),
		Status:             string(s.Status),
		CurrentPeriodStart: s.CurrentPeriodStart.Format("2006-01-02T15:04:05Z"),
		CurrentPeriodEnd:   s.CurrentPeriodEnd.Format("2006-01-02T15:04:05Z"),
	}
	if s.TrialEndsAt != nil {
		resp.TrialEndsAt = s.TrialEndsAt.Format("2006-01-02T15:04:05Z")
	}
	if s.CancelledAt != nil {
		resp.CancelledAt = s.CancelledAt.Format("2006-01-02T15:04:05Z")
	}
	if s.PaymentToken != "" {
		resp.PaymentToken = s.PaymentToken
	}
	return resp
}

func (a *SubscriptionAppService) CreatePlan(ctx context.Context, req CreatePlanRequest) (*PlanResponse, error) {
	id, err := a.sf.NextID()
	if err != nil {
		return nil, err
	}
	plan, err := a.svc.CreatePlan(ctx, id, req.Name, req.Description, req.Amount, req.Interval, req.IntervalCount, req.TrialDays, req.Features)
	if err != nil {
		return nil, err
	}
	return planToResponse(plan), nil
}

func (a *SubscriptionAppService) ListPlans(ctx context.Context) ([]*PlanResponse, error) {
	plans, err := a.svc.ListPlans(ctx)
	if err != nil {
		return nil, err
	}
	resp := make([]*PlanResponse, len(plans))
	for i, p := range plans {
		resp[i] = planToResponse(p)
	}
	return resp, nil
}

func (a *SubscriptionAppService) GetPlan(ctx context.Context, id int64) (*PlanResponse, error) {
	plan, err := a.svc.GetPlan(ctx, kernel.ID(id))
	if err != nil {
		return nil, err
	}
	return planToResponse(plan), nil
}

func (a *SubscriptionAppService) UpdatePlan(ctx context.Context, id int64, req CreatePlanRequest) (*PlanResponse, error) {
	plan, err := a.svc.UpdatePlan(ctx, kernel.ID(id), req.Name, req.Description, req.Amount, req.Interval, req.IntervalCount, req.TrialDays, req.Features)
	if err != nil {
		return nil, err
	}
	return planToResponse(plan), nil
}

func (a *SubscriptionAppService) Subscribe(ctx context.Context, userID int64, req SubscribeRequest) (*SubscriptionResponse, error) {
	id, err := a.sf.NextID()
	if err != nil {
		return nil, err
	}
	sub, err := a.svc.SubscribeWithToken(ctx, id, kernel.ID(userID), kernel.ID(req.PlanID), req.PaymentToken)
	if err != nil {
		return nil, err
	}
	return subToResponse(sub), nil
}

func (a *SubscriptionAppService) AttachPaymentToken(ctx context.Context, id int64, token string) (*SubscriptionResponse, error) {
	sub, err := a.svc.AttachPaymentToken(ctx, kernel.ID(id), token)
	if err != nil {
		return nil, err
	}
	return subToResponse(sub), nil
}

func (a *SubscriptionAppService) GetSubscription(ctx context.Context, id int64) (*SubscriptionResponse, error) {
	sub, err := a.svc.GetSubscription(ctx, kernel.ID(id))
	if err != nil {
		return nil, err
	}
	return subToResponse(sub), nil
}

func (a *SubscriptionAppService) ListUserSubscriptions(ctx context.Context, userID int64) ([]*SubscriptionResponse, error) {
	subs, err := a.svc.ListUserSubscriptions(ctx, kernel.ID(userID))
	if err != nil {
		return nil, err
	}
	resp := make([]*SubscriptionResponse, len(subs))
	for i, s := range subs {
		resp[i] = subToResponse(s)
	}
	return resp, nil
}

func (a *SubscriptionAppService) ActivateSubscription(ctx context.Context, id int64) (*SubscriptionResponse, error) {
	sub, err := a.svc.ActivateSubscription(ctx, kernel.ID(id))
	if err != nil {
		return nil, err
	}
	return subToResponse(sub), nil
}

func (a *SubscriptionAppService) ActivatePlan(ctx context.Context, id int64) (*PlanResponse, error) {
	plan, err := a.svc.ActivatePlan(ctx, kernel.ID(id))
	if err != nil {
		return nil, err
	}
	return planToResponse(plan), nil
}

func (a *SubscriptionAppService) CancelSubscription(ctx context.Context, id int64) (*SubscriptionResponse, error) {
	sub, err := a.svc.CancelSubscription(ctx, kernel.ID(id))
	if err != nil {
		return nil, err
	}
	return subToResponse(sub), nil
}

func (a *SubscriptionAppService) ChangePlan(ctx context.Context, id, newPlanID int64) (*SubscriptionResponse, error) {
	sub, err := a.svc.ChangePlan(ctx, kernel.ID(id), kernel.ID(newPlanID))
	if err != nil {
		return nil, err
	}
	return subToResponse(sub), nil
}
