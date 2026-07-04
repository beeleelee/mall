package checkout

import (
	"time"

	"github.com/beeleelee/mall/domain/kernel"
)

type CheckoutSession struct {
	kernel.AggregateRoot
	UserID          kernel.ID
	CartID          kernel.ID
	CartSnapshot    CartSnapshot
	ShippingAddress *Address
	BillingAddress  *Address
	ShippingOption  *ShippingOption
	PaymentHandler  string
	MandateID       kernel.ID
	WalletProvider  string
	WalletToken     string
	Subtotal        int64
	ShippingCost    int64
	TaxAmount       int64
	GrandTotal      int64
	Status          CheckoutStatus
	ContinueURL     string
	CompletedAt     *time.Time
}

func NewCheckoutSession(id, userID, cartID kernel.ID, snapshot CartSnapshot) (*CheckoutSession, error) {
	if userID <= 0 {
		return nil, kernel.NewDomainError(kernel.ErrInvalidArgument, "user_id must be positive")
	}
	if cartID <= 0 {
		return nil, kernel.NewDomainError(kernel.ErrInvalidArgument, "cart_id must be positive")
	}
	if len(snapshot.Items) == 0 {
		return nil, kernel.NewDomainError(kernel.ErrInvalidArgument, "cart snapshot must not be empty")
	}

	s := &CheckoutSession{
		AggregateRoot: kernel.NewAggregateRoot(id),
		UserID:        userID,
		CartID:        cartID,
		CartSnapshot:  snapshot,
		Status:        CheckoutStatusIncomplete,
		Subtotal:      snapshot.Total,
	}
	s.AddEvent(CheckoutCreatedEvent{CheckoutID: id, UserID: userID})
	return s, nil
}

func NewCheckoutSessionFromSnapshot(
	id, userID, cartID kernel.ID,
	snapshot CartSnapshot,
	shippingAddress, billingAddress *Address,
	shippingOption *ShippingOption,
	paymentHandler string,
	mandateID kernel.ID,
	walletProvider, walletToken string,
	subtotal, shippingCost, taxAmount, grandTotal int64,
	status CheckoutStatus,
	continueURL string,
	completedAt *time.Time,
	createdAt, updatedAt time.Time,
) *CheckoutSession {
	s := &CheckoutSession{
		AggregateRoot:   kernel.NewAggregateRoot(id),
		UserID:          userID,
		CartID:          cartID,
		CartSnapshot:    snapshot,
		ShippingAddress: shippingAddress,
		BillingAddress:  billingAddress,
		ShippingOption:  shippingOption,
		PaymentHandler:  paymentHandler,
		MandateID:       mandateID,
		WalletProvider:  walletProvider,
		WalletToken:     walletToken,
		Subtotal:        subtotal,
		ShippingCost:    shippingCost,
		TaxAmount:       taxAmount,
		GrandTotal:      grandTotal,
		Status:          status,
		ContinueURL:     continueURL,
		CompletedAt:     completedAt,
	}
	s.CreatedAt = createdAt
	s.UpdatedAt = updatedAt
	return s
}

func (s *CheckoutSession) SetShippingAddress(address Address) error {
	if s.Status != CheckoutStatusIncomplete {
		return kernel.NewDomainError(kernel.ErrInvalidArgument, "cannot set shipping address in current state: "+string(s.Status))
	}
	if !address.IsValid() {
		return kernel.NewDomainError(kernel.ErrInvalidArgument, "invalid shipping address")
	}
	s.ShippingAddress = &address
	s.touch()
	s.AddEvent(CheckoutUpdatedEvent{CheckoutID: s.ID, UserID: s.UserID})
	return nil
}

func (s *CheckoutSession) SetBillingAddress(address Address) error {
	if s.Status != CheckoutStatusIncomplete {
		return kernel.NewDomainError(kernel.ErrInvalidArgument, "cannot set billing address in current state: "+string(s.Status))
	}
	if !address.IsValid() {
		return kernel.NewDomainError(kernel.ErrInvalidArgument, "invalid billing address")
	}
	s.BillingAddress = &address
	s.touch()
	s.AddEvent(CheckoutUpdatedEvent{CheckoutID: s.ID, UserID: s.UserID})
	return nil
}

func (s *CheckoutSession) SelectShippingOption(option ShippingOption) error {
	if s.Status != CheckoutStatusIncomplete {
		return kernel.NewDomainError(kernel.ErrInvalidArgument, "cannot select shipping option in current state: "+string(s.Status))
	}
	if option.ID == "" {
		return kernel.NewDomainError(kernel.ErrInvalidArgument, "shipping option id must not be empty")
	}
	s.ShippingOption = &option
	s.ShippingCost = option.Cost
	s.recalculateTotal()
	s.touch()
	s.AddEvent(CheckoutUpdatedEvent{CheckoutID: s.ID, UserID: s.UserID})
	return nil
}

