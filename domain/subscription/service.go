package subscription

import (
	"context"
	"time"

	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/trace"

	"github.com/beeleelee/mall/domain/kernel"
)

var subTracer = otel.Tracer("mall.domain.subscription")

type PlanRepository interface {
	Save(ctx context.Context, plan *Plan) error
	FindByID(ctx context.Context, id kernel.ID) (*Plan, error)
	FindAll(ctx context.Context) ([]*Plan, error)
	FindActive(ctx context.Context) ([]*Plan, error)
}

type SubscriptionRepository interface {
	Save(ctx context.Context, sub *Subscription) error
	FindByID(ctx context.Context, id kernel.ID) (*Subscription, error)
	FindByUserID(ctx context.Context, userID kernel.ID) ([]*Subscription, error)
	FindActiveByUserID(ctx context.Context, userID kernel.ID) (*Subscription, error)
	FindDueForBilling(ctx context.Context, now time.Time) ([]*Subscription, error)
}

type SubscriptionEventPublisher interface {
	PublishSubscriptionEvent(ctx context.Context, sub *Subscription) error
}

type SubscriptionService struct {
	planRepo PlanRepository
	subRepo  SubscriptionRepository
	publisher SubscriptionEventPublisher
	logger   kernel.Logger
}

func NewSubscriptionService(
	planRepo PlanRepository,
	subRepo SubscriptionRepository,
	publisher SubscriptionEventPublisher,
	logger kernel.Logger,
) *SubscriptionService {
	return &SubscriptionService{
		planRepo:  planRepo,
		subRepo:   subRepo,
		publisher: publisher,
		logger:    logger,
	}
}

func (s *SubscriptionService) CreatePlan(ctx context.Context, id kernel.ID, name, description string, amount int64, interval string, intervalCount, trialDays int, features []string) (*Plan, error) {
	ctx, span := subTracer.Start(ctx, "subscription.create_plan",
		trace.WithAttributes(attribute.Int64("plan_id", id.Int64())),
	)
	defer span.End()

	plan, err := NewPlan(id, name, description, amount, interval, intervalCount, trialDays, features)
	if err != nil {
		return nil, err
	}
	if err := s.planRepo.Save(ctx, plan); err != nil {
		return nil, err
	}
	s.logger.Info(ctx, "subscription.plan_created",
		kernel.Field("plan_id", plan.ID.String()),
		kernel.Field("name", plan.Name),
	)
	return plan, nil
}

func (s *SubscriptionService) ListPlans(ctx context.Context) ([]*Plan, error) {
	return s.planRepo.FindActive(ctx)
}

func (s *SubscriptionService) ListAllPlans(ctx context.Context) ([]*Plan, error) {
	return s.planRepo.FindAll(ctx)
}

func (s *SubscriptionService) GetPlan(ctx context.Context, id kernel.ID) (*Plan, error) {
	return s.planRepo.FindByID(ctx, id)
}

func (s *SubscriptionService) UpdatePlan(ctx context.Context, id kernel.ID, name, description string, amount int64, interval string, intervalCount, trialDays int, features []string) (*Plan, error) {
	ctx, span := subTracer.Start(ctx, "subscription.update_plan",
		trace.WithAttributes(attribute.Int64("plan_id", id.Int64())),
	)
	defer span.End()

	plan, err := s.planRepo.FindByID(ctx, id)
	if err != nil {
		return nil, err
	}
	if name != "" {
		plan.Name = name
	}
	if description != "" {
		plan.Description = description
	}
	if amount > 0 {
		plan.Amount = amount
	}
	if interval == "month" || interval == "year" {
		plan.Interval = interval
	}
	if intervalCount > 0 {
		plan.IntervalCount = intervalCount
	}
	if trialDays >= 0 {
		plan.TrialDays = trialDays
	}
	if features != nil {
		plan.Features = features
	}
	if err := s.planRepo.Save(ctx, plan); err != nil {
		return nil, err
	}
	s.logger.Info(ctx, "subscription.plan_updated",
		kernel.Field("plan_id", plan.ID.String()),
	)
	return plan, nil
}

func (s *SubscriptionService) DeactivatePlan(ctx context.Context, id kernel.ID) error {
	plan, err := s.planRepo.FindByID(ctx, id)
	if err != nil {
		return err
	}
	if err := plan.Deactivate(); err != nil {
		return err
	}
	return s.planRepo.Save(ctx, plan)
}

func (s *SubscriptionService) ActivatePlan(ctx context.Context, id kernel.ID) (*Plan, error) {
	plan, err := s.planRepo.FindByID(ctx, id)
	if err != nil {
		return nil, err
	}
	if err := plan.Activate(); err != nil {
		return nil, err
	}
	return plan, s.planRepo.Save(ctx, plan)
}

