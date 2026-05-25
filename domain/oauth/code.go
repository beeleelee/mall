package oauth

import (
	"crypto/rand"
	"encoding/hex"
	"time"

	"github.com/beeleelee/mall/domain/kernel"
)

type AuthorizationCode struct {
	Code        string
	ClientID    string
	UserID      kernel.ID
	RedirectURI string
	Scope       string
	ExpiresAt   time.Time
	Used        bool
}

func NewAuthorizationCode(clientID string, userID kernel.ID, redirectURI, scope string, ttl time.Duration) (*AuthorizationCode, error) {
	if clientID == "" {
		return nil, kernel.NewDomainError(kernel.ErrInvalidArgument, "client_id must not be empty")
	}
	if userID <= 0 {
		return nil, kernel.NewDomainError(kernel.ErrInvalidArgument, "user_id must be positive")
	}
	if redirectURI == "" {
		return nil, kernel.NewDomainError(kernel.ErrInvalidArgument, "redirect_uri must not be empty")
	}

	code, err := generateCode()
	if err != nil {
		return nil, err
	}

	return &AuthorizationCode{
		Code:        code,
		ClientID:    clientID,
		UserID:      userID,
		RedirectURI: redirectURI,
		Scope:       scope,
		ExpiresAt:   time.Now().Add(ttl),
		Used:        false,
	}, nil
}

func (a *AuthorizationCode) IsExpired() bool {
	return time.Now().After(a.ExpiresAt)
}

func (a *AuthorizationCode) MarkUsed() error {
	if a.Used {
		return kernel.NewDomainError(kernel.ErrConflict, "authorization code already used")
	}
	a.Used = true
	return nil
}

func generateCode() (string, error) {
	b := make([]byte, 32)
	if _, err := rand.Read(b); err != nil {
		return "", kernel.NewDomainErrorWithCause(kernel.ErrInternal, "failed to generate code", err)
	}
	return hex.EncodeToString(b), nil
}
