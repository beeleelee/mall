package checkout

import (
	"context"
	"sync"

	"github.com/beeleelee/mall/domain/kernel"
)

type fakeCheckoutRepo struct {
	mu       sync.Mutex
	sessions map[kernel.ID]*CheckoutSession
	byUser   map[kernel.ID]kernel.ID
}

func newFakeCheckoutRepo() *fakeCheckoutRepo {
	return &fakeCheckoutRepo{
		sessions: make(map[kernel.ID]*CheckoutSession),
		byUser:   make(map[kernel.ID]kernel.ID),
	}
}

func (f *fakeCheckoutRepo) Save(_ context.Context, s *CheckoutSession) error {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.sessions[s.ID] = s
	f.byUser[s.UserID] = s.ID
	return nil
}

func (f *fakeCheckoutRepo) FindByID(_ context.Context, id kernel.ID) (*CheckoutSession, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	s, ok := f.sessions[id]
	if !ok {
		return nil, kernel.NewDomainError(kernel.ErrNotFound, "checkout session not found")
	}
	return s, nil
}

func (f *fakeCheckoutRepo) FindByUserID(_ context.Context, userID kernel.ID) (*CheckoutSession, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	id, ok := f.byUser[userID]
	if !ok {
		return nil, kernel.NewDomainError(kernel.ErrNotFound, "checkout session not found")
	}
	s, ok := f.sessions[id]
	if !ok {
		return nil, kernel.NewDomainError(kernel.ErrNotFound, "checkout session not found")
	}
	return s, nil
}

func (f *fakeCheckoutRepo) Delete(_ context.Context, id kernel.ID) error {
	f.mu.Lock()
	defer f.mu.Unlock()
	s, ok := f.sessions[id]
	if !ok {
		return kernel.NewDomainError(kernel.ErrNotFound, "checkout session not found")
	}
	delete(f.byUser, s.UserID)
	delete(f.sessions, id)
	return nil
}

type fakeTaxService struct{}

func (fakeTaxService) CalculateTax(_ context.Context, input TaxInput) (*TaxResult, error) {
	return &TaxResult{TaxAmount: 0, Provider: "passthrough"}, nil
}

type fakePriceCalculator struct{}

func (fakePriceCalculator) Calculate(_ context.Context, input PriceInput) (*PriceResult, error) {
	var subtotal int64
	for _, item := range input.Items {
		subtotal += item.TotalPrice()
	}
	return &PriceResult{
		Subtotal:   subtotal,
		Shipping:   input.ShippingCost,
		Tax:        input.TaxAmount,
		GrandTotal: subtotal + input.ShippingCost + input.TaxAmount,
	}, nil
}

type fakePublisher struct {
	mu        sync.Mutex
	published []*CheckoutSession
}

func newFakePublisher() *fakePublisher {
	return &fakePublisher{}
}

func (f *fakePublisher) PublishCheckoutUpdated(_ context.Context, _ *CheckoutSession) error {
	f.mu.Lock()
	defer f.mu.Unlock()
	return nil
}

type fakeLoggerCheckout struct{}

func (fakeLoggerCheckout) Debug(_ context.Context, _ string, _ ...kernel.LogField)          {}
func (fakeLoggerCheckout) Info(_ context.Context, _ string, _ ...kernel.LogField)           {}
func (fakeLoggerCheckout) Warn(_ context.Context, _ string, _ ...kernel.LogField)           {}
func (fakeLoggerCheckout) Error(_ context.Context, _ string, _ error, _ ...kernel.LogField) {}
