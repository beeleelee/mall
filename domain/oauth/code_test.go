package oauth

import (
	"testing"
	"time"

	"github.com/beeleelee/mall/domain/kernel"
)

func TestNewAuthorizationCode_Valid(t *testing.T) {
	c, err := NewAuthorizationCode("test-client", 1, "https://example.com/cb", "read", 10*time.Minute)
	if err != nil {
		t.Fatal(err)
	}
	if c.ClientID != "test-client" {
		t.Errorf("expected test-client, got %s", c.ClientID)
	}
	if c.UserID != 1 {
		t.Errorf("expected user 1, got %d", c.UserID)
	}
	if c.Code == "" {
		t.Error("expected non-empty code")
	}
	if c.Used {
		t.Error("expected unused code")
	}
}

func TestNewAuthorizationCode_EmptyClientID(t *testing.T) {
	_, err := NewAuthorizationCode("", 1, "https://example.com/cb", "read", 10*time.Minute)
	if !kernel.IsInvalidArgument(err) {
		t.Errorf("expected invalid argument, got %v", err)
	}
}

func TestNewAuthorizationCode_InvalidUserID(t *testing.T) {
	_, err := NewAuthorizationCode("client", 0, "https://example.com/cb", "read", 10*time.Minute)
	if !kernel.IsInvalidArgument(err) {
		t.Errorf("expected invalid argument, got %v", err)
	}
}

func TestAuthorizationCode_IsExpired(t *testing.T) {
	c, err := NewAuthorizationCode("client", 1, "https://example.com/cb", "read", -1*time.Minute)
	if err != nil {
		t.Fatal(err)
	}
	if !c.IsExpired() {
		t.Error("expected expired code")
	}
}

func TestAuthorizationCode_MarkUsed(t *testing.T) {
	c, err := NewAuthorizationCode("client", 1, "https://example.com/cb", "read", 10*time.Minute)
	if err != nil {
		t.Fatal(err)
	}
	if err := c.MarkUsed(); err != nil {
		t.Fatal(err)
	}
	if !c.Used {
		t.Error("expected used code")
	}
	if err := c.MarkUsed(); err == nil {
		t.Error("expected error on double use")
	}
}
