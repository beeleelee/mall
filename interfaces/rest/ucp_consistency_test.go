package rest

import (
	"testing"

	"github.com/beeleelee/mall/interfaces/mcp"
)

// supportedCaps mirrors the middleware list in main.go so the UCP profile and
// the negotiation middleware stay consistent. Update both when adding a
// capability.
var supportedCaps = []string{
	"dev.ucp.shopping.catalog",
	"dev.ucp.shopping.cart",
	"dev.ucp.shopping.checkout",
	"dev.ucp.shopping.order",
	"dev.ucp.shopping.ecp",
	"dev.ucp.shopping.ap2_mandate",
	"dev.ucp.shopping.payment_token_exchange",
	"dev.ucp.shopping.stripe_payment",
	"dev.ucp.shopping.fulfillment",
	"dev.ucp.shopping.discount",
	"dev.ucp.shopping.identity",
	"dev.ucp.shopping.webhook",
	"dev.ucp.shopping.oauth",
	"dev.ucp.shopping.inventory",
	"dev.ucp.shopping.admin",
	"dev.ucp.shopping.admin.dashboard",
	"dev.ucp.shopping.reviews",
	"dev.ucp.shopping.wishlist",
	"dev.ucp.shopping.subscriptions",
	"dev.ucp.shopping.notifications",
	"dev.a2a.agent",
}

func TestUCPProfile_CapabilityConsistency(t *testing.T) {
	profile := DefaultProfile()

	// Every capability declared in supportedCaps must exist in the profile.
	for _, cap := range supportedCaps {
		if _, ok := profile.Capabilities[cap]; !ok {
			t.Errorf("supportedCaps %q missing from UCP profile", cap)
		}
	}

	// Every profile capability must be present in supportedCaps.
	for cap := range profile.Capabilities {
		found := false
		for _, c := range supportedCaps {
			if c == cap {
				found = true
				break
			}
		}
		if !found {
			t.Errorf("profile capability %q missing from supportedCaps", cap)
		}
	}
}

func TestUCPProfile_CapabilitiesHaveBindings(t *testing.T) {
	profile := DefaultProfile()

	for cap, c := range profile.Capabilities {
		if c.Bindings.REST == nil && c.Bindings.MCP == nil {
			t.Errorf("capability %q has no bindings", cap)
		}
		if c.Bindings.REST != nil && len(c.Bindings.REST.Endpoints) == 0 {
			t.Errorf("capability %q has empty REST binding", cap)
		}
		if c.Bindings.MCP != nil && len(c.Bindings.MCP.Tools) == 0 {
			t.Errorf("capability %q has empty MCP binding", cap)
		}
	}
}

func TestUCPProfile_MCPToolsRegistered(t *testing.T) {
	profile := DefaultProfile()

	providers := []mcp.ToolProvider{
		mcp.NewCatalogMCPHandler(nil),
		mcp.NewCartMCPHandler(nil, nil),
		mcp.NewCheckoutMCPHandler(nil, nil),
		mcp.NewOrderMCPHandler(nil),
		mcp.NewDiscountMCPHandler(nil, nil),
		mcp.NewInventoryMCPHandler(nil, nil),
		mcp.NewPaymentMCPHandler(nil, nil),
		mcp.NewIdentityMCPHandler(nil, nil),
		mcp.NewWebhookMCPHandler(nil),
		mcp.NewFulfillmentMCPHandler(nil),
		mcp.NewOAuthMCPHandler(nil),
		mcp.NewNotificationMCPHandler(nil),
		mcp.NewSubscriptionMCPHandler(nil),
		mcp.NewReviewMCPHandler(nil, nil),
		mcp.NewWishlistMCPHandler(nil),
		mcp.NewAdminMCPHandler(nil, nil, nil, nil, nil, nil),
		mcp.NewAnalyticsMCPHandler(nil, nil),
	}

	registered := map[string]bool{}
	for _, p := range providers {
		for _, tDef := range p.ListTools() {
			registered[tDef.Name] = true
		}
	}

	for cap, c := range profile.Capabilities {
		if c.Bindings.MCP == nil {
			continue
		}
		for _, tool := range c.Bindings.MCP.Tools {
			if !registered[tool] {
				t.Errorf("capability %q declares MCP tool %q that is not registered", cap, tool)
			}
		}
	}
}
