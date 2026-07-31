package subscription

import (
	"testing"

	"github.com/beeleelee/mall/domain/kernel"
)

func validPlan(t *testing.T) *Plan {
	t.Helper()
	p, err := NewPlan(1, "Basic", "Basic plan", 999, "month", 1, 0, []string{"feature1"})
	if err != nil {
		t.Fatal(err)
	}
	return p
}

func validSubscription(t *testing.T, id, userID kernel.ID, plan *Plan) *Subscription {
	t.Helper()
	s, err := NewSubscription(id, userID, plan.ID, plan)
	if err != nil {
		t.Fatal(err)
	}
	return s
}

func TestNewPlan_Success(t *testing.T) {
	p, err := NewPlan(1, "Pro", "Pro plan", 1999, "month", 1, 7, []string{"f1", "f2"})
	if err != nil {
		t.Fatal(err)
	}
	if p.Name != "Pro" {
		t.Errorf("expected Pro, got %s", p.Name)
	}
	if p.Amount != 1999 {
		t.Errorf("expected 1999, got %d", p.Amount)
	}
	if p.Interval != "month" {
		t.Errorf("expected month, got %s", p.Interval)
	}
	if p.TrialDays != 7 {
		t.Errorf("expected 7, got %d", p.TrialDays)
	}
	if p.Status != PlanStatusActive {
		t.Errorf("expected active, got %s", p.Status)
	}
}

func TestNewPlan_NilFeatures(t *testing.T) {
	p, err := NewPlan(1, "Min", "", 500, "month", 1, 0, nil)
	if err != nil {
		t.Fatal(err)
	}
	if len(p.Features) != 0 {
		t.Errorf("expected empty features, got %v", p.Features)
	}
}

func TestNewPlan_InvalidID(t *testing.T) {
	_, err := NewPlan(0, "X", "", 100, "month", 1, 0, nil)
	if !kernel.IsInvalidArgument(err) {
		t.Fatalf("expected invalid argument, got %v", err)
	}
}

func TestNewPlan_EmptyName(t *testing.T) {
	_, err := NewPlan(1, "", "", 100, "month", 1, 0, nil)
	if !kernel.IsInvalidArgument(err) {
		t.Fatalf("expected invalid argument, got %v", err)
	}
}

func TestNewPlan_ZeroAmount(t *testing.T) {
	_, err := NewPlan(1, "X", "", 0, "month", 1, 0, nil)
	if !kernel.IsInvalidArgument(err) {
		t.Fatalf("expected invalid argument, got %v", err)
	}
}

func TestNewPlan_InvalidInterval(t *testing.T) {
	_, err := NewPlan(1, "X", "", 100, "weekly", 1, 0, nil)
	if !kernel.IsInvalidArgument(err) {
		t.Fatalf("expected invalid argument, got %v", err)
	}
}

func TestPlan_Deactivate(t *testing.T) {
	p := validPlan(t)
	if err := p.Deactivate(); err != nil {
		t.Fatal(err)
	}
	if p.Status != PlanStatusInactive {
		t.Errorf("expected inactive, got %s", p.Status)
	}
}

func TestPlan_Deactivate_AlreadyInactive(t *testing.T) {
	p := validPlan(t)
	p.Deactivate()
	if err := p.Deactivate(); !kernel.IsInvalidArgument(err) {
		t.Fatalf("expected invalid argument, got %v", err)
	}
}

func TestPlan_Activate(t *testing.T) {
	p := validPlan(t)
	p.Deactivate()
	if err := p.Activate(); err != nil {
		t.Fatal(err)
	}
	if p.Status != PlanStatusActive {
		t.Errorf("expected active, got %s", p.Status)
	}
}

func TestPlan_Activate_AlreadyActive(t *testing.T) {
	p := validPlan(t)
	if err := p.Activate(); !kernel.IsInvalidArgument(err) {
		t.Fatalf("expected invalid argument, got %v", err)
	}
}

func TestNewSubscription_Success_Pending(t *testing.T) {
	p := validPlan(t)
	s, err := NewSubscription(1, 100, p.ID, p)
	if err != nil {
		t.Fatal(err)
	}
	if s.Status != SubscriptionStatusPending {
		t.Errorf("expected pending, got %s", s.Status)
	}
	if s.UserID != 100 {
		t.Errorf("expected user 100, got %d", s.UserID)
	}
	if s.PlanID != p.ID {
		t.Errorf("expected plan %d, got %d", p.ID, s.PlanID)
	}
	if s.CurrentPeriodEnd.Before(s.CurrentPeriodStart) {
		t.Error("period end should be after period start")
	}
	if len(s.Events()) != 1 {
		t.Errorf("expected 1 event, got %d", len(s.Events()))
	}
}

