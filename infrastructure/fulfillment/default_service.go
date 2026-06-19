package fulfillment

import (
	"context"

	domain "github.com/beeleelee/mall/domain/fulfillment"
)

type DefaultFulfillmentService struct{}

func NewDefaultFulfillmentService() *DefaultFulfillmentService {
	return &DefaultFulfillmentService{}
}

func (s *DefaultFulfillmentService) CalculateRates(_ context.Context, input domain.RateInput) (*domain.RateResult, error) {
	domestic := input.DestinationCountry == "US"

	options := []domain.ShippingOption{
		{
			ID:        "standard",
			Name:      "Standard Shipping",
			Cost:      500,
			Estimated: "5-8 business days",
			Carrier:   "Default Carrier",
		},
		{
			ID:        "express",
			Name:      "Express Shipping",
			Cost:      1500,
			Estimated: "2-3 business days",
			Carrier:   "Default Carrier",
		},
	}

	if domestic {
		options = append(options, domain.ShippingOption{
			ID:        "overnight",
			Name:      "Overnight Shipping",
			Cost:      3500,
			Estimated: "Next business day",
			Carrier:   "Default Carrier",
		})
	} else {
		for i := range options {
			options[i].Cost *= 3
		}
	}

	return &domain.RateResult{Options: options}, nil
}
