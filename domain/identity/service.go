package identity

import (
	"context"
	"crypto/rand"
	"crypto/sha256"
	"encoding/hex"
	"time"

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

func generateResetToken() (string, string, error) {
	raw := make([]byte, 32)
	if _, err := rand.Read(raw); err != nil {
		return "", "", kernel.NewDomainErrorWithCause(kernel.ErrInternal, "generate reset token", err)
	}

	rawToken := hex.EncodeToString(raw)
	hash := hashToken(rawToken)
	return rawToken, hash, nil
}

func hashToken(token string) string {
	sum := sha256.Sum256([]byte(token))
	return hex.EncodeToString(sum[:])
}

func (s *IdentityService) RequestPasswordReset(ctx context.Context, email string, tokenRepo PasswordResetTokenRepository, sf *kernel.Snowflake) (string, error) {
	user, err := s.repo.FindByEmail(ctx, email)
	if err != nil {
		// Don't reveal whether email exists
		return "", nil
	}

	rawToken, hash, err := generateResetToken()
	if err != nil {
		return "", err
	}

	id, err := sf.NextID()
	if err != nil {
		return "", err
	}

	token := NewPasswordResetToken(id, user.ID, hash, time.Now().Add(1*time.Hour))
	if err := tokenRepo.Save(ctx, token); err != nil {
		return "", err
	}

	s.logger.Info(ctx, "identity.password_reset_requested", kernel.Field("user_id", user.ID.String()))
	return rawToken, nil
}

func (s *IdentityService) ResetPassword(ctx context.Context, rawToken, newPassword string, tokenRepo PasswordResetTokenRepository) error {
	hash := hashToken(rawToken)

	token, err := tokenRepo.FindByHash(ctx, hash)
	if err != nil {
		return kernel.NewDomainError(kernel.ErrInvalidArgument, "invalid or expired reset token")
	}

	if token.Used {
		return kernel.NewDomainError(kernel.ErrInvalidArgument, "reset token already used")
	}

	if token.IsExpired() {
		return kernel.NewDomainError(kernel.ErrInvalidArgument, "reset token has expired")
	}

	user, err := s.repo.FindByID(ctx, token.UserID)
	if err != nil {
		return err
	}

	password, err := NewPassword(newPassword)
	if err != nil {
		return err
	}

	if err := user.ChangePassword(password); err != nil {
		return err
	}

	if err := s.repo.Save(ctx, user); err != nil {
		return err
	}

	token.MarkUsed()
	if err := tokenRepo.MarkUsed(ctx, token.ID); err != nil {
		return err
	}

	s.logger.Info(ctx, "identity.password_reset_completed", kernel.Field("user_id", user.ID.String()))
	return nil
}
