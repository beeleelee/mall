package middleware

import (
	"encoding/json"
	"net/http"
)

func SagaAuthMiddleware(secret string) func(http.Handler) http.Handler {
	if secret == "" {
		return func(next http.Handler) http.Handler {
			return next
		}
	}
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			if r.Header.Get("X-Saga-Secret") != secret {
				w.Header().Set("Content-Type", "application/json")
				w.WriteHeader(http.StatusForbidden)
				json.NewEncoder(w).Encode(map[string]string{
					"error":             "permission_denied",
					"error_description": "invalid saga secret",
				})
				return
			}
			next.ServeHTTP(w, r)
		})
	}
}
