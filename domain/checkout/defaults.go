package checkout

import "context"

type DefaultTaxService struct{}

func NewDefaultTaxService() *DefaultTaxService {
	return &DefaultTaxService{}
}

func (s *DefaultTaxService) CalculateTax(_ context.Context, input TaxInput) (*TaxResult, error) {
	return &TaxResult{TaxAmount: 0, Provider: "passthrough"}, nil
}

type DefaultPriceCalculator struct{}

func NewDefaultPriceCalculator() *DefaultPriceCalculator {
	return &DefaultPriceCalculator{}
}

func (c *DefaultPriceCalculator) Calculate(_ context.Context, input PriceInput) (*PriceResult, error) {
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
