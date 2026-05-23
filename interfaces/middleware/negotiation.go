package middleware

import (
	"context"
	"net/http"
	"strings"
)

type negotiationContextKey struct{}

type NegotiationResult struct {
	Requested []string
	Supported []string
	Intersect []string
}

func NegotiationMiddleware(supported []string) func(http.Handler) http.Handler {
	supportedMap := make(map[string]bool, len(supported))
	for _, c := range supported {
		supportedMap[strings.ToLower(c)] = true
	}

	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			result := NegotiationResult{
				Supported: supported,
			}

			agent, ok := AgentFromContext(r.Context())
			if ok {
				result.Requested = agent.Capabilities
			}

			result.Intersect = intersect(result.Requested, supportedMap)

			ctx := context.WithValue(r.Context(), negotiationContextKey{}, result)
			next.ServeHTTP(w, r.WithContext(ctx))
		})
	}
}

func NegotiationFromContext(ctx context.Context) (NegotiationResult, bool) {
	n, ok := ctx.Value(negotiationContextKey{}).(NegotiationResult)
	return n, ok
}

func intersect(requested []string, supported map[string]bool) []string {
	if len(requested) == 0 {
		result := make([]string, 0, len(supported))
		for c := range supported {
			result = append(result, c)
		}
		return result
	}
	var result []string
	for _, c := range requested {
		if supported[strings.ToLower(c)] {
			result = append(result, c)
		}
	}
	return result
}
