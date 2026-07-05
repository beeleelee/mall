package mcp

import (
	"context"
	"encoding/json"

	catalog "github.com/beeleelee/mall/domain/catalog"
	identity "github.com/beeleelee/mall/domain/identity"
	inventory "github.com/beeleelee/mall/domain/inventory"
	"github.com/beeleelee/mall/domain/kernel"
	order "github.com/beeleelee/mall/domain/order"
)

type AdminMCPHandler struct {
	catalogSvc   *catalog.CatalogService
	orderSvc     *order.OrderService
	identitySvc  *identity.IdentityService
	users        identity.UserRepository
	inventorySvc *inventory.InventoryService
	sf           *kernel.Snowflake
}

func NewAdminMCPHandler(
	catalogSvc *catalog.CatalogService,
	orderSvc *order.OrderService,
	identitySvc *identity.IdentityService,
	users identity.UserRepository,
	inventorySvc *inventory.InventoryService,
	sf *kernel.Snowflake,
) *AdminMCPHandler {
	return &AdminMCPHandler{
		catalogSvc:   catalogSvc,
		orderSvc:     orderSvc,
		identitySvc:  identitySvc,
		users:        users,
		inventorySvc: inventorySvc,
		sf:           sf,
	}
}

var adminTools = []ToolDefinition{
	{
		Name:        "create_product",
		Description: "[Admin] Create a new product",
		InputSchema: InputSchema{
			Type: "object",
			Properties: map[string]PropertySchema{
				"admin_user_id": {Type: "number", Description: "Admin user ID"},
				"sku":           {Type: "string", Description: "Product SKU"},
				"name":          {Type: "string", Description: "Product name"},
				"description":   {Type: "string", Description: "Product description"},
				"category":      {Type: "string", Description: "Product category"},
				"price_amount":  {Type: "number", Description: "Price in cents"},
				"currency":      {Type: "string", Description: "Currency code (e.g. USD)"},
				"attributes":    {Type: "string", Description: "JSON object of custom attributes (optional)"},
			},
		},
	},
	{
		Name:        "update_product",
		Description: "[Admin] Update an existing product",
		InputSchema: InputSchema{
			Type: "object",
			Properties: map[string]PropertySchema{
				"admin_user_id": {Type: "number", Description: "Admin user ID"},
				"id":            {Type: "number", Description: "Product ID"},
				"name":          {Type: "string", Description: "Product name"},
				"description":   {Type: "string", Description: "Product description"},
				"category":      {Type: "string", Description: "Product category"},
				"price_amount":  {Type: "number", Description: "Price in cents"},
				"currency":      {Type: "string", Description: "Currency code (e.g. USD)"},
				"status":        {Type: "string", Description: "Product status (active/inactive, optional)"},
				"attributes":    {Type: "string", Description: "JSON object of custom attributes (optional)"},
			},
		},
	},
	{
		Name:        "delete_product",
		Description: "[Admin] Delete a product",
		InputSchema: InputSchema{
			Type: "object",
			Properties: map[string]PropertySchema{
				"admin_user_id": {Type: "number", Description: "Admin user ID"},
				"id":            {Type: "number", Description: "Product ID"},
			},
		},
	},
	{
		Name:        "list_all_orders",
		Description: "[Admin] List all orders with pagination",
		InputSchema: InputSchema{
			Type: "object",
			Properties: map[string]PropertySchema{
				"admin_user_id": {Type: "number", Description: "Admin user ID"},
				"offset":        {Type: "number", Description: "Pagination offset (optional)"},
				"limit":         {Type: "number", Description: "Page size (optional)"},
			},
		},
	},
	{
		Name:        "list_users",
		Description: "[Admin] List all users with pagination",
		InputSchema: InputSchema{
			Type: "object",
			Properties: map[string]PropertySchema{
				"admin_user_id": {Type: "number", Description: "Admin user ID"},
				"offset":        {Type: "number", Description: "Pagination offset (optional)"},
				"limit":         {Type: "number", Description: "Page size (optional)"},
			},
		},
	},
	{
		Name:        "activate_user",
		Description: "[Admin] Activate a suspended user",
		InputSchema: InputSchema{
			Type: "object",
			Properties: map[string]PropertySchema{
				"admin_user_id": {Type: "number", Description: "Admin user ID"},
				"user_id":       {Type: "number", Description: "User ID to activate"},
			},
		},
	},
	{
		Name:        "set_stock",
		Description: "[Admin] Set inventory stock for a product",
		InputSchema: InputSchema{
			Type: "object",
			Properties: map[string]PropertySchema{
				"admin_user_id":       {Type: "number", Description: "Admin user ID"},
				"product_id":          {Type: "number", Description: "Product ID"},
				"quantity":            {Type: "number", Description: "Stock quantity"},
				"low_stock_threshold": {Type: "number", Description: "Low stock threshold (optional, default 10)"},
			},
		},
	},
	{
		Name:        "get_stock",
		Description: "[Admin] Get inventory stock for a product",
		InputSchema: InputSchema{
			Type: "object",
			Properties: map[string]PropertySchema{
				"admin_user_id": {Type: "number", Description: "Admin user ID"},
				"product_id":    {Type: "number", Description: "Product ID"},
			},
		},
	},
	{
		Name:        "list_low_stock",
		Description: "[Admin] List all products with low stock",
		InputSchema: InputSchema{
			Type: "object",
			Properties: map[string]PropertySchema{
				"admin_user_id": {Type: "number", Description: "Admin user ID"},
				"threshold":     {Type: "number", Description: "Low stock threshold (optional, default 10)"},
			},
		},
	},
}

