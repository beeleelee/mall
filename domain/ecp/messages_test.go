package ecp

import (
	"encoding/json"
	"testing"
)

func TestECPMessageConstants(t *testing.T) {
	tests := []struct {
		got  string
		want string
	}{
		{MethodStateUpdate, "state.update"},
		{MethodCredentialsSubmit, "credentials.submit"},
		{MethodPaymentAuthorize, "payment.authorize"},
		{MethodAddressSelect, "address.select"},
	}
	for _, tt := range tests {
		if tt.got != tt.want {
			t.Errorf("got %q, want %q", tt.got, tt.want)
		}
	}
}

func TestECPMessageJSONRoundtrip(t *testing.T) {
	msg := ECPMessage{
		JSONRPC: "2.0",
		Method:  MethodStateUpdate,
		Params:  nil,
		ID:      float64(1),
	}
	data, err := json.Marshal(msg)
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}

	var decoded ECPMessage
	if err := json.Unmarshal(data, &decoded); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	if decoded.JSONRPC != "2.0" {
		t.Errorf("JSONRPC = %q, want 2.0", decoded.JSONRPC)
	}
	if decoded.Method != MethodStateUpdate {
		t.Errorf("Method = %q, want %q", decoded.Method, MethodStateUpdate)
	}
}

func TestECPMessage_NoID(t *testing.T) {
	msg := ECPMessage{
		JSONRPC: "2.0",
		Method:  MethodPaymentAuthorize,
	}
	data, err := json.Marshal(msg)
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}

	var decoded ECPMessage
	if err := json.Unmarshal(data, &decoded); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	if decoded.ID != nil {
		t.Errorf("expected nil ID, got %v", decoded.ID)
	}
}

func TestECPMessage_WithParams(t *testing.T) {
	params := StateUpdateParams{
		CheckoutID: 123,
		Status:     "incomplete",
	}
	msg := ECPMessage{
		JSONRPC: "2.0",
		Method:  MethodStateUpdate,
		Params:  params,
		ID:      float64(1),
	}
	data, err := json.Marshal(msg)
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}

	var decoded struct {
		JSONRPC string          `json:"jsonrpc"`
		Method  string          `json:"method"`
		Params  json.RawMessage `json:"params,omitempty"`
		ID      any             `json:"id,omitempty"`
	}
	if err := json.Unmarshal(data, &decoded); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}

	var p StateUpdateParams
	if err := json.Unmarshal(decoded.Params, &p); err != nil {
		t.Fatalf("unmarshal params: %v", err)
	}
	if p.CheckoutID != 123 {
		t.Errorf("CheckoutID = %d, want 123", p.CheckoutID)
	}
	if p.Status != "incomplete" {
		t.Errorf("Status = %q, want incomplete", p.Status)
	}
}

func TestECPResponseJSONRoundtrip(t *testing.T) {
	resp := ECPResponse{
		JSONRPC: "2.0",
		Result:  StateUpdateResult{CheckoutID: 1, Status: "completed"},
		ID:      float64(1),
	}
	data, err := json.Marshal(resp)
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}

	var decoded ECPResponse
	if err := json.Unmarshal(data, &decoded); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	if decoded.JSONRPC != "2.0" {
		t.Errorf("JSONRPC = %q", decoded.JSONRPC)
	}
}

func TestECPErrorJSONRoundtrip(t *testing.T) {
	errMsg := ECPError{Code: -32600, Message: "Invalid Request"}
	data, err := json.Marshal(errMsg)
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}

	var decoded ECPError
	if err := json.Unmarshal(data, &decoded); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	if decoded.Code != -32600 {
		t.Errorf("Code = %d, want -32600", decoded.Code)
	}
	if decoded.Message != "Invalid Request" {
		t.Errorf("Message = %q, want Invalid Request", decoded.Message)
	}
}

func TestECPResponse_WithError(t *testing.T) {
	resp := ECPResponse{
		JSONRPC: "2.0",
		Error:   &ECPError{Code: -32601, Message: "Method not found"},
		ID:      float64(1),
	}
	data, err := json.Marshal(resp)
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}

	var decoded ECPResponse
	if err := json.Unmarshal(data, &decoded); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	if decoded.Error == nil {
		t.Fatal("expected non-nil Error")
	}
	if decoded.Error.Code != -32601 {
		t.Errorf("Code = %d, want -32601", decoded.Error.Code)
	}
}

func TestStateUpdateParamsJSONRoundtrip(t *testing.T) {
	in := StateUpdateParams{
		CheckoutID: 42,
		Status:     "ready_for_complete",
		Fields:     map[string]any{"key": "value"},
	}
	data, err := json.Marshal(in)
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}

	var out StateUpdateParams
	if err := json.Unmarshal(data, &out); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	if out.CheckoutID != 42 {
		t.Errorf("CheckoutID = %d, want 42", out.CheckoutID)
	}
	if out.Status != "ready_for_complete" {
		t.Errorf("Status = %q", out.Status)
	}
	if out.Fields == nil {
		t.Fatal("expected non-nil Fields")
	}
	if out.Fields["key"] != "value" {
		t.Errorf("Fields[key] = %v", out.Fields["key"])
	}
}

