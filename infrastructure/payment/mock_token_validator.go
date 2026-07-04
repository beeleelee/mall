package payment

import (
	"context"
	"time"

	"github.com/beeleelee/mall/domain/kernel"
	domain "github.com/beeleelee/mall/domain/payment"
)

type MockWalletTokenValidator struct{}

func NewMockWalletTokenValidator() *MockWalletTokenValidator {
	return &MockWalletTokenValidator{}
}

func (v *MockWalletTokenValidator) ValidateToken(ctx context.Context, token, provider string) (*domain.TokenValidationResult, error) {
	if token == "" {
		return nil, kernel.NewDomainError(kernel.ErrInvalidArgument, "token must not be empty")
	}
	if provider == "" {
		return nil, kernel.NewDomainError(kernel.ErrInvalidArgument, "provider must not be empty")
	}

	return &domain.TokenValidationResult{
		Provider: provider,
		Token:    "validated-" + token,
		Expiry:   time.Now().Add(24 * time.Hour),
	}, nil
}