func (h *AdminMCPHandler) ListTools() []ToolDefinition {
	return adminTools
}

func (h *AdminMCPHandler) HandleTool(ctx context.Context, name string, raw json.RawMessage) (any, error) {
	// All admin tools require an admin_user_id check first
	var adminCheck struct {
		AdminUserID int64 `json:"admin_user_id"`
	}
	if err := json.Unmarshal(raw, &adminCheck); err != nil {
		return nil, kernel.NewDomainError(kernel.ErrInvalidArgument, "invalid arguments")
	}
	if err := h.checkAdmin(ctx, kernel.ID(adminCheck.AdminUserID)); err != nil {
		return nil, err
	}

	switch name {
	case "create_product":
		return h.callCreateProduct(ctx, raw)
	case "update_product":
		return h.callUpdateProduct(ctx, raw)
	case "delete_product":
		return h.callDeleteProduct(ctx, raw)
	case "list_all_orders":
		return h.callListAllOrders(ctx, raw)
	case "list_users":
		return h.callListUsers(ctx, raw)
	case "activate_user":
		return h.callActivateUser(ctx, raw)
	case "set_stock":
		return h.callSetStock(ctx, raw)
	case "get_stock":
		return h.callGetStock(ctx, raw)
	case "list_low_stock":
		return h.callListLowStock(ctx, raw)
	default:
		return nil, kernel.NewDomainError(kernel.ErrInvalidArgument, "unknown tool: "+name)
	}
}

func (h *AdminMCPHandler) checkAdmin(ctx context.Context, userID kernel.ID) error {
	if userID <= 0 {
		return kernel.NewDomainError(kernel.ErrPermissionDenied, "admin_user_id must be positive")
	}
	user, err := h.users.FindByID(ctx, userID)
	if err != nil {
		return kernel.NewDomainError(kernel.ErrPermissionDenied, "admin authentication failed: user not found")
	}
	if !user.HasRole(identity.UserRoleAdmin) {
		return kernel.NewDomainError(kernel.ErrPermissionDenied, "user is not an admin")
	}
	return nil
}

// --- create_product ---

type createProductArgs struct {
	AdminUserID int64  `json:"admin_user_id"`
	SKU         string `json:"sku"`
	Name        string `json:"name"`
	Description string `json:"description"`
	Category    string `json:"category"`
	CategoryID  int64  `json:"category_id,omitempty"`
	PriceAmount int64  `json:"price_amount"`
	Currency    string `json:"currency"`
	Attributes  string `json:"attributes,omitempty"`
}

func (h *AdminMCPHandler) callCreateProduct(ctx context.Context, raw json.RawMessage) (any, error) {
	var args createProductArgs
	if err := json.Unmarshal(raw, &args); err != nil {
		return nil, kernel.NewDomainError(kernel.ErrInvalidArgument, "invalid arguments")
	}
	if args.SKU == "" {
		return nil, kernel.NewDomainError(kernel.ErrInvalidArgument, "sku must not be empty")
	}
	if args.Name == "" {
		return nil, kernel.NewDomainError(kernel.ErrInvalidArgument, "name must not be empty")
	}
	if args.Currency == "" {
		args.Currency = "USD"
	}

	var attrs map[string]any
	if args.Attributes != "" {
		if err := json.Unmarshal([]byte(args.Attributes), &attrs); err != nil {
			return nil, kernel.NewDomainError(kernel.ErrInvalidArgument, "attributes must be valid JSON object")
		}
	}

	id, err := h.sf.NextID()
	if err != nil {
		return nil, err
	}

	product, err := h.catalogSvc.CreateProduct(ctx, id,
		catalog.SKU(args.SKU), args.Name, args.Description, args.Category, kernel.ID(args.CategoryID),
		catalog.Money{Amount: args.PriceAmount, Currency: args.Currency},
		attrs)
	if err != nil {
		return nil, err
	}

	return adminProductToMap(product), nil
}

