package rest

import (
	"encoding/json"
	"net/http"
	"strconv"
	"strings"

	"github.com/zeromicro/go-zero/rest/httpx"

	app "github.com/beeleelee/mall/application/identity"
	"github.com/beeleelee/mall/domain/kernel"
)

type IdentityHandler struct {
	svc *app.IdentityAppService
}

func NewIdentityHandler(svc *app.IdentityAppService) *IdentityHandler {
	return &IdentityHandler{svc: svc}
}

func (h *IdentityHandler) Register(w http.ResponseWriter, r *http.Request) {
	var req app.RegisterRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		httpx.Error(w, err)
		return
	}

	resp, err := h.svc.Register(r.Context(), req)
	if err != nil {
		writeDomainError(w, err)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusCreated)
	json.NewEncoder(w).Encode(resp)
}

func (h *IdentityHandler) Login(w http.ResponseWriter, r *http.Request) {
	var req app.LoginRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		httpx.Error(w, err)
		return
	}

	resp, err := h.svc.Login(r.Context(), req)
	if err != nil {
		writeDomainError(w, err)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	json.NewEncoder(w).Encode(resp)
}

func (h *IdentityHandler) GetUser(w http.ResponseWriter, r *http.Request) {
	idStr := strings.TrimPrefix(r.URL.Path, "/api/v1/users/")
	id, err := strconv.ParseInt(idStr, 10, 64)
	if err != nil {
		httpx.Error(w, kernel.NewDomainError(kernel.ErrInvalidArgument, "invalid user id"))
		return
	}

	resp, err := h.svc.GetUser(r.Context(), id)
	if err != nil {
		writeDomainError(w, err)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	json.NewEncoder(w).Encode(resp)
}

func (h *IdentityHandler) SuspendUser(w http.ResponseWriter, r *http.Request) {
	idStr := strings.TrimPrefix(r.URL.Path, "/api/v1/users/")
	idStr = strings.TrimSuffix(idStr, "/suspend")
	id, err := strconv.ParseInt(idStr, 10, 64)
	if err != nil {
		httpx.Error(w, kernel.NewDomainError(kernel.ErrInvalidArgument, "invalid user id"))
		return
	}

	resp, err := h.svc.SuspendUser(r.Context(), id)
	if err != nil {
		writeDomainError(w, err)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	json.NewEncoder(w).Encode(resp)
}