func (s *SubscriptionService) ActivateSubscription(ctx context.Context, id kernel.ID) (*Subscription, error) {
	ctx, span := subTracer.Start(ctx, "subscription.activate",
		trace.WithAttributes(attribute.Int64("subscription_id", id.Int64())),
	)
	defer span.End()

	sub, err := s.subRepo.FindByID(ctx, id)
	if err != nil {
		return nil, err
	}

	if err := sub.Activate(); err != nil {
		return nil, err
	}

	if err := s.subRepo.Save(ctx, sub); err != nil {
		return nil, err
	}

	s.publishEvents(ctx, sub)
	s.logger.Info(ctx, "subscription.activated",
		kernel.Field("subscription_id", sub.ID.String()),
	)
	return sub, nil
}

func (s *SubscriptionService) Subscribe(ctx context.Context, id, userID, planID kernel.ID) (*Subscription, error) {
	ctx, span := subTracer.Start(ctx, "subscription.subscribe",
		trace.WithAttributes(
			attribute.Int64("subscription_id", id.Int64()),
			attribute.Int64("user_id", userID.Int64()),
			attribute.Int64("plan_id", planID.Int64()),
		),
	)
	defer span.End()

	plan, err := s.planRepo.FindByID(ctx, planID)
	if err != nil {
		return nil, err
	}

	sub, err := NewSubscription(id, userID, planID, plan)
	if err != nil {
		return nil, err
	}

	if err := s.subRepo.Save(ctx, sub); err != nil {
		return nil, err
	}

	s.publishEvents(ctx, sub)
	s.logger.Info(ctx, "subscription.created",
		kernel.Field("subscription_id", sub.ID.String()),
		kernel.Field("user_id", sub.UserID.String()),
	)
	return sub, nil
}

func (s *SubscriptionService) GetSubscription(ctx context.Context, id kernel.ID) (*Subscription, error) {
	return s.subRepo.FindByID(ctx, id)
}

func (s *SubscriptionService) ListUserSubscriptions(ctx context.Context, userID kernel.ID) ([]*Subscription, error) {
	return s.subRepo.FindByUserID(ctx, userID)
}

func (s *SubscriptionService) CancelSubscription(ctx context.Context, id kernel.ID) (*Subscription, error) {
	ctx, span := subTracer.Start(ctx, "subscription.cancel",
		trace.WithAttributes(attribute.Int64("subscription_id", id.Int64())),
	)
	defer span.End()

	sub, err := s.subRepo.FindByID(ctx, id)
	if err != nil {
		return nil, err
	}

	if err := sub.Cancel(); err != nil {
		return nil, err
	}

	if err := s.subRepo.Save(ctx, sub); err != nil {
		return nil, err
	}

	s.publishEvents(ctx, sub)
	s.logger.Info(ctx, "subscription.cancelled",
		kernel.Field("subscription_id", sub.ID.String()),
	)
	return sub, nil
}

func (s *SubscriptionService) ChangePlan(ctx context.Context, id, newPlanID kernel.ID) (*Subscription, error) {
	ctx, span := subTracer.Start(ctx, "subscription.change_plan",
		trace.WithAttributes(
			attribute.Int64("subscription_id", id.Int64()),
			attribute.Int64("new_plan_id", newPlanID.Int64()),
		),
	)
	defer span.End()

	sub, err := s.subRepo.FindByID(ctx, id)
	if err != nil {
		return nil, err
	}

	newPlan, err := s.planRepo.FindByID(ctx, newPlanID)
	if err != nil {
		return nil, err
	}

	if err := sub.ChangePlan(newPlan); err != nil {
		return nil, err
	}

	if err := s.subRepo.Save(ctx, sub); err != nil {
		return nil, err
	}

	s.publishEvents(ctx, sub)
	s.logger.Info(ctx, "subscription.plan_changed",
		kernel.Field("subscription_id", sub.ID.String()),
	)
	return sub, nil
}

func (s *SubscriptionService) HandleBillingCycle(ctx context.Context, subID kernel.ID) (*Subscription, error) {
	ctx, span := subTracer.Start(ctx, "subscription.billing_cycle",
		trace.WithAttributes(attribute.Int64("subscription_id", subID.Int64())),
	)
	defer span.End()

	sub, err := s.subRepo.FindByID(ctx, subID)
	if err != nil {
		return nil, err
	}

	plan, err := s.planRepo.FindByID(ctx, sub.PlanID)
	if err != nil {
		return nil, err
	}

	if err := sub.Renew(plan); err != nil {
		return nil, err
	}

	if err := s.subRepo.Save(ctx, sub); err != nil {
		return nil, err
	}

	s.publishEvents(ctx, sub)
	s.logger.Info(ctx, "subscription.renewed",
		kernel.Field("subscription_id", sub.ID.String()),
	)
	return sub, nil
}

func (s *SubscriptionService) publishEvents(ctx context.Context, sub *Subscription) {
	for _, event := range sub.Events() {
		_ = event // logged via publisher
	}
	if err := s.publisher.PublishSubscriptionEvent(ctx, sub); err != nil {
		s.logger.Error(ctx, "failed to publish subscription event", err,
			kernel.Field("subscription_id", sub.ID.String()),
		)
	}
	sub.ClearEvents()
}