// --- update_product ---

type updateProductArgs struct {
	AdminUserID int64  `json:"admin_user_id"`
	ID          int64  `json:"id"`
	Name        string `json:"name"`
	Description string `json:"description"`
	Category    string `json:"category"`
	CategoryID  int64  `json:"category_id,omitempty"`
	PriceAmount int64  `json:"price_amount"`
	Currency    string `json:"currency"`
	Status      string `json:"status,omitempty"`
	Attributes  string `json:"attributes,omitempty"`
}

func (h *AdminMCPHandler) callUpdateProduct(ctx context.Context, raw json.RawMessage) (any, error) {
	var args updateProductArgs
	if err := json.Unmarshal(raw, &args); err != nil {
		return nil, kernel.NewDomainError(kernel.ErrInvalidArgument, "invalid arguments")
	}
	if args.ID <= 0 {
		return nil, kernel.NewDomainError(kernel.ErrInvalidArgument, "id must be positive")
	}
	if args.Currency == "" {
		args.Currency = "USD"
	}

	var attrs map[string]any
	if args.Attributes != "" {
		if err := json.Unmarshal([]byte(args.Attributes), &attrs); err != nil {
			return nil, kernel.NewDomainError(kernel.ErrInvalidArgument, "attributes must be valid JSON object")
		}
	}

	product, err := h.catalogSvc.UpdateProduct(ctx, kernel.ID(args.ID),
		args.Name, args.Description, args.Category, kernel.ID(args.CategoryID),
		catalog.Money{Amount: args.PriceAmount, Currency: args.Currency},
		catalog.ProductStatus(args.Status), attrs)
	if err != nil {
		return nil, err
	}

	return adminProductToMap(product), nil
}

// --- delete_product ---

type deleteProductArgs struct {
	AdminUserID int64 `json:"admin_user_id"`
	ID          int64 `json:"id"`
}

func (h *AdminMCPHandler) callDeleteProduct(ctx context.Context, raw json.RawMessage) (any, error) {
	var args deleteProductArgs
	if err := json.Unmarshal(raw, &args); err != nil {
		return nil, kernel.NewDomainError(kernel.ErrInvalidArgument, "invalid arguments")
	}
	if args.ID <= 0 {
		return nil, kernel.NewDomainError(kernel.ErrInvalidArgument, "id must be positive")
	}

	if err := h.catalogSvc.DeleteProduct(ctx, kernel.ID(args.ID)); err != nil {
		return nil, err
	}

	return map[string]string{"status": "deleted"}, nil
}

// --- list_all_orders ---

type listAllOrdersArgs struct {
	AdminUserID int64 `json:"admin_user_id"`
	Offset      int   `json:"offset,omitempty"`
	Limit       int   `json:"limit,omitempty"`
}

func (h *AdminMCPHandler) callListAllOrders(ctx context.Context, raw json.RawMessage) (any, error) {
	var args listAllOrdersArgs
	if err := json.Unmarshal(raw, &args); err != nil {
		return nil, kernel.NewDomainError(kernel.ErrInvalidArgument, "invalid arguments")
	}

	orders, err := h.orderSvc.ListAllOrders(ctx, args.Offset, args.Limit)
	if err != nil {
		return nil, err
	}

	result := make([]map[string]any, len(orders))
	for i, o := range orders {
		result[i] = orderToMap(o)
	}
	return result, nil
}

// --- list_users ---

type listUsersArgs struct {
	AdminUserID int64 `json:"admin_user_id"`
	Offset      int   `json:"offset,omitempty"`
	Limit       int   `json:"limit,omitempty"`
}

func (h *AdminMCPHandler) callListUsers(ctx context.Context, raw json.RawMessage) (any, error) {
	var args listUsersArgs
	if err := json.Unmarshal(raw, &args); err != nil {
		return nil, kernel.NewDomainError(kernel.ErrInvalidArgument, "invalid arguments")
	}

	users, err := h.identitySvc.ListUsers(ctx, args.Offset, args.Limit)
	if err != nil {
		return nil, err
	}

	result := make([]map[string]any, len(users))
	for i, u := range users {
		result[i] = adminUserToMap(u)
	}
	return result, nil
}

// --- activate_user ---

type activateUserArgs struct {
	AdminUserID int64 `json:"admin_user_id"`
	UserID      int64 `json:"user_id"`
}

