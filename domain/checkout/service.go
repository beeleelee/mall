package checkout

import (
	"context"

	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/trace"

	"github.com/beeleelee/mall/domain/kernel"
)

var tracer = otel.Tracer("mall.domain.checkout")

type MandateVerifier interface {
	VerifyAndExecute(ctx context.Context, mandateID, userID kernel.ID, amount int64) error
	VerifyAndExecuteWithToken(ctx context.Context, mandateID, userID kernel.ID, amount int64, token, provider string) error
}

type StripeResult struct {
	ContinueURL  string
	ClientSecret string
}

type StripePaymentProcessor interface {
	CreateCheckoutSession(ctx context.Context, checkout *CheckoutSession) (stripeURL, sessionID string, err error)
	CreatePaymentIntent(ctx context.Context, checkoutID kernel.ID, amount int64) (clientSecret, intentID string, err error)
	GetPaymentIntentStatus(ctx context.Context, paymentIntentID string) (status string, err error)
}

type CheckoutService struct {
	repo              CheckoutRepository
	taxSvc            TaxService
	priceCalc         PriceCalculator
	publisher         CheckoutEventPublisher
	logger            kernel.Logger
	mandateVerifier   MandateVerifier
	stripeProcessor   StripePaymentProcessor
}

func NewCheckoutService(
	repo CheckoutRepository,
	taxSvc TaxService,
	priceCalc PriceCalculator,
	publisher CheckoutEventPublisher,
	logger kernel.Logger,
	mandateVerifier MandateVerifier,
	stripeProcessor StripePaymentProcessor,
) *CheckoutService {
	return &CheckoutService{
		repo:            repo,
		taxSvc:          taxSvc,
		priceCalc:       priceCalc,
		publisher:       publisher,
		logger:          logger,
		mandateVerifier: mandateVerifier,
		stripeProcessor: stripeProcessor,
	}
}

type CreateCheckoutInput struct {
	CheckoutID kernel.ID
	UserID     kernel.ID
	CartID     kernel.ID
	CartItems  []CartSnapshotItem
}

func (s *CheckoutService) CreateCheckout(ctx context.Context, input CreateCheckoutInput) (*CheckoutSession, error) {
	ctx, span := tracer.Start(ctx, "checkout.create",
		trace.WithAttributes(
			attribute.Int64("user_id", input.UserID.Int64()),
			attribute.Int64("cart_id", input.CartID.Int64()),
			attribute.Int("items", len(input.CartItems)),
		),
	)
	defer span.End()

	if input.UserID <= 0 {
		return nil, kernel.NewDomainError(kernel.ErrInvalidArgument, "user_id must be positive")
	}
	if len(input.CartItems) == 0 {
		return nil, kernel.NewDomainError(kernel.ErrInvalidArgument, "cart must not be empty")
	}

	snapshot := NewCartSnapshot(input.CartItems)
	session, err := NewCheckoutSession(input.CheckoutID, input.UserID, input.CartID, snapshot)
	if err != nil {
		return nil, err
	}

	if err := s.repo.Save(ctx, session); err != nil {
		return nil, err
	}

	s.publishEvents(ctx, session)
	s.logger.Info(ctx, "checkout.created", kernel.Field("checkout_id", session.ID.String()), kernel.Field("user_id", input.UserID.String()))
	return session, nil
}

func (s *CheckoutService) GetCheckout(ctx context.Context, id kernel.ID) (*CheckoutSession, error) {
	return s.repo.FindByID(ctx, id)
}

func (s *CheckoutService) GetCheckoutByUser(ctx context.Context, userID kernel.ID) (*CheckoutSession, error) {
	return s.repo.FindByUserID(ctx, userID)
}

func (s *CheckoutService) SetShippingAddress(ctx context.Context, id kernel.ID, address Address) (*CheckoutSession, error) {
	session, err := s.repo.FindByID(ctx, id)
	if err != nil {
		return nil, err
	}

	if err := session.SetShippingAddress(address); err != nil {
		return nil, err
	}

	if err := s.repo.Save(ctx, session); err != nil {
		return nil, err
	}

	s.publishEvents(ctx, session)
	return session, nil
}

func (s *CheckoutService) SetBillingAddress(ctx context.Context, id kernel.ID, address Address) (*CheckoutSession, error) {
	session, err := s.repo.FindByID(ctx, id)
	if err != nil {
		return nil, err
	}

	if err := session.SetBillingAddress(address); err != nil {
		return nil, err
	}

	if err := s.repo.Save(ctx, session); err != nil {
		return nil, err
	}

	s.publishEvents(ctx, session)
	return session, nil
}

