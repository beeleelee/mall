package identity

import (
	"context"
	"testing"

	"github.com/beeleelee/mall/domain/kernel"
)

func newIdentityService() *IdentityService {
	return NewIdentityService(newFakeUserRepository(), fakeLogger{})
}

func registerUser(t *testing.T, svc *IdentityService, id int64, email, name, password string) *User {
	t.Helper()
	u, err := svc.Register(context.Background(), kernel.ID(id), email, name, password)
	if err != nil {
		t.Fatalf("Register(%d, %s) failed: %v", id, email, err)
	}
	return u
}

func TestIdentityService_RegisterAndGetUser(t *testing.T) {
	svc := newIdentityService()
	u := registerUser(t, svc, 1, "alice@example.com", "Alice", "securepass123")

	got, err := svc.GetUser(context.Background(), u.ID)
	if err != nil {
		t.Fatalf("GetUser failed: %v", err)
	}
	if got.Email != "alice@example.com" {
		t.Errorf("expected alice@example.com, got %s", got.Email)
	}
	if got.Name != "Alice" {
		t.Errorf("expected Alice, got %s", got.Name)
	}
}

func TestIdentityService_Register_DuplicateEmail(t *testing.T) {
	svc := newIdentityService()
	registerUser(t, svc, 1, "dup@example.com", "First", "password123")

	_, err := svc.Register(context.Background(), 2, "dup@example.com", "Second", "otherpass456")
	if !kernel.IsAlreadyExists(err) {
		t.Errorf("expected AlreadyExists error, got %v", err)
	}
}

func TestIdentityService_Login_Success(t *testing.T) {
	svc := newIdentityService()
	registerUser(t, svc, 1, "login@example.com", "Login User", "mypassword")

	u, err := svc.Login(context.Background(), "login@example.com", "mypassword")
	if err != nil {
		t.Fatalf("Login failed: %v", err)
	}
	if u.Email != "login@example.com" {
		t.Errorf("expected login@example.com, got %s", u.Email)
	}
}

func TestIdentityService_Login_WrongPassword(t *testing.T) {
	svc := newIdentityService()
	registerUser(t, svc, 1, "wrongpw@example.com", "User", "correctpass")

	_, err := svc.Login(context.Background(), "wrongpw@example.com", "wrongpass")
	if !kernel.IsUnauthenticated(err) {
		t.Errorf("expected Unauthenticated error, got %v", err)
	}
}

func TestIdentityService_Login_SuspendedUser(t *testing.T) {
	svc := newIdentityService()
	u := registerUser(t, svc, 1, "suspend@example.com", "User", "password123")

	svc.SuspendUser(context.Background(), u.ID)

	_, err := svc.Login(context.Background(), "suspend@example.com", "password123")
	if !kernel.IsPermissionDenied(err) {
		t.Errorf("expected PermissionDenied error for suspended user, got %v", err)
	}
}

func TestIdentityService_Login_UserNotFound(t *testing.T) {
	svc := newIdentityService()
	_, err := svc.Login(context.Background(), "nobody@example.com", "password")
	if !kernel.IsNotFound(err) {
		t.Errorf("expected NotFound error, got %v", err)
	}
}

func TestIdentityService_GetUser_NotFound(t *testing.T) {
	svc := newIdentityService()
	_, err := svc.GetUser(context.Background(), 999)
	if !kernel.IsNotFound(err) {
		t.Errorf("expected NotFound error, got %v", err)
	}
}

func TestIdentityService_GetUser_InvalidID(t *testing.T) {
	svc := newIdentityService()
	_, err := svc.GetUser(context.Background(), 0)
	if !kernel.IsInvalidArgument(err) {
		t.Errorf("expected InvalidArgument error, got %v", err)
	}
}

func TestIdentityService_SuspendUser(t *testing.T) {
	svc := newIdentityService()
	u := registerUser(t, svc, 1, "tosuspend@example.com", "User", "password123")

	suspended, err := svc.SuspendUser(context.Background(), u.ID)
	if err != nil {
		t.Fatalf("SuspendUser failed: %v", err)
	}
	if suspended.Status != UserStatusSuspended {
		t.Errorf("expected suspended status, got %s", suspended.Status)
	}

	got, err := svc.GetUser(context.Background(), u.ID)
	if err != nil {
		t.Fatalf("GetUser after suspend failed: %v", err)
	}
	if got.Status != UserStatusSuspended {
		t.Errorf("expected user to be suspended, got %s", got.Status)
	}
}

func TestIdentityService_SuspendUser_NotFound(t *testing.T) {
	svc := newIdentityService()
	_, err := svc.SuspendUser(context.Background(), 999)
	if !kernel.IsNotFound(err) {
		t.Errorf("expected NotFound error, got %v", err)
	}
}

func TestIdentityService_Register_InvalidPassword(t *testing.T) {
	svc := newIdentityService()
	_, err := svc.Register(context.Background(), 1, "test@example.com", "User", "short")
	if !kernel.IsInvalidArgument(err) {
		t.Errorf("expected InvalidArgument error for short password, got %v", err)
	}
}

func TestIdentityService_Register_InvalidEmail(t *testing.T) {
	svc := newIdentityService()
	_, err := svc.Register(context.Background(), 1, "not-email", "User", "password123")
	if !kernel.IsInvalidArgument(err) {
		t.Errorf("expected InvalidArgument error for bad email, got %v", err)
	}
}
