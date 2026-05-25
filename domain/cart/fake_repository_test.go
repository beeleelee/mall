package cart

import (
	"context"
	"sync"

	"github.com/beeleelee/mall/domain/kernel"
)

type fakeCartRepo struct {
	mu     sync.Mutex
	carts  map[kernel.ID]*Cart
	byUser map[kernel.ID]kernel.ID
}

func newFakeCartRepo() *fakeCartRepo {
	return &fakeCartRepo{
		carts:  make(map[kernel.ID]*Cart),
		byUser: make(map[kernel.ID]kernel.ID),
	}
}

func (f *fakeCartRepo) Save(_ context.Context, cart *Cart) error {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.carts[cart.ID] = cart
	f.byUser[cart.UserID] = cart.ID
	return nil
}

func (f *fakeCartRepo) FindByID(_ context.Context, id kernel.ID) (*Cart, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	c, ok := f.carts[id]
	if !ok {
		return nil, kernel.NewDomainError(kernel.ErrNotFound, "cart not found")
	}
	return c, nil
}

func (f *fakeCartRepo) FindByUserID(_ context.Context, userID kernel.ID) (*Cart, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	id, ok := f.byUser[userID]
	if !ok {
		return nil, kernel.NewDomainError(kernel.ErrNotFound, "cart not found")
	}
	c, ok := f.carts[id]
	if !ok {
		return nil, kernel.NewDomainError(kernel.ErrNotFound, "cart not found")
	}
	return c, nil
}

func (f *fakeCartRepo) Delete(_ context.Context, id kernel.ID) error {
	f.mu.Lock()
	defer f.mu.Unlock()
	c, ok := f.carts[id]
	if !ok {
		return kernel.NewDomainError(kernel.ErrNotFound, "cart not found")
	}
	delete(f.byUser, c.UserID)
	delete(f.carts, id)
	return nil
}

type fakeCartPublisher struct {
	mu       sync.Mutex
	published []*Cart
}

func newFakeCartPublisher() *fakeCartPublisher {
	return &fakeCartPublisher{}
}

func (f *fakeCartPublisher) PublishCartUpdated(_ context.Context, cart *Cart) error {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.published = append(f.published, cart)
	return nil
}

type fakeLoggerCart struct{}

func (fakeLoggerCart) Debug(_ context.Context, _ string, _ ...kernel.LogField)         {}
func (fakeLoggerCart) Info(_ context.Context, _ string, _ ...kernel.LogField)          {}
func (fakeLoggerCart) Warn(_ context.Context, _ string, _ ...kernel.LogField)          {}
func (fakeLoggerCart) Error(_ context.Context, _ string, _ error, _ ...kernel.LogField) {}
