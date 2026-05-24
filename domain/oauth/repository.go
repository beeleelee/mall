package oauth

import (
	"context"

	"github.com/beeleelee/mall/domain/kernel"
)

type OAuthClientRepository interface {
	Save(ctx context.Context, client *OAuthClient) error
	FindByClientID(ctx context.Context, clientID string) (*OAuthClient, error)
	FindByID(ctx context.Context, id kernel.ID) (*OAuthClient, error)
}

type AuthorizationCodeRepository interface {
	Save(ctx context.Context, code *AuthorizationCode) error
	FindByCode(ctx context.Context, code string) (*AuthorizationCode, error)
	Delete(ctx context.Context, code string) error
}

type RefreshTokenRepository interface {
	Save(ctx context.Context, token *RefreshToken) error
	FindByID(ctx context.Context, id string) (*RefreshToken, error)
	Revoke(ctx context.Context, id string) error
}
