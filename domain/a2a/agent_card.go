package a2a

type AgentCard struct {
	Name               string                    `json:"name"`
	Description        string                    `json:"description"`
	URL                string                    `json:"url"`
	Provider           AgentProvider             `json:"provider"`
	Version            string                    `json:"version"`
	Capabilities       AgentCapabilities         `json:"capabilities"`
	Skills             []AgentSkill              `json:"skills"`
	Interfaces         []AgentInterface          `json:"interfaces"`
	SecuritySchemes    map[string]SecurityScheme `json:"securitySchemes,omitempty"`
	DefaultInputModes  []string                  `json:"defaultInputModes"`
	DefaultOutputModes []string                  `json:"defaultOutputModes"`
}

type AgentProvider struct {
	Organization string `json:"organization"`
	URL          string `json:"url"`
}

type AgentCapabilities struct {
	Streaming         bool `json:"streaming"`
	PushNotifications bool `json:"pushNotifications"`
	ExtendedAgentCard bool `json:"extendedAgentCard"`
}

type AgentSkill struct {
	ID          string   `json:"id"`
	Name        string   `json:"name"`
	Description string   `json:"description"`
	Tags        []string `json:"tags,omitempty"`
	Examples    []string `json:"examples,omitempty"`
	InputModes  []string `json:"inputModes,omitempty"`
	OutputModes []string `json:"outputModes,omitempty"`
}

type AgentInterface struct {
	Type    string `json:"type"`
	URL     string `json:"url"`
	Version string `json:"version,omitempty"`
}

type SecurityScheme struct {
	Type         string `json:"type"`
	Scheme       string `json:"scheme,omitempty"`
	BearerFormat string `json:"bearerFormat,omitempty"`
}

func DefaultAgentCard(baseURL string) *AgentCard {
	return &AgentCard{
		Name:        "Mall E-Commerce Agent",
		Description: "A UCP-native e-commerce agent that can search products, manage carts, handle checkout, process orders, and more.",
		URL:         baseURL,
		Provider: AgentProvider{
			Organization: "Mall",
			URL:          baseURL,
		},
		Version: "1.0.0",
		Capabilities: AgentCapabilities{
			Streaming:         true,
			PushNotifications: true,
			ExtendedAgentCard: true,
		},
		Skills: []AgentSkill{
			{
				ID:          "catalog",
				Name:        "Catalog Search & Discovery",
				Description: "Search products, look up by SKU, get product details",
				Tags:        []string{"catalog", "products", "search"},
				Examples:    []string{"Search for running shoes", "Get product details for product 123", "Look up item by SKU ABC-123"},
				InputModes:  []string{"text"},
				OutputModes: []string{"text"},
			},
			{
				ID:          "cart",
				Name:        "Shopping Cart Management",
				Description: "Create carts, add/update/remove items, view cart contents",
				Tags:        []string{"cart", "shopping"},
				Examples:    []string{"Add 2 running shoes to my cart", "Show me my cart", "Remove item from cart"},
				InputModes:  []string{"text"},
				OutputModes: []string{"text"},
			},
			{
				ID:          "checkout",
				Name:        "Checkout & Purchase",
				Description: "Create checkout sessions, set addresses, select shipping and payment, complete purchase",
				Tags:        []string{"checkout", "purchase", "payment"},
				Examples:    []string{"Checkout my cart", "Set shipping address", "Complete purchase"},
				InputModes:  []string{"text"},
				OutputModes: []string{"text"},
			},
			{
				ID:          "order",
				Name:        "Order Management",
				Description: "List orders, view order details, manage order lifecycle",
				Tags:        []string{"orders", "tracking"},
				Examples:    []string{"Show my orders", "What is the status of order 456?"},
				InputModes:  []string{"text"},
				OutputModes: []string{"text"},
			},
			{
				ID:          "identity",
				Name:        "User Identity",
				Description: "Register, login, and manage user accounts",
				Tags:        []string{"identity", "auth", "users"},
				Examples:    []string{"Register a new account", "Login with email and password"},
				InputModes:  []string{"text"},
				OutputModes: []string{"text"},
			},
			{
				ID:          "discount",
				Name:        "Discount Codes",
				Description: "Validate discount codes and apply promotions",
				Tags:        []string{"discount", "promo", "coupon"},
				Examples:    []string{"Is code SAVE10 valid?", "Apply my discount code"},
				InputModes:  []string{"text"},
				OutputModes: []string{"text"},
			},
			{
				ID:          "inventory",
				Name:        "Inventory Status",
				Description: "Check stock levels and low-stock products",
				Tags:        []string{"inventory", "stock"},
				Examples:    []string{"How many units of product 123 are in stock?", "Show low stock items"},
				InputModes:  []string{"text"},
				OutputModes: []string{"text"},
			},
			{
				ID:          "payment",
				Name:        "Payment Mandates",
				Description: "List and manage payment mandates for recurring billing",
				Tags:        []string{"payment", "mandate", "billing"},
				Examples:    []string{"Show my payment mandates", "What mandates do I have?"},
				InputModes:  []string{"text"},
				OutputModes: []string{"text"},
			},
			{
				ID:          "fulfillment",
				Name:        "Shipping & Fulfillment",
				Description: "Calculate shipping rates and fulfillment options",
				Tags:        []string{"shipping", "fulfillment", "rates"},
				Examples:    []string{"What are shipping rates to the US?", "How much is express shipping?"},
				InputModes:  []string{"text"},
				OutputModes: []string{"text"},
			},
			{
				ID:          "review",
				Name:        "Product Reviews",
				Description: "List reviews written by a user or for a product",
				Tags:        []string{"review", "rating"},
				Examples:    []string{"Show my product reviews", "What did users say about product 123?"},
				InputModes:  []string{"text"},
				OutputModes: []string{"text"},
			},
			{
				ID:          "wishlist",
				Name:        "Wishlist",
				Description: "View and manage saved products",
				Tags:        []string{"wishlist", "saved"},
				Examples:    []string{"Show my wishlist", "What's on my wishlist?"},
				InputModes:  []string{"text"},
				OutputModes: []string{"text"},
			},
			{
				ID:          "subscription",
				Name:        "Subscriptions",
				Description: "List and manage recurring subscriptions and plans",
				Tags:        []string{"subscription", "recurring", "plans"},
				Examples:    []string{"Show my subscriptions", "What plans are available?"},
				InputModes:  []string{"text"},
				OutputModes: []string{"text"},
			},
			{
				ID:          "notification",
				Name:        "Notifications",
				Description: "List notifications and check unread counts",
				Tags:        []string{"notification", "alerts"},
				Examples:    []string{"Show my notifications", "How many unread notifications do I have?"},
				InputModes:  []string{"text"},
				OutputModes: []string{"text"},
			},
		},
		Interfaces: []AgentInterface{
			{
				Type:    "json-rpc-2.0",
				URL:     baseURL + "/a2a",
				Version: "1.0.0",
			},
		},
		SecuritySchemes: map[string]SecurityScheme{
			"bearer": {
				Type:         "http",
				Scheme:       "bearer",
				BearerFormat: "JWT",
			},
		},
		DefaultInputModes:  []string{"text"},
		DefaultOutputModes: []string{"text"},
	}
}