func (s *CheckoutService) SelectShippingOption(ctx context.Context, id kernel.ID, option ShippingOption) (*CheckoutSession, error) {
	session, err := s.repo.FindByID(ctx, id)
	if err != nil {
		return nil, err
	}

	if err := session.SelectShippingOption(option); err != nil {
		return nil, err
	}

	if err := s.repo.Save(ctx, session); err != nil {
		return nil, err
	}

	s.publishEvents(ctx, session)
	return session, nil
}

func (s *CheckoutService) SelectMandate(ctx context.Context, id kernel.ID, mandateID kernel.ID) (*CheckoutSession, error) {
	session, err := s.repo.FindByID(ctx, id)
	if err != nil {
		return nil, err
	}

	if err := session.SelectMandate(mandateID); err != nil {
		return nil, err
	}

	if err := s.repo.Save(ctx, session); err != nil {
		return nil, err
	}

	s.publishEvents(ctx, session)
	return session, nil
}

func (s *CheckoutService) SubmitPaymentToken(ctx context.Context, id kernel.ID, provider, token string) (*CheckoutSession, error) {
	session, err := s.repo.FindByID(ctx, id)
	if err != nil {
		return nil, err
	}

	if err := session.SubmitPaymentToken(provider, token); err != nil {
		return nil, err
	}

	if err := s.repo.Save(ctx, session); err != nil {
		return nil, err
	}

	s.publishEvents(ctx, session)
	return session, nil
}

func (s *CheckoutService) SelectPaymentHandler(ctx context.Context, id kernel.ID, handler string) (*CheckoutSession, error) {
	session, err := s.repo.FindByID(ctx, id)
	if err != nil {
		return nil, err
	}

	if err := session.SelectPaymentHandler(handler); err != nil {
		return nil, err
	}

	if err := s.repo.Save(ctx, session); err != nil {
		return nil, err
	}

	s.publishEvents(ctx, session)
	return session, nil
}

func (s *CheckoutService) CalculateTax(ctx context.Context, id kernel.ID) (*CheckoutSession, error) {
	session, err := s.repo.FindByID(ctx, id)
	if err != nil {
		return nil, err
	}

	result, err := s.taxSvc.CalculateTax(ctx, TaxInput{
		Items:    session.CartSnapshot.Items,
		Subtotal: session.Subtotal,
		Cost:     session.ShippingCost,
		Address:  session.ShippingAddress,
	})
	if err != nil {
		return nil, err
	}

	session.SetTaxAmount(result.TaxAmount)

	if err := s.repo.Save(ctx, session); err != nil {
		return nil, err
	}

	s.publishEvents(ctx, session)
	return session, nil
}

func (s *CheckoutService) MarkReady(ctx context.Context, id kernel.ID) (*CheckoutSession, error) {
	session, err := s.repo.FindByID(ctx, id)
	if err != nil {
		return nil, err
	}

	if err := session.MarkReady(); err != nil {
		return nil, err
	}

	result, err := s.priceCalc.Calculate(ctx, PriceInput{
		Items:        session.CartSnapshot.Items,
		ShippingCost: session.ShippingCost,
		TaxAmount:    session.TaxAmount,
	})
	if err != nil {
		return nil, err
	}

	session.Subtotal = result.Subtotal
	session.ShippingCost = result.Shipping
	session.TaxAmount = result.Tax
	session.GrandTotal = result.GrandTotal

	if err := s.repo.Save(ctx, session); err != nil {
		return nil, err
	}

	s.publishEvents(ctx, session)
	return session, nil
}

