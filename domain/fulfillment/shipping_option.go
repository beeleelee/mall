package fulfillment

type ShippingOption struct {
	ID        string `json:"id"`
	Name      string `json:"name"`
	Cost      int64  `json:"cost"`
	Estimated string `json:"estimated,omitempty"`
	Carrier   string `json:"carrier,omitempty"`
}

type RateInput struct {
	DestinationCountry string
	DestinationState   string
	DestinationCity    string
	Items              []RateItem
}

type RateItem struct {
	Weight   float64
	Quantity int
}

type RateResult struct {
	Options []ShippingOption
}
