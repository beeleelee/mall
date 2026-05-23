package rest

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestUCPHandler_ServeHTTP(t *testing.T) {
	handler := NewUCPHandler(nil)

	req := httptest.NewRequest(http.MethodGet, "/.well-known/ucp", nil)
	rec := httptest.NewRecorder()

	handler.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", rec.Code)
	}

	if ct := rec.Header().Get("Content-Type"); ct != "application/json" {
		t.Fatalf("expected application/json, got %s", ct)
	}

	var profile Profile
	if err := json.NewDecoder(rec.Body).Decode(&profile); err != nil {
		t.Fatalf("failed to decode profile: %v", err)
	}

	if profile.UCPVersion != UCPVersion1_0 {
		t.Errorf("expected UCP version 1.0, got %s", profile.UCPVersion)
	}

	if profile.ProfileVersion != "1.0.0" {
		t.Errorf("expected profile version 1.0.0, got %s", profile.ProfileVersion)
	}

	if profile.Merchant.Name != "Mall" {
		t.Errorf("expected merchant name Mall, got %s", profile.Merchant.Name)
	}

	cat, ok := profile.Capabilities["dev.ucp.shopping.catalog"]
	if !ok {
		t.Fatal("expected dev.ucp.shopping.catalog capability")
	}

	if cat.Version != "1.0.0" {
		t.Errorf("expected catalog version 1.0.0, got %s", cat.Version)
	}

	if cat.Bindings.MCP == nil {
		t.Fatal("expected MCP binding")
	}

	mcpTools := map[string]bool{}
	for _, t := range cat.Bindings.MCP.Tools {
		mcpTools[t] = true
	}

	for _, tool := range []string{"search_catalog", "lookup_catalog", "get_product"} {
		if !mcpTools[tool] {
			t.Errorf("missing MCP tool: %s", tool)
		}
	}

	if cat.Bindings.REST == nil {
		t.Fatal("expected REST binding")
	}

	if cat.Bindings.REST.BaseURL != "/api/v1/catalog" {
		t.Errorf("expected base URL /api/v1/catalog, got %s", cat.Bindings.REST.BaseURL)
	}
}

func TestUCPHandler_MethodNotAllowed(t *testing.T) {
	handler := NewUCPHandler(nil)

	req := httptest.NewRequest(http.MethodPost, "/.well-known/ucp", nil)
	rec := httptest.NewRecorder()

	handler.ServeHTTP(rec, req)

	if rec.Code != http.StatusMethodNotAllowed {
		t.Fatalf("expected 405, got %d", rec.Code)
	}
}

func TestUCPHandler_CustomProfile(t *testing.T) {
	profile := &Profile{
		UCPVersion:     UCPVersion1_0,
		ProfileVersion: "2.0.0",
		Merchant: MerchantInfo{
			Name: "Custom Merchant",
		},
		Capabilities: map[string]Capability{},
		Authentication: AuthenticationInfo{
			OAuth2: OAuth2Config{
				AuthorizationURL: "/custom/auth",
				TokenURL:         "/custom/token",
			},
		},
	}

	handler := NewUCPHandler(profile)

	req := httptest.NewRequest(http.MethodGet, "/.well-known/ucp", nil)
	rec := httptest.NewRecorder()

	handler.ServeHTTP(rec, req)

	var result Profile
	json.NewDecoder(rec.Body).Decode(&result)

	if result.ProfileVersion != "2.0.0" {
		t.Errorf("expected 2.0.0, got %s", result.ProfileVersion)
	}
	if result.Authentication.OAuth2.AuthorizationURL != "/custom/auth" {
		t.Errorf("expected /custom/auth, got %s", result.Authentication.OAuth2.AuthorizationURL)
	}
}
