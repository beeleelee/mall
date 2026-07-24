package subscription

import (
	"context"
	"testing"
	"time"

	"github.com/beeleelee/mall/domain/kernel"
	domain "github.com/beeleelee/mall/domain/subscription"
)

type fakePlanRepo struct {
	plans map[kernel.ID]*domain.Plan
}

func newFakePlanRepo() *fakePlanRepo {
	return &fakePlanRepo{plans: make(map[kernel.ID]*domain.Plan)}
}

func (f *fakePlanRepo) Save(_ context.Context, p *domain.Plan) error {
	f.plans[p.ID] = p
	return nil
}

func (f *fakePlanRepo) FindByID(_ context.Context, id kernel.ID) (*domain.Plan, error) {
	p, ok := f.plans[id]
	if !ok {
		return nil, kernel.NewDomainError(kernel.ErrNotFound, "plan not found")
	}
	return p, nil
}

func (f *fakePlanRepo) FindAll(_ context.Context) ([]*domain.Plan, error) {
	plans := make([]*domain.Plan, 0, len(f.plans))
	for _, p := range f.plans {
		plans = append(plans, p)
	}
	return plans, nil
}

func (f *fakePlanRepo) FindActive(_ context.Context) ([]*domain.Plan, error) {
	plans := make([]*domain.Plan, 0)
	for _, p := range f.plans {
		if p.Status == domain.PlanStatusActive {
			plans = append(plans, p)
		}
	}
	return plans, nil
}

type fakeSubRepo struct {
	subs map[kernel.ID]*domain.Subscription
}

func newFakeSubRepo() *fakeSubRepo {
	return &fakeSubRepo{subs: make(map[kernel.ID]*domain.Subscription)}
}

func (f *fakeSubRepo) Save(_ context.Context, s *domain.Subscription) error {
	f.subs[s.ID] = s
	return nil
}

func (f *fakeSubRepo) FindByID(_ context.Context, id kernel.ID) (*domain.Subscription, error) {
	s, ok := f.subs[id]
	if !ok {
		return nil, kernel.NewDomainError(kernel.ErrNotFound, "subscription not found")
	}
	return s, nil
}

func (f *fakeSubRepo) FindByUserID(_ context.Context, userID kernel.ID) ([]*domain.Subscription, error) {
	subs := make([]*domain.Subscription, 0)
	for _, s := range f.subs {
		if s.UserID == userID {
			subs = append(subs, s)
		}
	}
	return subs, nil
}

func (f *fakeSubRepo) FindActiveByUserID(_ context.Context, userID kernel.ID) (*domain.Subscription, error) {
	for _, s := range f.subs {
		if s.UserID == userID && s.IsActive() {
			return s, nil
		}
	}
	return nil, kernel.NewDomainError(kernel.ErrNotFound, "no active subscription")
}

func (f *fakeSubRepo) FindDueForBilling(_ context.Context, now time.Time) ([]*domain.Subscription, error) {
	return nil, nil
}

type fakeSubPub struct{}

func (fakeSubPub) PublishSubscriptionEvent(_ context.Context, _ *domain.Subscription) error {
	return nil
}

type fakeLogger struct{}

func (fakeLogger) Debug(_ context.Context, _ string, _ ...kernel.LogField) {}
func (fakeLogger) Info(_ context.Context, _ string, _ ...kernel.LogField)  {}
func (fakeLogger) Warn(_ context.Context, _ string, _ ...kernel.LogField)  {}
func (fakeLogger) Error(_ context.Context, _ string, _ error, _ ...kernel.LogField) {}

func newTestAppService(t *testing.T) *SubscriptionAppService {
	t.Helper()
	planRepo := newFakePlanRepo()
	subRepo := newFakeSubRepo()
	pub := fakeSubPub{}
	logger := fakeLogger{}
	svc := domain.NewSubscriptionService(planRepo, subRepo, pub, logger)
	sf, err := kernel.NewSnowflake(1)
	if err != nil {
		t.Fatal(err)
	}
	return NewSubscriptionAppService(svc, sf)
}

func TestApp_CreatePlan(t *testing.T) {
	a := newTestAppService(t)
	resp, err := a.CreatePlan(context.Background(), CreatePlanRequest{
		Name: "Basic", Amount: 999, Interval: "month", IntervalCount: 1,
	})
	if err != nil {
		t.Fatal(err)
	}
	if resp.ID == 0 {
		t.Error("expected non-zero id")
	}
	if resp.Name != "Basic" {
		t.Errorf("Name = %s, want Basic", resp.Name)
	}
	if resp.Status != "active" {
		t.Errorf("Status = %s, want active", resp.Status)
	}
}

func TestApp_ListPlans(t *testing.T) {
	a := newTestAppService(t)
	a.CreatePlan(context.Background(), CreatePlanRequest{Name: "A", Amount: 100, Interval: "month", IntervalCount: 1})
	a.CreatePlan(context.Background(), CreatePlanRequest{Name: "B", Amount: 200, Interval: "month", IntervalCount: 1})

	plans, err := a.ListPlans(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	if len(plans) != 2 {
		t.Errorf("len = %d, want 2", len(plans))
	}
}

func TestApp_Subscribe(t *testing.T) {
	a := newTestAppService(t)
	plan, err := a.CreatePlan(context.Background(), CreatePlanRequest{
		Name: "Basic", Amount: 999, Interval: "month", IntervalCount: 1,
	})
	if err != nil {
		t.Fatal(err)
	}

	sub, err := a.Subscribe(context.Background(), 100, SubscribeRequest{PlanID: plan.ID})
	if err != nil {
		t.Fatal(err)
	}
	if sub.ID == 0 {
		t.Error("expected non-zero subscription id")
	}
	if sub.UserID != 100 {
		t.Errorf("UserID = %d, want 100", sub.UserID)
	}
	if sub.Status != "pending" {
		t.Errorf("Status = %s, want pending", sub.Status)
	}
}

func TestApp_CancelSubscription(t *testing.T) {
	a := newTestAppService(t)
	plan, _ := a.CreatePlan(context.Background(), CreatePlanRequest{
		Name: "Basic", Amount: 999, Interval: "month", IntervalCount: 1,
	})
	sub, _ := a.Subscribe(context.Background(), 100, SubscribeRequest{PlanID: plan.ID})
	cancelled, err := a.CancelSubscription(context.Background(), sub.ID)
	if err != nil {
		t.Fatal(err)
	}
	if cancelled.Status != "cancelled" {
		t.Errorf("Status = %s, want cancelled", cancelled.Status)
	}
}
