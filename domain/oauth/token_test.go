package oauth

import (
	"testing"
	"time"

	"github.com/beeleelee/mall/domain/kernel"
)

func TestSignAndValidateJWT(t *testing.T) {
	secret := []byte("test-secret-key")
	tokenStr, err := SignJWT(42, "test-client", "read", 15*time.Minute, secret)
	if err != nil {
		t.Fatal(err)
	}
	if tokenStr == "" {
		t.Fatal("expected non-empty token")
	}

	claims, err := ValidateJWT(tokenStr, secret)
	if err != nil {
		t.Fatal(err)
	}
	if claims.Subject != "42" {
		t.Errorf("expected subject 42, got %s", claims.Subject)
	}
	if claims.ClientID != "test-client" {
		t.Errorf("expected client_id test-client, got %s", claims.ClientID)
	}
	if claims.Scope != "read" {
		t.Errorf("expected scope read, got %s", claims.Scope)
	}
}

func TestValidateJWT_WrongSecret(t *testing.T) {
	tokenStr, err := SignJWT(1, "client", "read", 15*time.Minute, []byte("correct-secret"))
	if err != nil {
		t.Fatal(err)
	}
	_, err = ValidateJWT(tokenStr, []byte("wrong-secret"))
	if !kernel.IsUnauthenticated(err) {
		t.Errorf("expected unauthenticated, got %v", err)
	}
}

func TestValidateJWT_Expired(t *testing.T) {
	secret := []byte("test-secret")
	tokenStr, err := SignJWT(1, "client", "read", -1*time.Minute, secret)
	if err != nil {
		t.Fatal(err)
	}
	_, err = ValidateJWT(tokenStr, secret)
	if !kernel.IsUnauthenticated(err) {
		t.Errorf("expected unauthenticated, got %v", err)
	}
}

func TestNewRefreshToken_Valid(t *testing.T) {
	rt, err := NewRefreshToken("client", 1, "read", 30*24*time.Hour)
	if err != nil {
		t.Fatal(err)
	}
	if rt.ClientID != "client" {
		t.Errorf("expected client, got %s", rt.ClientID)
	}
	if rt.Revoked {
		t.Error("expected not revoked")
	}
}

func TestNewRefreshToken_EmptyClientID(t *testing.T) {
	_, err := NewRefreshToken("", 1, "read", 30*24*time.Hour)
	if !kernel.IsInvalidArgument(err) {
		t.Errorf("expected invalid argument, got %v", err)
	}
}

func TestRefreshToken_IsExpired(t *testing.T) {
	rt, err := NewRefreshToken("client", 1, "read", -1*time.Hour)
	if err != nil {
		t.Fatal(err)
	}
	if !rt.IsExpired() {
		t.Error("expected expired")
	}
}

func TestRefreshToken_Revoke(t *testing.T) {
	rt, err := NewRefreshToken("client", 1, "read", 30*24*time.Hour)
	if err != nil {
		t.Fatal(err)
	}
	rt.Revoke()
	if !rt.Revoked {
		t.Error("expected revoked")
	}
}
