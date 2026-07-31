package subscription

import (
	"context"
	"testing"
	"time"

	"github.com/beeleelee/mall/domain/kernel"
)

type fakePlanRepo struct {
	plans map[kernel.ID]*Plan
}

func newFakePlanRepo() *fakePlanRepo {
	return &fakePlanRepo{plans: make(map[kernel.ID]*Plan)}
}

func (f *fakePlanRepo) Save(_ context.Context, p *Plan) error {
	f.plans[p.ID] = p
	return nil
}

func (f *fakePlanRepo) FindByID(_ context.Context, id kernel.ID) (*Plan, error) {
	p, ok := f.plans[id]
	if !ok {
		return nil, kernel.NewDomainError(kernel.ErrNotFound, "plan not found")
	}
	return p, nil
}

func (f *fakePlanRepo) FindAll(_ context.Context) ([]*Plan, error) {
	plans := make([]*Plan, 0, len(f.plans))
	for _, p := range f.plans {
		plans = append(plans, p)
	}
	return plans, nil
}

func (f *fakePlanRepo) FindActive(_ context.Context) ([]*Plan, error) {
	plans := make([]*Plan, 0)
	for _, p := range f.plans {
		if p.Status == PlanStatusActive {
			plans = append(plans, p)
		}
	}
	return plans, nil
}

type fakeSubRepo struct {
	subs map[kernel.ID]*Subscription
}

func newFakeSubRepo() *fakeSubRepo {
	return &fakeSubRepo{subs: make(map[kernel.ID]*Subscription)}
}

func (f *fakeSubRepo) Save(_ context.Context, s *Subscription) error {
	f.subs[s.ID] = s
	return nil
}

func (f *fakeSubRepo) FindByID(_ context.Context, id kernel.ID) (*Subscription, error) {
	s, ok := f.subs[id]
	if !ok {
		return nil, kernel.NewDomainError(kernel.ErrNotFound, "subscription not found")
	}
	return s, nil
}

func (f *fakeSubRepo) FindByUserID(_ context.Context, userID kernel.ID) ([]*Subscription, error) {
	subs := make([]*Subscription, 0)
	for _, s := range f.subs {
		if s.UserID == userID {
			subs = append(subs, s)
		}
	}
	return subs, nil
}

func (f *fakeSubRepo) FindActiveByUserID(_ context.Context, userID kernel.ID) (*Subscription, error) {
	for _, s := range f.subs {
		if s.UserID == userID && s.IsActive() {
			return s, nil
		}
	}
	return nil, kernel.NewDomainError(kernel.ErrNotFound, "no active subscription")
}

func (f *fakeSubRepo) FindDueForBilling(_ context.Context, now time.Time) ([]*Subscription, error) {
	subs := make([]*Subscription, 0)
	for _, s := range f.subs {
		if s.CurrentPeriodEnd.Before(now) && s.Status == SubscriptionStatusActive {
			subs = append(subs, s)
		}
	}
	return subs, nil
}

func (f *fakeSubRepo) FindTrialsEnded(_ context.Context, now time.Time) ([]*Subscription, error) {
	subs := make([]*Subscription, 0)
	for _, s := range f.subs {
		if s.Status == SubscriptionStatusTrialing && s.TrialEndsAt != nil && s.TrialEndsAt.Before(now) {
			subs = append(subs, s)
		}
	}
	return subs, nil
}

type fakeSubPub struct{}

func (fakeSubPub) PublishSubscriptionEvent(_ context.Context, _ *Subscription) error {
	return nil
}

type fakeCharger struct {
	failOn []kernel.ID
}

func (c *fakeCharger) Charge(_ context.Context, subID, _ kernel.ID, _ int64, _ string) error {
	for _, id := range c.failOn {
		if id == subID {
			return kernel.NewDomainError(kernel.ErrUnavailable, "charge failed")
		}
	}
	return nil
}

type fakeLogger struct{}

