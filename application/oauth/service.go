package oauth

import (
	"context"

	domain "github.com/beeleelee/mall/domain/oauth"
	"github.com/beeleelee/mall/domain/kernel"
)

type AuthorizeRequest struct {
	ClientID    string
	RedirectURI string
	Scope       string
	UserID      int64
}

type AuthorizeResponse struct {
	Code        string
	RedirectURI string
}

type TokenExchangeRequest struct {
	Code         string
	ClientID     string
	ClientSecret string
	RedirectURI  string
}

type TokenResponse struct {
	AccessToken  string
	RefreshToken string
	ExpiresIn    int64
	Scope        string
}

type TokenRefreshRequest struct {
	RefreshToken string
	ClientID     string
	ClientSecret string
}

type RevokeRequest struct {
	Token        string
	ClientID     string
	ClientSecret string
}

type OAuthAppService struct {
	domain *domain.OAuthService
	logger kernel.Logger
}

func NewOAuthAppService(domain *domain.OAuthService, logger kernel.Logger) *OAuthAppService {
	return &OAuthAppService{domain: domain, logger: logger}
}

func (s *OAuthAppService) Authorize(ctx context.Context, req AuthorizeRequest) (*AuthorizeResponse, error) {
	out, err := s.domain.Authorize(ctx, domain.AuthorizeInput{
		ClientID:    req.ClientID,
		RedirectURI: req.RedirectURI,
		Scope:       req.Scope,
		UserID:      kernel.ID(req.UserID),
	})
	if err != nil {
		return nil, err
	}
	return &AuthorizeResponse{Code: out.Code, RedirectURI: out.RedirectURI}, nil
}

func (s *OAuthAppService) Exchange(ctx context.Context, req TokenExchangeRequest) (*TokenResponse, error) {
	out, err := s.domain.Exchange(ctx, domain.TokenExchangeInput{
		Code:         req.Code,
		ClientID:     req.ClientID,
		ClientSecret: req.ClientSecret,
		RedirectURI:  req.RedirectURI,
	})
	if err != nil {
		return nil, err
	}
	return &TokenResponse{
		AccessToken:  out.AccessToken,
		RefreshToken: out.RefreshToken,
		ExpiresIn:    out.ExpiresIn,
		Scope:        out.Scope,
	}, nil
}

func (s *OAuthAppService) Refresh(ctx context.Context, req TokenRefreshRequest) (*TokenResponse, error) {
	out, err := s.domain.Refresh(ctx, domain.TokenRefreshInput{
		RefreshToken: req.RefreshToken,
		ClientID:     req.ClientID,
		ClientSecret: req.ClientSecret,
	})
	if err != nil {
		return nil, err
	}
	return &TokenResponse{
		AccessToken:  out.AccessToken,
		RefreshToken: out.RefreshToken,
		ExpiresIn:    out.ExpiresIn,
		Scope:        out.Scope,
	}, nil
}

func (s *OAuthAppService) Revoke(ctx context.Context, req RevokeRequest) error {
	return s.domain.Revoke(ctx, domain.RevokeInput{
		Token:        req.Token,
		ClientID:     req.ClientID,
		ClientSecret: req.ClientSecret,
	})
}
