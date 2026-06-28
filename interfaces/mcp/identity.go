package mcp

import (
	"context"
	"encoding/json"

	"github.com/beeleelee/mall/domain/kernel"

	domain "github.com/beeleelee/mall/domain/identity"
)

type IdentityMCPHandler struct {
	svc *domain.IdentityService
	sf  *kernel.Snowflake
}

func NewIdentityMCPHandler(svc *domain.IdentityService, sf *kernel.Snowflake) *IdentityMCPHandler {
	return &IdentityMCPHandler{svc: svc, sf: sf}
}

var identityTools = []ToolDefinition{
	{
		Name:        "register_user",
		Description: "Register a new user account",
		InputSchema: InputSchema{
			Type: "object",
			Properties: map[string]PropertySchema{
				"email":    {Type: "string", Description: "Email address"},
				"password": {Type: "string", Description: "Password (min 8 characters)"},
				"name":     {Type: "string", Description: "Display name"},
			},
		},
	},
	{
		Name:        "login_user",
		Description: "Authenticate a user and return user info",
		InputSchema: InputSchema{
			Type: "object",
			Properties: map[string]PropertySchema{
				"email":    {Type: "string", Description: "Email address"},
				"password": {Type: "string", Description: "Password"},
			},
		},
	},
	{
		Name:        "get_user",
		Description: "Get user information by ID",
		InputSchema: InputSchema{
			Type: "object",
			Properties: map[string]PropertySchema{
				"id": {Type: "number", Description: "User ID"},
			},
		},
	},
	{
		Name:        "suspend_user",
		Description: "Suspend a user account",
		InputSchema: InputSchema{
			Type: "object",
			Properties: map[string]PropertySchema{
				"id": {Type: "number", Description: "User ID"},
			},
		},
	},
}

func (h *IdentityMCPHandler) ListTools() []ToolDefinition {
	return identityTools
}

func (h *IdentityMCPHandler) HandleTool(ctx context.Context, name string, raw json.RawMessage) (any, error) {
	switch name {
	case "register_user":
		return h.callRegister(ctx, raw)
	case "login_user":
		return h.callLogin(ctx, raw)
	case "get_user":
		return h.callGetUser(ctx, raw)
	case "suspend_user":
		return h.callSuspendUser(ctx, raw)
	default:
		return nil, kernel.NewDomainError(kernel.ErrInvalidArgument, "unknown tool: "+name)
	}
}

type registerUserArgs struct {
	Email    string `json:"email"`
	Password string `json:"password"`
	Name     string `json:"name"`
}

func (h *IdentityMCPHandler) callRegister(ctx context.Context, raw json.RawMessage) (any, error) {
	var args registerUserArgs
	if err := json.Unmarshal(raw, &args); err != nil {
		return nil, kernel.NewDomainError(kernel.ErrInvalidArgument, "invalid arguments")
	}
	if args.Email == "" {
		return nil, kernel.NewDomainError(kernel.ErrInvalidArgument, "email must not be empty")
	}
	if args.Password == "" {
		return nil, kernel.NewDomainError(kernel.ErrInvalidArgument, "password must not be empty")
	}
	if args.Name == "" {
		return nil, kernel.NewDomainError(kernel.ErrInvalidArgument, "name must not be empty")
	}

	id, err := h.sf.NextID()
	if err != nil {
		return nil, err
	}

	user, err := h.svc.Register(ctx, id, args.Email, args.Name, args.Password)
	if err != nil {
		return nil, err
	}

	return userToMap(user), nil
}

type loginUserArgs struct {
	Email    string `json:"email"`
	Password string `json:"password"`
}

func (h *IdentityMCPHandler) callLogin(ctx context.Context, raw json.RawMessage) (any, error) {
	var args loginUserArgs
	if err := json.Unmarshal(raw, &args); err != nil {
		return nil, kernel.NewDomainError(kernel.ErrInvalidArgument, "invalid arguments")
	}
	if args.Email == "" {
		return nil, kernel.NewDomainError(kernel.ErrInvalidArgument, "email must not be empty")
	}
	if args.Password == "" {
		return nil, kernel.NewDomainError(kernel.ErrInvalidArgument, "password must not be empty")
	}

	user, err := h.svc.Login(ctx, args.Email, args.Password)
	if err != nil {
		return nil, err
	}

	return userToMap(user), nil
}

type getUserArgs struct {
	ID int64 `json:"id"`
}

func (h *IdentityMCPHandler) callGetUser(ctx context.Context, raw json.RawMessage) (any, error) {
	var args getUserArgs
	if err := json.Unmarshal(raw, &args); err != nil {
		return nil, kernel.NewDomainError(kernel.ErrInvalidArgument, "invalid arguments")
	}
	if args.ID <= 0 {
		return nil, kernel.NewDomainError(kernel.ErrInvalidArgument, "id must be positive")
	}

	user, err := h.svc.GetUser(ctx, kernel.ID(args.ID))
	if err != nil {
		return nil, err
	}

	return userToMap(user), nil
}

type suspendUserArgs struct {
	ID int64 `json:"id"`
}

func (h *IdentityMCPHandler) callSuspendUser(ctx context.Context, raw json.RawMessage) (any, error) {
	var args suspendUserArgs
	if err := json.Unmarshal(raw, &args); err != nil {
		return nil, kernel.NewDomainError(kernel.ErrInvalidArgument, "invalid arguments")
	}
	if args.ID <= 0 {
		return nil, kernel.NewDomainError(kernel.ErrInvalidArgument, "id must be positive")
	}

	user, err := h.svc.SuspendUser(ctx, kernel.ID(args.ID))
	if err != nil {
		return nil, err
	}

	return userToMap(user), nil
}

func userToMap(user *domain.User) map[string]any {
	roles := make([]string, len(user.Roles))
	for i, r := range user.Roles {
		roles[i] = string(r)
	}

	return map[string]any{
		"id":     user.ID.Int64(),
		"email":  user.Email,
		"name":   user.Name,
		"status": string(user.Status),
		"roles":  roles,
	}
}
