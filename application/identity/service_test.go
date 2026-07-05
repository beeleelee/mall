package identity

import (
	"context"
	"testing"

	domain "github.com/beeleelee/mall/domain/identity"
	"github.com/beeleelee/mall/domain/kernel"
)

func newTestAppService(t *testing.T) *IdentityAppService {
	t.Helper()
	repo := newFakeUserRepository()
	logger := fakeLogger{}
	svc := domain.NewIdentityService(repo, logger)
	sf, err := kernel.NewSnowflake(1)
	if err != nil {
		t.Fatalf("NewSnowflake failed: %v", err)
	}
	return NewIdentityAppService(svc, repo, newFakePasswordResetTokenRepo(), logger, sf)
}

func TestAppService_Register_Success(t *testing.T) {
	app := newTestAppService(t)
	resp, err := app.Register(context.Background(), RegisterRequest{
		Email:    "test@example.com",
		Password: "securepass123",
		Name:     "Test User",
	})
	if err != nil {
		t.Fatalf("Register failed: %v", err)
	}
	if resp.UserID <= 0 {
		t.Errorf("expected positive UserID, got %d", resp.UserID)
	}
	if resp.Email != "test@example.com" {
		t.Errorf("expected test@example.com, got %s", resp.Email)
	}
	if resp.Name != "Test User" {
		t.Errorf("expected Test User, got %s", resp.Name)
	}
}

func TestAppService_Register_Duplicate(t *testing.T) {
	app := newTestAppService(t)
	_, err := app.Register(context.Background(), RegisterRequest{
		Email:    "dup@example.com",
		Password: "password123",
		Name:     "First",
	})
	if err != nil {
		t.Fatalf("first Register failed: %v", err)
	}
	_, err = app.Register(context.Background(), RegisterRequest{
		Email:    "dup@example.com",
		Password: "otherpass456",
		Name:     "Second",
	})
	if err == nil {
		t.Fatal("expected error for duplicate email")
	}
}

func TestAppService_Login_Success(t *testing.T) {
	app := newTestAppService(t)
	_, err := app.Register(context.Background(), RegisterRequest{
		Email:    "login@example.com",
		Password: "mypassword",
		Name:     "Login User",
	})
	if err != nil {
		t.Fatalf("Register failed: %v", err)
	}

	resp, err := app.Login(context.Background(), LoginRequest{
		Email:    "login@example.com",
		Password: "mypassword",
	})
	if err != nil {
		t.Fatalf("Login failed: %v", err)
	}
	if resp.UserID <= 0 {
		t.Errorf("expected positive UserID, got %d", resp.UserID)
	}
	if resp.Email != "login@example.com" {
		t.Errorf("expected login@example.com, got %s", resp.Email)
	}
}

func TestAppService_Login_WrongPassword(t *testing.T) {
	app := newTestAppService(t)
	_, err := app.Register(context.Background(), RegisterRequest{
		Email:    "wrongpw@example.com",
		Password: "correctpass",
		Name:     "User",
	})
	if err != nil {
		t.Fatalf("Register failed: %v", err)
	}

	_, err = app.Login(context.Background(), LoginRequest{
		Email:    "wrongpw@example.com",
		Password: "wrongpass",
	})
	if err == nil {
		t.Fatal("expected error for wrong password")
	}
}

func TestAppService_GetUser_Success(t *testing.T) {
	app := newTestAppService(t)
	reg, err := app.Register(context.Background(), RegisterRequest{
		Email:    "getuser@example.com",
		Password: "password123",
		Name:     "Get User",
	})
	if err != nil {
		t.Fatalf("Register failed: %v", err)
	}

	resp, err := app.GetUser(context.Background(), reg.UserID)
	if err != nil {
		t.Fatalf("GetUser failed: %v", err)
	}
	if resp.Email != "getuser@example.com" {
		t.Errorf("expected getuser@example.com, got %s", resp.Email)
	}
	if resp.Status != "active" {
		t.Errorf("expected status active, got %s", resp.Status)
	}
}

func TestAppService_GetUser_NotFound(t *testing.T) {
	app := newTestAppService(t)
	_, err := app.GetUser(context.Background(), 999)
	if err == nil {
		t.Fatal("expected error for non-existent user")
	}
}

func TestAppService_SuspendUser(t *testing.T) {
	app := newTestAppService(t)
	reg, err := app.Register(context.Background(), RegisterRequest{
		Email:    "suspend@example.com",
		Password: "password123",
		Name:     "Suspend User",
	})
	if err != nil {
		t.Fatalf("Register failed: %v", err)
	}

	resp, err := app.SuspendUser(context.Background(), reg.UserID)
	if err != nil {
		t.Fatalf("SuspendUser failed: %v", err)
	}
	if resp.Status != "suspended" {
		t.Errorf("expected status suspended, got %s", resp.Status)
	}
}