func TestNewSubscription_WithTrial(t *testing.T) {
	p, err := NewPlan(1, "Trial", "", 999, "month", 1, 14, nil)
	if err != nil {
		t.Fatal(err)
	}
	s, err := NewSubscription(1, 100, p.ID, p)
	if err != nil {
		t.Fatal(err)
	}
	if s.Status != SubscriptionStatusTrialing {
		t.Errorf("expected trialing, got %s", s.Status)
	}
	if s.TrialEndsAt == nil {
		t.Fatal("expected trial end date")
	}
}

func TestNewSubscription_NilPlan(t *testing.T) {
	_, err := NewSubscription(1, 100, 1, nil)
	if !kernel.IsInvalidArgument(err) {
		t.Fatalf("expected invalid argument, got %v", err)
	}
}

func TestNewSubscription_InactivePlan(t *testing.T) {
	p := validPlan(t)
	p.Deactivate()
	_, err := NewSubscription(1, 100, p.ID, p)
	if !kernel.IsInvalidArgument(err) {
		t.Fatalf("expected invalid argument, got %v", err)
	}
}

func TestNewSubscription_InvalidID(t *testing.T) {
	p := validPlan(t)
	_, err := NewSubscription(0, 100, p.ID, p)
	if !kernel.IsInvalidArgument(err) {
		t.Fatalf("expected invalid argument, got %v", err)
	}
}

func TestSubscription_Activate(t *testing.T) {
	s := validSubscription(t, 1, 100, validPlan(t))
	if err := s.Activate(); err != nil {
		t.Fatal(err)
	}
	if s.Status != SubscriptionStatusActive {
		t.Errorf("expected active, got %s", s.Status)
	}
}

func TestSubscription_Activate_InvalidState(t *testing.T) {
	s := validSubscription(t, 1, 100, validPlan(t))
	s.Activate()
	if err := s.Activate(); !kernel.IsInvalidArgument(err) {
		t.Fatalf("expected invalid argument, got %v", err)
	}
}

func TestSubscription_Cancel(t *testing.T) {
	s := validSubscription(t, 1, 100, validPlan(t))
	s.Activate()
	if err := s.Cancel(); err != nil {
		t.Fatal(err)
	}
	if s.Status != SubscriptionStatusCancelled {
		t.Errorf("expected cancelled, got %s", s.Status)
	}
	if s.CancelledAt == nil {
		t.Fatal("expected cancelled_at")
	}
}

func TestSubscription_Cancel_Double(t *testing.T) {
	s := validSubscription(t, 1, 100, validPlan(t))
	s.Activate()
	s.Cancel()
	if err := s.Cancel(); !kernel.IsInvalidArgument(err) {
		t.Fatalf("expected invalid argument, got %v", err)
	}
}

func TestSubscription_Expire(t *testing.T) {
	s := validSubscription(t, 1, 100, validPlan(t))
	s.Activate()
	if err := s.Expire(); err != nil {
		t.Fatal(err)
	}
	if s.Status != SubscriptionStatusExpired {
		t.Errorf("expected expired, got %s", s.Status)
	}
}

func TestSubscription_Expire_InvalidState(t *testing.T) {
	s := validSubscription(t, 1, 100, validPlan(t))
	if err := s.Expire(); !kernel.IsInvalidArgument(err) {
		t.Fatalf("expected invalid argument, got %v", err)
	}
}

func TestSubscription_FailPayment(t *testing.T) {
	s := validSubscription(t, 1, 100, validPlan(t))
	s.Activate()
	if err := s.FailPayment(); err != nil {
		t.Fatal(err)
	}
	if s.Status != SubscriptionStatusPastDue {
		t.Errorf("expected past_due, got %s", s.Status)
	}
}

func TestSubscription_FailPayment_FromPastDue(t *testing.T) {
	s := validSubscription(t, 1, 100, validPlan(t))
	s.Activate()
	s.FailPayment()
	if err := s.FailPayment(); err != nil {
		t.Fatal(err)
	}
}

