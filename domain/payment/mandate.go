package payment

import (
	"time"

	"github.com/beeleelee/mall/domain/kernel"
)

type MandateStatus string

const (
	MandateStatusRequested MandateStatus = "requested"
	MandateStatusApproved  MandateStatus = "approved"
	MandateStatusExecuted  MandateStatus = "executed"
	MandateStatusSettled   MandateStatus = "settled"
	MandateStatusRefunded  MandateStatus = "refunded"
	MandateStatusCancelled MandateStatus = "cancelled"
	MandateStatusExpired   MandateStatus = "expired"
)

type MandateScope struct {
	MaxAmount       int64     `json:"max_amount"`
	MerchantID      kernel.ID `json:"merchant_id"`
	Expiry          time.Time `json:"expiry"`
	AllowedHandlers []string  `json:"allowed_handlers,omitempty"`
}

type Mandate struct {
	kernel.AggregateRoot
	UserID    kernel.ID
	Status    MandateStatus
	Scope     MandateScope
	Signature string
	Token     string
}

func NewMandate(id, userID kernel.ID, scope MandateScope) (*Mandate, error) {
	if userID <= 0 {
		return nil, kernel.NewDomainError(kernel.ErrInvalidArgument, "user_id must be positive")
	}
	if scope.MaxAmount <= 0 {
		return nil, kernel.NewDomainError(kernel.ErrInvalidArgument, "max_amount must be positive")
	}
	if scope.MerchantID <= 0 {
		return nil, kernel.NewDomainError(kernel.ErrInvalidArgument, "merchant_id must be positive")
	}
	if scope.Expiry.Before(time.Now()) {
		return nil, kernel.NewDomainError(kernel.ErrInvalidArgument, "expiry must be in the future")
	}

	m := &Mandate{
		AggregateRoot: kernel.NewAggregateRoot(id),
		UserID:        userID,
		Status:        MandateStatusRequested,
		Scope:         scope,
	}
	m.AddEvent(MandateRequestedEvent{
		MandateID:  id,
		UserID:     userID,
		MaxAmount:  scope.MaxAmount,
		MerchantID: scope.MerchantID,
	})
	return m, nil
}

func (m *Mandate) Approve(signature string) error {
	if m.Status != MandateStatusRequested {
		return kernel.NewDomainError(kernel.ErrInvalidArgument, "can only approve requested mandates")
	}
	if signature == "" {
		return kernel.NewDomainError(kernel.ErrInvalidArgument, "signature must not be empty")
	}
	if time.Now().After(m.Scope.Expiry) {
		m.Status = MandateStatusExpired
		return kernel.NewDomainError(kernel.ErrInvalidArgument, "mandate has expired")
	}
	m.Status = MandateStatusApproved
	m.Signature = signature
	m.AddEvent(MandateApprovedEvent{
		MandateID: m.ID,
		UserID:    m.UserID,
	})
	return nil
}

func (m *Mandate) Execute(token string) error {
	if m.Status != MandateStatusApproved {
		return kernel.NewDomainError(kernel.ErrInvalidArgument, "can only execute approved mandates")
	}
	if token == "" {
		return kernel.NewDomainError(kernel.ErrInvalidArgument, "token must not be empty")
	}
	m.Status = MandateStatusExecuted
	m.Token = token
	m.AddEvent(MandateExecutedEvent{
		MandateID: m.ID,
		UserID:    m.UserID,
		Token:     token,
	})
	return nil
}

func (m *Mandate) Settle() error {
	if m.Status != MandateStatusExecuted {
		return kernel.NewDomainError(kernel.ErrInvalidArgument, "can only settle executed mandates")
	}
	m.Status = MandateStatusSettled
	m.AddEvent(MandateSettledEvent{
		MandateID: m.ID,
		UserID:    m.UserID,
	})
	return nil
}

func (m *Mandate) Refund(amount int64) error {
	if m.Status != MandateStatusSettled {
		return kernel.NewDomainError(kernel.ErrInvalidArgument, "can only refund settled mandates")
	}
	if amount <= 0 {
		return kernel.NewDomainError(kernel.ErrInvalidArgument, "refund amount must be positive")
	}
	if amount > m.Scope.MaxAmount {
		return kernel.NewDomainError(kernel.ErrInvalidArgument, "refund amount exceeds mandate max amount")
	}
	m.Status = MandateStatusRefunded
	m.AddEvent(MandateRefundedEvent{
		MandateID: m.ID,
		UserID:    m.UserID,
		Amount:    amount,
	})
	return nil
}

func (m *Mandate) Cancel() error {
	if m.Status == MandateStatusSettled {
		return kernel.NewDomainError(kernel.ErrInvalidArgument, "cannot cancel settled mandate")
	}
	if m.Status == MandateStatusCancelled {
		return kernel.NewDomainError(kernel.ErrConflict, "mandate already cancelled")
	}
	m.Status = MandateStatusCancelled
	m.AddEvent(MandateCancelledEvent{
		MandateID: m.ID,
		UserID:    m.UserID,
	})
	return nil
}

func (m *Mandate) IsActive() bool {
	return m.Status == MandateStatusApproved || m.Status == MandateStatusExecuted
}

func (m *Mandate) HasExpired() bool {
	return time.Now().After(m.Scope.Expiry)
}
