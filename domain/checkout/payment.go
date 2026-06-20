package checkout

type HandlerSpec struct {
	Name             string   `json:"name"`
	Provider         string   `json:"provider"`
	Capabilities     []string `json:"capabilities,omitempty"`
	SupportedRegions []string `json:"supported_regions,omitempty"`
	RequiresAP2      bool     `json:"requires_ap2,omitempty"`
	MinAmount        int64    `json:"min_amount,omitempty"`
	MaxAmount        int64    `json:"max_amount,omitempty"`
}

type PaymentHandlerRegistry struct {
	handlers []HandlerSpec
}

func NewPaymentHandlerRegistry() *PaymentHandlerRegistry {
	return &PaymentHandlerRegistry{
		handlers: []HandlerSpec{
			{
				Name:             "mock",
				Provider:         "mock",
				Capabilities:     []string{"payment"},
				SupportedRegions: []string{"*"},
				RequiresAP2:      false,
			},
			{
				Name:             "stripe",
				Provider:         "stripe",
				Capabilities:     []string{"payment", "refund"},
				SupportedRegions: []string{"US", "CA", "GB", "DE", "FR", "AU"},
				RequiresAP2:      false,
				MaxAmount:        99999900,
			},
			{
				Name:             "ap2_mandate",
				Provider:         "ap2",
				Capabilities:     []string{"payment", "autonomous"},
				SupportedRegions: []string{"*"},
				RequiresAP2:      true,
			},
			{
				Name:             "shop_pay",
				Provider:         "shopify",
				Capabilities:     []string{"payment", "wallet", "express"},
				SupportedRegions: []string{"US", "CA", "GB", "AU"},
				RequiresAP2:      false,
				MaxAmount:        99999900,
			},
			{
				Name:             "google_pay",
				Provider:         "google",
				Capabilities:     []string{"payment", "wallet", "tokenization"},
				SupportedRegions: []string{"US", "CA", "GB", "DE", "FR", "JP", "AU"},
				RequiresAP2:      false,
				MaxAmount:        99999900,
			},
			{
				Name:             "apple_pay",
				Provider:         "apple",
				Capabilities:     []string{"payment", "wallet", "tokenization"},
				SupportedRegions: []string{"US", "CA", "GB", "FR", "DE", "AU", "JP", "SG"},
				RequiresAP2:      false,
				MaxAmount:        99999900,
			},
		},
	}
}

func (r *PaymentHandlerRegistry) GetHandlers() []HandlerSpec {
	return r.handlers
}

func (r *PaymentHandlerRegistry) FindByName(name string) *HandlerSpec {
	for _, h := range r.handlers {
		if h.Name == name {
			return &h
		}
	}
	return nil
}

func (r *PaymentHandlerRegistry) Negotiate(amount int64, region string, requested string) *HandlerSpec {
	if requested != "" {
		if h := r.FindByName(requested); h != nil {
			return h
		}
		return r.FindByName("mock")
	}

	for _, h := range r.handlers {
		if h.RequiresAP2 {
			continue
		}
		if h.MaxAmount > 0 && amount > h.MaxAmount {
			continue
		}
		if h.MinAmount > 0 && amount < h.MinAmount {
			continue
		}
		for _, reg := range h.SupportedRegions {
			if reg == "*" || reg == region {
				return &h
			}
		}
	}

	return r.FindByName("mock")
}

type AP2Verifier struct{}

func NewAP2Verifier() *AP2Verifier {
	return &AP2Verifier{}
}

func (v *AP2Verifier) VerifyMandate(mandateToken string) bool {
	return mandateToken != ""
}
