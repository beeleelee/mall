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

func (e TokenExchangedEvent) EventName() string      { return "payment.token_exchanged" }
func (e TokenExchangedEvent) OccurredAt() time.Time  { return time.Now() }
func (e TokenExchangedEvent) AggregateID() kernel.ID { return e.MandateID }

type TokenVerificationFailedEvent struct {
	MandateID kernel.ID `json:"mandate_id"`
	UserID    kernel.ID `json:"user_id"`
	Provider  string    `json:"provider"`
	Reason    string    `json:"reason"`
}

func (e TokenVerificationFailedEvent) EventName() string      { return "payment.token_verification_failed" }
func (e TokenVerificationFailedEvent) OccurredAt() time.Time  { return time.Now() }
func (e TokenVerificationFailedEvent) AggregateID() kernel.ID { return e.MandateID }
