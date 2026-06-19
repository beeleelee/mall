package rest

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/beeleelee/mall/domain/kernel"
	domain "github.com/beeleelee/mall/domain/oauth"

	app "github.com/beeleelee/mall/application/oauth"
)

type fakeClientRepoOAuth struct {
	mu map[string]*domain.OAuthClient
}

func newFakeClientRepoOAuth() *fakeClientRepoOAuth {
	return &fakeClientRepoOAuth{mu: make(map[string]*domain.OAuthClient)}
}

func (f *fakeClientRepoOAuth) Save(_ context.Context, c *domain.OAuthClient) error {
	f.mu[c.ClientID] = c
	return nil
}

func (f *fakeClientRepoOAuth) FindByClientID(_ context.Context, clientID string) (*domain.OAuthClient, error) {
	c, ok := f.mu[clientID]
	if !ok {
		return nil, kernel.NewDomainError(kernel.ErrNotFound, "client not found")
	}
	return c, nil
}

func (f *fakeClientRepoOAuth) FindByID(_ context.Context, id kernel.ID) (*domain.OAuthClient, error) {
	for _, c := range f.mu {
		if c.ID == id {
			return c, nil
		}
	}
	return nil, kernel.NewDomainError(kernel.ErrNotFound, "client not found")
}

type fakeCodeRepoOAuth struct {
	mu map[string]*domain.AuthorizationCode
}

func newFakeCodeRepoOAuth() *fakeCodeRepoOAuth {
	return &fakeCodeRepoOAuth{mu: make(map[string]*domain.AuthorizationCode)}
}

func (f *fakeCodeRepoOAuth) Save(_ context.Context, c *domain.AuthorizationCode) error {
	f.mu[c.Code] = c
	return nil
}

func (f *fakeCodeRepoOAuth) FindByCode(_ context.Context, code string) (*domain.AuthorizationCode, error) {
	c, ok := f.mu[code]
	if !ok {
		return nil, kernel.NewDomainError(kernel.ErrNotFound, "code not found")
	}
	return c, nil
}

func (f *fakeCodeRepoOAuth) Delete(_ context.Context, code string) error {
	delete(f.mu, code)
	return nil
}

type fakeTokenRepoOAuth struct {
	mu map[string]*domain.RefreshToken
}

func newFakeTokenRepoOAuth() *fakeTokenRepoOAuth {
	return &fakeTokenRepoOAuth{mu: make(map[string]*domain.RefreshToken)}
}

func (f *fakeTokenRepoOAuth) Save(_ context.Context, t *domain.RefreshToken) error {
	f.mu[t.ID] = t
	return nil
}

func (f *fakeTokenRepoOAuth) FindByID(_ context.Context, id string) (*domain.RefreshToken, error) {
	t, ok := f.mu[id]
	if !ok {
		return nil, kernel.NewDomainError(kernel.ErrNotFound, "token not found")
	}
	return t, nil
}

func (f *fakeTokenRepoOAuth) Revoke(_ context.Context, id string) error {
	t, ok := f.mu[id]
	if !ok {
		return kernel.NewDomainError(kernel.ErrNotFound, "token not found")
	}
	t.Revoked = true
	return nil
}

type fakeLoggerOAuth struct{}

func (fakeLoggerOAuth) Debug(_ context.Context, _ string, _ ...kernel.LogField)          {}
func (fakeLoggerOAuth) Info(_ context.Context, _ string, _ ...kernel.LogField)           {}
func (fakeLoggerOAuth) Warn(_ context.Context, _ string, _ ...kernel.LogField)           {}
func (fakeLoggerOAuth) Error(_ context.Context, _ string, _ error, _ ...kernel.LogField) {}

