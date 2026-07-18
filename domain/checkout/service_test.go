package checkout

import (
	"context"
	"testing"

	"github.com/beeleelee/mall/domain/kernel"
)

func checkoutTestService() *CheckoutService {
	return NewCheckoutService(
		newFakeCheckoutRepo(),
		fakeTaxService{},
		fakePriceCalculator{},
		newFakePublisher(),
		fakeLoggerCheckout{},
		nil,
		nil,
	)
}

func TestCheckoutService_CreateCheckout_Success(t *testing.T) {
	svc := checkoutTestService()
	ctx := context.Background()

	s, err := svc.CreateCheckout(ctx, CreateCheckoutInput{
		CheckoutID: 1,
		UserID:     42,
		CartID:     10,
		CartItems:  sampleSnapshot().Items,
	})
	if err != nil {
		t.Fatal(err)
	}
	if s.ID != 1 {
		t.Errorf("expected ID 1, got %d", s.ID)
	}
	if s.Status != CheckoutStatusIncomplete {
		t.Errorf("expected incomplete, got %s", s.Status)
	}
}

func TestCheckoutService_CreateCheckout_InvalidUser(t *testing.T) {
	svc := checkoutTestService()
	_, err := svc.CreateCheckout(context.Background(), CreateCheckoutInput{
		CheckoutID: 1,
		UserID:     0,
		CartID:     10,
		CartItems:  sampleSnapshot().Items,
	})
	if !kernel.IsInvalidArgument(err) {
		t.Errorf("expected invalid argument, got %v", err)
	}
}

func TestCheckoutService_CreateCheckout_EmptyCart(t *testing.T) {
	svc := checkoutTestService()
	_, err := svc.CreateCheckout(context.Background(), CreateCheckoutInput{
		CheckoutID: 1,
		UserID:     42,
		CartID:     10,
	})
	if !kernel.IsInvalidArgument(err) {
		t.Errorf("expected invalid argument, got %v", err)
	}
}

func TestCheckoutService_SetShippingAddress(t *testing.T) {
	svc := checkoutTestService()
	ctx := context.Background()

	svc.CreateCheckout(ctx, CreateCheckoutInput{CheckoutID: 1, UserID: 42, CartID: 10, CartItems: sampleSnapshot().Items})

	s, err := svc.SetShippingAddress(ctx, 1, sampleAddress())
	if err != nil {
		t.Fatal(err)
	}
	if s.ShippingAddress == nil {
		t.Fatal("expected non-nil shipping address")
	}
}

func TestCheckoutService_SetBillingAddress(t *testing.T) {
	svc := checkoutTestService()
	ctx := context.Background()

	svc.CreateCheckout(ctx, CreateCheckoutInput{CheckoutID: 1, UserID: 42, CartID: 10, CartItems: sampleSnapshot().Items})

	s, err := svc.SetBillingAddress(ctx, 1, sampleAddress())
	if err != nil {
		t.Fatal(err)
	}
	if s.BillingAddress == nil {
		t.Fatal("expected non-nil billing address")
	}
}

func TestCheckoutService_SelectShippingOption(t *testing.T) {
	svc := checkoutTestService()
	ctx := context.Background()

	svc.CreateCheckout(ctx, CreateCheckoutInput{CheckoutID: 1, UserID: 42, CartID: 10, CartItems: sampleSnapshot().Items})

	s, err := svc.SelectShippingOption(ctx, 1, sampleShippingOption())
	if err != nil {
		t.Fatal(err)
	}
	if s.ShippingOption == nil {
		t.Fatal("expected non-nil shipping option")
	}
}

func TestCheckoutService_SelectPaymentHandler(t *testing.T) {
	svc := checkoutTestService()
	ctx := context.Background()

	svc.CreateCheckout(ctx, CreateCheckoutInput{CheckoutID: 1, UserID: 42, CartID: 10, CartItems: sampleSnapshot().Items})

	s, err := svc.SelectPaymentHandler(ctx, 1, "stripe")
	if err != nil {
		t.Fatal(err)
	}
	if s.PaymentHandler != "stripe" {
		t.Errorf("expected stripe, got %s", s.PaymentHandler)
	}
}

func TestCheckoutService_FullFlow(t *testing.T) {
	svc := checkoutTestService()
	ctx := context.Background()

	s, err := svc.CreateCheckout(ctx, CreateCheckoutInput{CheckoutID: 1, UserID: 42, CartID: 10, CartItems: sampleSnapshot().Items})
	if err != nil {
		t.Fatal(err)
	}

	s, _ = svc.SetShippingAddress(ctx, s.ID, sampleAddress())
	s, _ = svc.SetBillingAddress(ctx, s.ID, sampleAddress())
	s, _ = svc.SelectShippingOption(ctx, s.ID, sampleShippingOption())
	s, _ = svc.SelectPaymentHandler(ctx, s.ID, "stripe")
	s, _ = svc.CalculateTax(ctx, s.ID)
	s, err = svc.MarkReady(ctx, s.ID)
	if err != nil {
		t.Fatal(err)
	}
	if s.Status != CheckoutStatusReadyForComplete {
		t.Errorf("expected ready_for_complete, got %s", s.Status)
	}

	s, err = svc.Complete(ctx, s.ID)
	if err != nil {
		t.Fatal(err)
	}
	if s.Status != CheckoutStatusCompleted {
		t.Errorf("expected completed, got %s", s.Status)
	}
}

func TestCheckoutService_GetCheckout_NotFound(t *testing.T) {
	svc := checkoutTestService()
	_, err := svc.GetCheckout(context.Background(), 999)
	if !kernel.IsNotFound(err) {
		t.Errorf("expected not found, got %v", err)
	}
}

func TestCheckoutService_Cancel(t *testing.T) {
	svc := checkoutTestService()
	ctx := context.Background()

	svc.CreateCheckout(ctx, CreateCheckoutInput{CheckoutID: 1, UserID: 42, CartID: 10, CartItems: sampleSnapshot().Items})

	s, err := svc.Cancel(ctx, 1)
	if err != nil {
		t.Fatal(err)
	}
	if s.Status != CheckoutStatusCancelled {
		t.Errorf("expected cancelled, got %s", s.Status)
	}
}