func (s *CheckoutSession) SelectPaymentHandler(handler string) error {
	if s.Status != CheckoutStatusIncomplete {
		return kernel.NewDomainError(kernel.ErrInvalidArgument, "cannot select payment handler in current state: "+string(s.Status))
	}
	if handler == "" {
		return kernel.NewDomainError(kernel.ErrInvalidArgument, "payment handler must not be empty")
	}
	s.PaymentHandler = handler
	s.touch()
	s.AddEvent(CheckoutUpdatedEvent{CheckoutID: s.ID, UserID: s.UserID})
	return nil
}

func (s *CheckoutSession) SetTaxAmount(amount int64) {
	s.TaxAmount = amount
	s.recalculateTotal()
	s.touch()
	s.AddEvent(CheckoutUpdatedEvent{CheckoutID: s.ID, UserID: s.UserID})
}

func (s *CheckoutSession) MarkReady() error {
	if s.Status != CheckoutStatusIncomplete {
		return kernel.NewDomainError(kernel.ErrInvalidArgument, "cannot mark ready in current state: "+string(s.Status))
	}
	if s.ShippingAddress == nil {
		return kernel.NewDomainError(kernel.ErrInvalidArgument, "shipping address required")
	}
	if s.BillingAddress == nil {
		return kernel.NewDomainError(kernel.ErrInvalidArgument, "billing address required")
	}
	if s.ShippingOption == nil {
		return kernel.NewDomainError(kernel.ErrInvalidArgument, "shipping option required")
	}
	if s.PaymentHandler == "" {
		return kernel.NewDomainError(kernel.ErrInvalidArgument, "payment handler required")
	}
	s.Status = CheckoutStatusReadyForComplete
	s.touch()
	s.AddEvent(CheckoutReadyForCompleteEvent{CheckoutID: s.ID, UserID: s.UserID})
	return nil
}

func (s *CheckoutSession) Escalate(continueURL string) error {
	if s.Status != CheckoutStatusIncomplete {
		return kernel.NewDomainError(kernel.ErrInvalidArgument, "cannot escalate in current state: "+string(s.Status))
	}
	if continueURL == "" {
		return kernel.NewDomainError(kernel.ErrInvalidArgument, "continue_url must not be empty")
	}
	s.Status = CheckoutStatusRequiresEscalation
	s.ContinueURL = continueURL
	s.touch()
	s.AddEvent(CheckoutRequiresEscalationEvent{CheckoutID: s.ID, UserID: s.UserID, ContinueURL: continueURL})
	return nil
}

func (s *CheckoutSession) SelectMandate(mandateID kernel.ID) error {
	if s.Status != CheckoutStatusIncomplete {
		return kernel.NewDomainError(kernel.ErrInvalidArgument, "can only select mandate when incomplete")
	}
	if mandateID <= 0 {
		return kernel.NewDomainError(kernel.ErrInvalidArgument, "mandate_id must be positive")
	}
	s.MandateID = mandateID
	s.touch()
	return nil
}

func (s *CheckoutSession) SubmitPaymentToken(provider, token string) error {
	if s.Status != CheckoutStatusIncomplete {
		return kernel.NewDomainError(kernel.ErrInvalidArgument, "can only submit payment token when incomplete")
	}
	if provider == "" {
		return kernel.NewDomainError(kernel.ErrInvalidArgument, "wallet provider must not be empty")
	}
	if token == "" {
		return kernel.NewDomainError(kernel.ErrInvalidArgument, "payment token must not be empty")
	}
	s.WalletProvider = provider
	s.WalletToken = token
	s.touch()
	s.AddEvent(CheckoutUpdatedEvent{CheckoutID: s.ID, UserID: s.UserID})
	return nil
}

func (s *CheckoutSession) Complete() error {
	if s.Status != CheckoutStatusReadyForComplete && s.Status != CheckoutStatusRequiresEscalation {
		return kernel.NewDomainError(kernel.ErrInvalidArgument, "cannot complete checkout in current state: "+string(s.Status))
	}
	now := time.Now()
	s.Status = CheckoutStatusCompleted
	s.CompletedAt = &now
	s.touch()
	s.AddEvent(CheckoutCompletedEvent{CheckoutID: s.ID, UserID: s.UserID})
	return nil
}

func (s *CheckoutSession) Cancel() error {
	if s.Status == CheckoutStatusCompleted {
		return kernel.NewDomainError(kernel.ErrInvalidArgument, "cannot cancel completed checkout")
	}
	if s.Status == CheckoutStatusCancelled {
		return kernel.NewDomainError(kernel.ErrConflict, "checkout already cancelled")
	}
	s.Status = CheckoutStatusCancelled
	s.touch()
	s.AddEvent(CheckoutCancelledEvent{CheckoutID: s.ID, UserID: s.UserID})
	return nil
}

func (s *CheckoutSession) IsEscalationRequired() bool {
	return s.Status == CheckoutStatusRequiresEscalation
}

func (s *CheckoutSession) recalculateTotal() {
	s.GrandTotal = s.Subtotal + s.ShippingCost + s.TaxAmount
}

func (s *CheckoutSession) touch() {
	s.UpdatedAt = time.Now()
}
