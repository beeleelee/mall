package identity

import (
	"testing"

	"github.com/beeleelee/mall/domain/kernel"
)

func validPassword() *Password {
	return NewPasswordFromHash("$2a$10$dummyhashfordummytestpassword1234567890abcdef")
}

func TestNewUser_Success(t *testing.T) {
	u, err := NewUser(1, "test@example.com", "Test User", validPassword(), nil)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if u.ID != 1 {
		t.Errorf("expected ID 1, got %d", u.ID)
	}
	if u.Email != "test@example.com" {
		t.Errorf("expected test@example.com, got %s", u.Email)
	}
	if u.Name != "Test User" {
		t.Errorf("expected Test User, got %s", u.Name)
	}
	if u.Status != UserStatusActive {
		t.Errorf("expected status active, got %s", u.Status)
	}
	if len(u.Roles) != 1 || u.Roles[0] != UserRoleCustomer {
		t.Errorf("expected default customer role, got %v", u.Roles)
	}
}

func TestNewUser_NormalizesEmail(t *testing.T) {
	u, err := NewUser(1, "TEST@Example.COM", "User", validPassword(), nil)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if u.Email != "test@example.com" {
		t.Errorf("expected lowercase email, got %s", u.Email)
	}
}

func TestNewUser_EmptyEmail(t *testing.T) {
	_, err := NewUser(1, "", "User", validPassword(), nil)
	if !kernel.IsInvalidArgument(err) {
		t.Errorf("expected invalid argument error, got %v", err)
	}
}

func TestNewUser_InvalidEmail(t *testing.T) {
	_, err := NewUser(1, "not-an-email", "User", validPassword(), nil)
	if !kernel.IsInvalidArgument(err) {
		t.Errorf("expected invalid argument error, got %v", err)
	}
}

func TestNewUser_EmptyName(t *testing.T) {
	_, err := NewUser(1, "test@example.com", "", validPassword(), nil)
	if !kernel.IsInvalidArgument(err) {
		t.Errorf("expected invalid argument error, got %v", err)
	}
}

func TestNewUser_NilPassword(t *testing.T) {
	_, err := NewUser(1, "test@example.com", "User", nil, nil)
	if !kernel.IsInvalidArgument(err) {
		t.Errorf("expected invalid argument error, got %v", err)
	}
}

func TestNewUser_WithRoles(t *testing.T) {
	u, err := NewUser(1, "admin@example.com", "Admin", validPassword(), []UserRole{UserRoleAdmin})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(u.Roles) != 1 || u.Roles[0] != UserRoleAdmin {
		t.Errorf("expected admin role, got %v", u.Roles)
	}
}

func TestNewUser_EmitsRegisteredEvent(t *testing.T) {
	u, err := NewUser(1, "test@example.com", "User", validPassword(), nil)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	events := u.Events()
	if len(events) != 1 {
		t.Fatalf("expected 1 event, got %d", len(events))
	}
	re, ok := events[0].(*UserRegistered)
	if !ok {
		t.Fatalf("expected UserRegistered event, got %T", events[0])
	}
	if re.Email != "test@example.com" {
		t.Errorf("expected test@example.com, got %s", re.Email)
	}
}

func TestNewPassword_Success(t *testing.T) {
	p, err := NewPassword("securepassword123")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if p == nil {
		t.Fatal("password should not be nil")
	}
}

func TestNewPassword_TooShort(t *testing.T) {
	_, err := NewPassword("short")
	if !kernel.IsInvalidArgument(err) {
		t.Errorf("expected invalid argument error, got %v", err)
	}
}

func TestNewPassword_TooLong(t *testing.T) {
	plain := make([]byte, 129)
	for i := range plain {
		plain[i] = 'a'
	}
	_, err := NewPassword(string(plain))
	if !kernel.IsInvalidArgument(err) {
		t.Errorf("expected invalid argument error, got %v", err)
	}
}

func TestPassword_Verify(t *testing.T) {
	p, err := NewPassword("mysecretpassword")
	if err != nil {
		t.Fatalf("NewPassword failed: %v", err)
	}
	if !p.Verify("mysecretpassword") {
		t.Error("expected Verify to return true for correct password")
	}
	if p.Verify("wrongpassword") {
		t.Error("expected Verify to return false for wrong password")
	}
}

func TestUser_VerifyPassword(t *testing.T) {
	p := NewPasswordFromHash("$2a$10$dummyhash")
	u, _ := NewUser(1, "test@example.com", "User", p, nil)

	if u.VerifyPassword("wrong") {
		t.Error("expected false for wrong password against dummy hash")
	}
}

func TestUser_Suspend_Success(t *testing.T) {
	u, _ := NewUser(1, "test@example.com", "User", validPassword(), nil)
	u.ClearEvents()

	err := u.Suspend()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if u.Status != UserStatusSuspended {
		t.Errorf("expected suspended, got %s", u.Status)
	}
	events := u.Events()
	if len(events) != 1 {
		t.Fatalf("expected 1 event, got %d", len(events))
	}
	if _, ok := events[0].(*UserSuspended); !ok {
		t.Fatalf("expected UserSuspended event, got %T", events[0])
	}
}

func TestUser_Suspend_AlreadySuspended(t *testing.T) {
	u, _ := NewUser(1, "test@example.com", "User", validPassword(), nil)
	u.ClearEvents()
	u.Suspend()
	u.ClearEvents()

	err := u.Suspend()
	if !kernel.IsInvalidArgument(err) {
		t.Errorf("expected invalid argument error for duplicate suspend, got %v", err)
	}
}

func TestUser_Activate_Success(t *testing.T) {
	u, _ := NewUser(1, "test@example.com", "User", validPassword(), nil)
	u.Suspend()
	u.ClearEvents()

	err := u.Activate()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if u.Status != UserStatusActive {
		t.Errorf("expected active, got %s", u.Status)
	}
}

func TestUser_Activate_AlreadyActive(t *testing.T) {
	u, _ := NewUser(1, "test@example.com", "User", validPassword(), nil)
	u.ClearEvents()

	err := u.Activate()
	if !kernel.IsInvalidArgument(err) {
		t.Errorf("expected invalid argument error for duplicate activate, got %v", err)
	}
}

func TestUser_ChangeName_Success(t *testing.T) {
	u, _ := NewUser(1, "test@example.com", "Old", validPassword(), nil)
	u.ClearEvents()

	err := u.ChangeName("New Name")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if u.Name != "New Name" {
		t.Errorf("expected New Name, got %s", u.Name)
	}
}

func TestUser_ChangeName_Empty(t *testing.T) {
	u, _ := NewUser(1, "test@example.com", "User", validPassword(), nil)
	u.ClearEvents()

	err := u.ChangeName("")
	if !kernel.IsInvalidArgument(err) {
		t.Errorf("expected invalid argument error, got %v", err)
	}
}

func TestUser_HasRole(t *testing.T) {
	u, _ := NewUser(1, "test@example.com", "User", validPassword(), []UserRole{UserRoleCustomer})
	if !u.HasRole(UserRoleCustomer) {
		t.Error("expected user to have customer role")
	}
	if u.HasRole(UserRoleAdmin) {
		t.Error("expected user not to have admin role")
	}
}

func TestUser_Equals(t *testing.T) {
	u1, _ := NewUser(1, "a@example.com", "A", validPassword(), nil)
	u2, _ := NewUser(1, "b@example.com", "B", validPassword(), nil)
	if !u1.Equals(u2.Entity) {
		t.Error("users with same ID should be equal")
	}
}
