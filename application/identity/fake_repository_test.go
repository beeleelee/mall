package identity

import (
	"context"
	"sync"

	domain "github.com/beeleelee/mall/domain/identity"
	"github.com/beeleelee/mall/domain/kernel"
)

type fakeUserRepository struct {
	mu     sync.Mutex
	users  map[kernel.ID]*domain.User
	emails map[string]kernel.ID
}

func newFakeUserRepository() *fakeUserRepository {
	return &fakeUserRepository{
		users:  make(map[kernel.ID]*domain.User),
		emails: make(map[string]kernel.ID),
	}
}

func (f *fakeUserRepository) Save(_ context.Context, user *domain.User) error {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.users[user.ID] = user
	f.emails[user.Email] = user.ID
	return nil
}

func (f *fakeUserRepository) FindByID(_ context.Context, id kernel.ID) (*domain.User, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	u, ok := f.users[id]
	if !ok {
		return nil, kernel.NewDomainError(kernel.ErrNotFound, "user not found")
	}
	return u, nil
}

func (f *fakeUserRepository) FindByEmail(_ context.Context, email string) (*domain.User, error) {
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

func (f *fakeUserRepository) FindAll(_ context.Context, offset, limit int) ([]*domain.User, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	result := make([]*domain.User, 0, len(f.users))
	for _, u := range f.users {
		result = append(result, u)
	}
	if offset >= len(result) {
		return []*domain.User{}, nil
	}
	end := offset + limit
	if end > len(result) {
		end = len(result)
	}
	return result[offset:end], nil
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
