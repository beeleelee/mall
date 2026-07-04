package payment

import (
	"context"
	"time"

	"github.com/beeleelee/mall/domain/kernel"
)

type TokenValidationResult struct {
	Provider string `json:"provider"`
	Token    string `json:"token"`
	Expiry   time.Time `json:"expiry,omitempty"`
}

type WalletTokenValidator interface {
	ValidateToken(ctx context.Context, token, provider string) (*TokenValidationResult, error)
}

type TokenExchangedEvent struct {
	MandateID kernel.ID `json:"mandate_id"`
	UserID    kernel.ID `json:"user_id"`
	Provider  string    `json:"provider"`
	Token     string    `json:"token"`
}

func (e TokenExchangedEvent) EventName() string { return "payment.token_exchanged" }

type TokenVerificationFailedEvent struct {
	MandateID kernel.ID `json:"mandate_id"`
	UserID    kernel.ID `json:"user_id"`
	Provider  string    `json:"provider"`
	Reason    string    `json:"reason"`
}

func (e TokenVerificationFailedEvent) EventName() string { return "payment.token_verification_failed" }
