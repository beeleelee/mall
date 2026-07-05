package rest

import (
	"encoding/json"
	"net/http"


	app "github.com/beeleelee/mall/application/oauth"
	"github.com/beeleelee/mall/domain/kernel"
)

type OAuthHandler struct {
	svc *app.OAuthAppService
}

func NewOAuthHandler(svc *app.OAuthAppService) *OAuthHandler {
	return &OAuthHandler{svc: svc}
}

type authorizeReq struct {
	ClientID    string `json:"client_id"`
	RedirectURI string `json:"redirect_uri"`
	Scope       string `json:"scope,omitempty"`
	UserID      int64  `json:"user_id"`
}

type tokenReq struct {
	GrantType    string `json:"grant_type"`
	Code         string `json:"code,omitempty"`
	RefreshToken string `json:"refresh_token,omitempty"`
	ClientID     string `json:"client_id"`
	ClientSecret string `json:"client_secret"`
	RedirectURI  string `json:"redirect_uri,omitempty"`
}

type revokeReq struct {
	Token        string `json:"token"`
	ClientID     string `json:"client_id"`
	ClientSecret string `json:"client_secret"`
}

func (h *OAuthHandler) Authorize(w http.ResponseWriter, r *http.Request) {
	var req authorizeReq
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeDomainError(w, err)
		return
	}

	resp, err := h.svc.Authorize(r.Context(), app.AuthorizeRequest{
		ClientID:    req.ClientID,
		RedirectURI: req.RedirectURI,
		Scope:       req.Scope,
		UserID:      req.UserID,
	})
	if err != nil {
		writeDomainError(w, err)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	json.NewEncoder(w).Encode(resp)
}

func (h *OAuthHandler) Token(w http.ResponseWriter, r *http.Request) {
	var req tokenReq
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeDomainError(w, err)
		return
	}

	switch req.GrantType {
	case "authorization_code":
		resp, err := h.svc.Exchange(r.Context(), app.TokenExchangeRequest{
			Code:         req.Code,
			ClientID:     req.ClientID,
			ClientSecret: req.ClientSecret,
			RedirectURI:  req.RedirectURI,
		})
		if err != nil {
			writeDomainError(w, err)
			return
		}

		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		json.NewEncoder(w).Encode(resp)

	case "refresh_token":
		resp, err := h.svc.Refresh(r.Context(), app.TokenRefreshRequest{
			RefreshToken: req.RefreshToken,
			ClientID:     req.ClientID,
			ClientSecret: req.ClientSecret,
		})
		if err != nil {
			writeDomainError(w, err)
			return
		}

		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		json.NewEncoder(w).Encode(resp)

	default:
		writeDomainError(w, kernel.NewDomainError(kernel.ErrInvalidArgument, "unsupported grant_type: "+req.GrantType))
	}
}

func (h *OAuthHandler) Revoke(w http.ResponseWriter, r *http.Request) {
	var req revokeReq
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeDomainError(w, err)
		return
	}

	if err := h.svc.Revoke(r.Context(), app.RevokeRequest{
		Token:        req.Token,
		ClientID:     req.ClientID,
		ClientSecret: req.ClientSecret,
	}); err != nil {
		writeDomainError(w, err)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	json.NewEncoder(w).Encode(map[string]string{"status": "ok"})
}