func newTestOAuthHandler(t *testing.T) *OAuthHandler {
	t.Helper()
	clients := newFakeClientRepoOAuth()
	codes := newFakeCodeRepoOAuth()
	tokens := newFakeTokenRepoOAuth()
	logger := fakeLoggerOAuth{}

	client, err := domain.NewClient(1, "test-client", "test-secret", []string{"https://example.com/cb"}, []string{"read", "write"})
	if err != nil {
		t.Fatal(err)
	}
	if err := clients.Save(context.Background(), client); err != nil {
		t.Fatal(err)
	}

	domainSvc := domain.NewOAuthService(clients, codes, tokens, logger, []byte("jwt-secret"))
	appSvc := app.NewOAuthAppService(domainSvc, logger)
	return NewOAuthHandler(appSvc)
}

func TestOAuthHandler_Authorize_Success(t *testing.T) {
	h := newTestOAuthHandler(t)

	body := map[string]any{
		"client_id":    "test-client",
		"redirect_uri": "https://example.com/cb",
		"scope":        "read",
		"user_id":      42,
	}
	b, _ := json.Marshal(body)

	w := httptest.NewRecorder()
	r := httptest.NewRequest(http.MethodPost, "/oauth/authorize", bytes.NewReader(b))
	r.Header.Set("Content-Type", "application/json")

	h.Authorize(w, r)

	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", w.Code, w.Body.String())
	}

	var resp app.AuthorizeResponse
	if err := json.Unmarshal(w.Body.Bytes(), &resp); err != nil {
		t.Fatal(err)
	}
	if resp.Code == "" {
		t.Error("expected non-empty authorization code")
	}
}

func TestOAuthHandler_Authorize_InvalidClient(t *testing.T) {
	h := newTestOAuthHandler(t)

	body := map[string]any{
		"client_id":    "nonexistent",
		"redirect_uri": "https://example.com/cb",
		"user_id":      42,
	}
	b, _ := json.Marshal(body)

	w := httptest.NewRecorder()
	r := httptest.NewRequest(http.MethodPost, "/oauth/authorize", bytes.NewReader(b))
	r.Header.Set("Content-Type", "application/json")

	h.Authorize(w, r)

	if w.Code != http.StatusNotFound {
		t.Fatalf("expected 404, got %d: %s", w.Code, w.Body.String())
	}
}

func TestOAuthHandler_Token_AuthorizationCode(t *testing.T) {
	h := newTestOAuthHandler(t)

	authBody := map[string]any{
		"client_id":    "test-client",
		"redirect_uri": "https://example.com/cb",
		"scope":        "read",
		"user_id":      42,
	}
	b, _ := json.Marshal(authBody)

	w := httptest.NewRecorder()
	r := httptest.NewRequest(http.MethodPost, "/oauth/authorize", bytes.NewReader(b))
	r.Header.Set("Content-Type", "application/json")
	h.Authorize(w, r)
	if w.Code != http.StatusOK {
		t.Fatalf("authorize failed: %d: %s", w.Code, w.Body.String())
	}

	var authResp app.AuthorizeResponse
	json.Unmarshal(w.Body.Bytes(), &authResp)

	tokenBody := map[string]string{
		"grant_type":    "authorization_code",
		"code":          authResp.Code,
		"client_id":     "test-client",
		"client_secret": "test-secret",
		"redirect_uri":  "https://example.com/cb",
	}
	b, _ = json.Marshal(tokenBody)

	w = httptest.NewRecorder()
	r = httptest.NewRequest(http.MethodPost, "/oauth/token", bytes.NewReader(b))
	r.Header.Set("Content-Type", "application/json")
	h.Token(w, r)

	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", w.Code, w.Body.String())
	}

	var tokenResp app.TokenResponse
	json.Unmarshal(w.Body.Bytes(), &tokenResp)
	if tokenResp.AccessToken == "" {
		t.Error("expected non-empty access_token")
	}
	if tokenResp.RefreshToken == "" {
		t.Error("expected non-empty refresh_token")
	}
}

