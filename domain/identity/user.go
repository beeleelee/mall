package identity

import (
	"regexp"
	"strings"
	"time"

	"golang.org/x/crypto/bcrypt"

	"github.com/beeleelee/mall/domain/kernel"
)

var emailRegex = regexp.MustCompile(`^[a-zA-Z0-9._%+\-]+@[a-zA-Z0-9.\-]+\.[a-zA-Z]{2,}$`)

type UserStatus string

const (
	UserStatusActive    UserStatus = "active"
	UserStatusSuspended UserStatus = "suspended"
)

type UserRole string

const (
	UserRoleCustomer UserRole = "customer"
	UserRoleAdmin    UserRole = "admin"
)

type Password struct {
	hash string
}

func NewPassword(plaintext string) (*Password, error) {
	if len(plaintext) < 8 {
		return nil, kernel.NewDomainError(kernel.ErrInvalidArgument, "password must be at least 8 characters")
	}
	if len(plaintext) > 128 {
		return nil, kernel.NewDomainError(kernel.ErrInvalidArgument, "password must not exceed 128 characters")
	}
	hash, err := bcrypt.GenerateFromPassword([]byte(plaintext), bcrypt.DefaultCost)
	if err != nil {
		return nil, kernel.NewDomainErrorWithCause(kernel.ErrInternal, "password hashing failed", err)
	}
	return &Password{hash: string(hash)}, nil
}

func NewPasswordFromHash(hash string) *Password {
	return &Password{hash: hash}
}

func (p *Password) Verify(plaintext string) bool {
	err := bcrypt.CompareHashAndPassword([]byte(p.hash), []byte(plaintext))
	return err == nil
}

func (p *Password) Hash() string {
	return p.hash
}

type User struct {
	kernel.AggregateRoot
	Email    string
	Name     string
	password *Password
	Status   UserStatus
	Roles    []UserRole
}

func NewUser(id kernel.ID, email, name string, password *Password, roles []UserRole) (*User, error) {
	if email == "" {
		return nil, kernel.NewDomainError(kernel.ErrInvalidArgument, "email must not be empty")
	}
	if !emailRegex.MatchString(strings.ToLower(email)) {
		return nil, kernel.NewDomainError(kernel.ErrInvalidArgument, "invalid email format")
	}
	if name == "" {
		return nil, kernel.NewDomainError(kernel.ErrInvalidArgument, "name must not be empty")
	}
	if password == nil {
		return nil, kernel.NewDomainError(kernel.ErrInvalidArgument, "password must not be nil")
	}
	if len(roles) == 0 {
		roles = []UserRole{UserRoleCustomer}
	}

	u := &User{
		AggregateRoot: kernel.NewAggregateRoot(id),
		Email:         strings.ToLower(email),
		Name:          name,
		password:      password,
		Status:        UserStatusActive,
		Roles:         roles,
	}

	u.AddEvent(&UserRegistered{
		UserID: id,
		Email:  u.Email,
	})

	return u, nil
}

func (u *User) Suspend() error {
	if u.Status == UserStatusSuspended {
		return kernel.NewDomainError(kernel.ErrInvalidArgument, "user is already suspended")
	}
	old := u.Status
	u.Status = UserStatusSuspended
	u.UpdatedAt = time.Now()
	u.AddEvent(&UserSuspended{
		UserID:    u.ID,
		OldStatus: old,
		NewStatus: u.Status,
	})
	return nil
}

func (u *User) Activate() error {
	if u.Status == UserStatusActive {
		return kernel.NewDomainError(kernel.ErrInvalidArgument, "user is already active")
	}
	old := u.Status
	u.Status = UserStatusActive
	u.UpdatedAt = time.Now()
	u.AddEvent(&UserActivated{
		UserID:    u.ID,
		OldStatus: old,
		NewStatus: u.Status,
	})
	return nil
}

func (u *User) ChangeName(name string) error {
	if name == "" {
		return kernel.NewDomainError(kernel.ErrInvalidArgument, "name must not be empty")
	}
	u.Name = name
	u.UpdatedAt = time.Now()
	u.AddEvent(&UserUpdated{
		UserID: u.ID,
	})
	return nil
}

func (u *User) ChangePassword(password *Password) error {
	if password == nil {
		return kernel.NewDomainError(kernel.ErrInvalidArgument, "password must not be nil")
	}
	u.password = password
	u.UpdatedAt = time.Now()
	u.AddEvent(&UserPasswordChanged{
		UserID: u.ID,
	})
	return nil
}

func (u *User) HasRole(role UserRole) bool {
	for _, r := range u.Roles {
		if r == role {
			return true
		}
	}
	return false
}

func (u *User) SetPasswordHash(hash string) {
	u.password = NewPasswordFromHash(hash)
}

func (u *User) VerifyPassword(plaintext string) bool {
	return u.password.Verify(plaintext)
}

func (u *User) PasswordHash() string {
	return u.password.Hash()
}

type UserRegistered struct {
	UserID kernel.ID
	Email  string
}

func (e *UserRegistered) EventName() string      { return "identity.user.registered" }
func (e *UserRegistered) OccurredAt() time.Time  { return time.Now() }
func (e *UserRegistered) AggregateID() kernel.ID { return e.UserID }

type UserSuspended struct {
	UserID    kernel.ID
	OldStatus UserStatus
	NewStatus UserStatus
}

func (e *UserSuspended) EventName() string      { return "identity.user.suspended" }
func (e *UserSuspended) OccurredAt() time.Time  { return time.Now() }
func (e *UserSuspended) AggregateID() kernel.ID { return e.UserID }

type UserActivated struct {
	UserID    kernel.ID
	OldStatus UserStatus
	NewStatus UserStatus
}

func (e *UserActivated) EventName() string      { return "identity.user.activated" }
func (e *UserActivated) OccurredAt() time.Time  { return time.Now() }
func (e *UserActivated) AggregateID() kernel.ID { return e.UserID }

type UserUpdated struct {
	UserID kernel.ID
}

func (e *UserUpdated) EventName() string      { return "identity.user.updated" }
func (e *UserUpdated) OccurredAt() time.Time  { return time.Now() }
func (e *UserUpdated) AggregateID() kernel.ID { return e.UserID }

type UserPasswordChanged struct {
	UserID kernel.ID
}

func (e *UserPasswordChanged) EventName() string      { return "identity.user.password_changed" }
func (e *UserPasswordChanged) OccurredAt() time.Time  { return time.Now() }
func (e *UserPasswordChanged) AggregateID() kernel.ID { return e.UserID }
