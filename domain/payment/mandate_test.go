package payment

import (
	"testing"
	"time"

	"github.com/beeleelee/mall/domain/kernel"
)

func TestNewMandate(t *testing.T) {
	m, err := NewMandate(1, 1, MandateScope{
		MaxAmount:  10000,
		MerchantID: 1,
		Expiry:     time.Now().Add(24 * time.Hour),
	})
	if err != nil {
		t.Fatal(err)
	}
	if m.Status != MandateStatusRequested {
		t.Fatalf("expected requested, got %s", m.Status)
	}
}

func TestApproveMandate(t *testing.T) {
	m, _ := NewMandate(1, 1, MandateScope{
		MaxAmount:  10000,
		MerchantID: 1,
		Expiry:     time.Now().Add(24 * time.Hour),
	})

	if err := m.Approve("test-sig"); err != nil {
		t.Fatal(err)
	}
	if m.Status != MandateStatusApproved {
		t.Fatalf("expected approved, got %s", m.Status)
	}
	if m.Signature != "test-sig" {
		t.Fatalf("expected test-sig, got %s", m.Signature)
	}
}

func TestMandateLifecycle(t *testing.T) {
	m, _ := NewMandate(1, 1, MandateScope{
		MaxAmount:  10000,
		MerchantID: 1,
		Expiry:     time.Now().Add(24 * time.Hour),
	})

	if err := m.Approve("sig"); err != nil {
		t.Fatal(err)
	}

	if err := m.Execute("tok_123"); err != nil {
		t.Fatal(err)
	}
	if m.Token != "tok_123" {
		t.Fatalf("expected tok_123, got %s", m.Token)
	}

	if err := m.Settle(); err != nil {
		t.Fatal(err)
	}
	if m.Status != MandateStatusSettled {
		t.Fatalf("expected settled, got %s", m.Status)
	}
}

func TestMandateValidation(t *testing.T) {
	_, err := NewMandate(1, 0, MandateScope{
		MaxAmount:  10000,
		MerchantID: 1,
		Expiry:     time.Now().Add(24 * time.Hour),
	})
	if err == nil {
		t.Fatal("expected error for zero user_id")
	}

	_, err = NewMandate(1, 1, MandateScope{
		MaxAmount:  0,
		MerchantID: 1,
		Expiry:     time.Now().Add(24 * time.Hour),
	})
	if err == nil {
		t.Fatal("expected error for zero max_amount")
	}

	scope := MandateScope{
		MaxAmount:  10000,
		MerchantID: 1,
		Expiry:     time.Now().Add(-1 * time.Hour),
	}
	_, err = NewMandate(1, 1, scope)
	if err == nil {
		t.Fatal("expected error for past expiry")
	}
}

func TestMandateApproveEmptySignature(t *testing.T) {
	m, _ := NewMandate(1, 1, MandateScope{
		MaxAmount:  10000,
		MerchantID: 1,
		Expiry:     time.Now().Add(24 * time.Hour),
	})

	if err := m.Approve(""); err == nil {
		t.Fatal("expected error for empty signature")
	}
}

func TestMandateCancel(t *testing.T) {
	m, _ := NewMandate(1, 1, MandateScope{
		MaxAmount:  10000,
		MerchantID: 1,
		Expiry:     time.Now().Add(24 * time.Hour),
	})

	if err := m.Cancel(); err != nil {
		t.Fatal(err)
	}
	if m.Status != MandateStatusCancelled {
		t.Fatalf("expected cancelled, got %s", m.Status)
	}

	if err := m.Cancel(); err == nil {
		t.Fatal("expected error cancelling already cancelled mandate")
	}
}

func TestIsActive(t *testing.T) {
	m, _ := NewMandate(1, 1, MandateScope{
		MaxAmount:  10000,
		MerchantID: 1,
		Expiry:     time.Now().Add(24 * time.Hour),
	})

	if m.IsActive() {
		t.Fatal("requested mandate should not be active")
	}

	m.Approve("sig")
	if !m.IsActive() {
		t.Fatal("approved mandate should be active")
	}

	m.Execute("tok")
	if !m.IsActive() {
		t.Fatal("executed mandate should be active")
	}

	m.Settle()
	if m.IsActive() {
		t.Fatal("settled mandate should not be active")
	}
}

func TestInvalidTransitions(t *testing.T) {
	m, _ := NewMandate(1, 1, MandateScope{
		MaxAmount:  10000,
		MerchantID: 1,
		Expiry:     time.Now().Add(24 * time.Hour),
	})

	if err := m.Settle(); !kernel.IsInvalidArgument(err) {
		t.Fatalf("expected invalid argument, got %v", err)
	}

	if err := m.Execute("tok"); !kernel.IsInvalidArgument(err) {
		t.Fatalf("expected invalid argument, got %v", err)
	}
}