func TestOAuthHandler_Token_RefreshToken(t *testing.T) {
	h := newTestOAuthHandler(t)

	authBody := map[string]any{
		"client_id":    "test-client",
		"redirect_uri": "https://example.com/cb",
		"user_id":      42,
	}
	b, _ := json.Marshal(authBody)

	w := httptest.NewRecorder()
	r := httptest.NewRequest(http.MethodPost, "/oauth/authorize", bytes.NewReader(b))
	r.Header.Set("Content-Type", "application/json")
	h.Authorize(w, r)
	if w.Code != http.StatusOK {
		t.Fatalf("authorize failed: %d: %s", w.Code, w.Body.String())
	}

	var authResp app.AuthorizeResponse
	json.Unmarshal(w.Body.Bytes(), &authResp)

	exBody := map[string]string{
		"grant_type":    "authorization_code",
		"code":          authResp.Code,
		"client_id":     "test-client",
		"client_secret": "test-secret",
		"redirect_uri":  "https://example.com/cb",
	}
	b, _ = json.Marshal(exBody)

	w = httptest.NewRecorder()
	r = httptest.NewRequest(http.MethodPost, "/oauth/token", bytes.NewReader(b))
	r.Header.Set("Content-Type", "application/json")
	h.Token(w, r)
	if w.Code != http.StatusOK {
		t.Fatalf("exchange failed: %d: %s", w.Code, w.Body.String())
	}

	var tokenResp app.TokenResponse
	json.Unmarshal(w.Body.Bytes(), &tokenResp)

	refreshBody := map[string]string{
		"grant_type":    "refresh_token",
		"refresh_token": tokenResp.RefreshToken,
		"client_id":     "test-client",
		"client_secret": "test-secret",
	}
	b, _ = json.Marshal(refreshBody)

	w = httptest.NewRecorder()
	r = httptest.NewRequest(http.MethodPost, "/oauth/token", bytes.NewReader(b))
	r.Header.Set("Content-Type", "application/json")
	h.Token(w, r)

	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", w.Code, w.Body.String())
	}
}

func TestOAuthHandler_Token_InvalidGrantType(t *testing.T) {
	h := newTestOAuthHandler(t)

	body := map[string]string{
		"grant_type": "invalid",
		"client_id":  "test-client",
	}
	b, _ := json.Marshal(body)

	w := httptest.NewRecorder()
	r := httptest.NewRequest(http.MethodPost, "/oauth/token", bytes.NewReader(b))
	r.Header.Set("Content-Type", "application/json")
	h.Token(w, r)

	if w.Code != http.StatusBadRequest {
		t.Fatalf("expected 400, got %d: %s", w.Code, w.Body.String())
	}
}

func TestOAuthHandler_Revoke_Success(t *testing.T) {
	h := newTestOAuthHandler(t)

	authBody := map[string]any{
		"client_id":    "test-client",
		"redirect_uri": "https://example.com/cb",
		"user_id":      42,
	}
	b, _ := json.Marshal(authBody)

	w := httptest.NewRecorder()
	r := httptest.NewRequest(http.MethodPost, "/oauth/authorize", bytes.NewReader(b))
	r.Header.Set("Content-Type", "application/json")
	h.Authorize(w, r)

	var authResp app.AuthorizeResponse
	json.Unmarshal(w.Body.Bytes(), &authResp)

	exBody := map[string]string{
		"grant_type":    "authorization_code",
		"code":          authResp.Code,
		"client_id":     "test-client",
		"client_secret": "test-secret",
		"redirect_uri":  "https://example.com/cb",
	}
	b, _ = json.Marshal(exBody)

	w = httptest.NewRecorder()
	r = httptest.NewRequest(http.MethodPost, "/oauth/token", bytes.NewReader(b))
	r.Header.Set("Content-Type", "application/json")
	h.Token(w, r)

	var tokenResp app.TokenResponse
	json.Unmarshal(w.Body.Bytes(), &tokenResp)

	revokeBody := map[string]string{
		"token":         tokenResp.RefreshToken,
		"client_id":     "test-client",
		"client_secret": "test-secret",
	}
	b, _ = json.Marshal(revokeBody)

	w = httptest.NewRecorder()
	r = httptest.NewRequest(http.MethodPost, "/oauth/revoke", bytes.NewReader(b))
	r.Header.Set("Content-Type", "application/json")
	h.Revoke(w, r)

	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", w.Code, w.Body.String())
	}
}
