package middleware

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestNegotiationMiddleware_AllSupported(t *testing.T) {
	supported := []string{"dev.ucp.shopping.catalog", "dev.ucp.shopping.cart"}

	handler := NegotiationMiddleware(supported)(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		result, ok := NegotiationFromContext(r.Context())
		if !ok {
			t.Error("expected negotiation result in context")
		}
		if len(result.Intersect) != 2 {
			t.Errorf("expected 2 intersected, got %d", len(result.Intersect))
		}
	}))

	req := httptest.NewRequest(http.MethodGet, "/", nil)
	rec := httptest.NewRecorder()

	handler.ServeHTTP(rec, req)
}

func TestNegotiationMiddleware_WithAgent(t *testing.T) {
	supported := []string{"dev.ucp.shopping.catalog", "dev.ucp.shopping.cart", "dev.ucp.shopping.checkout"}

	handler := NegotiationMiddleware(supported)(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		result, _ := NegotiationFromContext(r.Context())
		if len(result.Intersect) != 1 {
			t.Errorf("expected 1 intersected (catalog), got %d: %v", len(result.Intersect), result.Intersect)
		}
		if result.Intersect[0] != "dev.ucp.shopping.catalog" {
			t.Errorf("expected catalog, got %s", result.Intersect[0])
		}
	}))

	// Apply UCP agent middleware first, then negotiation
	stack := UCPAgentMiddleware(handler)

	req := httptest.NewRequest(http.MethodGet, "/", nil)
	req.Header.Set("UCP-Agent", "TestBot/1.0; dev.ucp.shopping.catalog")
	rec := httptest.NewRecorder()

	stack.ServeHTTP(rec, req)
}

func TestIntersect_EmptyRequested(t *testing.T) {
	supported := map[string]bool{"a": true, "b": true}
	result := intersect(nil, supported)
	// When nothing is requested, all supported capabilities are returned
	if len(result) != 2 {
		t.Errorf("expected 2, got %d", len(result))
	}
}

func TestNegotiationFromContext_Missing(t *testing.T) {
	_, ok := NegotiationFromContext(context.Background())
	if ok {
		t.Error("expected false for missing negotiation result")
	}
}
