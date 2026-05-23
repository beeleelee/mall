package middleware

import (
	"context"
	"net/http"
	"strings"
)

type agentContextKey struct{}

type AgentInfo struct {
	Name         string
	Version      string
	Capabilities []string
	Raw          string
}

func parseUCPAgent(header string) AgentInfo {
	info := AgentInfo{Raw: header}
	parts := strings.SplitN(header, "/", 2)
	if len(parts) > 0 {
		info.Name = strings.TrimSpace(parts[0])
	}
	if len(parts) > 1 {
		verAndCaps := strings.SplitN(parts[1], ";", 2)
		info.Version = strings.TrimSpace(verAndCaps[0])
		if len(verAndCaps) > 1 {
			caps := strings.Split(verAndCaps[1], ",")
			for _, c := range caps {
				info.Capabilities = append(info.Capabilities, strings.TrimSpace(c))
			}
		}
	}
	return info
}

func UCPAgentMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		header := r.Header.Get("UCP-Agent")
		if header != "" {
			info := parseUCPAgent(header)
			ctx := context.WithValue(r.Context(), agentContextKey{}, info)
			r = r.WithContext(ctx)
		}
		next.ServeHTTP(w, r)
	})
}

func AgentFromContext(ctx context.Context) (AgentInfo, bool) {
	info, ok := ctx.Value(agentContextKey{}).(AgentInfo)
	return info, ok
}
