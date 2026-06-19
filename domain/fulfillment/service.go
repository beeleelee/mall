package fulfillment

import (
	"context"
)

type RateCalculator interface {
	CalculateRates(ctx context.Context, input RateInput) (*RateResult, error)
}
