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

type fakeStripeProcessor struct {
	createCheckoutSessionFn func(ctx context.Context, checkout *CheckoutSession) (string, string, error)
	createPaymentIntentFn   func(ctx context.Context, checkoutID kernel.ID, amount int64) (string, string, error)
	getPaymentIntentStatusFn func(ctx context.Context, paymentIntentID string) (string, error)
}

func (f *fakeStripeProcessor) CreateCheckoutSession(ctx context.Context, checkout *CheckoutSession) (string, string, error) {
	return f.createCheckoutSessionFn(ctx, checkout)
}

func (f *fakeStripeProcessor) CreatePaymentIntent(ctx context.Context, checkoutID kernel.ID, amount int64) (string, string, error) {
	return f.createPaymentIntentFn(ctx, checkoutID, amount)
}

func (f *fakeStripeProcessor) GetPaymentIntentStatus(ctx context.Context, paymentIntentID string) (string, error) {
	return f.getPaymentIntentStatusFn(ctx, paymentIntentID)
}

func TestCheckoutService_CreateStripePaymentIntent_Success(t *testing.T) {
	svc := checkoutTestServiceWithStripe()
	ctx := context.Background()

	setupFullCheckout(ctx, t, svc)

	clientSecret, intentID, err := svc.CreateStripePaymentIntent(ctx, 1)
	if err != nil {
		t.Fatal(err)
	}
	if clientSecret != "secret_pi_1" {
		t.Errorf("expected secret_pi_1, got %s", clientSecret)
	}
	if intentID != "pi_1" {
		t.Errorf("expected pi_1, got %s", intentID)
	}
}

func TestCheckoutService_CreateStripePaymentIntent_WrongHandler(t *testing.T) {
	svc := checkoutTestServiceWithStripe()
	ctx := context.Background()

	setupFullCheckout(ctx, t, svc)
	svc.SelectPaymentHandler(ctx, 1, "mock")

	_, _, err := svc.CreateStripePaymentIntent(ctx, 1)
	if !kernel.IsInvalidArgument(err) {
		t.Errorf("expected invalid argument, got %v", err)
	}
}

func TestCheckoutService_CreateStripePaymentIntent_NoProcessor(t *testing.T) {
	svc := checkoutTestService()
	ctx := context.Background()

	setupFullCheckout(ctx, t, svc)

	_, _, err := svc.CreateStripePaymentIntent(ctx, 1)
	if !kernel.IsInternal(err) {
		t.Errorf("expected internal error, got %v", err)
	}
}

func TestCheckoutService_ConfirmStripePayment_Success(t *testing.T) {
	svc := checkoutTestServiceWithStripe()
	ctx := context.Background()

	setupFullCheckout(ctx, t, svc)

	// First escalate via StartComplete to get into requires_escalation
	svc.StartComplete(ctx, 1, "")

	_, err := svc.ConfirmStripePayment(ctx, 1, "cs_test_123")
	if err != nil {
		t.Fatal(err)
	}

	session, _ := svc.GetCheckout(ctx, 1)
	if session.Status != CheckoutStatusCompleted {
		t.Errorf("expected completed, got %s", session.Status)
	}
}

func TestCheckoutService_ConfirmStripePayment_SessionIDMismatch(t *testing.T) {
	svc := checkoutTestServiceWithStripe()
	ctx := context.Background()

	setupFullCheckout(ctx, t, svc)

	svc.StartComplete(ctx, 1, "")

	_, err := svc.ConfirmStripePayment(ctx, 1, "wrong_session_id")
	if !kernel.IsInvalidArgument(err) {
		t.Errorf("expected invalid argument, got %v", err)
	}
}

func TestCheckoutService_StartComplete_StripeCheckoutMode(t *testing.T) {
	svc := checkoutTestServiceWithStripe()
	ctx := context.Background()

	setupFullCheckout(ctx, t, svc)

	session, escalated, err := svc.StartComplete(ctx, 1, "")
	if err != nil {
		t.Fatal(err)
	}
	if !escalated {
		t.Errorf("expected escalated=true for stripe checkout mode")
	}
	if session.Status != CheckoutStatusRequiresEscalation {
		t.Errorf("expected requires_escalation, got %s", session.Status)
	}
	if session.StripeSessionID != "cs_test_123" {
		t.Errorf("expected stripe_session_id cs_test_123, got %s", session.StripeSessionID)
	}
	if session.ContinueURL != "https://checkout.stripe.com/cs_test_123" {
		t.Errorf("expected stripe checkout URL, got %s", session.ContinueURL)
	}
}

func TestCheckoutService_StartComplete_StripePIMode(t *testing.T) {
	svc := checkoutTestServiceWithStripe()
	ctx := context.Background()

	setupFullCheckout(ctx, t, svc)

	// Create a PI first
	_, _, err := svc.CreateStripePaymentIntent(ctx, 1)
	if err != nil {
		t.Fatal(err)
	}

	session, escalated, err := svc.StartComplete(ctx, 1, "")
	if err != nil {
		t.Fatal(err)
	}
	if escalated {
		t.Errorf("expected escalated=false for stripe PI mode")
	}
	if session.Status != CheckoutStatusCompleted {
		t.Errorf("expected completed, got %s", session.Status)
	}
	if session.StripePaymentStatus != "succeeded" {
		t.Errorf("expected stripe_payment_status succeeded, got %s", session.StripePaymentStatus)
	}
}

func TestCheckoutService_Complete_StripePIMode(t *testing.T) {
	svc := checkoutTestServiceWithStripe()
	ctx := context.Background()

	setupFullCheckout(ctx, t, svc)

	svc.CreateStripePaymentIntent(ctx, 1)
	svc.MarkReady(ctx, 1)

	session, err := svc.Complete(ctx, 1)
	if err != nil {
		t.Fatal(err)
	}
	if session.Status != CheckoutStatusCompleted {
		t.Errorf("expected completed, got %s", session.Status)
	}
}

func setupFullCheckout(ctx context.Context, t *testing.T, svc *CheckoutService) {
	t.Helper()
	_, err := svc.CreateCheckout(ctx, CreateCheckoutInput{
		CheckoutID: 1, UserID: 42, CartID: 10, CartItems: sampleSnapshot().Items,
	})
	if err != nil {
		t.Fatal(err)
	}
	svc.SetShippingAddress(ctx, 1, sampleAddress())
	svc.SetBillingAddress(ctx, 1, sampleAddress())
	svc.SelectShippingOption(ctx, 1, sampleShippingOption())
	svc.SelectPaymentHandler(ctx, 1, "stripe")
}

func checkoutTestServiceWithStripe() *CheckoutService {
	return NewCheckoutService(
		newFakeCheckoutRepo(),
		fakeTaxService{},
		fakePriceCalculator{},
		newFakePublisher(),
		fakeLoggerCheckout{},
		nil,
		&fakeStripeProcessor{
			createCheckoutSessionFn: func(_ context.Context, _ *CheckoutSession) (string, string, error) {
				return "https://checkout.stripe.com/cs_test_123", "cs_test_123", nil
			},
			createPaymentIntentFn: func(_ context.Context, _ kernel.ID, _ int64) (string, string, error) {
				return "secret_pi_1", "pi_1", nil
			},
			getPaymentIntentStatusFn: func(_ context.Context, _ string) (string, error) {
				return "succeeded", nil
			},
		},
	)
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
