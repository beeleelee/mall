package payment

import (
	"context"
	"fmt"
	"time"

	"github.com/stripe/stripe-go/v74/paymentmethod"

	domainPayment "github.com/beeleelee/mall/domain/payment"
)

type StripeWalletTokenValidator struct {
	client *StripeClient
}

func NewStripeWalletTokenValidator(client *StripeClient) *StripeWalletTokenValidator {
	return &StripeWalletTokenValidator{client: client}
}

func (v *StripeWalletTokenValidator) ValidateToken(ctx context.Context, token, provider string) (*domainPayment.TokenValidationResult, error) {
	if provider != "stripe" {
		return nil, fmt.Errorf("unsupported provider: %s", provider)
	}

	if token == "" {
		return nil, fmt.Errorf("token must not be empty")
	}

	pm, err := paymentmethod.Get(token, nil)
	if err != nil {
		return nil, fmt.Errorf("validate stripe payment method: %w", err)
	}

	return &domainPayment.TokenValidationResult{
		Provider: "stripe",
		Token:    pm.ID,
		Expiry:   time.Now().Add(24 * time.Hour),
	}, nil
}

var _ domainPayment.WalletTokenValidator = (*StripeWalletTokenValidator)(nil)
