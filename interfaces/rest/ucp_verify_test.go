package rest

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestUCPProfile_Structure(t *testing.T) {
	handler := NewUCPHandler(nil)

	r := httptest.NewRequest(http.MethodGet, "/.well-known/ucp", nil)
	w := httptest.NewRecorder()
	handler.ServeHTTP(w, r)

	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", w.Code)
	}

	var profile Profile
	if err := json.NewDecoder(w.Body).Decode(&profile); err != nil {
		t.Fatalf("expected valid JSON: %v", err)
	}

	if profile.UCPVersion != UCPVersion1_0 {
		t.Errorf("expected ucp_version 1.0, got %s", profile.UCPVersion)
	}
	if profile.ProfileVersion == "" {
		t.Error("profile_version must not be empty")
	}
	if profile.Merchant.Name == "" {
		t.Error("merchant.name must not be empty")
	}
	if len(profile.Capabilities) == 0 {
		t.Error("at least one capability must be declared")
	}
}

func TestUCPProfile_RequiredCapabilities(t *testing.T) {
	handler := NewUCPHandler(nil)

	r := httptest.NewRequest(http.MethodGet, "/.well-known/ucp", nil)
	w := httptest.NewRecorder()
	handler.ServeHTTP(w, r)

	var profile Profile
	json.NewDecoder(w.Body).Decode(&profile)

	required := []string{
		"dev.ucp.shopping.catalog",
		"dev.ucp.shopping.cart",
		"dev.ucp.shopping.checkout",
		"dev.ucp.shopping.order",
		"dev.a2a.agent",
	}
	for _, cap := range required {
		if _, ok := profile.Capabilities[cap]; !ok {
			t.Errorf("required capability %q not found in profile", cap)
		}
	}
}

func TestUCPProfile_CapabilityBindings(t *testing.T) {
	handler := NewUCPHandler(nil)

	r := httptest.NewRequest(http.MethodGet, "/.well-known/ucp", nil)
	w := httptest.NewRecorder()
	handler.ServeHTTP(w, r)

	var profile Profile
	json.NewDecoder(w.Body).Decode(&profile)

	for name, cap := range profile.Capabilities {
		if cap.Bindings.MCP == nil && cap.Bindings.REST == nil {
			t.Errorf("capability %q has no MCP or REST bindings", name)
		}
		if cap.Bindings.MCP != nil && len(cap.Bindings.MCP.Tools) == 0 {
			t.Errorf("capability %q MCP binding has no tools", name)
		}
		if cap.Version == "" {
			t.Errorf("capability %q has no version", name)
		}
	}
}

func TestUCPProfile_Authentication(t *testing.T) {
	handler := NewUCPHandler(nil)

	r := httptest.NewRequest(http.MethodGet, "/.well-known/ucp", nil)
	w := httptest.NewRecorder()
	handler.ServeHTTP(w, r)

	var profile Profile
	json.NewDecoder(w.Body).Decode(&profile)

	if profile.Authentication.OAuth2.AuthorizationURL == "" {
		t.Error("authentication.oauth2.authorization_url must not be empty")
	}
	if profile.Authentication.OAuth2.TokenURL == "" {
		t.Error("authentication.oauth2.token_url must not be empty")
	}
}

func TestUCPProfile_JSONSerialization(t *testing.T) {
	handler := NewUCPHandler(nil)

	r := httptest.NewRequest(http.MethodGet, "/.well-known/ucp", nil)
	w := httptest.NewRecorder()
	handler.ServeHTTP(w, r)

	var raw any
	if err := json.NewDecoder(w.Body).Decode(&raw); err != nil {
		t.Fatalf("profile must be valid JSON: %v", err)
	}

	rawMap, ok := raw.(map[string]any)
	if !ok {
		t.Fatal("profile must be a JSON object")
	}

	if _, ok := rawMap["ucp_version"]; !ok {
		t.Error("profile missing ucp_version")
	}
	if _, ok := rawMap["capabilities"]; !ok {
		t.Error("profile missing capabilities")
	}
	if _, ok := rawMap["merchant"]; !ok {
		t.Error("profile missing merchant")
	}
	if _, ok := rawMap["authentication"]; !ok {
		t.Error("profile missing authentication")
	}
}
