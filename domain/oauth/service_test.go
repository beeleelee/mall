package oauth

import (
	"context"
	"testing"

	"github.com/beeleelee/mall/domain/kernel"
)

func newTestService(t *testing.T) *OAuthService {
	t.Helper()
	clients := newFakeClientRepo()
	codes := newFakeCodeRepo()
	tokens := newFakeTokenRepo()

	const id kernel.ID = 1
	client, err := NewClient(id, "test-client", "test-secret", []string{"https://example.com/cb"}, []string{"read", "write"})
	if err != nil {
		t.Fatal(err)
	}
	if err := clients.Save(context.Background(), client); err != nil {
		t.Fatal(err)
	}

	return NewOAuthService(clients, codes, tokens, fakeLogger{}, []byte("jwt-secret"))
}

func TestAuthorize_Success(t *testing.T) {
	svc := newTestService(t)
	ctx := context.Background()

	out, err := svc.Authorize(ctx, AuthorizeInput{
		ClientID:    "test-client",
		RedirectURI: "https://example.com/cb",
		Scope:       "read",
		UserID:      42,
	})
	if err != nil {
		t.Fatal(err)
	}
	if out.Code == "" {
		t.Error("expected non-empty authorization code")
	}
	if out.RedirectURI != "https://example.com/cb" {
		t.Errorf("expected https://example.com/cb, got %s", out.RedirectURI)
	}
}

func TestAuthorize_InvalidClient(t *testing.T) {
	svc := newTestService(t)
	_, err := svc.Authorize(context.Background(), AuthorizeInput{
		ClientID:    "nonexistent",
		RedirectURI: "https://example.com/cb",
		UserID:      42,
	})
	if !kernel.IsNotFound(err) {
		t.Errorf("expected not found, got %v", err)
	}
}

func TestAuthorize_RedirectURIMismatch(t *testing.T) {
	svc := newTestService(t)
	_, err := svc.Authorize(context.Background(), AuthorizeInput{
		ClientID:    "test-client",
		RedirectURI: "https://evil.com/cb",
		UserID:      42,
	})
	if !kernel.IsInvalidArgument(err) {
		t.Errorf("expected invalid argument, got %v", err)
	}
}

func TestAuthorize_ScopeNotAllowed(t *testing.T) {
	svc := newTestService(t)
	_, err := svc.Authorize(context.Background(), AuthorizeInput{
		ClientID:    "test-client",
		RedirectURI: "https://example.com/cb",
		Scope:       "admin",
		UserID:      42,
	})
	if !kernel.IsInvalidArgument(err) {
		t.Errorf("expected invalid argument, got %v", err)
	}
}

func TestExchange_Success(t *testing.T) {
	svc := newTestService(t)
	ctx := context.Background()

	authOut, err := svc.Authorize(ctx, AuthorizeInput{
		ClientID:    "test-client",
		RedirectURI: "https://example.com/cb",
		Scope:       "read",
		UserID:      42,
	})
	if err != nil {
		t.Fatal(err)
	}

	tokenResp, err := svc.Exchange(ctx, TokenExchangeInput{
		Code:         authOut.Code,
		ClientID:     "test-client",
		ClientSecret: "test-secret",
		RedirectURI:  "https://example.com/cb",
	})
	if err != nil {
		t.Fatal(err)
	}
	if tokenResp.AccessToken == "" {
		t.Error("expected non-empty access token")
	}
	if tokenResp.RefreshToken == "" {
		t.Error("expected non-empty refresh token")
	}
	if tokenResp.ExpiresIn <= 0 {
		t.Error("expected positive expires_in")
	}
}

func TestExchange_InvalidClientSecret(t *testing.T) {
	svc := newTestService(t)
	ctx := context.Background()

	authOut, err := svc.Authorize(ctx, AuthorizeInput{
		ClientID:    "test-client",
		RedirectURI: "https://example.com/cb",
		UserID:      42,
	})
	if err != nil {
		t.Fatal(err)
	}

	_, err = svc.Exchange(ctx, TokenExchangeInput{
		Code:         authOut.Code,
		ClientID:     "test-client",
		ClientSecret: "wrong-secret",
		RedirectURI:  "https://example.com/cb",
	})
	if !kernel.IsUnauthenticated(err) {
		t.Errorf("expected unauthenticated, got %v", err)
	}
}

func TestExchange_CodeAlreadyUsed(t *testing.T) {
	svc := newTestService(t)
	ctx := context.Background()

	authOut, err := svc.Authorize(ctx, AuthorizeInput{
		ClientID:    "test-client",
		RedirectURI: "https://example.com/cb",
		UserID:      42,
	})
	if err != nil {
		t.Fatal(err)
	}

	_, err = svc.Exchange(ctx, TokenExchangeInput{
		Code:         authOut.Code,
		ClientID:     "test-client",
		ClientSecret: "test-secret",
		RedirectURI:  "https://example.com/cb",
	})
	if err != nil {
		t.Fatal(err)
	}

	_, err = svc.Exchange(ctx, TokenExchangeInput{
		Code:         authOut.Code,
		ClientID:     "test-client",
		ClientSecret: "test-secret",
		RedirectURI:  "https://example.com/cb",
	})
	if !kernel.IsNotFound(err) {
		t.Errorf("expected not found (code already deleted), got %v", err)
	}
}

func TestRefresh_Success(t *testing.T) {
	svc := newTestService(t)
	ctx := context.Background()

	authOut, err := svc.Authorize(ctx, AuthorizeInput{
		ClientID:    "test-client",
		RedirectURI: "https://example.com/cb",
		UserID:      42,
	})
	if err != nil {
		t.Fatal(err)
	}

	tokenResp, err := svc.Exchange(ctx, TokenExchangeInput{
		Code:         authOut.Code,
		ClientID:     "test-client",
		ClientSecret: "test-secret",
		RedirectURI:  "https://example.com/cb",
	})
	if err != nil {
		t.Fatal(err)
	}

	newTokenResp, err := svc.Refresh(ctx, TokenRefreshInput{
		RefreshToken: tokenResp.RefreshToken,
		ClientID:     "test-client",
		ClientSecret: "test-secret",
	})
	if err != nil {
		t.Fatal(err)
	}
	if newTokenResp.AccessToken == "" {
		t.Error("expected non-empty access token")
	}
}

func TestRevoke_Success(t *testing.T) {
	svc := newTestService(t)
	ctx := context.Background()

	authOut, err := svc.Authorize(ctx, AuthorizeInput{
		ClientID:    "test-client",
		RedirectURI: "https://example.com/cb",
		UserID:      42,
	})
	if err != nil {
		t.Fatal(err)
	}

	tokenResp, err := svc.Exchange(ctx, TokenExchangeInput{
		Code:         authOut.Code,
		ClientID:     "test-client",
		ClientSecret: "test-secret",
		RedirectURI:  "https://example.com/cb",
	})
	if err != nil {
		t.Fatal(err)
	}

	err = svc.Revoke(ctx, RevokeInput{
		Token:        tokenResp.RefreshToken,
		ClientID:     "test-client",
		ClientSecret: "test-secret",
	})
	if err != nil {
		t.Fatal(err)
	}

	_, err = svc.Refresh(ctx, TokenRefreshInput{
		RefreshToken: tokenResp.RefreshToken,
		ClientID:     "test-client",
		ClientSecret: "test-secret",
	})
	if !kernel.IsUnauthenticated(err) {
		t.Errorf("expected unauthenticated after revocation, got %v", err)
	}
}