func TestSubscription_ChangePlan(t *testing.T) {
	s := validSubscription(t, 1, 100, validPlan(t))
	s.Activate()
	p2, err := NewPlan(2, "Pro", "", 1999, "month", 1, 0, nil)
	if err != nil {
		t.Fatal(err)
	}
	if err := s.ChangePlan(p2); err != nil {
		t.Fatal(err)
	}
	if s.PlanID != 2 {
		t.Errorf("expected plan 2, got %d", s.PlanID)
	}
}

func TestSubscription_ChangePlan_InvalidState(t *testing.T) {
	s := validSubscription(t, 1, 100, validPlan(t))
	p2, _ := NewPlan(2, "Pro", "", 1999, "month", 1, 0, nil)
	if err := s.ChangePlan(p2); !kernel.IsInvalidArgument(err) {
		t.Fatalf("expected invalid argument, got %v", err)
	}
}

func TestSubscription_ChangePlan_Nil(t *testing.T) {
	s := validSubscription(t, 1, 100, validPlan(t))
	s.Activate()
	if err := s.ChangePlan(nil); !kernel.IsInvalidArgument(err) {
		t.Fatalf("expected invalid argument, got %v", err)
	}
}

func TestSubscription_ChangePlan_Inactive(t *testing.T) {
	s := validSubscription(t, 1, 100, validPlan(t))
	s.Activate()
	p2, _ := NewPlan(2, "Pro", "", 1999, "month", 1, 0, nil)
	p2.Deactivate()
	if err := s.ChangePlan(p2); !kernel.IsInvalidArgument(err) {
		t.Fatalf("expected invalid argument, got %v", err)
	}
}

func TestSubscription_Renew(t *testing.T) {
	s := validSubscription(t, 1, 100, validPlan(t))
	s.Activate()
	p := validPlan(t)
	if err := s.Renew(p); err != nil {
		t.Fatal(err)
	}
	if s.CurrentPeriodEnd.Before(s.CurrentPeriodStart) {
		t.Error("period end should be after period start after renew")
	}
}

func TestSubscription_AttachPaymentToken(t *testing.T) {
	s := validSubscription(t, 1, 100, validPlan(t))
	if err := s.AttachPaymentToken("tok_test_123"); err != nil {
		t.Fatal(err)
	}
	if s.PaymentToken != "tok_test_123" {
		t.Errorf("expected payment token set, got %q", s.PaymentToken)
	}
}

func TestSubscription_AttachPaymentToken_Empty(t *testing.T) {
	s := validSubscription(t, 1, 100, validPlan(t))
	if err := s.AttachPaymentToken(""); !kernel.IsInvalidArgument(err) {
		t.Fatalf("expected invalid argument, got %v", err)
	}
}

func TestSubscription_AttachPaymentToken_InvalidState(t *testing.T) {
	s := validSubscription(t, 1, 100, validPlan(t))
	s.Activate()
	s.Cancel()
	if err := s.AttachPaymentToken("tok_test_123"); !kernel.IsInvalidArgument(err) {
		t.Fatalf("expected invalid argument, got %v", err)
	}
}

func TestSubscription_Expire_FromTrialing(t *testing.T) {
	p, _ := NewPlan(1, "Trial", "", 999, "month", 1, 7, nil)
	s, err := NewSubscription(1, 100, p.ID, p)
	if err != nil {
		t.Fatal(err)
	}
	if s.Status != SubscriptionStatusTrialing {
		t.Fatalf("expected trialing, got %s", s.Status)
	}
	if err := s.Expire(); err != nil {
		t.Fatal(err)
	}
	if s.Status != SubscriptionStatusExpired {
		t.Errorf("expected expired, got %s", s.Status)
	}
}

func TestSubscription_IsActive(t *testing.T) {
	s := validSubscription(t, 1, 100, validPlan(t))
	if s.IsActive() {
		t.Error("expected pending to be not active")
	}
	s.Activate()
	if !s.IsActive() {
		t.Error("expected active to be active")
	}
	s.Cancel()
	if s.IsActive() {
		t.Error("expected cancelled to be not active")
	}
}

func TestSubscription_EventCount(t *testing.T) {
	s := validSubscription(t, 1, 100, validPlan(t))
	s.ClearEvents()
	s.Activate()
	if len(s.Events()) != 1 {
		t.Fatalf("expected 1 event after activate, got %d", len(s.Events()))
	}
	if s.Events()[0].EventName() != "subscription.activated" {
		t.Errorf("expected subscription.activated, got %s", s.Events()[0].EventName())
	}
}
