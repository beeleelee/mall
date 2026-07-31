package subscription

import (
	"time"

	"github.com/beeleelee/mall/domain/kernel"
)

type PlanStatus string

const (
	PlanStatusActive   PlanStatus = "active"
	PlanStatusInactive PlanStatus = "inactive"
)

type SubscriptionStatus string

const (
	SubscriptionStatusPending   SubscriptionStatus = "pending"
	SubscriptionStatusTrialing  SubscriptionStatus = "trialing"
	SubscriptionStatusActive    SubscriptionStatus = "active"
	SubscriptionStatusPastDue   SubscriptionStatus = "past_due"
	SubscriptionStatusCancelled SubscriptionStatus = "cancelled"
	SubscriptionStatusExpired   SubscriptionStatus = "expired"
)

type Plan struct {
	kernel.Entity
	Name          string
	Description   string
	Amount        int64
	Interval      string
	IntervalCount int
	TrialDays     int
	Features      []string
	Status        PlanStatus
}

func NewPlan(id kernel.ID, name, description string, amount int64, interval string, intervalCount, trialDays int, features []string) (*Plan, error) {
	if id <= 0 {
		return nil, kernel.NewDomainError(kernel.ErrInvalidArgument, "plan id must be positive")
	}
	if name == "" {
		return nil, kernel.NewDomainError(kernel.ErrInvalidArgument, "plan name is required")
	}
	if amount <= 0 {
		return nil, kernel.NewDomainError(kernel.ErrInvalidArgument, "plan amount must be positive")
	}
	if interval != "month" && interval != "year" {
		return nil, kernel.NewDomainError(kernel.ErrInvalidArgument, "interval must be month or year")
	}
	if intervalCount <= 0 {
		return nil, kernel.NewDomainError(kernel.ErrInvalidArgument, "interval count must be positive")
	}
	if trialDays < 0 {
		return nil, kernel.NewDomainError(kernel.ErrInvalidArgument, "trial days must not be negative")
	}
	if features == nil {
		features = []string{}
	}
	return &Plan{
		Entity:        kernel.NewEntity(id),
		Name:          name,
		Description:   description,
		Amount:        amount,
		Interval:      interval,
		IntervalCount: intervalCount,
		TrialDays:     trialDays,
		Features:      features,
		Status:        PlanStatusActive,
	}, nil
}

func (p *Plan) Deactivate() error {
	if p.Status != PlanStatusActive {
		return kernel.NewDomainError(kernel.ErrInvalidArgument, "plan is not active")
	}
	p.Status = PlanStatusInactive
	p.UpdatedAt = time.Now()
	return nil
}

func (p *Plan) Activate() error {
	if p.Status != PlanStatusInactive {
		return kernel.NewDomainError(kernel.ErrInvalidArgument, "plan is not inactive")
	}
	p.Status = PlanStatusActive
	p.UpdatedAt = time.Now()
	return nil
}

type Subscription struct {
	kernel.AggregateRoot
	UserID             kernel.ID
	PlanID             kernel.ID
	Status             SubscriptionStatus
	CurrentPeriodStart time.Time
	CurrentPeriodEnd   time.Time
	TrialEndsAt        *time.Time
	CancelledAt        *time.Time
	PaymentToken       string
}

func NewSubscription(id, userID, planID kernel.ID, plan *Plan) (*Subscription, error) {
	if id <= 0 {
		return nil, kernel.NewDomainError(kernel.ErrInvalidArgument, "subscription id must be positive")
	}
	if userID <= 0 {
		return nil, kernel.NewDomainError(kernel.ErrInvalidArgument, "user id must be positive")
	}
	if planID <= 0 {
		return nil, kernel.NewDomainError(kernel.ErrInvalidArgument, "plan id must be positive")
	}
	if plan == nil {
		return nil, kernel.NewDomainError(kernel.ErrInvalidArgument, "plan is required")
	}
	if plan.Status != PlanStatusActive {
		return nil, kernel.NewDomainError(kernel.ErrInvalidArgument, "plan is not active")
	}

	now := time.Now()
	periodStart := now
	periodEnd := periodStart.AddDate(0, plan.IntervalCount, 0)
	if plan.Interval == "year" {
		periodEnd = periodStart.AddDate(plan.IntervalCount, 0, 0)
	}

	status := SubscriptionStatusPending
	var trialEndsAt *time.Time
	if plan.TrialDays > 0 {
		t := now.AddDate(0, 0, plan.TrialDays)
		trialEndsAt = &t
		status = SubscriptionStatusTrialing
	}

	s := &Subscription{
		AggregateRoot:      kernel.NewAggregateRoot(id),
		UserID:             userID,
		PlanID:             planID,
		Status:             status,
		CurrentPeriodStart: periodStart,
		CurrentPeriodEnd:   periodEnd,
		TrialEndsAt:        trialEndsAt,
	}
	s.AddEvent(SubscriptionCreatedEvent{
		SubscriptionID: id,
		UserID:         userID,
		PlanID:         planID,
	})
	return s, nil
}

