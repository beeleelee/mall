package oauth

import (
	"strings"

	"github.com/beeleelee/mall/domain/kernel"
	"golang.org/x/crypto/bcrypt"
)

type ClientStatus string

const (
	ClientStatusActive   ClientStatus = "active"
	ClientStatusDisabled ClientStatus = "disabled"
)

type OAuthClient struct {
	kernel.AggregateRoot
	ClientID     string
	SecretHash   string
	RedirectURIs []string
	Scopes       []string
	Status       ClientStatus
}

func NewClient(id kernel.ID, clientID, secret string, redirectURIs, scopes []string) (*OAuthClient, error) {
	if clientID == "" {
		return nil, kernel.NewDomainError(kernel.ErrInvalidArgument, "client_id must not be empty")
	}
	if secret == "" {
		return nil, kernel.NewDomainError(kernel.ErrInvalidArgument, "client_secret must not be empty")
	}
	if len(redirectURIs) == 0 {
		return nil, kernel.NewDomainError(kernel.ErrInvalidArgument, "at least one redirect_uri is required")
	}
	for _, uri := range redirectURIs {
		if uri == "" || !isValidURI(uri) {
			return nil, kernel.NewDomainError(kernel.ErrInvalidArgument, "invalid redirect_uri: "+uri)
		}
	}
	if len(scopes) == 0 {
		return nil, kernel.NewDomainError(kernel.ErrInvalidArgument, "at least one scope is required")
	}
	for _, s := range scopes {
		if s == "" {
			return nil, kernel.NewDomainError(kernel.ErrInvalidArgument, "scope must not be empty")
		}
	}

	hash, err := bcrypt.GenerateFromPassword([]byte(secret), bcrypt.DefaultCost)
	if err != nil {
		return nil, kernel.NewDomainErrorWithCause(kernel.ErrInternal, "failed to hash client secret", err)
	}

	return &OAuthClient{
		AggregateRoot: kernel.NewAggregateRoot(id),
		ClientID:      clientID,
		SecretHash:    string(hash),
		RedirectURIs:  redirectURIs,
		Scopes:        scopes,
		Status:        ClientStatusActive,
	}, nil
}

func NewClientFromHash(id kernel.ID, clientID, secretHash string, redirectURIs, scopes []string, status ClientStatus) *OAuthClient {
	return &OAuthClient{
		AggregateRoot: kernel.NewAggregateRoot(id),
		ClientID:      clientID,
		SecretHash:    secretHash,
		RedirectURIs:  redirectURIs,
		Scopes:        scopes,
		Status:        status,
	}
}

func (c *OAuthClient) VerifySecret(secret string) bool {
	err := bcrypt.CompareHashAndPassword([]byte(c.SecretHash), []byte(secret))
	return err == nil
}

func (c *OAuthClient) IsValidRedirectURI(uri string) bool {
	for _, r := range c.RedirectURIs {
		if r == uri {
			return true
		}
	}
	return false
}

func (c *OAuthClient) HasScope(scope string) bool {
	for _, s := range c.Scopes {
		if s == scope {
			return true
		}
	}
	return false
}

func isValidURI(uri string) bool {
	return strings.HasPrefix(uri, "http://") || strings.HasPrefix(uri, "https://") || strings.HasPrefix(uri, "/")
}
