package order

import (
	"context"
	"sync"

	"github.com/beeleelee/mall/domain/kernel"
)

type fakeOrderRepo struct {
	mu     sync.Mutex
	orders map[kernel.ID]*Order
	byUser map[kernel.ID][]kernel.ID
}

func newFakeOrderRepo() *fakeOrderRepo {
	return &fakeOrderRepo{
		orders: make(map[kernel.ID]*Order),
		byUser: make(map[kernel.ID][]kernel.ID),
	}
}

func (f *fakeOrderRepo) Save(_ context.Context, o *Order) error {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.orders[o.ID] = o
	f.byUser[o.UserID] = append(f.byUser[o.UserID], o.ID)
	return nil
}

func (f *fakeOrderRepo) FindByID(_ context.Context, id kernel.ID) (*Order, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	o, ok := f.orders[id]
	if !ok {
		return nil, kernel.NewDomainError(kernel.ErrNotFound, "order not found")
	}
	return o, nil
}

func (f *fakeOrderRepo) FindByUserID(_ context.Context, userID kernel.ID) ([]*Order, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	ids, ok := f.byUser[userID]
	if !ok {
		return nil, kernel.NewDomainError(kernel.ErrNotFound, "no orders found for user")
	}
	result := make([]*Order, 0, len(ids))
	for _, id := range ids {
		if o, ok := f.orders[id]; ok {
			result = append(result, o)
		}
	}
	return result, nil
}

func (f *fakeOrderRepo) FindByCheckoutID(_ context.Context, checkoutID kernel.ID) (*Order, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	for _, o := range f.orders {
		if o.CheckoutID == checkoutID {
			return o, nil
		}
	}
	return nil, kernel.NewDomainError(kernel.ErrNotFound, "order not found for checkout")
}

func (f *fakeOrderRepo) Delete(_ context.Context, id kernel.ID) error {
	f.mu.Lock()
	defer f.mu.Unlock()
	o, ok := f.orders[id]
	if !ok {
		return kernel.NewDomainError(kernel.ErrNotFound, "order not found")
	}
	delete(f.orders, id)
	delete(f.byUser, o.UserID)
	return nil
}

type fakeOrderPublisher struct {
	mu sync.Mutex
}

func newFakeOrderPublisher() *fakeOrderPublisher {
	return &fakeOrderPublisher{}
}

func (f *fakeOrderPublisher) PublishOrderEvent(_ context.Context, _ *Order) error {
	return nil
}

type fakeLoggerOrder struct{}

func (fakeLoggerOrder) Debug(_ context.Context, _ string, _ ...kernel.LogField)          {}
func (fakeLoggerOrder) Info(_ context.Context, _ string, _ ...kernel.LogField)           {}
func (fakeLoggerOrder) Warn(_ context.Context, _ string, _ ...kernel.LogField)           {}
func (fakeLoggerOrder) Error(_ context.Context, _ string, _ error, _ ...kernel.LogField) {}