func (fakeLogger) Debug(_ context.Context, _ string, _ ...kernel.LogField)          {}
func (fakeLogger) Info(_ context.Context, _ string, _ ...kernel.LogField)           {}
func (fakeLogger) Warn(_ context.Context, _ string, _ ...kernel.LogField)           {}
func (fakeLogger) Error(_ context.Context, _ string, _ error, _ ...kernel.LogField) {}

func newTestService(t *testing.T) (*SubscriptionService, *fakePlanRepo, *fakeSubRepo) {
	t.Helper()
	planRepo := newFakePlanRepo()
	subRepo := newFakeSubRepo()
	pub := fakeSubPub{}
	logger := fakeLogger{}
	return NewSubscriptionService(planRepo, subRepo, pub, logger), planRepo, subRepo
}

func newTestServiceWithCharger(t *testing.T, c BillingCharger) (*SubscriptionService, *fakePlanRepo, *fakeSubRepo) {
	t.Helper()
	planRepo := newFakePlanRepo()
	subRepo := newFakeSubRepo()
	pub := fakeSubPub{}
	logger := fakeLogger{}
	return NewSubscriptionService(planRepo, subRepo, pub, logger, WithBillingCharger(c)), planRepo, subRepo
}

func TestCreatePlan_Success(t *testing.T) {
	svc, repo, _ := newTestService(t)
	p, err := svc.CreatePlan(context.Background(), 1, "Basic", "Basic plan", 999, "month", 1, 0, nil)
	if err != nil {
		t.Fatal(err)
	}
	if p.ID != 1 {
		t.Errorf("expected id 1, got %d", p.ID)
	}
	if _, ok := repo.plans[1]; !ok {
		t.Error("plan not stored in repo")
	}
}

func TestCreatePlan_Invalid(t *testing.T) {
	svc, _, _ := newTestService(t)
	_, err := svc.CreatePlan(context.Background(), 0, "", "", 0, "bad", 0, 0, nil)
	if !kernel.IsInvalidArgument(err) {
		t.Fatalf("expected invalid argument, got %v", err)
	}
}

