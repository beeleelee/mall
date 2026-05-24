package identity

import (
	"context"
	"sync"

	"github.com/beeleelee/mall/domain/kernel"
)

type fakeUserRepository struct {
	mu     sync.Mutex
	users  map[kernel.ID]*User
	emails map[string]kernel.ID
}

func newFakeUserRepository() *fakeUserRepository {
	return &fakeUserRepository{
		users:  make(map[kernel.ID]*User),
		emails: make(map[string]kernel.ID),
	}
}

func (f *fakeUserRepository) Save(_ context.Context, user *User) error {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.users[user.ID] = user
	f.emails[user.Email] = user.ID
	return nil
}

func (f *fakeUserRepository) FindByID(_ context.Context, id kernel.ID) (*User, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	u, ok := f.users[id]
	if !ok {
		return nil, kernel.NewDomainError(kernel.ErrNotFound, "user not found")
	}
	return u, nil
}

func (f *fakeUserRepository) FindByEmail(_ context.Context, email string) (*User, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	id, ok := f.emails[email]
	if !ok {
		return nil, kernel.NewDomainError(kernel.ErrNotFound, "user not found")
	}
	u, ok := f.users[id]
	if !ok {
		return nil, kernel.NewDomainError(kernel.ErrNotFound, "user not found")
	}
	return u, nil
}

func (f *fakeUserRepository) Delete(_ context.Context, id kernel.ID) error {
	f.mu.Lock()
	defer f.mu.Unlock()
	u, ok := f.users[id]
	if !ok {
		return kernel.NewDomainError(kernel.ErrNotFound, "user not found")
	}
	delete(f.emails, u.Email)
	delete(f.users, id)
	return nil
}

type fakeLogger struct{}

func (fakeLogger) Debug(_ context.Context, _ string, _ ...kernel.LogField)          {}
func (fakeLogger) Info(_ context.Context, _ string, _ ...kernel.LogField)           {}
func (fakeLogger) Warn(_ context.Context, _ string, _ ...kernel.LogField)           {}
func (fakeLogger) Error(_ context.Context, _ string, _ error, _ ...kernel.LogField) {}
