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
	Tools []string `json:"tools"`
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
						Tools: []string{"search_catalog", "lookup_catalog", "get_product"},
					},
					REST: &RESTBinding{
						BaseURL: "/api/v1/catalog",
						Endpoints: map[string]string{
							"search": "GET /search",
							"lookup": "GET /lookup",
							"detail": "GET /products/{id}",
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
