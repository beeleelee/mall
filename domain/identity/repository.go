package identity

import (
	"context"

	"github.com/beeleelee/mall/domain/kernel"
)

type UserRepository interface {
	Save(ctx context.Context, user *User) error
	FindByID(ctx context.Context, id kernel.ID) (*User, error)
	FindByEmail(ctx context.Context, email string) (*User, error)
	Delete(ctx context.Context, id kernel.ID) error
}
