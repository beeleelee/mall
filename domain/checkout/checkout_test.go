package checkout

import (
	"testing"

	"github.com/beeleelee/mall/domain/kernel"
)

func sampleSnapshot() CartSnapshot {
	return NewCartSnapshot([]CartSnapshotItem{
		{ProductID: 100, SKU: "SKU001", Name: "Product 1", Quantity: 2, UnitPrice: 1000},
		{ProductID: 101, SKU: "SKU002", Name: "Product 2", Quantity: 1, UnitPrice: 2000},
	})
}

func sampleAddress() Address {
	return Address{
		Line1: "123 Main St", City: "Portland", State: "OR", PostalCode: "97201", Country: "US",
	}
}

func sampleShippingOption() ShippingOption {
	return ShippingOption{ID: "std", Name: "Standard", Cost: 500, Estimated: "5-7 business days"}
}

func TestNewCheckoutSession_Valid(t *testing.T) {
	s, err := NewCheckoutSession(1, 42, 10, sampleSnapshot())
	if err != nil {
		t.Fatal(err)
	}
	if s.ID != 1 {
		t.Errorf("expected ID 1, got %d", s.ID)
	}
	if s.UserID != 42 {
		t.Errorf("expected user 42, got %d", s.UserID)
	}
	if s.Status != CheckoutStatusIncomplete {
		t.Errorf("expected incomplete, got %s", s.Status)
	}
	if s.Subtotal != 4000 {
		t.Errorf("expected subtotal 4000, got %d", s.Subtotal)
	}
}

func TestNewCheckoutSession_InvalidUserID(t *testing.T) {
	_, err := NewCheckoutSession(1, 0, 10, sampleSnapshot())
	if !kernel.IsInvalidArgument(err) {
		t.Errorf("expected invalid argument, got %v", err)
	}
}

func TestNewCheckoutSession_InvalidCartID(t *testing.T) {
	_, err := NewCheckoutSession(1, 42, 0, sampleSnapshot())
	if !kernel.IsInvalidArgument(err) {
		t.Errorf("expected invalid argument, got %v", err)
	}
}

func TestNewCheckoutSession_EmptySnapshot(t *testing.T) {
	_, err := NewCheckoutSession(1, 42, 10, NewCartSnapshot(nil))
	if !kernel.IsInvalidArgument(err) {
		t.Errorf("expected invalid argument, got %v", err)
	}
}

func TestSetShippingAddress_Success(t *testing.T) {
	s, _ := NewCheckoutSession(1, 42, 10, sampleSnapshot())
	addr := sampleAddress()
	if err := s.SetShippingAddress(addr); err != nil {
		t.Fatal(err)
	}
	if s.ShippingAddress == nil {
		t.Fatal("expected non-nil shipping address")
	}
	if s.ShippingAddress.City != "Portland" {
		t.Errorf("expected Portland, got %s", s.ShippingAddress.City)
	}
}

func TestSetShippingAddress_InvalidAddress(t *testing.T) {
	s, _ := NewCheckoutSession(1, 42, 10, sampleSnapshot())
	err := s.SetShippingAddress(Address{Line1: "123 Main St"})
	if !kernel.IsInvalidArgument(err) {
		t.Errorf("expected invalid argument, got %v", err)
	}
}

func TestSetShippingAddress_WrongState(t *testing.T) {
	s, _ := NewCheckoutSession(1, 42, 10, sampleSnapshot())
	s.Complete() // transition to completed first — wait, need to go through proper flow
	// This should fail since MarkReady (which sets status to ready) requires addresses…
	// Let's shortcut: just modify status
	s.Status = CheckoutStatusCompleted

	err := s.SetShippingAddress(sampleAddress())
	if !kernel.IsInvalidArgument(err) {
		t.Errorf("expected invalid argument, got %v", err)
	}
}

func TestSetBillingAddress_Success(t *testing.T) {
	s, _ := NewCheckoutSession(1, 42, 10, sampleSnapshot())
	if err := s.SetBillingAddress(sampleAddress()); err != nil {
		t.Fatal(err)
	}
	if s.BillingAddress == nil {
		t.Fatal("expected non-nil billing address")
	}
}

func TestSelectShippingOption_Success(t *testing.T) {
	s, _ := NewCheckoutSession(1, 42, 10, sampleSnapshot())
	opt := sampleShippingOption()
	if err := s.SelectShippingOption(opt); err != nil {
		t.Fatal(err)
	}
	if s.ShippingOption == nil {
		t.Fatal("expected non-nil shipping option")
	}
	if s.ShippingCost != 500 {
		t.Errorf("expected shipping cost 500, got %d", s.ShippingCost)
	}
}

func TestSelectShippingOption_EmptyID(t *testing.T) {
	s, _ := NewCheckoutSession(1, 42, 10, sampleSnapshot())
	err := s.SelectShippingOption(ShippingOption{})
	if !kernel.IsInvalidArgument(err) {
		t.Errorf("expected invalid argument, got %v", err)
	}
}

func TestSelectPaymentHandler_Success(t *testing.T) {
	s, _ := NewCheckoutSession(1, 42, 10, sampleSnapshot())
	if err := s.SelectPaymentHandler("stripe"); err != nil {
		t.Fatal(err)
	}
	if s.PaymentHandler != "stripe" {
		t.Errorf("expected stripe, got %s", s.PaymentHandler)
	}
}

