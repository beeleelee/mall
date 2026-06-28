package mcp

import (
	"context"
	"encoding/json"

	"github.com/beeleelee/mall/domain/kernel"

	domain "github.com/beeleelee/mall/domain/oauth"
)

type OAuthMCPHandler struct {
	svc *domain.OAuthService
}

func NewOAuthMCPHandler(svc *domain.OAuthService) *OAuthMCPHandler {
	return &OAuthMCPHandler{svc: svc}
}

var oauthTools = []ToolDefinition{
	{
		Name:        "authorize",
		Description: "Generate an authorization code for a user",
		InputSchema: InputSchema{
			Type: "object",
			Properties: map[string]PropertySchema{
				"client_id":    {Type: "string", Description: "OAuth client ID"},
				"redirect_uri": {Type: "string", Description: "Redirect URI"},
				"scope":        {Type: "string", Description: "Requested scope (optional)"},
				"user_id":      {Type: "number", Description: "User ID"},
			},
		},
	},
	{
		Name:        "token",
		Description: "Exchange an authorization code for tokens, or refresh a token",
		InputSchema: InputSchema{
			Type: "object",
			Properties: map[string]PropertySchema{
				"grant_type":    {Type: "string", Description: "Grant type: authorization_code or refresh_token"},
				"code":          {Type: "string", Description: "Authorization code (for authorization_code grant)"},
				"refresh_token": {Type: "string", Description: "Refresh token (for refresh_token grant)"},
				"client_id":     {Type: "string", Description: "OAuth client ID"},
				"client_secret": {Type: "string", Description: "OAuth client secret"},
				"redirect_uri":  {Type: "string", Description: "Redirect URI (for authorization_code grant)"},
			},
		},
	},
	{
		Name:        "revoke",
		Description: "Revoke a refresh token",
		InputSchema: InputSchema{
			Type: "object",
			Properties: map[string]PropertySchema{
				"token":        {Type: "string", Description: "Refresh token to revoke"},
				"client_id":    {Type: "string", Description: "OAuth client ID"},
				"client_secret": {Type: "string", Description: "OAuth client secret"},
			},
		},
	},
}

func (h *OAuthMCPHandler) ListTools() []ToolDefinition {
	return oauthTools
}

func (h *OAuthMCPHandler) HandleTool(ctx context.Context, name string, raw json.RawMessage) (any, error) {
	switch name {
	case "authorize":
		return h.callAuthorize(ctx, raw)
	case "token":
		return h.callToken(ctx, raw)
	case "revoke":
		return h.callRevoke(ctx, raw)
	default:
		return nil, kernel.NewDomainError(kernel.ErrInvalidArgument, "unknown tool: "+name)
	}
}

type authorizeArgs struct {
	ClientID    string `json:"client_id"`
	RedirectURI string `json:"redirect_uri"`
	Scope       string `json:"scope,omitempty"`
	UserID      int64  `json:"user_id"`
}

func (h *OAuthMCPHandler) callAuthorize(ctx context.Context, raw json.RawMessage) (any, error) {
	var args authorizeArgs
	if err := json.Unmarshal(raw, &args); err != nil {
		return nil, kernel.NewDomainError(kernel.ErrInvalidArgument, "invalid arguments")
	}
	if args.ClientID == "" {
		return nil, kernel.NewDomainError(kernel.ErrInvalidArgument, "client_id must not be empty")
	}
	if args.RedirectURI == "" {
		return nil, kernel.NewDomainError(kernel.ErrInvalidArgument, "redirect_uri must not be empty")
	}
	if args.UserID <= 0 {
		return nil, kernel.NewDomainError(kernel.ErrInvalidArgument, "user_id must be positive")
	}

	out, err := h.svc.Authorize(ctx, domain.AuthorizeInput{
		ClientID:    args.ClientID,
		RedirectURI: args.RedirectURI,
		Scope:       args.Scope,
		UserID:      kernel.ID(args.UserID),
	})
	if err != nil {
		return nil, err
	}

	return map[string]any{
		"code":         out.Code,
		"redirect_uri": out.RedirectURI,
	}, nil
}

