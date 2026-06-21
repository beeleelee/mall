package middleware

import (
	"context"
	"encoding/json"
	"net/http"
	"strings"

	domain "github.com/beeleelee/mall/domain/oauth"
	"github.com/beeleelee/mall/domain/identity"
	"github.com/beeleelee/mall/domain/kernel"
)

type userContextKey struct{}

type UserInfo struct {
	UserID   int64
	ClientID string
	Scope    string
}

func UserFromContext(ctx context.Context) (UserInfo, bool) {
	u, ok := ctx.Value(userContextKey{}).(UserInfo)
	return u, ok
}

func ContextWithUser(ctx context.Context, info UserInfo) context.Context {
	return context.WithValue(ctx, userContextKey{}, info)
}

func AuthMiddleware(jwtSecret []byte) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			auth := r.Header.Get("Authorization")
			if auth == "" || !strings.HasPrefix(auth, "Bearer ") {
				writeAuthError(w, "missing or malformed authorization header")
				return
			}

			tokenStr := strings.TrimPrefix(auth, "Bearer ")
			claims, err := domain.ValidateJWT(tokenStr, jwtSecret)
			if err != nil {
				writeAuthError(w, err.Error())
				return
			}

			userID, _ := parseInt64(claims.Subject)

			info := UserInfo{
				UserID:   userID,
				ClientID: claims.ClientID,
				Scope:    claims.Scope,
			}

			ctx := context.WithValue(r.Context(), userContextKey{}, info)
			next.ServeHTTP(w, r.WithContext(ctx))
		})
	}
}

func AdminMiddleware(userRepo *identity.UserRepository) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			info, ok := UserFromContext(r.Context())
			if !ok {
				writeAuthError(w, "user not authenticated")
				return
			}

			user, err := (*userRepo).FindByID(r.Context(), kernel.ID(info.UserID))
			if err != nil {
				writeAuthError(w, "user not found")
				return
			}

			if !user.HasRole(identity.UserRoleAdmin) {
				writeForbidden(w, "admin role required")
				return
			}

			next.ServeHTTP(w, r)
		})
	}
}

func parseInt64(s string) (int64, bool) {
	var n int64
	var neg bool
	for i, c := range s {
		if c == '-' && i == 0 {
			neg = true
			continue
		}
		if c < '0' || c > '9' {
			return 0, false
		}
		n = n*10 + int64(c-'0')
	}
	if neg {
		n = -n
	}
	return n, true
}

func writeAuthError(w http.ResponseWriter, msg string) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusUnauthorized)
	json.NewEncoder(w).Encode(map[string]string{
		"error":             "invalid_token",
		"error_description": msg,
	})
}

func writeForbidden(w http.ResponseWriter, msg string) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusForbidden)
	json.NewEncoder(w).Encode(map[string]string{
		"error":             "permission_denied",
		"error_description": msg,
	})
}
