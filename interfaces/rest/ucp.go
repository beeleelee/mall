package rest

import (
	"encoding/json"
	"net/http"
)

type UCPVersion string

const (
	UCPVersion1_0 UCPVersion = "1.0"
)

type Profile struct {
	UCPVersion     UCPVersion            `json:"ucp_version"`
	ProfileVersion string                `json:"profile_version"`
	Merchant       MerchantInfo          `json:"merchant"`
	Capabilities   map[string]Capability `json:"capabilities"`
	Authentication AuthenticationInfo    `json:"authentication"`
}

type MerchantInfo struct {
	Name        string `json:"name"`
	Description string `json:"description"`
}

type Capability struct {
	Version  string             `json:"version"`
	Bindings CapabilityBindings `json:"bindings"`
}

type CapabilityBindings struct {
	MCP  *MCPBinding  `json:"mcp,omitempty"`
	REST *RESTBinding `json:"rest,omitempty"`
}

type MCPBinding struct {
	Tools         []string `json:"tools"`
	TransportType string   `json:"transport_type,omitempty"`
	Endpoint      string   `json:"endpoint,omitempty"`
}

type RESTBinding struct {
	BaseURL   string            `json:"base_url"`
	Endpoints map[string]string `json:"endpoints"`
}

type AuthenticationInfo struct {
	OAuth2 OAuth2Config `json:"oauth2"`
}

type OAuth2Config struct {
	AuthorizationURL string `json:"authorization_url"`
	TokenURL         string `json:"token_url"`
}

