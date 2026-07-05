package identity

import (
	"context"

	domain "github.com/beeleelee/mall/domain/identity"
	"github.com/beeleelee/mall/domain/kernel"
)

type RegisterRequest struct {
	Email    string
	Password string
	Name     string
}

type RegisterResponse struct {
	UserID int64
	Email  string
	Name   string
}

type LoginRequest struct {
	Email    string
	Password string
}

type LoginResponse struct {
	UserID int64
	Email  string
	Name   string
	Roles  []string
}

type UserResponse struct {
	ID     int64
	Email  string
	Name   string
	Status string
	Roles  []string
}

type IdentityAppService struct {
	domain    *domain.IdentityService
	users     domain.UserRepository
	tokenRepo domain.PasswordResetTokenRepository
	logger    kernel.Logger
	sf        *kernel.Snowflake
}

func NewIdentityAppService(domain *domain.IdentityService, users domain.UserRepository, tokenRepo domain.PasswordResetTokenRepository, logger kernel.Logger, sf *kernel.Snowflake) *IdentityAppService {
	return &IdentityAppService{
		domain:    domain,
		users:     users,
		tokenRepo: tokenRepo,
		logger:    logger,
		sf:        sf,
	}
}

func (s *IdentityAppService) Register(ctx context.Context, req RegisterRequest) (*RegisterResponse, error) {
	id, err := s.sf.NextID()
	if err != nil {
		return nil, err
	}

	user, err := s.domain.Register(ctx, id, req.Email, req.Name, req.Password)
	if err != nil {
		return nil, err
	}

	return &RegisterResponse{
		UserID: user.ID.Int64(),
		Email:  user.Email,
		Name:   user.Name,
	}, nil
}

func (s *IdentityAppService) Login(ctx context.Context, req LoginRequest) (*LoginResponse, error) {
	user, err := s.domain.Login(ctx, req.Email, req.Password)
	if err != nil {
		return nil, err
	}

	roles := make([]string, len(user.Roles))
	for i, r := range user.Roles {
		roles[i] = string(r)
	}

	return &LoginResponse{
		UserID: user.ID.Int64(),
		Email:  user.Email,
		Name:   user.Name,
		Roles:  roles,
	}, nil
}

func (s *IdentityAppService) GetUser(ctx context.Context, id int64) (*UserResponse, error) {
	user, err := s.domain.GetUser(ctx, kernel.ID(id))
	if err != nil {
		return nil, err
	}

	roles := make([]string, len(user.Roles))
	for i, r := range user.Roles {
		roles[i] = string(r)
	}

	return &UserResponse{
		ID:     user.ID.Int64(),
		Email:  user.Email,
		Name:   user.Name,
		Status: string(user.Status),
		Roles:  roles,
	}, nil
}

func (s *IdentityAppService) SuspendUser(ctx context.Context, id int64) (*UserResponse, error) {
	user, err := s.domain.SuspendUser(ctx, kernel.ID(id))
	if err != nil {
		return nil, err
	}

	roles := make([]string, len(user.Roles))
	for i, r := range user.Roles {
		roles[i] = string(r)
	}

	return &UserResponse{
		ID:     user.ID.Int64(),
		Email:  user.Email,
		Name:   user.Name,
		Status: string(user.Status),
		Roles:  roles,
	}, nil
}

func (s *IdentityAppService) ActivateUser(ctx context.Context, id int64) (*UserResponse, error) {
	user, err := s.domain.ActivateUser(ctx, kernel.ID(id))
	if err != nil {
		return nil, err
	}

	roles := make([]string, len(user.Roles))
	for i, r := range user.Roles {
		roles[i] = string(r)
	}

	return &UserResponse{
		ID:     user.ID.Int64(),
		Email:  user.Email,
		Name:   user.Name,
		Status: string(user.Status),
		Roles:  roles,
	}, nil
}

func (s *IdentityAppService) RequestPasswordReset(ctx context.Context, email string) (string, error) {
	return s.domain.RequestPasswordReset(ctx, email, s.tokenRepo, s.sf)
}

func (s *IdentityAppService) ResetPassword(ctx context.Context, token, newPassword string) error {
	return s.domain.ResetPassword(ctx, token, newPassword, s.tokenRepo)
}

func (s *IdentityAppService) ListUsers(ctx context.Context, offset, limit int) ([]*UserResponse, error) {
	users, err := s.domain.ListUsers(ctx, offset, limit)
	if err != nil {
		return nil, err
	}

	result := make([]*UserResponse, 0, len(users))
	for _, user := range users {
		roles := make([]string, len(user.Roles))
		for i, r := range user.Roles {
			roles[i] = string(r)
		}
		result = append(result, &UserResponse{
			ID:     user.ID.Int64(),
			Email:  user.Email,
			Name:   user.Name,
			Status: string(user.Status),
			Roles:  roles,
		})
	}
	return result, nil
}
