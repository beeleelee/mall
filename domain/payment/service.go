package payment

import (
	"context"

	"github.com/beeleelee/mall/domain/kernel"
)

type PaymentService struct {
	repo           MandateRepository
	logger         kernel.Logger
	tokenValidator WalletTokenValidator
}

func NewPaymentService(repo MandateRepository, logger kernel.Logger, opts ...PaymentServiceOption) *PaymentService {
	svc := &PaymentService{repo: repo, logger: logger}
	for _, opt := range opts {
		opt(svc)
	}
	return svc
}

type PaymentServiceOption func(*PaymentService)

func WithWalletTokenValidator(v WalletTokenValidator) PaymentServiceOption {
	return func(s *PaymentService) {
		s.tokenValidator = v
	}
}

func (s *PaymentService) RequestMandate(ctx context.Context, id, userID kernel.ID, scope MandateScope) (*Mandate, error) {
	m, err := NewMandate(id, userID, scope)
	if err != nil {
		return nil, err
	}

	if err := s.repo.Save(ctx, m); err != nil {
		return nil, err
	}

	s.logger.Info(ctx, "mandate.requested", kernel.Field("mandate_id", m.ID.String()), kernel.Field("user_id", userID.String()))
	return m, nil
}

func (s *PaymentService) ApproveMandate(ctx context.Context, id kernel.ID, signature string) (*Mandate, error) {
	m, err := s.repo.FindByID(ctx, id)
	if err != nil {
		return nil, err
	}

	if err := m.Approve(signature); err != nil {
		return nil, err
	}

	if err := s.repo.Save(ctx, m); err != nil {
		return nil, err
	}

	s.logger.Info(ctx, "mandate.approved", kernel.Field("mandate_id", m.ID.String()))
	return m, nil
}

func (s *PaymentService) ExecuteMandate(ctx context.Context, id kernel.ID, token string) (*Mandate, error) {
	m, err := s.repo.FindByID(ctx, id)
	if err != nil {
		return nil, err
	}

	if err := m.Execute(token); err != nil {
		return nil, err
	}

	if err := s.repo.Save(ctx, m); err != nil {
		return nil, err
	}

	s.logger.Info(ctx, "mandate.executed", kernel.Field("mandate_id", m.ID.String()))
	return m, nil
}

func (s *PaymentService) ExchangeWalletToken(ctx context.Context, mandateID kernel.ID, token, provider string, userID kernel.ID) (*Mandate, error) {
	if s.tokenValidator == nil {
		return nil, kernel.NewDomainError(kernel.ErrInvalidArgument, "wallet token validator not configured")
	}

	result, err := s.tokenValidator.ValidateToken(ctx, token, provider)
	if err != nil {
		m, getErr := s.repo.FindByID(ctx, mandateID)
		if getErr == nil {
			m.AddEvent(TokenVerificationFailedEvent{
				MandateID: mandateID,
				UserID:    userID,
				Provider:  provider,
				Reason:    err.Error(),
			})
			_ = s.repo.Save(ctx, m)
		}
		return nil, kernel.NewDomainErrorWithCause(kernel.ErrInvalidArgument, "token validation failed", err)
	}

	m, err := s.repo.FindByID(ctx, mandateID)
	if err != nil {
		return nil, err
	}

	if m.UserID != userID {
		return nil, kernel.NewDomainError(kernel.ErrPermissionDenied, "mandate does not belong to this user")
	}

	if err := m.Execute(result.Token); err != nil {
		return nil, err
	}

	m.AddEvent(TokenExchangedEvent{
		MandateID: mandateID,
		UserID:    userID,
		Provider:  provider,
		Token:     result.Token,
	})

	if err := s.repo.Save(ctx, m); err != nil {
		return nil, err
	}

	s.logger.Info(ctx, "payment.token_exchanged", kernel.Field("mandate_id", m.ID.String()), kernel.Field("provider", provider))
	return m, nil
}

func (s *PaymentService) SettleMandate(ctx context.Context, id kernel.ID) (*Mandate, error) {
	m, err := s.repo.FindByID(ctx, id)
	if err != nil {
		return nil, err
	}

	if err := m.Settle(); err != nil {
		return nil, err
	}

	if err := s.repo.Save(ctx, m); err != nil {
		return nil, err
	}

	s.logger.Info(ctx, "mandate.settled", kernel.Field("mandate_id", m.ID.String()))
	return m, nil
}

func (s *PaymentService) CancelMandate(ctx context.Context, id kernel.ID) (*Mandate, error) {
	m, err := s.repo.FindByID(ctx, id)
	if err != nil {
		return nil, err
	}

	if err := m.Cancel(); err != nil {
		return nil, err
	}

	if err := s.repo.Save(ctx, m); err != nil {
		return nil, err
	}

	s.logger.Info(ctx, "mandate.cancelled", kernel.Field("mandate_id", m.ID.String()))
	return m, nil
}

func (s *PaymentService) GetMandate(ctx context.Context, id kernel.ID) (*Mandate, error) {
	return s.repo.FindByID(ctx, id)
}

func (s *PaymentService) ListUserMandates(ctx context.Context, userID kernel.ID) ([]*Mandate, error) {
	return s.repo.FindByUserID(ctx, userID)
}
