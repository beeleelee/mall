package middleware

import (
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestParseUCPAgent_Full(t *testing.T) {
	info := parseUCPAgent("MyAgent/1.0; dev.ucp.shopping.catalog, dev.ucp.shopping.cart")

	if info.Name != "MyAgent" {
		t.Errorf("expected MyAgent, got %s", info.Name)
	}
	if info.Version != "1.0" {
		t.Errorf("expected 1.0, got %s", info.Version)
	}
	if len(info.Capabilities) != 2 {
		t.Fatalf("expected 2 capabilities, got %d", len(info.Capabilities))
	}
	if info.Capabilities[0] != "dev.ucp.shopping.catalog" {
		t.Errorf("expected catalog, got %s", info.Capabilities[0])
	}
}

func TestParseUCPAgent_NameOnly(t *testing.T) {
	info := parseUCPAgent("MyAgent")
	if info.Name != "MyAgent" {
		t.Errorf("expected MyAgent, got %s", info.Name)
	}
	if info.Version != "" {
		t.Errorf("expected empty version, got %s", info.Version)
	}
}

func TestParseUCPAgent_Empty(t *testing.T) {
	info := parseUCPAgent("")
	if info.Name != "" {
		t.Errorf("expected empty name, got %s", info.Name)
	}
}

func TestUCPAgentMiddleware(t *testing.T) {
	handler := UCPAgentMiddleware(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		info, ok := AgentFromContext(r.Context())
		if !ok {
			t.Error("expected agent info in context")
		}
		if info.Name != "TestBot" {
			t.Errorf("expected TestBot, got %s", info.Name)
		}
	}))

	req := httptest.NewRequest(http.MethodGet, "/", nil)
	req.Header.Set("UCP-Agent", "TestBot/2.0; dev.ucp.shopping.catalog")
	rec := httptest.NewRecorder()

	handler.ServeHTTP(rec, req)
}

func TestUCPAgentMiddleware_NoHeader(t *testing.T) {
	handler := UCPAgentMiddleware(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, ok := AgentFromContext(r.Context())
		if ok {
			t.Error("expected no agent info when header is missing")
		}
	}))

	req := httptest.NewRequest(http.MethodGet, "/", nil)
	rec := httptest.NewRecorder()

	handler.ServeHTTP(rec, req)
}
