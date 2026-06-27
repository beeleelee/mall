package oauth

import (
	"context"
	"time"

	"github.com/beeleelee/mall/domain/kernel"
)

type OAuthService struct {
	clients    OAuthClientRepository
	codes      AuthorizationCodeRepository
	tokens     RefreshTokenRepository
	logger     kernel.Logger
	jwtSecret  []byte
	codeTTL    time.Duration
	accessTTL  time.Duration
	refreshTTL time.Duration
}

func NewOAuthService(
	clients OAuthClientRepository,
	codes AuthorizationCodeRepository,
	tokens RefreshTokenRepository,
	logger kernel.Logger,
	jwtSecret []byte,
) *OAuthService {
	return &OAuthService{
		clients:    clients,
		codes:      codes,
		tokens:     tokens,
		logger:     logger,
		jwtSecret:  jwtSecret,
		codeTTL:    10 * time.Minute,
		accessTTL:  15 * time.Minute,
		refreshTTL: 30 * 24 * time.Hour,
	}
}

type AuthorizeInput struct {
	ClientID    string
	RedirectURI string
	Scope       string
	UserID      kernel.ID
}

type AuthorizeOutput struct {
	Code        string
	RedirectURI string
}

type TokenExchangeInput struct {
	Code         string
	ClientID     string
	ClientSecret string
	RedirectURI  string
}

type TokenRefreshInput struct {
	RefreshToken string
	ClientID     string
	ClientSecret string
}

type RevokeInput struct {
	Token        string
	ClientID     string
	ClientSecret string
}

func (s *OAuthService) Authorize(ctx context.Context, input AuthorizeInput) (*AuthorizeOutput, error) {
	s.logger.Info(ctx, "oauth.authorize", kernel.Field("client_id", input.ClientID))

	client, err := s.clients.FindByClientID(ctx, input.ClientID)
	if err != nil {
		return nil, err
	}

	if client.Status != ClientStatusActive {
		return nil, kernel.NewDomainError(kernel.ErrPermissionDenied, "client is disabled")
	}

	if !client.IsValidRedirectURI(input.RedirectURI) {
		return nil, kernel.NewDomainError(kernel.ErrInvalidArgument, "redirect_uri mismatch")
	}

	if input.Scope != "" && !client.HasScope(input.Scope) {
		return nil, kernel.NewDomainError(kernel.ErrInvalidArgument, "requested scope not allowed")
	}

	code, err := NewAuthorizationCode(input.ClientID, input.UserID, input.RedirectURI, input.Scope, s.codeTTL)
	if err != nil {
		return nil, err
	}

	if err := s.codes.Save(ctx, code); err != nil {
		return nil, err
	}

	return &AuthorizeOutput{
		Code:        code.Code,
		RedirectURI: input.RedirectURI,
	}, nil
}

func (s *OAuthService) Exchange(ctx context.Context, input TokenExchangeInput) (*TokenResponse, error) {
	s.logger.Info(ctx, "oauth.exchange", kernel.Field("client_id", input.ClientID))

	client, err := s.clients.FindByClientID(ctx, input.ClientID)
	if err != nil {
		return nil, err
	}

	if !client.VerifySecret(input.ClientSecret) {
		return nil, kernel.NewDomainError(kernel.ErrUnauthenticated, "invalid client credentials")
	}

	code, err := s.codes.FindByCode(ctx, input.Code)
	if err != nil {
		return nil, err
	}

	if code.ClientID != input.ClientID {
		return nil, kernel.NewDomainError(kernel.ErrInvalidArgument, "code was not issued for this client")
	}

	if code.IsExpired() {
		return nil, kernel.NewDomainError(kernel.ErrInvalidArgument, "authorization code expired")
	}

	if err := code.MarkUsed(); err != nil {
		return nil, err
	}

	if err := s.codes.Delete(ctx, code.Code); err != nil {
		s.logger.Error(ctx, "oauth.exchange: failed to delete used code", err)
	}

	accessToken, err := SignJWT(code.UserID, input.ClientID, code.Scope, s.accessTTL, s.jwtSecret)
	if err != nil {
		return nil, err
	}

	refreshToken, err := NewRefreshToken(input.ClientID, code.UserID, code.Scope, s.refreshTTL)
	if err != nil {
		return nil, err
	}

	if err := s.tokens.Save(ctx, refreshToken); err != nil {
		return nil, err
	}

	s.logger.Info(ctx, "oauth.exchange completed", kernel.Field("user_id", code.UserID.String()))

	return &TokenResponse{
		AccessToken:  accessToken,
		RefreshToken: refreshToken.ID,
		ExpiresIn:    int64(s.accessTTL.Seconds()),
		Scope:        code.Scope,
	}, nil
}

func (s *OAuthService) Refresh(ctx context.Context, input TokenRefreshInput) (*TokenResponse, error) {
	s.logger.Info(ctx, "oauth.refresh", kernel.Field("client_id", input.ClientID))

	client, err := s.clients.FindByClientID(ctx, input.ClientID)
	if err != nil {
		return nil, err
	}

	if !client.VerifySecret(input.ClientSecret) {
		return nil, kernel.NewDomainError(kernel.ErrUnauthenticated, "invalid client credentials")
	}

	rt, err := s.tokens.FindByID(ctx, input.RefreshToken)
	if err != nil {
		return nil, err
	}

	if rt.Revoked {
		return nil, kernel.NewDomainError(kernel.ErrUnauthenticated, "refresh token revoked")
	}

	if rt.IsExpired() {
		return nil, kernel.NewDomainError(kernel.ErrUnauthenticated, "refresh token expired")
	}

	_ = s.tokens.Revoke(ctx, rt.ID)

	accessToken, err := SignJWT(rt.UserID, input.ClientID, rt.Scope, s.accessTTL, s.jwtSecret)
	if err != nil {
		return nil, err
	}

	newRefreshToken, err := NewRefreshToken(input.ClientID, rt.UserID, rt.Scope, s.refreshTTL)
	if err != nil {
		return nil, err
	}

	if err := s.tokens.Save(ctx, newRefreshToken); err != nil {
		return nil, err
	}

	return &TokenResponse{
		AccessToken:  accessToken,
		RefreshToken: newRefreshToken.ID,
		ExpiresIn:    int64(s.accessTTL.Seconds()),
		Scope:        rt.Scope,
	}, nil
}

func (s *OAuthService) Revoke(ctx context.Context, input RevokeInput) error {
	s.logger.Info(ctx, "oauth.revoke", kernel.Field("client_id", input.ClientID))

	client, err := s.clients.FindByClientID(ctx, input.ClientID)
	if err != nil {
		return err
	}

	if !client.VerifySecret(input.ClientSecret) {
		return kernel.NewDomainError(kernel.ErrUnauthenticated, "invalid client credentials")
	}

	return s.tokens.Revoke(ctx, input.Token)
}