func (s *CheckoutService) StartComplete(ctx context.Context, id kernel.ID, continueURL string) (*CheckoutSession, bool, error) {
	ctx, span := tracer.Start(ctx, "checkout.start_complete",
		trace.WithAttributes(attribute.Int64("checkout_id", id.Int64())),
	)
	defer span.End()

	session, err := s.repo.FindByID(ctx, id)
	if err != nil {
		return nil, false, err
	}

	if session.Status != CheckoutStatusIncomplete {
		return nil, false, kernel.NewDomainError(kernel.ErrInvalidArgument, "cannot start complete in current state: "+string(session.Status))
	}

	if session.ShippingAddress == nil || session.BillingAddress == nil ||
		session.ShippingOption == nil || session.PaymentHandler == "" {
		return nil, false, kernel.NewDomainError(kernel.ErrInvalidArgument, "shipping address, billing address, shipping option, and payment handler required")
	}

	if continueURL != "" {
		if session.PaymentHandler == "ap2_mandate" && s.mandateVerifier != nil && session.MandateID > 0 {
			if err := s.verifyMandate(ctx, session); err != nil {
				return nil, false, err
			}
		}

		if err := session.Escalate(continueURL); err != nil {
			return nil, false, err
		}
		if err := s.repo.Save(ctx, session); err != nil {
			return nil, false, err
		}
		s.publishEvents(ctx, session)
		return session, true, nil
	}

	// Stripe Checkout mode (no PI yet): create Checkout Session, escalate
	if session.PaymentHandler == "stripe" && s.stripeProcessor != nil && session.StripePaymentIntentID == "" {
		stripeURL, stripeSessionID, err := s.stripeProcessor.CreateCheckoutSession(ctx, session)
		if err != nil {
			return nil, false, err
		}
		if err := session.SetStripeSessionID(stripeSessionID); err != nil {
			return nil, false, err
		}
		if err := session.Escalate(stripeURL); err != nil {
			return nil, false, err
		}
		if err := s.repo.Save(ctx, session); err != nil {
			return nil, false, err
		}
		s.publishEvents(ctx, session)
		s.logger.Info(ctx, "checkout.stripe_escalated", kernel.Field("checkout_id", session.ID.String()), kernel.Field("stripe_session_id", stripeSessionID))
		return session, true, nil
	}

	// Stripe PI mode: verify PaymentIntent succeeded
	if session.PaymentHandler == "stripe" && s.stripeProcessor != nil && session.StripePaymentIntentID != "" {
		status, err := s.stripeProcessor.GetPaymentIntentStatus(ctx, session.StripePaymentIntentID)
		if err != nil {
			return nil, false, err
		}
		if status != "succeeded" {
			return nil, false, kernel.NewDomainError(kernel.ErrInvalidArgument, "stripe payment intent not succeeded, status: "+status)
		}
		session.SetStripePaymentStatus(status)
	}

	if session.PaymentHandler == "ap2_mandate" && s.mandateVerifier != nil && session.MandateID > 0 {
		if err := s.verifyMandate(ctx, session); err != nil {
			return nil, false, err
		}
	}

	if err := session.MarkReady(); err != nil {
		return nil, false, err
	}
	if err := session.Complete(); err != nil {
		return nil, false, err
	}

	if err := s.repo.Save(ctx, session); err != nil {
		return nil, false, err
	}
	s.publishEvents(ctx, session)
	s.logger.Info(ctx, "checkout.completed", kernel.Field("checkout_id", session.ID.String()), kernel.Field("user_id", session.UserID.String()))
	return session, false, nil
}

func (s *CheckoutService) Complete(ctx context.Context, id kernel.ID) (*CheckoutSession, error) {
	ctx, span := tracer.Start(ctx, "checkout.complete",
		trace.WithAttributes(attribute.Int64("checkout_id", id.Int64())),
	)
	defer span.End()

	session, err := s.repo.FindByID(ctx, id)
	if err != nil {
		return nil, err
	}

	if session.PaymentHandler == "ap2_mandate" && s.mandateVerifier != nil && session.MandateID > 0 {
		if err := s.verifyMandate(ctx, session); err != nil {
			return nil, err
		}
	}

	if session.PaymentHandler == "stripe" && s.stripeProcessor != nil && session.StripePaymentIntentID != "" {
		status, err := s.stripeProcessor.GetPaymentIntentStatus(ctx, session.StripePaymentIntentID)
		if err != nil {
			return nil, err
		}
		if status != "succeeded" {
			return nil, kernel.NewDomainError(kernel.ErrInvalidArgument, "stripe payment intent not succeeded, status: "+status)
		}
		session.SetStripePaymentStatus(status)
	}

	if err := session.Complete(); err != nil {
		return nil, err
	}

	if err := s.repo.Save(ctx, session); err != nil {
		return nil, err
	}

	s.publishEvents(ctx, session)
	s.logger.Info(ctx, "checkout.completed", kernel.Field("checkout_id", session.ID.String()), kernel.Field("user_id", session.UserID.String()))
	return session, nil
}