func DefaultProfile() *Profile {
	return &Profile{
		UCPVersion:     UCPVersion1_0,
		ProfileVersion: "1.0.0",
		Merchant: MerchantInfo{
			Name:        "Mall",
			Description: "A UCP-native e-commerce platform for the agentic commerce era",
		},
		Capabilities: map[string]Capability{
			"dev.ucp.shopping.catalog": {
				Version: "1.0.0",
				Bindings: CapabilityBindings{
					MCP: &MCPBinding{
						Tools:         []string{"search_catalog", "lookup_catalog", "get_product"},
						TransportType: "json-rpc-2.0",
						Endpoint:      "/mcp",
					},
					REST: &RESTBinding{
						BaseURL: "/api/v1/catalog",
						Endpoints: map[string]string{
							"search":      "GET /search",
							"lookup":      "GET /lookup",
							"get_product": "GET /products/{id}",
						},
					},
				},
			},
			"dev.ucp.shopping.cart": {
				Version: "1.0.0",
				Bindings: CapabilityBindings{
					MCP: &MCPBinding{
						Tools:         []string{"get_cart", "add_cart_item", "update_cart_item", "remove_cart_item", "clear_cart"},
						TransportType: "json-rpc-2.0",
						Endpoint:      "/mcp",
					},
					REST: &RESTBinding{
						BaseURL: "/api/v1/carts",
						Endpoints: map[string]string{
							"create_or_get": "POST /",
							"get":           "GET /{id}",
							"add_item":      "POST /{id}/items",
							"update_qty":    "PUT /{id}/items/{productId}",
							"remove_item":   "DELETE /{id}/items/{productId}",
							"clear":         "DELETE /{id}",
						},
					},
				},
			},
			"dev.ucp.shopping.checkout": {
				Version: "1.0.0",
				Bindings: CapabilityBindings{
					MCP: &MCPBinding{
						Tools:         []string{"create_checkout", "get_checkout", "set_shipping_address", "set_billing_address", "select_shipping_option", "select_payment_handler", "select_mandate", "complete_checkout", "cancel_checkout"},
						TransportType: "json-rpc-2.0",
						Endpoint:      "/mcp",
					},
					REST: &RESTBinding{
						BaseURL: "/api/v1/checkouts",
						Endpoints: map[string]string{
							"create":               "POST /",
							"get":                  "GET /{id}",
							"set_shipping_address": "POST /{id}/shipping-address",
							"set_billing_address":  "POST /{id}/billing-address",
							"select_shipping":      "POST /{id}/shipping-option",
							"select_payment":       "POST /{id}/payment-handler",
							"select_mandate":       "POST /{id}/mandate",
							"complete":             "POST /{id}/complete",
							"cancel":               "POST /{id}/cancel",
						},
					},
				},
			},
			"dev.ucp.shopping.order": {
				Version: "1.0.0",
				Bindings: CapabilityBindings{
					MCP: &MCPBinding{
						Tools:         []string{"list_orders", "get_order", "process_order", "ship_order", "deliver_order", "return_order", "cancel_order"},
						TransportType: "json-rpc-2.0",
						Endpoint:      "/mcp",
					},
					REST: &RESTBinding{
						BaseURL: "/api/v1/orders",
						Endpoints: map[string]string{
							"get":       "GET /{id}",
							"list_user": "GET /",
						},
					},
				},
			},
			"dev.ucp.shopping.ecp": {
				Version: "1.0.0",
				Bindings: CapabilityBindings{
					REST: &RESTBinding{
						BaseURL: "/api/v1/checkouts",
						Endpoints: map[string]string{
							"complete": "POST /{id}/complete",
						},
					},
				},
			},
			"dev.ucp.shopping.ap2_mandate": {
				Version: "1.0.0",
				Bindings: CapabilityBindings{
					MCP: &MCPBinding{
						Tools:         []string{"create_mandate", "list_mandates", "get_mandate", "approve_mandate", "execute_mandate", "settle_mandate", "cancel_mandate"},
						TransportType: "json-rpc-2.0",
						Endpoint:      "/mcp",
					},
					REST: &RESTBinding{
						BaseURL: "/api/v1/payments",
						Endpoints: map[string]string{
							"request": "POST /mandates",
							"approve": "POST /mandates/{id}/approve",
							"execute": "POST /mandates/{id}/execute",
							"settle":  "POST /mandates/{id}/settle",
							"cancel":  "POST /mandates/{id}/cancel",
							"get":     "GET /mandates/{id}",
							"list":    "GET /mandates",
						},
					},
				},
			},
			"dev.ucp.shopping.fulfillment": {
				Version: "1.0.0",
				Bindings: CapabilityBindings{
					MCP: &MCPBinding{
						Tools:         []string{"calculate_rates"},
						TransportType: "json-rpc-2.0",
						Endpoint:      "/mcp",
					},
					REST: &RESTBinding{
						BaseURL: "/api/v1/fulfillment",
						Endpoints: map[string]string{
							"rates": "POST /rates",
						},
					},
				},
			},
			"dev.ucp.shopping.inventory": {
				Version: "1.0.0",
				Bindings: CapabilityBindings{
					MCP: &MCPBinding{
						Tools:         []string{"set_stock", "get_stock", "list_low_stock"},
						TransportType: "json-rpc-2.0",
						Endpoint:      "/mcp",
					},
					REST: &RESTBinding{
						BaseURL: "/api/v1/admin/inventory",
						Endpoints: map[string]string{
							"set_stock": "POST /",
							"get_stock": "GET /{productId}",
							"low_stock": "GET /low-stock",
						},
					},
				},
			},
			"dev.ucp.shopping.discount": {
				Version: "1.0.0",
				Bindings: CapabilityBindings{
					MCP: &MCPBinding{
						Tools:         []string{"create_discount_code", "validate_discount_code", "apply_discount_code", "deactivate_discount_code"},
						TransportType: "json-rpc-2.0",
						Endpoint:      "/mcp",
					},
					REST: &RESTBinding{
						BaseURL: "/api/v1/discounts",
						Endpoints: map[string]string{
							"create":   "POST /codes",
							"validate": "POST /codes/validate",
							"apply":    "POST /codes/apply",
						},
					},
				},
			},
			"dev.ucp.shopping.identity": {
				Version: "1.0.0",
				Bindings: CapabilityBindings{
					MCP: &MCPBinding{
						Tools:         []string{"register_user", "login_user", "get_user", "suspend_user"},
						TransportType: "json-rpc-2.0",
						Endpoint:      "/mcp",
					},
					REST: &RESTBinding{
						BaseURL: "/api/v1/auth",
						Endpoints: map[string]string{
							"register": "POST /register",
							"login":    "POST /login",
							"get_user": "GET /users/{id}",
							"suspend":  "POST /users/{id}/suspend",
						},
					},
				},
			},
			"dev.ucp.shopping.webhook": {
				Version: "1.0.0",
				Bindings: CapabilityBindings{
					MCP: &MCPBinding{
						Tools:         []string{"register_webhook", "list_webhooks", "delete_webhook"},
						TransportType: "json-rpc-2.0",
						Endpoint:      "/mcp",
					},
					REST: &RESTBinding{
						BaseURL: "/api/v1/webhooks",
						Endpoints: map[string]string{
							"register": "POST /",
							"list":     "GET /",
							"delete":   "DELETE /{id}",
						},
					},
				},
			},
			"dev.ucp.shopping.oauth": {
				Version: "1.0.0",
				Bindings: CapabilityBindings{
					MCP: &MCPBinding{
						Tools:         []string{"authorize", "token", "revoke"},
						TransportType: "json-rpc-2.0",
						Endpoint:      "/mcp",
					},
					REST: &RESTBinding{
						BaseURL: "/oauth",
						Endpoints: map[string]string{
							"authorize": "POST /authorize",
							"token":     "POST /token",
							"revoke":    "POST /revoke",
						},
					},
				},
			},
			"dev.ucp.shopping.admin": {
				Version: "1.0.0",
				Bindings: CapabilityBindings{
					MCP: &MCPBinding{
						Tools:         []string{"create_product", "update_product", "delete_product", "list_all_orders", "list_users", "activate_user", "set_stock", "get_stock", "list_low_stock"},
						TransportType: "json-rpc-2.0",
						Endpoint:      "/mcp",
					},
					REST: &RESTBinding{
						BaseURL: "/api/v1/admin",
						Endpoints: map[string]string{
							"create_product": "POST /products",
							"update_product": "PUT /products/{id}",
							"delete_product": "DELETE /products/{id}",
							"list_orders":    "GET /orders",
							"list_users":     "GET /users",
							"activate_user":  "POST /users/{id}/activate",
							"set_stock":      "POST /inventory",
							"get_stock":      "GET /inventory/{productId}",
							"list_low_stock": "GET /inventory/low-stock",
						},
					},
				},
			},
		},
		Authentication: AuthenticationInfo{
			OAuth2: OAuth2Config{
				AuthorizationURL: "/oauth/authorize",
				TokenURL:         "/oauth/token",
			},
		},
	}
}

type UCPHandler struct {
	profile *Profile
}

func NewUCPHandler(profile *Profile) *UCPHandler {
	if profile == nil {
		profile = DefaultProfile()
	}
	return &UCPHandler{profile: profile}
}

func (h *UCPHandler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	json.NewEncoder(w).Encode(h.profile)
}
