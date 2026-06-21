package identity

import (
	"context"

	"github.com/beeleelee/mall/domain/kernel"
)

type IdentityService struct {
	repo   UserRepository
	logger kernel.Logger
}

func NewIdentityService(repo UserRepository, logger kernel.Logger) *IdentityService {
	return &IdentityService{
		repo:   repo,
		logger: logger,
	}
}

func (s *IdentityService) Register(ctx context.Context, id kernel.ID, email, name, plaintextPassword string) (*User, error) {
	s.logger.Info(ctx, "identity.register", kernel.Field("email", email))

	existing, err := s.repo.FindByEmail(ctx, email)
	if err != nil && !kernel.IsNotFound(err) {
		return nil, err
	}
	if existing != nil {
		return nil, kernel.NewDomainError(kernel.ErrAlreadyExists, "email already registered")
	}

	password, err := NewPassword(plaintextPassword)
	if err != nil {
		return nil, err
	}

	user, err := NewUser(id, email, name, password, nil)
	if err != nil {
		return nil, err
	}

	if err := s.repo.Save(ctx, user); err != nil {
		return nil, err
	}

	s.logger.Info(ctx, "identity.register completed", kernel.Field("user_id", id.String()))
	return user, nil
}

func (s *IdentityService) Login(ctx context.Context, email, plaintextPassword string) (*User, error) {
	s.logger.Info(ctx, "identity.login", kernel.Field("email", email))

	user, err := s.repo.FindByEmail(ctx, email)
	if err != nil {
		return nil, err
	}

	if !user.VerifyPassword(plaintextPassword) {
		return nil, kernel.NewDomainError(kernel.ErrUnauthenticated, "invalid credentials")
	}

	if user.Status == UserStatusSuspended {
		return nil, kernel.NewDomainError(kernel.ErrPermissionDenied, "account is suspended")
	}

	s.logger.Info(ctx, "identity.login completed", kernel.Field("user_id", user.ID.String()))
	return user, nil
}

func (s *IdentityService) GetUser(ctx context.Context, id kernel.ID) (*User, error) {
	if id <= 0 {
		return nil, kernel.NewDomainError(kernel.ErrInvalidArgument, "user id must be positive")
	}

	user, err := s.repo.FindByID(ctx, id)
	if err != nil {
		return nil, err
	}

	return user, nil
}

func (s *IdentityService) SuspendUser(ctx context.Context, id kernel.ID) (*User, error) {
	s.logger.Info(ctx, "identity.suspend_user", kernel.Field("user_id", id.String()))

	user, err := s.repo.FindByID(ctx, id)
	if err != nil {
		return nil, err
	}

	if err := user.Suspend(); err != nil {
		return nil, err
	}

	if err := s.repo.Save(ctx, user); err != nil {
		return nil, err
	}

	return user, nil
}

func (s *IdentityService) ActivateUser(ctx context.Context, id kernel.ID) (*User, error) {
	s.logger.Info(ctx, "identity.activate_user", kernel.Field("user_id", id.String()))

	user, err := s.repo.FindByID(ctx, id)
	if err != nil {
		return nil, err
	}

	if err := user.Activate(); err != nil {
		return nil, err
	}

	if err := s.repo.Save(ctx, user); err != nil {
		return nil, err
	}

	return user, nil
}

func (s *IdentityService) ListUsers(ctx context.Context, offset, limit int) ([]*User, error) {
	return s.repo.FindAll(ctx, offset, limit)
}