func (s *CheckoutService) Cancel(ctx context.Context, id kernel.ID) (*CheckoutSession, error) {
	ctx, span := tracer.Start(ctx, "checkout.cancel",
		trace.WithAttributes(attribute.Int64("checkout_id", id.Int64())),
	)
	defer span.End()

	session, err := s.repo.FindByID(ctx, id)
	if err != nil {
		return nil, err
	}

	if err := session.Cancel(); err != nil {
		return nil, err
	}

	if err := s.repo.Save(ctx, session); err != nil {
		return nil, err
	}

	s.publishEvents(ctx, session)
	return session, nil
}

func (s *CheckoutService) ConfirmStripePayment(ctx context.Context, id kernel.ID, stripeSessionID string) (*CheckoutSession, error) {
	ctx, span := tracer.Start(ctx, "checkout.confirm_stripe",
		trace.WithAttributes(attribute.Int64("checkout_id", id.Int64())),
	)
	defer span.End()

	session, err := s.repo.FindByID(ctx, id)
	if err != nil {
		return nil, err
	}

	if session.StripeSessionID != stripeSessionID {
		return nil, kernel.NewDomainError(kernel.ErrInvalidArgument, "stripe session id mismatch")
	}

	if session.Status != CheckoutStatusRequiresEscalation {
		return nil, kernel.NewDomainError(kernel.ErrInvalidArgument, "cannot confirm stripe payment in current state: "+string(session.Status))
	}

	if err := session.MarkReady(); err != nil {
		return nil, err
	}
	if err := session.Complete(); err != nil {
		return nil, err
	}

	if err := s.repo.Save(ctx, session); err != nil {
		return nil, err
	}
	s.publishEvents(ctx, session)
	s.logger.Info(ctx, "checkout.stripe_confirmed", kernel.Field("checkout_id", session.ID.String()), kernel.Field("stripe_session_id", stripeSessionID))
	return session, nil
}

func (s *CheckoutService) CreateStripePaymentIntent(ctx context.Context, id kernel.ID) (string, string, error) {
	session, err := s.repo.FindByID(ctx, id)
	if err != nil {
		return "", "", err
	}

	if session.Status != CheckoutStatusIncomplete {
		return "", "", kernel.NewDomainError(kernel.ErrInvalidArgument, "cannot create payment intent in current state: "+string(session.Status))
	}

	if session.PaymentHandler != "stripe" {
		return "", "", kernel.NewDomainError(kernel.ErrInvalidArgument, "payment handler must be stripe")
	}

	if s.stripeProcessor == nil {
		return "", "", kernel.NewDomainError(kernel.ErrInternal, "stripe processor not configured")
	}

	clientSecret, intentID, err := s.stripeProcessor.CreatePaymentIntent(ctx, session.ID, session.GrandTotal)
	if err != nil {
		return "", "", err
	}

	if err := session.SetStripePaymentIntentID(intentID); err != nil {
		return "", "", err
	}

	if err := s.repo.Save(ctx, session); err != nil {
		return "", "", err
	}

	s.logger.Info(ctx, "checkout.stripe_pi_created", kernel.Field("checkout_id", session.ID.String()), kernel.Field("payment_intent_id", intentID))
	return clientSecret, intentID, nil
}

func (s *CheckoutService) verifyMandate(ctx context.Context, session *CheckoutSession) error {
	if session.WalletToken != "" && session.WalletProvider != "" {
		return s.mandateVerifier.VerifyAndExecuteWithToken(ctx, session.MandateID, session.UserID, session.GrandTotal, session.WalletToken, session.WalletProvider)
	}
	return s.mandateVerifier.VerifyAndExecute(ctx, session.MandateID, session.UserID, session.GrandTotal)
}

func (s *CheckoutService) publishEvents(ctx context.Context, session *CheckoutSession) {
	for _, event := range session.Events() {
		s.logger.Info(ctx, "checkout.event", kernel.Field("event", event.EventName()), kernel.Field("checkout_id", session.ID.String()))

		name := event.EventName()
		if name == "checkout.ready_for_complete" || name == "checkout.requires_escalation" || name == "checkout.completed" || name == "checkout.cancelled" {
			if err := s.publisher.PublishCheckoutUpdated(ctx, session); err != nil {
				s.logger.Error(ctx, "checkout.publish failed", err, kernel.Field("checkout_id", session.ID.String()))
			}
		}
	}
	session.ClearEvents()
}
