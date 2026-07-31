package subscription

import (
	"context"
	"testing"
	"time"

	"github.com/beeleelee/mall/domain/kernel"
	domain "github.com/beeleelee/mall/domain/subscription"
)

func TestBillingService_ProcessDueBilling(t *testing.T) {
	planRepo := newFakePlanRepo()
	subRepo := newFakeSubRepo()
	pub := fakeSubPub{}
	logger := fakeLogger{}

	p, _ := domain.NewPlan(1, "Basic", "", 999, "month", 1, 0, nil)
	_ = planRepo.Save(context.Background(), p)
	_ = subRepo.Save(context.Background(), &domain.Subscription{
		AggregateRoot: kernel.NewAggregateRoot(1), UserID: 100, PlanID: 1,
		Status: domain.SubscriptionStatusActive, PaymentToken: "tok_1",
		CurrentPeriodStart: time.Now().AddDate(0, -1, 0),
		CurrentPeriodEnd:   time.Now().AddDate(0, 0, -1),
	})

	domainSvc := domain.NewSubscriptionService(planRepo, subRepo, pub, logger)
	billing := NewSubscriptionBillingService(domainSvc, logger)

	if err := billing.ProcessDueBilling(context.Background()); err != nil {
		t.Fatal(err)
	}

	s, err := subRepo.FindByID(context.Background(), 1)
	if err != nil {
		t.Fatal(err)
	}
	if !s.CurrentPeriodEnd.After(time.Now()) {
		t.Error("expected subscription period to be extended after billing")
	}
}
