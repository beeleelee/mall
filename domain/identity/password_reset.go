package identity

import (
	"context"
	"time"

	"github.com/beeleelee/mall/domain/kernel"
)

type PasswordResetToken struct {
	kernel.Entity
	UserID    kernel.ID
	TokenHash string
	ExpiresAt time.Time
	Used      bool
}

func NewPasswordResetToken(id, userID kernel.ID, tokenHash string, expiresAt time.Time) *PasswordResetToken {
	return &PasswordResetToken{
		Entity:    kernel.NewEntity(id),
		UserID:    userID,
		TokenHash: tokenHash,
		ExpiresAt: expiresAt,
		Used:      false,
	}
}

func (t *PasswordResetToken) IsExpired() bool {
	return time.Now().After(t.ExpiresAt)
}

func (t *PasswordResetToken) MarkUsed() {
	t.Used = true
	t.UpdatedAt = time.Now()
}

type PasswordResetTokenRepository interface {
	Save(ctx context.Context, token *PasswordResetToken) error
	FindByHash(ctx context.Context, hash string) (*PasswordResetToken, error)
	MarkUsed(ctx context.Context, id kernel.ID) error
	DeleteExpired(ctx context.Context) error
}
