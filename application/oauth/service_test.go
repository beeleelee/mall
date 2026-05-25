package oauth

import (
	"context"
	"testing"

	domain "github.com/beeleelee/mall/domain/oauth"
	"github.com/beeleelee/mall/domain/kernel"
)

func newTestAppService(t *testing.T) *OAuthAppService {
	t.Helper()
	clients := newFakeClientRepo()
	codes := newFakeCodeRepo()
	tokens := newFakeTokenRepo()

	const id kernel.ID = 1
	client, err := domain.NewClient(id, "test-client", "test-secret", []string{"https://example.com/cb"}, []string{"read", "write"})
	if err != nil {
		t.Fatal(err)
	}
	if err := clients.Save(context.Background(), client); err != nil {
		t.Fatal(err)
	}

	svc := domain.NewOAuthService(clients, codes, tokens, fakeLogger{}, []byte("jwt-secret"))
	return NewOAuthAppService(svc, fakeLogger{})
}

func TestAppService_Authorize_Success(t *testing.T) {
	app := newTestAppService(t)
	resp, err := app.Authorize(context.Background(), AuthorizeRequest{
		ClientID:    "test-client",
		RedirectURI: "https://example.com/cb",
		Scope:       "read",
		UserID:      42,
	})
	if err != nil {
		t.Fatal(err)
	}
	if resp.Code == "" {
		t.Error("expected non-empty code")
	}
	if resp.RedirectURI != "https://example.com/cb" {
		t.Errorf("expected https://example.com/cb, got %s", resp.RedirectURI)
	}
}

func TestAppService_Exchange_Success(t *testing.T) {
	app := newTestAppService(t)
	ctx := context.Background()

	auth, err := app.Authorize(ctx, AuthorizeRequest{
		ClientID:    "test-client",
		RedirectURI: "https://example.com/cb",
		Scope:       "read",
		UserID:      42,
	})
	if err != nil {
		t.Fatal(err)
	}

	tokenResp, err := app.Exchange(ctx, TokenExchangeRequest{
		Code:         auth.Code,
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

func TestAppService_Exchange_InvalidSecret(t *testing.T) {
	app := newTestAppService(t)
	ctx := context.Background()

	auth, err := app.Authorize(ctx, AuthorizeRequest{
		ClientID:    "test-client",
		RedirectURI: "https://example.com/cb",
		UserID:      42,
	})
	if err != nil {
		t.Fatal(err)
	}

	_, err = app.Exchange(ctx, TokenExchangeRequest{
		Code:         auth.Code,
		ClientID:     "test-client",
		ClientSecret: "wrong-secret",
		RedirectURI:  "https://example.com/cb",
	})
	if !kernel.IsUnauthenticated(err) {
		t.Errorf("expected unauthenticated, got %v", err)
	}
}

func TestAppService_Refresh_Success(t *testing.T) {
	app := newTestAppService(t)
	ctx := context.Background()

	auth, err := app.Authorize(ctx, AuthorizeRequest{
		ClientID:    "test-client",
		RedirectURI: "https://example.com/cb",
		UserID:      42,
	})
	if err != nil {
		t.Fatal(err)
	}

	tokenResp, err := app.Exchange(ctx, TokenExchangeRequest{
		Code:         auth.Code,
		ClientID:     "test-client",
		ClientSecret: "test-secret",
		RedirectURI:  "https://example.com/cb",
	})
	if err != nil {
		t.Fatal(err)
	}

	newTokens, err := app.Refresh(ctx, TokenRefreshRequest{
		RefreshToken: tokenResp.RefreshToken,
		ClientID:     "test-client",
		ClientSecret: "test-secret",
	})
	if err != nil {
		t.Fatal(err)
	}
	if newTokens.AccessToken == "" {
		t.Error("expected non-empty access token")
	}
}

func TestAppService_Revoke_Success(t *testing.T) {
	app := newTestAppService(t)
	ctx := context.Background()

	auth, err := app.Authorize(ctx, AuthorizeRequest{
		ClientID:    "test-client",
		RedirectURI: "https://example.com/cb",
		UserID:      42,
	})
	if err != nil {
		t.Fatal(err)
	}

	tokenResp, err := app.Exchange(ctx, TokenExchangeRequest{
		Code:         auth.Code,
		ClientID:     "test-client",
		ClientSecret: "test-secret",
		RedirectURI:  "https://example.com/cb",
	})
	if err != nil {
		t.Fatal(err)
	}

	if err := app.Revoke(ctx, RevokeRequest{
		Token:        tokenResp.RefreshToken,
		ClientID:     "test-client",
		ClientSecret: "test-secret",
	}); err != nil {
		t.Fatal(err)
	}

	_, err = app.Refresh(ctx, TokenRefreshRequest{
		RefreshToken: tokenResp.RefreshToken,
		ClientID:     "test-client",
		ClientSecret: "test-secret",
	})
	if !kernel.IsUnauthenticated(err) {
		t.Errorf("expected unauthenticated after revocation, got %v", err)
	}
}
