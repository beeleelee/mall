package subscription

import (
	"context"

	"github.com/beeleelee/mall/domain/kernel"
	domain "github.com/beeleelee/mall/domain/subscription"
)

type MockBillingCharger struct {
	failEmptyToken bool
	failOn         map[int64]bool
}

func NewMockBillingCharger() *MockBillingCharger {
	return &MockBillingCharger{}
}

func (c *MockBillingCharger) FailOn(subID int64) {
	if c.failOn == nil {
		c.failOn = make(map[int64]bool)
	}
	c.failOn[subID] = true
}

func (c *MockBillingCharger) Charge(_ context.Context, subID, _ kernel.ID, _ int64, token string) error {
	if token == "" {
		return kernel.NewDomainError(kernel.ErrInvalidArgument, "payment token must not be empty")
	}
	if c.failOn[subID.Int64()] {
		return kernel.NewDomainErrorWithCause(kernel.ErrUnavailable, "mock charge failed", nil)
	}
	return nil
}

var _ domain.BillingCharger = (*MockBillingCharger)(nil)
