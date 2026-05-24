package oauth

import (
	"crypto/rand"
	"crypto/sha256"
	"encoding/hex"
	"time"

	"github.com/golang-jwt/jwt/v4"

	"github.com/beeleelee/mall/domain/kernel"
)

type TokenResponse struct {
	AccessToken  string
	RefreshToken string
	ExpiresIn    int64
	Scope        string
}

type RefreshToken struct {
	ID        string
	Hash      string
	ClientID  string
	UserID    kernel.ID
	Scope     string
	ExpiresAt time.Time
	Revoked   bool
}

func NewRefreshToken(clientID string, userID kernel.ID, scope string, ttl time.Duration) (*RefreshToken, error) {
	if clientID == "" {
		return nil, kernel.NewDomainError(kernel.ErrInvalidArgument, "client_id must not be empty")
	}
	if userID <= 0 {
		return nil, kernel.NewDomainError(kernel.ErrInvalidArgument, "user_id must be positive")
	}

	b := make([]byte, 32)
	if _, err := rand.Read(b); err != nil {
		return nil, kernel.NewDomainErrorWithCause(kernel.ErrInternal, "failed to generate refresh token", err)
	}
	tokenStr := hex.EncodeToString(b)
	hash := hashToken(tokenStr)

	return &RefreshToken{
		ID:        hash,
		Hash:      hash,
		ClientID:  clientID,
		UserID:    userID,
		Scope:     scope,
		ExpiresAt: time.Now().Add(ttl),
	}, nil
}

func (r *RefreshToken) IsExpired() bool {
	return time.Now().After(r.ExpiresAt)
}

func (r *RefreshToken) Revoke() {
	r.Revoked = true
}

type JWTClaims struct {
	jwt.RegisteredClaims
	ClientID string `json:"client_id,omitempty"`
	Scope    string `json:"scope,omitempty"`
}

func SignJWT(userID kernel.ID, clientID, scope string, ttl time.Duration, secret []byte) (string, error) {
	now := time.Now()
	claims := JWTClaims{
		RegisteredClaims: jwt.RegisteredClaims{
			Subject:   userID.String(),
			IssuedAt:  jwt.NewNumericDate(now),
			ExpiresAt: jwt.NewNumericDate(now.Add(ttl)),
		},
		ClientID: clientID,
		Scope:    scope,
	}

	token := jwt.NewWithClaims(jwt.SigningMethodHS256, claims)
	signed, err := token.SignedString(secret)
	if err != nil {
		return "", kernel.NewDomainErrorWithCause(kernel.ErrInternal, "failed to sign JWT", err)
	}
	return signed, nil
}

func ValidateJWT(tokenStr string, secret []byte) (*JWTClaims, error) {
	token, err := jwt.ParseWithClaims(tokenStr, &JWTClaims{}, func(t *jwt.Token) (any, error) {
		if _, ok := t.Method.(*jwt.SigningMethodHMAC); !ok {
			return nil, kernel.NewDomainError(kernel.ErrUnauthenticated, "unexpected signing method")
		}
		return secret, nil
	})
	if err != nil {
		return nil, kernel.NewDomainError(kernel.ErrUnauthenticated, "invalid token: "+err.Error())
	}

	claims, ok := token.Claims.(*JWTClaims)
	if !ok || !token.Valid {
		return nil, kernel.NewDomainError(kernel.ErrUnauthenticated, "invalid token claims")
	}

	return claims, nil
}

func hashToken(token string) string {
	h := sha256.Sum256([]byte(token))
	return hex.EncodeToString(h[:])
}