func (h *AdminMCPHandler) callActivateUser(ctx context.Context, raw json.RawMessage) (any, error) {
	var args activateUserArgs
	if err := json.Unmarshal(raw, &args); err != nil {
		return nil, kernel.NewDomainError(kernel.ErrInvalidArgument, "invalid arguments")
	}
	if args.UserID <= 0 {
		return nil, kernel.NewDomainError(kernel.ErrInvalidArgument, "user_id must be positive")
	}

	user, err := h.identitySvc.ActivateUser(ctx, kernel.ID(args.UserID))
	if err != nil {
		return nil, err
	}

	return adminUserToMap(user), nil
}

// --- set_stock ---

type adminSetStockArgs struct {
	AdminUserID       int64 `json:"admin_user_id"`
	ProductID         int64 `json:"product_id"`
	Quantity          int   `json:"quantity"`
	LowStockThreshold int   `json:"low_stock_threshold,omitempty"`
}

func (h *AdminMCPHandler) callSetStock(ctx context.Context, raw json.RawMessage) (any, error) {
	var args adminSetStockArgs
	if err := json.Unmarshal(raw, &args); err != nil {
		return nil, kernel.NewDomainError(kernel.ErrInvalidArgument, "invalid arguments")
	}
	if args.ProductID <= 0 {
		return nil, kernel.NewDomainError(kernel.ErrInvalidArgument, "product_id must be positive")
	}

	// Try to update existing stock first
	existing, err := h.inventorySvc.GetStock(ctx, kernel.ID(args.ProductID))
	if err == nil && existing != nil {
		item, err := h.inventorySvc.UpdateStock(ctx, kernel.ID(args.ProductID), args.Quantity)
		if err != nil {
			return nil, err
		}
		return inventoryItemToMap(item), nil
	}

	// Create new stock entry
	id, err := h.sf.NextID()
	if err != nil {
		return nil, err
	}

	threshold := args.LowStockThreshold
	if threshold <= 0 {
		threshold = 10
	}

	item, err := h.inventorySvc.SetStock(ctx, id, kernel.ID(args.ProductID), args.Quantity, threshold)
	if err != nil {
		return nil, err
	}

	return inventoryItemToMap(item), nil
}

// --- get_stock ---

type adminGetStockArgs struct {
	AdminUserID int64 `json:"admin_user_id"`
	ProductID   int64 `json:"product_id"`
}

func (h *AdminMCPHandler) callGetStock(ctx context.Context, raw json.RawMessage) (any, error) {
	var args adminGetStockArgs
	if err := json.Unmarshal(raw, &args); err != nil {
		return nil, kernel.NewDomainError(kernel.ErrInvalidArgument, "invalid arguments")
	}
	if args.ProductID <= 0 {
		return nil, kernel.NewDomainError(kernel.ErrInvalidArgument, "product_id must be positive")
	}

	item, err := h.inventorySvc.GetStock(ctx, kernel.ID(args.ProductID))
	if err != nil {
		return nil, err
	}

	return inventoryItemToMap(item), nil
}

// --- list_low_stock ---

type adminListLowStockArgs struct {
	AdminUserID int64 `json:"admin_user_id"`
	Threshold   int   `json:"threshold,omitempty"`
}

func (h *AdminMCPHandler) callListLowStock(ctx context.Context, raw json.RawMessage) (any, error) {
	var args adminListLowStockArgs
	if err := json.Unmarshal(raw, &args); err != nil {
		return nil, kernel.NewDomainError(kernel.ErrInvalidArgument, "invalid arguments")
	}

	items, err := h.inventorySvc.ListLowStock(ctx, args.Threshold)
	if err != nil {
		return nil, err
	}

	result := make([]map[string]any, len(items))
	for i, item := range items {
		result[i] = inventoryItemToMap(item)
	}
	return result, nil
}

// --- response helpers ---

func adminProductToMap(p *catalog.Product) map[string]any {
	return map[string]any{
		"id":          p.ID.Int64(),
		"sku":         string(p.SKU),
		"name":        p.Name,
		"description": p.Description,
		"category":    p.Category,
		"category_id": p.CategoryID.Int64(),
		"price":       p.Price.Amount,
		"currency":    p.Price.Currency,
		"status":      string(p.Status),
		"attributes":  p.Attributes,
	}
}

func adminUserToMap(u *identity.User) map[string]any {
	roles := make([]string, len(u.Roles))
	for i, r := range u.Roles {
		roles[i] = string(r)
	}
	return map[string]any{
		"id":     u.ID.Int64(),
		"email":  u.Email,
		"name":   u.Name,
		"status": string(u.Status),
		"roles":  roles,
	}
}