func TestStateUpdateResultJSONRoundtrip(t *testing.T) {
	in := StateUpdateResult{CheckoutID: 1, Status: "completed", ContinueURL: "https://example.com/continue"}
	data, err := json.Marshal(in)
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}

	var out StateUpdateResult
	if err := json.Unmarshal(data, &out); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	if out.CheckoutID != 1 {
		t.Errorf("CheckoutID = %d", out.CheckoutID)
	}
	if out.ContinueURL != "https://example.com/continue" {
		t.Errorf("ContinueURL = %q", out.ContinueURL)
	}
}

func TestCredentialsSubmitParamsJSONRoundtrip(t *testing.T) {
	in := CredentialsSubmitParams{
		CheckoutID:      1,
		ShippingLine1:   "123 Main St",
		ShippingCity:    "Portland",
		ShippingState:   "OR",
		ShippingPostal:  "97201",
		ShippingCountry: "US",
		BillingLine1:    "456 Oak Ave",
		BillingCity:     "New York",
		BillingState:    "NY",
		BillingPostal:   "10001",
		BillingCountry:  "US",
	}
	data, err := json.Marshal(in)
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}

	var out CredentialsSubmitParams
	if err := json.Unmarshal(data, &out); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	if out.CheckoutID != 1 {
		t.Errorf("CheckoutID = %d", out.CheckoutID)
	}
	if out.ShippingLine1 != "123 Main St" {
		t.Errorf("ShippingLine1 = %q", out.ShippingLine1)
	}
	if out.BillingCity != "New York" {
		t.Errorf("BillingCity = %q", out.BillingCity)
	}
}

func TestCredentialsSubmitParams_OptionalFields(t *testing.T) {
	in := CredentialsSubmitParams{
		CheckoutID:      1,
		ShippingLine1:   "123 Main St",
		ShippingCity:    "Portland",
		ShippingState:   "OR",
		ShippingPostal:  "97201",
		ShippingCountry: "US",
	}
	data, err := json.Marshal(in)
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}

	var out CredentialsSubmitParams
	if err := json.Unmarshal(data, &out); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	if out.ShippingLine2 != "" {
		t.Errorf("expected empty ShippingLine2, got %q", out.ShippingLine2)
	}
	if out.BillingLine2 != "" {
		t.Errorf("expected empty BillingLine2, got %q", out.BillingLine2)
	}
}

func TestCredentialsSubmitResultJSONRoundtrip(t *testing.T) {
	in := CredentialsSubmitResult{CheckoutID: 1, Status: "ready_for_complete"}
	data, err := json.Marshal(in)
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}

	var out CredentialsSubmitResult
	if err := json.Unmarshal(data, &out); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	if out.Status != "ready_for_complete" {
		t.Errorf("Status = %q", out.Status)
	}
}

func TestPaymentAuthorizeParamsJSONRoundtrip(t *testing.T) {
	in := PaymentAuthorizeParams{
		CheckoutID: 1,
		MandateID:  "mand_123",
		Signature:  "sig_abc",
	}
	data, err := json.Marshal(in)
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}

	var out PaymentAuthorizeParams
	if err := json.Unmarshal(data, &out); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	if out.MandateID != "mand_123" {
		t.Errorf("MandateID = %q", out.MandateID)
	}
	if out.Signature != "sig_abc" {
		t.Errorf("Signature = %q", out.Signature)
	}
}

func TestPaymentAuthorizeResultJSONRoundtrip(t *testing.T) {
	in := PaymentAuthorizeResult{CheckoutID: 1, Status: "authorized", Token: "tok_xyz"}
	data, err := json.Marshal(in)
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}

	var out PaymentAuthorizeResult
	if err := json.Unmarshal(data, &out); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	if out.Token != "tok_xyz" {
		t.Errorf("Token = %q", out.Token)
	}
}

func TestAddressSelectParamsJSONRoundtrip(t *testing.T) {
	in := AddressSelectParams{
		CheckoutID:  1,
		AddressType: "shipping",
		Line1:       "789 Pine St",
		City:        "Seattle",
		State:       "WA",
		PostalCode:  "98101",
		Country:     "US",
	}
	data, err := json.Marshal(in)
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}

	var out AddressSelectParams
	if err := json.Unmarshal(data, &out); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	if out.AddressType != "shipping" {
		t.Errorf("AddressType = %q", out.AddressType)
	}
	if out.Line1 != "789 Pine St" {
		t.Errorf("Line1 = %q", out.Line1)
	}
}

func TestAddressSelectResultJSONRoundtrip(t *testing.T) {
	in := AddressSelectResult{
		CheckoutID:  1,
		AddressType: "billing",
		Status:      "selected",
	}
	data, err := json.Marshal(in)
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}

	var out AddressSelectResult
	if err := json.Unmarshal(data, &out); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	if out.Status != "selected" {
		t.Errorf("Status = %q", out.Status)
	}
}