func (s *Subscription) Activate() error {
	if s.Status != SubscriptionStatusPending && s.Status != SubscriptionStatusTrialing && s.Status != SubscriptionStatusPastDue {
		return kernel.NewDomainError(kernel.ErrInvalidArgument, "cannot activate subscription in current state: "+string(s.Status))
	}
	oldStatus := s.Status
	s.Status = SubscriptionStatusActive
	s.touch()
	s.AddEvent(SubscriptionActivatedEvent{
		SubscriptionID: s.ID,
		UserID:         s.UserID,
		PlanID:         s.PlanID,
		PreviousStatus: oldStatus,
	})
	return nil
}

func (s *Subscription) Cancel() error {
	if s.Status == SubscriptionStatusCancelled || s.Status == SubscriptionStatusExpired {
		return kernel.NewDomainError(kernel.ErrInvalidArgument, "subscription is already "+string(s.Status))
	}
	now := time.Now()
	s.CancelledAt = &now
	s.Status = SubscriptionStatusCancelled
	s.touch()
	s.AddEvent(SubscriptionCancelledEvent{
		SubscriptionID: s.ID,
		UserID:         s.UserID,
		CancelledAt:    now,
	})
	return nil
}

func (s *Subscription) Expire() error {
	if s.Status != SubscriptionStatusActive && s.Status != SubscriptionStatusPastDue && s.Status != SubscriptionStatusTrialing {
		return kernel.NewDomainError(kernel.ErrInvalidArgument, "cannot expire subscription in current state: "+string(s.Status))
	}
	s.Status = SubscriptionStatusExpired
	s.touch()
	s.AddEvent(SubscriptionExpiredEvent{
		SubscriptionID: s.ID,
		UserID:         s.UserID,
	})
	return nil
}

func (s *Subscription) FailPayment() error {
	if s.Status != SubscriptionStatusActive && s.Status != SubscriptionStatusPastDue {
		return kernel.NewDomainError(kernel.ErrInvalidArgument, "cannot mark payment failed in current state: "+string(s.Status))
	}
	s.Status = SubscriptionStatusPastDue
	s.touch()
	s.AddEvent(SubscriptionPaymentFailedEvent{
		SubscriptionID: s.ID,
		UserID:         s.UserID,
	})
	return nil
}

func (s *Subscription) ChangePlan(newPlan *Plan) error {
	if s.Status != SubscriptionStatusActive && s.Status != SubscriptionStatusPastDue {
		return kernel.NewDomainError(kernel.ErrInvalidArgument, "cannot change plan in current state: "+string(s.Status))
	}
	if newPlan == nil {
		return kernel.NewDomainError(kernel.ErrInvalidArgument, "new plan is required")
	}
	if newPlan.Status != PlanStatusActive {
		return kernel.NewDomainError(kernel.ErrInvalidArgument, "new plan is not active")
	}
	oldPlanID := s.PlanID
	s.PlanID = newPlan.ID
	s.touch()
	s.AddEvent(SubscriptionPlanChangedEvent{
		SubscriptionID: s.ID,
		UserID:         s.UserID,
		OldPlanID:      oldPlanID,
		NewPlanID:      newPlan.ID,
	})
	return nil
}

func (s *Subscription) AttachPaymentToken(token string) error {
	if token == "" {
		return kernel.NewDomainError(kernel.ErrInvalidArgument, "payment token must not be empty")
	}
	if s.Status != SubscriptionStatusPending && s.Status != SubscriptionStatusTrialing && s.Status != SubscriptionStatusPastDue {
		return kernel.NewDomainError(kernel.ErrInvalidArgument, "cannot attach payment token in current state: "+string(s.Status))
	}
	s.PaymentToken = token
	s.touch()
	return nil
}

func (s *Subscription) Renew(plan *Plan) error {
	if s.Status != SubscriptionStatusActive && s.Status != SubscriptionStatusPastDue {
		return kernel.NewDomainError(kernel.ErrInvalidArgument, "cannot renew subscription in current state: "+string(s.Status))
	}
	if plan == nil {
		return kernel.NewDomainError(kernel.ErrInvalidArgument, "plan is required")
	}
	now := time.Now()
	s.CurrentPeriodStart = now
	if plan.Interval == "month" {
		s.CurrentPeriodEnd = now.AddDate(0, plan.IntervalCount, 0)
	} else {
		s.CurrentPeriodEnd = now.AddDate(plan.IntervalCount, 0, 0)
	}
	s.touch()
	return nil
}

func (s *Subscription) touch() {
	s.UpdatedAt = time.Now()
}

func (s *Subscription) IsActive() bool {
	return s.Status == SubscriptionStatusActive || s.Status == SubscriptionStatusTrialing
}