type tokenArgs struct {
	GrantType    string `json:"grant_type"`
	Code         string `json:"code,omitempty"`
	RefreshToken string `json:"refresh_token,omitempty"`
	ClientID     string `json:"client_id"`
	ClientSecret string `json:"client_secret"`
	RedirectURI  string `json:"redirect_uri,omitempty"`
}

func (h *OAuthMCPHandler) callToken(ctx context.Context, raw json.RawMessage) (any, error) {
	var args tokenArgs
	if err := json.Unmarshal(raw, &args); err != nil {
		return nil, kernel.NewDomainError(kernel.ErrInvalidArgument, "invalid arguments")
	}
	if args.ClientID == "" {
		return nil, kernel.NewDomainError(kernel.ErrInvalidArgument, "client_id must not be empty")
	}
	if args.ClientSecret == "" {
		return nil, kernel.NewDomainError(kernel.ErrInvalidArgument, "client_secret must not be empty")
	}

	switch args.GrantType {
	case "authorization_code":
		if args.Code == "" {
			return nil, kernel.NewDomainError(kernel.ErrInvalidArgument, "code must not be empty for authorization_code grant")
		}
		if args.RedirectURI == "" {
			return nil, kernel.NewDomainError(kernel.ErrInvalidArgument, "redirect_uri must not be empty for authorization_code grant")
		}

		out, err := h.svc.Exchange(ctx, domain.TokenExchangeInput{
			Code:         args.Code,
			ClientID:     args.ClientID,
			ClientSecret: args.ClientSecret,
			RedirectURI:  args.RedirectURI,
		})
		if err != nil {
			return nil, err
		}

		return map[string]any{
			"access_token":  out.AccessToken,
			"refresh_token": out.RefreshToken,
			"expires_in":    out.ExpiresIn,
			"scope":         out.Scope,
		}, nil

	case "refresh_token":
		if args.RefreshToken == "" {
			return nil, kernel.NewDomainError(kernel.ErrInvalidArgument, "refresh_token must not be empty")
		}

		out, err := h.svc.Refresh(ctx, domain.TokenRefreshInput{
			RefreshToken: args.RefreshToken,
			ClientID:     args.ClientID,
			ClientSecret: args.ClientSecret,
		})
		if err != nil {
			return nil, err
		}

		return map[string]any{
			"access_token":  out.AccessToken,
			"refresh_token": out.RefreshToken,
			"expires_in":    out.ExpiresIn,
			"scope":         out.Scope,
		}, nil

	default:
		return nil, kernel.NewDomainError(kernel.ErrInvalidArgument, "unsupported grant_type: "+args.GrantType)
	}
}

type revokeArgs struct {
	Token        string `json:"token"`
	ClientID     string `json:"client_id"`
	ClientSecret string `json:"client_secret"`
}

func (h *OAuthMCPHandler) callRevoke(ctx context.Context, raw json.RawMessage) (any, error) {
	var args revokeArgs
	if err := json.Unmarshal(raw, &args); err != nil {
		return nil, kernel.NewDomainError(kernel.ErrInvalidArgument, "invalid arguments")
	}
	if args.Token == "" {
		return nil, kernel.NewDomainError(kernel.ErrInvalidArgument, "token must not be empty")
	}
	if args.ClientID == "" {
		return nil, kernel.NewDomainError(kernel.ErrInvalidArgument, "client_id must not be empty")
	}
	if args.ClientSecret == "" {
		return nil, kernel.NewDomainError(kernel.ErrInvalidArgument, "client_secret must not be empty")
	}

	if err := h.svc.Revoke(ctx, domain.RevokeInput{
		Token:        args.Token,
		ClientID:     args.ClientID,
		ClientSecret: args.ClientSecret,
	}); err != nil {
		return nil, err
	}

	return map[string]string{"status": "revoked"}, nil
}