func TestListPlans(t *testing.T) {
	svc, planRepo, _ := newTestService(t)
	planRepo.Save(context.Background(), &Plan{Entity: kernel.NewEntity(1), Name: "A", Status: PlanStatusActive})
	planRepo.Save(context.Background(), &Plan{Entity: kernel.NewEntity(2), Name: "B", Status: PlanStatusActive})
	planRepo.Save(context.Background(), &Plan{Entity: kernel.NewEntity(3), Name: "C", Status: PlanStatusInactive})

	plans, err := svc.ListPlans(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	if len(plans) != 2 {
		t.Errorf("expected 2 active plans, got %d", len(plans))
	}
}

func TestGetPlan_Found(t *testing.T) {
	svc, planRepo, _ := newTestService(t)
	planRepo.Save(context.Background(), &Plan{Entity: kernel.NewEntity(1), Name: "Test", Status: PlanStatusActive})
	p, err := svc.GetPlan(context.Background(), 1)
	if err != nil {
		t.Fatal(err)
	}
	if p.Name != "Test" {
		t.Errorf("expected Test, got %s", p.Name)
	}
}

func TestGetPlan_NotFound(t *testing.T) {
	svc, _, _ := newTestService(t)
	_, err := svc.GetPlan(context.Background(), 999)
	if !kernel.IsNotFound(err) {
		t.Fatalf("expected not found, got %v", err)
	}
}

func TestUpdatePlan(t *testing.T) {
	svc, planRepo, _ := newTestService(t)
	planRepo.Save(context.Background(), &Plan{
		Entity: kernel.NewEntity(1), Name: "Old", Amount: 500,
		Interval: "month", IntervalCount: 1, Status: PlanStatusActive,
	})
	p, err := svc.UpdatePlan(context.Background(), 1, "New", "", 999, "year", 2, 0, nil)
	if err != nil {
		t.Fatal(err)
	}
	if p.Name != "New" {
		t.Errorf("expected New, got %s", p.Name)
	}
	if p.Amount != 999 {
		t.Errorf("expected 999, got %d", p.Amount)
	}
	if p.Interval != "year" {
		t.Errorf("expected year, got %s", p.Interval)
	}
}

func TestDeactivatePlan(t *testing.T) {
	svc, planRepo, _ := newTestService(t)
	p, _ := NewPlan(1, "X", "", 100, "month", 1, 0, nil)
	planRepo.Save(context.Background(), p)
	if err := svc.DeactivatePlan(context.Background(), 1); err != nil {
		t.Fatal(err)
	}
	updated, _ := planRepo.FindByID(context.Background(), 1)
	if updated.Status != PlanStatusInactive {
		t.Errorf("expected inactive, got %s", updated.Status)
	}
}

func TestActivatePlan_Success(t *testing.T) {
	svc, planRepo, _ := newTestService(t)
	p, _ := NewPlan(1, "X", "", 100, "month", 1, 0, nil)
	planRepo.Save(context.Background(), p)
	p.Status = PlanStatusInactive

	updated, err := svc.ActivatePlan(context.Background(), 1)
	if err != nil {
		t.Fatal(err)
	}
	if updated.Status != PlanStatusActive {
		t.Errorf("expected active, got %s", updated.Status)
	}
}

func TestActivatePlan_NotFound(t *testing.T) {
	svc, _, _ := newTestService(t)
	_, err := svc.ActivatePlan(context.Background(), 999)
	if !kernel.IsNotFound(err) {
		t.Fatalf("expected not found, got %v", err)
	}
}

func TestActivatePlan_AlreadyActive(t *testing.T) {
	svc, planRepo, _ := newTestService(t)
	p, _ := NewPlan(1, "X", "", 100, "month", 1, 0, nil)
	planRepo.Save(context.Background(), p)
	_, err := svc.ActivatePlan(context.Background(), 1)
	if !kernel.IsInvalidArgument(err) {
		t.Fatalf("expected invalid argument, got %v", err)
	}
}

func TestActivateSubscription_Success(t *testing.T) {
	svc, planRepo, subRepo := newTestService(t)
	p, _ := NewPlan(1, "Basic", "", 999, "month", 1, 0, nil)
	planRepo.Save(context.Background(), p)
	subRepo.Save(context.Background(), &Subscription{
		AggregateRoot: kernel.NewAggregateRoot(1), UserID: 100, PlanID: 1,
		Status: SubscriptionStatusPending,
	})
	s, err := svc.ActivateSubscription(context.Background(), 1)
	if err != nil {
		t.Fatal(err)
	}
	if s.Status != SubscriptionStatusActive {
		t.Errorf("expected active, got %s", s.Status)
	}
}

func TestActivateSubscription_NotFound(t *testing.T) {
	svc, _, _ := newTestService(t)
	_, err := svc.ActivateSubscription(context.Background(), 999)
	if !kernel.IsNotFound(err) {
		t.Fatalf("expected not found, got %v", err)
	}
}

func TestActivateSubscription_AlreadyActive(t *testing.T) {
	svc, planRepo, subRepo := newTestService(t)
	p, _ := NewPlan(1, "Basic", "", 999, "month", 1, 0, nil)
	planRepo.Save(context.Background(), p)
	subRepo.Save(context.Background(), &Subscription{
		AggregateRoot: kernel.NewAggregateRoot(1), UserID: 100, PlanID: 1,
		Status: SubscriptionStatusActive,
	})
	_, err := svc.ActivateSubscription(context.Background(), 1)
	if !kernel.IsInvalidArgument(err) {
		t.Fatalf("expected invalid argument, got %v", err)
	}
}

func TestSubscribe_Success(t *testing.T) {
	svc, planRepo, _ := newTestService(t)
	p, _ := NewPlan(1, "Basic", "", 999, "month", 1, 0, nil)
	planRepo.Save(context.Background(), p)

	sub, err := svc.Subscribe(context.Background(), 1, 100, 1)
	if err != nil {
		t.Fatal(err)
	}
	if sub.UserID != 100 {
		t.Errorf("expected user 100, got %d", sub.UserID)
	}
	if sub.PlanID != 1 {
		t.Errorf("expected plan 1, got %d", sub.PlanID)
	}
}

func TestSubscribe_PlanNotFound(t *testing.T) {
	svc, _, _ := newTestService(t)
	_, err := svc.Subscribe(context.Background(), 1, 100, 999)
	if !kernel.IsNotFound(err) {
		t.Fatalf("expected not found, got %v", err)
	}
}

func TestGetSubscription(t *testing.T) {
	svc, planRepo, subRepo := newTestService(t)
	p, _ := NewPlan(1, "Basic", "", 999, "month", 1, 0, nil)
	planRepo.Save(context.Background(), p)
	subRepo.Save(context.Background(), &Subscription{
		AggregateRoot: kernel.NewAggregateRoot(1),
		UserID:        100, PlanID: 1, Status: SubscriptionStatusPending,
	})
	s, err := svc.GetSubscription(context.Background(), 1)
	if err != nil {
		t.Fatal(err)
	}
	if s.ID != 1 {
		t.Errorf("expected id 1, got %d", s.ID)
	}
}

func TestListUserSubscriptions(t *testing.T) {
	svc, planRepo, subRepo := newTestService(t)
	p, _ := NewPlan(1, "Basic", "", 999, "month", 1, 0, nil)
	planRepo.Save(context.Background(), p)
	subRepo.Save(context.Background(), &Subscription{
		AggregateRoot: kernel.NewAggregateRoot(1), UserID: 100, PlanID: 1,
	})
	subRepo.Save(context.Background(), &Subscription{
		AggregateRoot: kernel.NewAggregateRoot(2), UserID: 100, PlanID: 1,
	})
	subs, err := svc.ListUserSubscriptions(context.Background(), 100)
	if err != nil {
		t.Fatal(err)
	}
	if len(subs) != 2 {
		t.Errorf("expected 2, got %d", len(subs))
	}
}

func TestCancelSubscription(t *testing.T) {
	svc, planRepo, subRepo := newTestService(t)
	p, _ := NewPlan(1, "Basic", "", 999, "month", 1, 0, nil)
	planRepo.Save(context.Background(), p)
	subRepo.Save(context.Background(), &Subscription{
		AggregateRoot: kernel.NewAggregateRoot(1), UserID: 100, PlanID: 1,
		Status: SubscriptionStatusActive,
	})
	s, err := svc.CancelSubscription(context.Background(), 1)
	if err != nil {
		t.Fatal(err)
	}
	if s.Status != SubscriptionStatusCancelled {
		t.Errorf("expected cancelled, got %s", s.Status)
	}
}

func TestCancelSubscription_NotFound(t *testing.T) {
	svc, _, _ := newTestService(t)
	_, err := svc.CancelSubscription(context.Background(), 999)
	if !kernel.IsNotFound(err) {
		t.Fatalf("expected not found, got %v", err)
	}
}

func TestChangePlan(t *testing.T) {
	svc, planRepo, subRepo := newTestService(t)
	p1, _ := NewPlan(1, "Basic", "", 999, "month", 1, 0, nil)
	p2, _ := NewPlan(2, "Pro", "", 1999, "month", 1, 0, nil)
	planRepo.Save(context.Background(), p1)
	planRepo.Save(context.Background(), p2)
	subRepo.Save(context.Background(), &Subscription{
		AggregateRoot: kernel.NewAggregateRoot(1), UserID: 100,
		PlanID: 1, Status: SubscriptionStatusActive,
	})
	s, err := svc.ChangePlan(context.Background(), 1, 2)
	if err != nil {
		t.Fatal(err)
	}
	if s.PlanID != 2 {
		t.Errorf("expected plan 2, got %d", s.PlanID)
	}
}

func TestHandleBillingCycle(t *testing.T) {
	svc, planRepo, subRepo := newTestService(t)
	p, _ := NewPlan(1, "Basic", "", 999, "month", 1, 0, nil)
	planRepo.Save(context.Background(), p)
	subRepo.Save(context.Background(), &Subscription{
		AggregateRoot: kernel.NewAggregateRoot(1), UserID: 100,
		PlanID: 1, Status: SubscriptionStatusActive,
		CurrentPeriodStart: time.Now().AddDate(0, -1, 0),
		CurrentPeriodEnd:   time.Now().AddDate(0, 0, -1),
	})
	s, err := svc.HandleBillingCycle(context.Background(), 1)
	if err != nil {
		t.Fatal(err)
	}
	if s.CurrentPeriodEnd.Before(time.Now()) {
		t.Error("period end should be in the future after renewal")
	}
}

func TestHandleBillingCycle_ChargeFailure_ActiveToPastDue(t *testing.T) {
	svc, planRepo, subRepo := newTestServiceWithCharger(t, &fakeCharger{failOn: []kernel.ID{1}})
	p, _ := NewPlan(1, "Basic", "", 999, "month", 1, 0, nil)
	planRepo.Save(context.Background(), p)
	subRepo.Save(context.Background(), &Subscription{
		AggregateRoot: kernel.NewAggregateRoot(1), UserID: 100,
		PlanID: 1, Status: SubscriptionStatusActive, PaymentToken: "tok_1",
		CurrentPeriodStart: time.Now().AddDate(0, -1, 0),
		CurrentPeriodEnd:   time.Now().AddDate(0, 0, -1),
	})
	s, err := svc.HandleBillingCycle(context.Background(), 1)
	if err != nil {
		t.Fatal(err)
	}
	if s.Status != SubscriptionStatusPastDue {
		t.Errorf("expected past_due after failed charge, got %s", s.Status)
	}
}

func TestHandleBillingCycle_ChargeFailure_PastDueToExpired(t *testing.T) {
	svc, planRepo, subRepo := newTestServiceWithCharger(t, &fakeCharger{failOn: []kernel.ID{1}})
	p, _ := NewPlan(1, "Basic", "", 999, "month", 1, 0, nil)
	planRepo.Save(context.Background(), p)
	subRepo.Save(context.Background(), &Subscription{
		AggregateRoot: kernel.NewAggregateRoot(1), UserID: 100,
		PlanID: 1, Status: SubscriptionStatusPastDue, PaymentToken: "tok_1",
		CurrentPeriodStart: time.Now().AddDate(0, -2, 0),
		CurrentPeriodEnd:   time.Now().AddDate(0, -1, 0),
	})
	s, err := svc.HandleBillingCycle(context.Background(), 1)
	if err != nil {
		t.Fatal(err)
	}
	if s.Status != SubscriptionStatusExpired {
		t.Errorf("expected expired after second failed charge, got %s", s.Status)
	}
}

func TestHandleBillingCycle_ChargeSuccess(t *testing.T) {
	svc, planRepo, subRepo := newTestServiceWithCharger(t, &fakeCharger{})
	p, _ := NewPlan(1, "Basic", "", 999, "month", 1, 0, nil)
	planRepo.Save(context.Background(), p)
	subRepo.Save(context.Background(), &Subscription{
		AggregateRoot: kernel.NewAggregateRoot(1), UserID: 100,
		PlanID: 1, Status: SubscriptionStatusActive, PaymentToken: "tok_1",
		CurrentPeriodStart: time.Now().AddDate(0, -1, 0),
		CurrentPeriodEnd:   time.Now().AddDate(0, 0, -1),
	})
	s, err := svc.HandleBillingCycle(context.Background(), 1)
	if err != nil {
		t.Fatal(err)
	}
	if s.Status != SubscriptionStatusActive {
		t.Errorf("expected active after successful charge, got %s", s.Status)
	}
	if !s.CurrentPeriodEnd.After(time.Now()) {
		t.Error("period end should be extended after successful charge")
	}
}

func TestAttachPaymentToken_Service(t *testing.T) {
	svc, planRepo, subRepo := newTestService(t)
	p, _ := NewPlan(1, "Basic", "", 999, "month", 1, 0, nil)
	planRepo.Save(context.Background(), p)
	subRepo.Save(context.Background(), &Subscription{
		AggregateRoot: kernel.NewAggregateRoot(1), UserID: 100, PlanID: 1,
		Status: SubscriptionStatusPending,
	})
	s, err := svc.AttachPaymentToken(context.Background(), 1, "tok_123")
	if err != nil {
		t.Fatal(err)
	}
	if s.PaymentToken != "tok_123" {
		t.Errorf("expected token tok_123, got %q", s.PaymentToken)
	}
}

func TestSubscribeWithToken(t *testing.T) {
	svc, planRepo, _ := newTestService(t)
	p, _ := NewPlan(1, "Basic", "", 999, "month", 1, 0, nil)
	planRepo.Save(context.Background(), p)

	sub, err := svc.SubscribeWithToken(context.Background(), 1, 100, 1, "tok_abc")
	if err != nil {
		t.Fatal(err)
	}
	if sub.PaymentToken != "tok_abc" {
		t.Errorf("expected payment token tok_abc, got %q", sub.PaymentToken)
	}
}

func TestExpireTrials_ExpiresTrialing(t *testing.T) {
	svc, planRepo, subRepo := newTestService(t)
	p, _ := NewPlan(1, "Trial", "", 999, "month", 1, 7, nil)
	planRepo.Save(context.Background(), p)
	trialEnd := time.Now().Add(-time.Hour)
	subRepo.Save(context.Background(), &Subscription{
		AggregateRoot: kernel.NewAggregateRoot(1), UserID: 100, PlanID: 1,
		Status: SubscriptionStatusTrialing, TrialEndsAt: &trialEnd,
	})

	expired, err := svc.ExpireTrials(context.Background(), time.Now())
	if err != nil {
		t.Fatal(err)
	}
	if len(expired) != 1 {
		t.Fatalf("expected 1 expired, got %d", len(expired))
	}
	if expired[0].Status != SubscriptionStatusExpired {
		t.Errorf("expected expired, got %s", expired[0].Status)
	}
}

func TestExpireTrials_ActivateOnSuccessfulCharge(t *testing.T) {
	svc, planRepo, subRepo := newTestServiceWithCharger(t, &fakeCharger{})
	p, _ := NewPlan(1, "Trial", "", 999, "month", 1, 7, nil)
	planRepo.Save(context.Background(), p)
	trialEnd := time.Now().Add(-time.Hour)
	subRepo.Save(context.Background(), &Subscription{
		AggregateRoot: kernel.NewAggregateRoot(1), UserID: 100, PlanID: 1,
		Status: SubscriptionStatusTrialing, TrialEndsAt: &trialEnd, PaymentToken: "tok_1",
	})

	expired, err := svc.ExpireTrials(context.Background(), time.Now())
	if err != nil {
		t.Fatal(err)
	}
	if len(expired) != 0 {
		t.Fatalf("expected no expired on successful trial charge, got %d", len(expired))
	}
	s, _ := subRepo.FindByID(context.Background(), 1)
	if s.Status != SubscriptionStatusActive {
		t.Errorf("expected active after successful trial charge, got %s", s.Status)
	}
}

func TestExpireTrials_SkipsFutureTrials(t *testing.T) {
	svc, planRepo, subRepo := newTestService(t)
	p, _ := NewPlan(1, "Trial", "", 999, "month", 1, 7, nil)
	planRepo.Save(context.Background(), p)
	trialEnd := time.Now().Add(time.Hour)
	subRepo.Save(context.Background(), &Subscription{
		AggregateRoot: kernel.NewAggregateRoot(1), UserID: 100, PlanID: 1,
		Status: SubscriptionStatusTrialing, TrialEndsAt: &trialEnd,
	})

	expired, err := svc.ExpireTrials(context.Background(), time.Now())
	if err != nil {
		t.Fatal(err)
	}
	if len(expired) != 0 {
		t.Fatalf("expected no expired for future trial, got %d", len(expired))
	}
}
