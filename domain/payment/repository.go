package payment

import (
	"context"

	"github.com/beeleelee/mall/domain/kernel"
)

type MandateRepository interface {
	Save(ctx context.Context, mandate *Mandate) error
	FindByID(ctx context.Context, id kernel.ID) (*Mandate, error)
	FindByUserID(ctx context.Context, userID kernel.ID) ([]*Mandate, error)
	FindActiveByUser(ctx context.Context, userID kernel.ID) ([]*Mandate, error)
	Delete(ctx context.Context, id kernel.ID) error
}