func TestSelectPaymentHandler_Empty(t *testing.T) {
	s, _ := NewCheckoutSession(1, 42, 10, sampleSnapshot())
	err := s.SelectPaymentHandler("")
	if !kernel.IsInvalidArgument(err) {
		t.Errorf("expected invalid argument, got %v", err)
	}
}

func TestSetTaxAmount(t *testing.T) {
	s, _ := NewCheckoutSession(1, 42, 10, sampleSnapshot())
	s.SelectShippingOption(sampleShippingOption())
	s.SetTaxAmount(800)
	if s.TaxAmount != 800 {
		t.Errorf("expected tax 800, got %d", s.TaxAmount)
	}
	if s.GrandTotal != 5300 {
		t.Errorf("expected grand total 5300 (4000+500+800), got %d", s.GrandTotal)
	}
}

func TestMarkReady_Success(t *testing.T) {
	s, _ := NewCheckoutSession(1, 42, 10, sampleSnapshot())
	s.SetShippingAddress(sampleAddress())
	s.SetBillingAddress(sampleAddress())
	s.SelectShippingOption(sampleShippingOption())
	s.SelectPaymentHandler("stripe")
	s.SetTaxAmount(800)

	if err := s.MarkReady(); err != nil {
		t.Fatal(err)
	}
	if s.Status != CheckoutStatusReadyForComplete {
		t.Errorf("expected ready_for_complete, got %s", s.Status)
	}
}

func TestMarkReady_MissingShippingAddress(t *testing.T) {
	s, _ := NewCheckoutSession(1, 42, 10, sampleSnapshot())
	s.SetBillingAddress(sampleAddress())
	s.SelectShippingOption(sampleShippingOption())
	s.SelectPaymentHandler("stripe")

	err := s.MarkReady()
	if !kernel.IsInvalidArgument(err) {
		t.Errorf("expected invalid argument, got %v", err)
	}
}

func TestMarkReady_MissingPaymentHandler(t *testing.T) {
	s, _ := NewCheckoutSession(1, 42, 10, sampleSnapshot())
	s.SetShippingAddress(sampleAddress())
	s.SetBillingAddress(sampleAddress())
	s.SelectShippingOption(sampleShippingOption())

	err := s.MarkReady()
	if !kernel.IsInvalidArgument(err) {
		t.Errorf("expected invalid argument, got %v", err)
	}
}

func TestComplete_Success(t *testing.T) {
	s, _ := NewCheckoutSession(1, 42, 10, sampleSnapshot())
	s.SetShippingAddress(sampleAddress())
	s.SetBillingAddress(sampleAddress())
	s.SelectShippingOption(sampleShippingOption())
	s.SelectPaymentHandler("stripe")
	s.MarkReady()

	if err := s.Complete(); err != nil {
		t.Fatal(err)
	}
	if s.Status != CheckoutStatusCompleted {
		t.Errorf("expected completed, got %s", s.Status)
	}
	if s.CompletedAt == nil {
		t.Error("expected non-nil completed_at")
	}
}

func TestComplete_WrongState(t *testing.T) {
	s, _ := NewCheckoutSession(1, 42, 10, sampleSnapshot())
	err := s.Complete()
	if !kernel.IsInvalidArgument(err) {
		t.Errorf("expected invalid argument, got %v", err)
	}
}

func TestCancel_Success(t *testing.T) {
	s, _ := NewCheckoutSession(1, 42, 10, sampleSnapshot())
	if err := s.Cancel(); err != nil {
		t.Fatal(err)
	}
	if s.Status != CheckoutStatusCancelled {
		t.Errorf("expected cancelled, got %s", s.Status)
	}
}

func TestCancel_AlreadyCancelled(t *testing.T) {
	s, _ := NewCheckoutSession(1, 42, 10, sampleSnapshot())
	s.Cancel()
	err := s.Cancel()
	if !kernel.IsConflict(err) {
		t.Errorf("expected conflict, got %v", err)
	}
}

func TestCancel_CompletedCheckout(t *testing.T) {
	s, _ := NewCheckoutSession(1, 42, 10, sampleSnapshot())
	s.SetShippingAddress(sampleAddress())
	s.SetBillingAddress(sampleAddress())
	s.SelectShippingOption(sampleShippingOption())
	s.SelectPaymentHandler("stripe")
	s.MarkReady()
	s.Complete()

	err := s.Cancel()
	if !kernel.IsInvalidArgument(err) {
		t.Errorf("expected invalid argument, got %v", err)
	}
}

func TestAddressIsValid(t *testing.T) {
	addr := Address{Line1: "123 Main St", City: "Portland", State: "OR", PostalCode: "97201", Country: "US"}
	if !addr.IsValid() {
		t.Error("expected valid address")
	}
}

func TestAddressIsInvalid(t *testing.T) {
	addr := Address{Line1: "123 Main St"}
	if addr.IsValid() {
		t.Error("expected invalid address")
	}
}

func TestCartSnapshotItemTotalPrice(t *testing.T) {
	item := CartSnapshotItem{Quantity: 3, UnitPrice: 1500}
	if item.TotalPrice() != 4500 {
		t.Errorf("expected 4500, got %d", item.TotalPrice())
	}
}

func TestCartSnapshotTotal(t *testing.T) {
	items := []CartSnapshotItem{
		{Quantity: 2, UnitPrice: 1000},
		{Quantity: 3, UnitPrice: 500},
	}
	snapshot := NewCartSnapshot(items)
	if snapshot.Total != 3500 {
		t.Errorf("expected 3500, got %d", snapshot.Total)
	}
}
