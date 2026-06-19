package ecp

type ECPMessage struct {
	JSONRPC string `json:"jsonrpc"`
	Method  string `json:"method"`
	Params  any    `json:"params,omitempty"`
	ID      any    `json:"id,omitempty"`
}

type ECPResponse struct {
	JSONRPC string `json:"jsonrpc"`
	Result  any    `json:"result,omitempty"`
	Error   *ECPError `json:"error,omitempty"`
	ID      any    `json:"id,omitempty"`
}

type ECPError struct {
	Code    int    `json:"code"`
	Message string `json:"message"`
}

const (
	MethodStateUpdate       = "state.update"
	MethodCredentialsSubmit = "credentials.submit"
	MethodPaymentAuthorize  = "payment.authorize"
	MethodAddressSelect     = "address.select"
)

type StateUpdateParams struct {
	CheckoutID int64                  `json:"checkout_id"`
	Status     string                 `json:"status"`
	Fields     map[string]any         `json:"fields,omitempty"`
}

type StateUpdateResult struct {
	CheckoutID  int64  `json:"checkout_id"`
	Status      string `json:"status"`
	ContinueURL string `json:"continue_url,omitempty"`
}

type CredentialsSubmitParams struct {
	CheckoutID      int64  `json:"checkout_id"`
	ShippingLine1   string `json:"shipping_line1"`
	ShippingLine2   string `json:"shipping_line2,omitempty"`
	ShippingCity    string `json:"shipping_city"`
	ShippingState   string `json:"shipping_state"`
	ShippingPostal  string `json:"shipping_postal"`
	ShippingCountry string `json:"shipping_country"`
	BillingLine1    string `json:"billing_line1"`
	BillingLine2    string `json:"billing_line2,omitempty"`
	BillingCity     string `json:"billing_city"`
	BillingState    string `json:"billing_state"`
	BillingPostal   string `json:"billing_postal"`
	BillingCountry  string `json:"billing_country"`
}

type CredentialsSubmitResult struct {
	CheckoutID int64  `json:"checkout_id"`
	Status     string `json:"status"`
}

type PaymentAuthorizeParams struct {
	CheckoutID int64  `json:"checkout_id"`
	MandateID  string `json:"mandate_id,omitempty"`
	Signature  string `json:"signature,omitempty"`
}

type PaymentAuthorizeResult struct {
	CheckoutID int64  `json:"checkout_id"`
	Status     string `json:"status"`
	Token      string `json:"token,omitempty"`
}

type AddressSelectParams struct {
	CheckoutID  int64  `json:"checkout_id"`
	AddressType string `json:"address_type"`
	Line1       string `json:"line1"`
	Line2       string `json:"line2,omitempty"`
	City        string `json:"city"`
	State       string `json:"state"`
	PostalCode  string `json:"postal_code"`
	Country     string `json:"country"`
}

type AddressSelectResult struct {
	CheckoutID  int64  `json:"checkout_id"`
	AddressType string `json:"address_type"`
	Status      string `json:"status"`
}
