package oauth

import (
	"context"
	"sync"

	"github.com/beeleelee/mall/domain/kernel"
	domain "github.com/beeleelee/mall/domain/oauth"
)

type fakeClientRepo struct {
	mu    sync.Mutex
	byID  map[kernel.ID]*domain.OAuthClient
	byCID map[string]*domain.OAuthClient
}

func newFakeClientRepo() *fakeClientRepo {
	return &fakeClientRepo{
		byID:  make(map[kernel.ID]*domain.OAuthClient),
		byCID: make(map[string]*domain.OAuthClient),
	}
}

func (f *fakeClientRepo) Save(_ context.Context, c *domain.OAuthClient) error {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.byID[c.ID] = c
	f.byCID[c.ClientID] = c
	return nil
}

func (f *fakeClientRepo) FindByClientID(_ context.Context, clientID string) (*domain.OAuthClient, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	c, ok := f.byCID[clientID]
	if !ok {
		return nil, kernel.NewDomainError(kernel.ErrNotFound, "client not found")
	}
	return c, nil
}

func (f *fakeClientRepo) FindByID(_ context.Context, id kernel.ID) (*domain.OAuthClient, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	c, ok := f.byID[id]
	if !ok {
		return nil, kernel.NewDomainError(kernel.ErrNotFound, "client not found")
	}
	return c, nil
}

type fakeCodeRepo struct {
	mu    sync.Mutex
	codes map[string]*domain.AuthorizationCode
}

func newFakeCodeRepo() *fakeCodeRepo {
	return &fakeCodeRepo{codes: make(map[string]*domain.AuthorizationCode)}
}

func (f *fakeCodeRepo) Save(_ context.Context, c *domain.AuthorizationCode) error {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.codes[c.Code] = c
	return nil
}

func (f *fakeCodeRepo) FindByCode(_ context.Context, code string) (*domain.AuthorizationCode, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	c, ok := f.codes[code]
	if !ok {
		return nil, kernel.NewDomainError(kernel.ErrNotFound, "code not found")
	}
	return c, nil
}

func (f *fakeCodeRepo) Delete(_ context.Context, code string) error {
	f.mu.Lock()
	defer f.mu.Unlock()
	delete(f.codes, code)
	return nil
}

type fakeTokenRepo struct {
	mu     sync.Mutex
	tokens map[string]*domain.RefreshToken
}

func newFakeTokenRepo() *fakeTokenRepo {
	return &fakeTokenRepo{tokens: make(map[string]*domain.RefreshToken)}
}

func (f *fakeTokenRepo) Save(_ context.Context, t *domain.RefreshToken) error {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.tokens[t.ID] = t
	return nil
}

func (f *fakeTokenRepo) FindByID(_ context.Context, id string) (*domain.RefreshToken, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	t, ok := f.tokens[id]
	if !ok {
		return nil, kernel.NewDomainError(kernel.ErrNotFound, "refresh token not found")
	}
	return t, nil
}

func (f *fakeTokenRepo) Revoke(_ context.Context, id string) error {
	f.mu.Lock()
	defer f.mu.Unlock()
	t, ok := f.tokens[id]
	if !ok {
		return kernel.NewDomainError(kernel.ErrNotFound, "refresh token not found")
	}
	t.Revoked = true
	return nil
}

type fakeLogger struct{}

func (fakeLogger) Debug(_ context.Context, _ string, _ ...kernel.LogField)          {}
func (fakeLogger) Info(_ context.Context, _ string, _ ...kernel.LogField)           {}
func (fakeLogger) Warn(_ context.Context, _ string, _ ...kernel.LogField)           {}
func (fakeLogger) Error(_ context.Context, _ string, _ error, _ ...kernel.LogField) {}
