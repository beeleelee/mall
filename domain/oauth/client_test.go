package oauth

import (
	"testing"

	"github.com/beeleelee/mall/domain/kernel"
)

func TestNewClient_Valid(t *testing.T) {
	const id kernel.ID = 1
	c, err := NewClient(id, "test-client", "super-secret", []string{"https://example.com/cb"}, []string{"read", "write"})
	if err != nil {
		t.Fatal(err)
	}
	if c.ClientID != "test-client" {
		t.Errorf("expected test-client, got %s", c.ClientID)
	}
	if c.Status != ClientStatusActive {
		t.Errorf("expected active, got %s", c.Status)
	}
}

func TestNewClient_EmptyClientID(t *testing.T) {
	_, err := NewClient(1, "", "secret", []string{"https://example.com/cb"}, []string{"read"})
	if !kernel.IsInvalidArgument(err) {
		t.Errorf("expected invalid argument, got %v", err)
	}
}

func TestNewClient_EmptySecret(t *testing.T) {
	_, err := NewClient(1, "client", "", []string{"https://example.com/cb"}, []string{"read"})
	if !kernel.IsInvalidArgument(err) {
		t.Errorf("expected invalid argument, got %v", err)
	}
}

func TestNewClient_NoRedirectURIs(t *testing.T) {
	_, err := NewClient(1, "client", "secret", nil, []string{"read"})
	if !kernel.IsInvalidArgument(err) {
		t.Errorf("expected invalid argument, got %v", err)
	}
}

func TestNewClient_NoScopes(t *testing.T) {
	_, err := NewClient(1, "client", "secret", []string{"https://example.com/cb"}, nil)
	if !kernel.IsInvalidArgument(err) {
		t.Errorf("expected invalid argument, got %v", err)
	}
}

func TestVerifySecret(t *testing.T) {
	c, err := NewClient(1, "client", "my-secret", []string{"https://example.com/cb"}, []string{"read"})
	if err != nil {
		t.Fatal(err)
	}
	if !c.VerifySecret("my-secret") {
		t.Error("expected secret to match")
	}
	if c.VerifySecret("wrong-secret") {
		t.Error("expected secret to not match")
	}
}

func TestIsValidRedirectURI(t *testing.T) {
	c, err := NewClient(1, "client", "secret", []string{"https://example.com/cb"}, []string{"read"})
	if err != nil {
		t.Fatal(err)
	}
	if !c.IsValidRedirectURI("https://example.com/cb") {
		t.Error("expected valid redirect URI")
	}
	if c.IsValidRedirectURI("https://evil.com/cb") {
		t.Error("expected invalid redirect URI")
	}
}

func TestHasScope(t *testing.T) {
	c, err := NewClient(1, "client", "secret", []string{"https://example.com/cb"}, []string{"read", "write"})
	if err != nil {
		t.Fatal(err)
	}
	if !c.HasScope("read") {
		t.Error("expected to have read scope")
	}
	if c.HasScope("admin") {
		t.Error("expected to not have admin scope")
	}
}
